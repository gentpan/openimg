package api

import (
	"testing"

	"github.com/google/uuid"
)

// parseSourceIDs 是修图入口的第一道闸:数量、格式、重复都在这里挡下来,后面
// 的归属校验才只需要关心"这张图是不是他的"。
func TestParseSourceIDs(t *testing.T) {
	a, b := uuid.New().String(), uuid.New().String()

	if _, ok := parseSourceIDs(nil); ok {
		t.Error("一张图都没给,应当拒绝")
	}
	if _, ok := parseSourceIDs([]string{}); ok {
		t.Error("空数组应当拒绝")
	}
	// 超过上限就拒,不是悄悄截断:截断会让用户以为四张之外的那些也参与了。
	too := make([]string, aiEditMaxSources+1)
	for i := range too {
		too[i] = uuid.New().String()
	}
	if _, ok := parseSourceIDs(too); ok {
		t.Errorf("超过 %d 张应当拒绝", aiEditMaxSources)
	}
	if _, ok := parseSourceIDs([]string{"not-a-uuid"}); ok {
		t.Error("非法 ID 应当拒绝")
	}

	ids, ok := parseSourceIDs([]string{" " + a + " ", b, a})
	if !ok {
		t.Fatal("合法输入被拒")
	}
	// 去重且保序:传给上游的顺序就是用户挑选的顺序,重复的那张不该被算两次。
	if len(ids) != 2 || ids[0].String() != a || ids[1].String() != b {
		t.Errorf("解析结果 = %v, 期望 [%s %s]", ids, a, b)
	}
}
