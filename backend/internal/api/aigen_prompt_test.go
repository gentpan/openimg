package api

import (
	"strings"
	"testing"
)

// 水印提示词里的每一条约束都对应一个具体的失败,而漏掉任何一条都不会报错——
// 只会拿回一张抠不干净、或者缩小后糊成一团的图,再白花一次额度。所以钉住。
func TestWatermarkPromptCarriesItsConstraints(t *testing.T) {
	got := aiWatermarkPrompt("一座极简的山峰")

	if !strings.Contains(got, "一座极简的山峰") {
		t.Error("用户那句话必须原样嵌进去")
	}

	// 抠图靠的是主体与背景分得开。渐变、投影、倒影都会让边缘糊掉。
	for _, must := range []string{
		"flat pure white background",
		"no shadow",
		"no reflection",
		"no gradient",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("少了抠图要的约束 %q —— 抠出来会带一圈灰边", must)
		}
	}

	// 最终按画面宽度的百分之十几渲染,细节到那个尺寸只剩脏点。
	for _, must := range []string{"bold simple shapes", "thick strokes", "no fine detail"} {
		if !strings.Contains(got, must) {
			t.Errorf("少了小尺寸可读性的约束 %q", must)
		}
	}

	// 主体顶到画布边缘的话,按锚点贴边摆放时会被切掉一角。
	if !strings.Contains(got, "margin on all four sides") {
		t.Error("少了留白约束 —— 贴边摆放时主体会被切角")
	}

	// 说的是"不要没描述过的文字",不是"不要文字"——用户可能就是要一个字母标。
	if !strings.Contains(got, "that was not described above") {
		t.Error("禁文字那条必须限定在「用户没描述过的」,否则字母标做不出来")
	}
}

// 用户那句话原样透传,不做任何清洗或截断:它是主体描述,改一个字就是另一枚图标。
func TestWatermarkPromptPassesUserTextVerbatim(t *testing.T) {
	for _, in := range []string{
		"字母 G",
		"an owl, minimal line art",
		"猫头鹰 / 圆形 / 单色",
		"", // 空也不该崩,前面的分支已经拦过空提示词
	} {
		if got := aiWatermarkPrompt(in); in != "" && !strings.Contains(got, in) {
			t.Errorf("输入 %q 没有原样出现在提示词里", in)
		}
	}
}
