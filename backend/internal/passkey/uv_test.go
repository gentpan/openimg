package passkey

import "testing"

// UV 必须是 required：空串等同于 "preferred"（验了最好、没验也收），系统据此
// 允许跳过生物识别——点一下就登进账号，指纹一次都不弹。对一个能直接登录的凭
// 证，那是把「你有这台设备」当成了「你是这个人」。
func TestUserVerificationRequired(t *testing.T) {
	s, err := New(nil, "openimg.io", "Openimg", "https://openimg.io")
	if err != nil {
		t.Fatal(err)
	}
	got := s.wa.Config.AuthenticatorSelection.UserVerification
	if string(got) != "required" {
		t.Errorf("UserVerification = %q，必须是 required", got)
	}
	if string(s.wa.Config.AuthenticatorSelection.ResidentKey) != "required" {
		t.Error("ResidentKey 必须是 required —— 发现式登录靠它")
	}
}
