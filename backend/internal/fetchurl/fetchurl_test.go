package fetchurl

import (
	"net/netip"
	"testing"
)

// 这张表就是这个功能的安全边界本身。每一行都对应一种"从公网连不到、从服务器
// 连得到"的地方——漏掉任何一行,这个接口就成了替人访问内网的代理。
func TestBlocked(t *testing.T) {
	blocked := []struct{ ip, why string }{
		{"127.0.0.1", "回环"},
		{"127.1.2.3", "整个 127/8 都是回环，不只是 .0.1"},
		{"::1", "IPv6 回环"},
		{"0.0.0.0", "未指定地址，很多栈会当成本机"},
		{"::", "IPv6 未指定"},
		{"10.0.0.1", "私网 A"},
		{"172.16.0.1", "私网 B 下边界"},
		{"172.31.255.255", "私网 B 上边界"},
		{"192.168.1.1", "私网 C"},
		{"fc00::1", "IPv6 唯一本地"},
		{"fd12:3456::1", "IPv6 唯一本地"},
		{"169.254.169.254", "云厂商元数据端点，实例凭证就放在这儿"},
		{"169.254.0.1", "链路本地"},
		{"fe80::1", "IPv6 链路本地"},
		{"224.0.0.1", "组播"},
		{"ff02::1", "IPv6 链路本地组播"},
		{"100.64.0.1", "CGNAT，运营商内网"},
		{"100.127.255.255", "CGNAT 上边界"},
		{"198.18.0.1", "基准测试段"},
		{"192.0.0.1", "IETF 协议专用"},
		{"192.0.2.1", "文档用"},
		{"198.51.100.1", "文档用"},
		{"203.0.113.1", "文档用"},
		{"2001:db8::1", "IPv6 文档用"},
		{"240.0.0.1", "保留"},
		{"255.255.255.255", "广播"},
		// 下面三条是最容易漏的：外层看着是普通 IPv6，里面裹着一个内网 v4。
		{"::ffff:127.0.0.1", "IPv4 映射：只看外层会放行"},
		{"::ffff:10.0.0.1", "IPv4 映射的私网"},
		{"64:ff9b::7f00:1", "NAT64 裹着 127.0.0.1"},
		{"2002:7f00:0001::", "6to4 裹着 127.0.0.1"},
		{"2002:a00:1::", "6to4 裹着 10.0.0.1"},
	}
	for _, c := range blocked {
		ip, err := netip.ParseAddr(c.ip)
		if err != nil {
			t.Fatalf("测试数据写错了 %q: %v", c.ip, err)
		}
		if !Blocked(ip) {
			t.Errorf("%s 必须挡住（%s）", c.ip, c.why)
		}
	}

	// 这些是正常的公网地址。挡住它们意味着功能坏了——一个不能用的守卫迟早
	// 会被人整个关掉。
	allowed := []string{
		"8.8.8.8", "1.1.1.1", "93.184.216.34",
		"172.15.0.1",  // 私网 B 的下边界外
		"172.32.0.1",  // 私网 B 的上边界外
		"100.63.255.255", "100.128.0.0", // CGNAT 两侧
		"11.0.0.1", "192.167.0.1", "192.169.0.1",
		"2606:4700:4700::1111",
		"2002:5db8:d822::", // 6to4 裹着一个公网 v4
	}
	for _, s := range allowed {
		ip := netip.MustParseAddr(s)
		if Blocked(ip) {
			t.Errorf("%s 不该被挡", s)
		}
	}

	if !Blocked(netip.Addr{}) {
		t.Error("零值地址要当成挡住，不能当成放行")
	}
}

func TestCheck(t *testing.T) {
	ok := []string{
		"http://example.com/a.png",
		"https://example.com/a.png",
		"https://example.com:443/a.png",
		"http://example.com:80/a.png",
	}
	for _, s := range ok {
		if _, err := Check(s); err != nil {
			t.Errorf("%q 应该通过，却是 %v", s, err)
		}
	}

	bad := []struct {
		in   string
		want error
	}{
		{"file:///etc/passwd", ErrScheme},
		{"ftp://example.com/a.png", ErrScheme},
		{"gopher://example.com/", ErrScheme},
		{"data:image/png;base64,AAAA", ErrScheme},
		{"javascript:alert(1)", ErrScheme},
		{"https:///a.png", ErrHost},
		// 不限端口的话，这个接口能被用来探测公网主机上任意端口开不开——
		// 回显的错误就是探针的返回值。
		{"http://example.com:8080/a.png", ErrPort},
		{"http://example.com:22/a.png", ErrPort},
		{"http://example.com:6379/a.png", ErrPort},
	}
	for _, c := range bad {
		if _, err := Check(c.in); err != c.want {
			t.Errorf("%q 应该是 %v，却是 %v", c.in, c.want, err)
		}
	}

	// 凭证不该跟着请求发出去。
	u, err := Check("https://user:pass@example.com/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if u.User != nil {
		t.Error("网址里的用户名密码要摘掉")
	}
}
