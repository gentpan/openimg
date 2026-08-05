package storage

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateUserEndpoint guards the one place where a user hands us a URL that
// our server will then request: their own bucket endpoint. Without this, an
// endpoint of http://169.254.169.254 or http://10.0.0.5:9000 turns the upload
// pipeline into an SSRF probe of our own network.
//
// Limits worth knowing: this resolves the hostname once, at save time. A
// hostname that resolves publicly now and privately later (DNS rebinding) is
// not caught here. The platform's own MinIO endpoint bypasses this entirely —
// it comes from the environment file, not from a user.
func ValidateUserEndpoint(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("endpoint 不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("endpoint 格式无效：%w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("endpoint 必须以 http:// 或 https:// 开头")
	}
	if u.Scheme == "http" {
		return fmt.Errorf("endpoint 必须使用 https（明文 http 会在传输中泄露你的密钥）")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint 缺少主机名")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("无法解析主机 %q：%w", host, err)
	}
	for _, ip := range ips {
		if err := checkPublicIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func checkPublicIP(ip net.IP) error {
	switch {
	case ip.IsLoopback():
		return fmt.Errorf("endpoint 指向本机地址（%s），不被允许", ip)
	case ip.IsPrivate():
		return fmt.Errorf("endpoint 指向内网地址（%s），不被允许", ip)
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// 169.254.169.254 is the cloud metadata endpoint — the classic SSRF target.
		return fmt.Errorf("endpoint 指向链路本地地址（%s），不被允许", ip)
	case ip.IsUnspecified():
		return fmt.Errorf("endpoint 指向未指定地址（%s），不被允许", ip)
	case ip.IsMulticast():
		return fmt.Errorf("endpoint 指向组播地址（%s），不被允许", ip)
	}
	// Carrier-grade NAT range (100.64.0.0/10) isn't covered by IsPrivate.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return fmt.Errorf("endpoint 指向运营商级 NAT 地址（%s），不被允许", ip)
	}
	return nil
}

// ValidatePublicBaseURL checks the CDN origin a user claims for their bucket.
// This one is never fetched by us — it's handed to browsers — so the bar is
// lower: it just has to be a syntactically valid http(s) URL.
func ValidatePublicBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("公开访问地址不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("公开访问地址格式无效：%w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("公开访问地址必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return fmt.Errorf("公开访问地址缺少域名")
	}
	return nil
}
