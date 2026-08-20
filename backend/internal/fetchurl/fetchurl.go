// Package fetchurl 让服务器按用户给的网址去取一张图,并且只取得到该取的东西。
//
// 这个包的重点不是下载,是**不下载什么**。让服务端按外部输入发请求,等于把服务
// 器变成一台代人发包的机器:内网服务、云厂商的元数据端点(169.254.169.254 上放
// 着实例凭证)、以及 localhost 上一切没设防的管理接口,平时都靠"外面连不进来"
// 活着。一个不设防的取图接口把这层假设整个拆掉。
//
// 三条防线,少一条都不够:
//
//  1. **按 IP 判定,不按主机名。** 域名想解析到 127.0.0.1 是完全合法的,黑名单
//     里写主机名一点用都没有。
//  2. **连的就是查过的那个 IP。** 先解析、判定通过、再让 http 去连主机名的话,
//     中间会再解析一次——攻击者让第一次返回公网地址、第二次返回内网地址即可
//     (DNS rebinding)。所以自定义 DialContext:解析、判定、直接连那个 IP。
//  3. **跳转每一跳都过同一道闸。** 302 到 http://169.254.169.254 是最省事的绕
//     法。因为连接都走上面那个 Dial,跳转自然也被拦下;这里再额外限制协议和
//     跳数。
package fetchurl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"
)

var (
	ErrScheme   = errors.New("只支持 http 和 https")
	ErrPort     = errors.New("只支持 80 和 443 端口")
	ErrHost     = errors.New("地址里没有主机名")
	ErrBlocked  = errors.New("这个地址指向内网或保留地址")
	ErrTooLarge = errors.New("文件超过上限")
	ErrStatus   = errors.New("对方没有正常返回")
)

// 除了 Go 自带的判定之外还要挡的段。自带的 IsPrivate/IsLoopback 覆盖不到这些,
// 而它们同样是"从公网连不到、从服务器连得到"的地方。
var extraBlocked = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT,运营商内网
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF 协议专用
	netip.MustParsePrefix("192.0.2.0/24"),   // 文档用
	netip.MustParsePrefix("198.18.0.0/15"),  // 基准测试
	netip.MustParsePrefix("198.51.100.0/24"),// 文档用
	netip.MustParsePrefix("203.0.113.0/24"), // 文档用
	netip.MustParsePrefix("240.0.0.0/4"),    // 保留
	netip.MustParsePrefix("2001:db8::/32"),  // 文档用
}

// 内嵌 IPv4 的 IPv6 段。`64:ff9b::7f00:1` 就是 127.0.0.1,只看外层会直接放行。
var (
	nat64 = netip.MustParsePrefix("64:ff9b::/96")
	sixTo4 = netip.MustParsePrefix("2002::/16")
)

// Blocked 判断一个地址能不能连。导出是为了能被穷举测试——这个包里真正需要正确
// 的就是这一个函数。
func Blocked(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	ip = ip.Unmap() // ::ffff:127.0.0.1 要按 127.0.0.1 判

	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, p := range extraBlocked {
		if p.Contains(ip) {
			return true
		}
	}
	// 255.255.255.255
	if ip.Is4() && ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return true
	}
	// 内嵌 IPv4 的两种转换地址:把里面那个 v4 掏出来再判一次。
	if ip.Is6() {
		b := ip.As16()
		if nat64.Contains(ip) {
			return Blocked(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
		}
		if sixTo4.Contains(ip) {
			return Blocked(netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}))
		}
	}
	return false
}

// Check 校验一条网址的形状。返回清洗过的 URL。
func Check(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, ErrScheme
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return nil, ErrScheme
	}
	if u.Hostname() == "" {
		return nil, ErrHost
	}
	// 只放行 80/443。IP 判定已经挡住了内网,但不限端口的话这个接口还能被用来
	// 探测公网主机上任意端口开不开——回显的错误信息就是探针的返回值。
	switch p := u.Port(); p {
	case "", "80", "443":
	default:
		return nil, ErrPort
	}
	// 认证信息不带出去。
	u.User = nil
	u.Fragment = ""
	return u, nil
}

type Fetcher struct {
	MaxBytes int64
	client   *http.Client
}

func New(maxBytes int64, timeout time.Duration) *Fetcher {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	tr := &http.Transport{
		DialContext:           safeDial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableKeepAlives:     true,
		// 不用代理:代理会把 DialContext 的判定绕过去——连的是代理的地址,
		// 真正的目标由代理去解析。
		Proxy: nil,
	}
	return &Fetcher{
		MaxBytes: maxBytes,
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("跳转太多次")
				}
				switch req.URL.Scheme {
				case "http", "https":
				default:
					return ErrScheme
				}
				switch req.URL.Port() {
				case "", "80", "443":
				default:
					return ErrPort
				}
				return nil
			},
		},
	}
}

// Get 取回内容。返回字节、Content-Type。
func (f *Fetcher) Get(ctx context.Context, u *url.URL) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "openimg-fetch/1.0 (+https://openimg.io)")

	resp, err := f.client.Do(req)
	if err != nil {
		// 把内网拦截的原因原样透出来,其余网络错误不回显——回显等于把这台机器
		// 变成一个能问"这个地址通不通"的探针。
		if errors.Is(err, ErrBlocked) {
			return nil, "", ErrBlocked
		}
		return nil, "", fmt.Errorf("取不到这个地址")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("%w（%d）", ErrStatus, resp.StatusCode)
	}
	// Content-Length 先拦一次,没必要为了发现"太大了"把整个文件拉完。
	if f.MaxBytes > 0 && resp.ContentLength > f.MaxBytes {
		return nil, "", ErrTooLarge
	}

	limit := f.MaxBytes
	if limit <= 0 {
		limit = 64 << 20
	}
	// 多读一个字节:读满了就说明超了,而不是"正好等于上限"。
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("读取中断")
	}
	if int64(len(data)) > limit {
		return nil, "", ErrTooLarge
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// safeDial 解析、判定、直接连查过的那个 IP。
//
// 关键在最后一步:把 IP 而不是主机名交给 Dialer。中间隔着第二次解析的话,
// 前面所有判定都可以被 DNS rebinding 绕开。
func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port != 80 && port != 443 {
		return nil, ErrPort
	}

	d := &net.Dialer{Timeout: 10 * time.Second}

	if ip, err := netip.ParseAddr(host); err == nil {
		if Blocked(ip) {
			return nil, ErrBlocked
		}
		return d.DialContext(ctx, network, netip.AddrPortFrom(ip, uint16(port)).String())
	}

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("解析不了这个主机名")
	}
	var lastErr error = ErrBlocked
	for _, ip := range ips {
		if Blocked(ip) {
			continue
		}
		conn, err := d.DialContext(ctx, network, netip.AddrPortFrom(ip.Unmap(), uint16(port)).String())
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
