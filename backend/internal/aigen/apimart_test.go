package aigen

import (
	"context"
	"encoding/json"
	"errors"
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
		{
			// 透明底那条路:模型必须换成认这两个键的那个,而且两个一起给。
			// 少了 output_format,alpha 会被输出格式吃掉。
			name: "透明底带上 background 与 output_format",
			req: Req{Prompt: "一枚圆形 logo", Model: TransparentModel,
				Size: "1:1", Resolution: "1k",
				Background: "transparent", OutputFormat: "png"},
			want: map[string]any{
				"model": TransparentModel, "prompt": "一枚圆形 logo", "n": 1.0,
				"size": "1:1", "resolution": "1k",
				"background": "transparent", "output_format": "png",
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

// 上游对不认得的参数是静默忽略,不是报错。所以"用默认模型要透明底"这件事
// 必须在发出去之前就失败——否则调用方会拿到一张不透明的图,而全程没有一处
// 报过错,直到那枚"水印"贴到照片上变成一个白方块。
func TestSubmitRejectsTransparentOnUnsupportedModel(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"code":0,"data":[{"task_id":"t1"}]}`))
	}))
	defer srv.Close()

	c := New("k", srv.URL)
	for _, req := range []Req{
		{Prompt: "p", Model: DefaultModel, Background: "transparent", OutputFormat: "png"},
		// 只给一个也算要了透明底:漏掉另一个是最容易犯的错,不该被放行。
		{Prompt: "p", Model: DefaultModel, Background: "transparent"},
		{Prompt: "p", Model: DefaultModel, OutputFormat: "png"},
	} {
		if _, err := c.Submit(context.Background(), req); !errors.Is(err, ErrTransparentUnsupported) {
			t.Errorf("model=%s background=%q output_format=%q 的错误 = %v, 期望 ErrTransparentUnsupported",
				req.Model, req.Background, req.OutputFormat, err)
		}
	}
	if called {
		t.Error("请求不该被发出去")
	}

	// 认得这两个键的模型必须放行,否则这道闸就把功能本身挡死了。
	for _, m := range []string{"gpt-image-1", "gpt-image-1-official"} {
		if !SupportsTransparent(m) {
			t.Errorf("%s 应当支持透明背景", m)
		}
	}
	if SupportsTransparent(DefaultModel) {
		t.Errorf("%s 不该被当成支持透明背景", DefaultModel)
	}
}
