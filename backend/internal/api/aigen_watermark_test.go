package api

import (
	"testing"

	"github.com/gentpan/openimg/backend/internal/aigen"
)

// 水印这个用途的几个参数彼此绑死,拆开任何一条都会得到一张"看起来生成成功
// 了、却用不了"的图。所以它们值得一条断言,而不是只活在注释里。
func TestWatermarkPlan(t *testing.T) {
	p := aiWatermarkPlan()

	// 模型:全站只用这一个。上游那条"透明背景"的参数只有 gpt-image-1 家族
	// 认,而我们不用它——透明底由客户端本机抠出来,不额外调一次上游。
	if p.Model != aigen.DefaultModel {
		t.Errorf("水印用的模型 %q,期望与全站一致", p.Model)
	}
	// 1:1:水印是一枚角标,非方形只会在合成时留下一条谁都不想要的长边。
	if p.Size != "1:1" || !aiAllowedSizes[p.Size] {
		t.Errorf("size = %q, 期望合法的 1:1", p.Size)
	}
	// 1k:4k 是最贵的一档,而水印按画面宽度的百分之十几渲染,多出来的像素
	// 一个都用不上。
	if p.Resolution != "1k" {
		t.Errorf("resolution = %q, 期望 1k", p.Resolution)
	}
	// 顺带钉住"这条路不必过 pickResolution"的前提:1k 属于免费档,所以
	// API 令牌会话与没关联 pic.bi 的用户都用得上。真把它改成收费档而忘了
	// 补校验,越权就从这里进来。
	if !freeRes(p.Resolution) {
		t.Errorf("水印档位 %q 不在免费档里,却绕过了 pickResolution", p.Resolution)
	}
}
