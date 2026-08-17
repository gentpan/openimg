package aigen

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 请求体的形状就是这个包的全部输出,所以它值得一个测试。
//
// 两条路径共用一段编码,好处是改一处两边都对,风险是改一处两边都错——尤其是
// image_urls:多传一个空数组,上游就会把一次纯文生图当成图生图处理。
func TestSubmitPayload(t *testing.T) {
	cases := []struct {
		name string
		req  Req
		want map[string]any
	}{
		{
			name: "文生图不带 image_urls",
			req:  Req{Prompt: "一只猫", Model: "m", Size: "1:1", Resolution: "1k"},
			want: map[string]any{
				"model": "m", "prompt": "一只猫", "n": 1.0,
				"size": "1:1", "resolution": "1k",
			},
		},
		{
			name: "修图带上源图地址",
			req: Req{Prompt: "换成夜景", Model: "m", Resolution: "2k",
				ImageURLs: []string{"https://cdn/a.png", "https://cdn/b.png"}},
			// size 留空就整个不传:上游据此让输出尺寸跟随输入图。
			want: map[string]any{
				"model": "m", "prompt": "换成夜景", "n": 1.0, "resolution": "2k",
				"image_urls": []any{"https://cdn/a.png", "https://cdn/b.png"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &got)
				_, _ = w.Write([]byte(`{"code":0,"data":[{"task_id":"t1"}]}`))
			}))
			defer srv.Close()

			id, err := New("k", srv.URL).Submit(context.Background(), tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if id != "t1" {
				t.Fatalf("任务号 = %q, 期望 t1", id)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("请求体字段 = %v, 期望 %v", got, tc.want)
			}
			for k, want := range tc.want {
				if a, b := toJSON(got[k]), toJSON(want); a != b {
					t.Errorf("%s = %s, 期望 %s", k, a, b)
				}
			}
		})
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
