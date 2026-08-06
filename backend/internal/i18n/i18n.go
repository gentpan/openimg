// Package i18n serves API messages in the caller's language.
//
// Why the server translates at all, when the client could: about half of these
// strings are produced deep in the stack — imageproc rejecting a format,
// storage failing to reach a bucket — and reach the handler as a plain error.
// Turning those into codes the frontend could look up would mean threading an
// identifier through every layer that currently returns `fmt.Errorf`. Answering
// in the caller's language keeps the change at the edges.
//
// The catalogue is a map per language and a Go check that they agree, so a
// missing translation fails a test rather than shipping a Chinese sentence into
// an English page.
package i18n

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type Lang string

const (
	ZH Lang = "zh"
	EN Lang = "en"
)

// Default is what an unlabelled request gets.
//
// Chinese, deliberately: every existing API consumer — PicGo configs, scripts,
// the Mac app — was written against Chinese messages and sends no
// Accept-Language. Defaulting to English would change what they see for no
// reason they asked for.
const Default = ZH

const ctxKey = "openimg.lang"

// Middleware records the caller's language for the rest of the request.
//
// It goes into two places. c.Set covers handlers, which is most of them. The
// request's context.Context covers the layer below: storeUpload and its kind
// take a ctx and no *gin.Context, and their errors are handed straight to the
// client as err.Error() — without this they would answer in Chinese no matter
// what the caller asked for.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := parse(c.GetHeader("Accept-Language"))
		c.Set(ctxKey, lang)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), langCtxKey{}, lang))
		c.Next()
	}
}

// langCtxKey is a private type so nothing else can collide with it, which is
// the documented way to key a context value.
type langCtxKey struct{}

// TCtx is T for code that has a context.Context but no *gin.Context.
//
// Falls back to Default when the context carries no language, so a background
// job or a test that never went through the middleware still gets a sentence.
func TCtx(ctx context.Context, key string, args ...any) string {
	lang := Default
	if v, ok := ctx.Value(langCtxKey{}).(Lang); ok {
		lang = v
	}
	return translate(lang, key, args...)
}

// LangOfCtx reports the language recorded on a context.
func LangOfCtx(ctx context.Context) Lang {
	if v, ok := ctx.Value(langCtxKey{}).(Lang); ok {
		return v
	}
	return Default
}

// parse reads the first tag it recognises.
//
// Not a full RFC 4647 negotiation: this serves two languages, and a q-value
// ranking between them would be code exercised by nothing. "zh", "zh-CN",
// "zh-Hans", "en-GB" all resolve; anything else falls to Default.
func parse(header string) Lang {
	for _, part := range strings.Split(header, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "zh"):
			return ZH
		case strings.HasPrefix(tag, "en"):
			return EN
		}
	}
	return Default
}

// Of returns the language chosen for this request.
func Of(c *gin.Context) Lang {
	if v, ok := c.Get(ctxKey); ok {
		if l, ok := v.(Lang); ok {
			return l
		}
	}
	return Default
}

// T looks up a message in the caller's language.
//
// An unknown key returns the key itself rather than an empty string: a response
// reading "upload.too_large" is obviously a bug, while "" looks like the server
// had nothing to say.
func T(c *gin.Context, key string, args ...any) string {
	return translate(Of(c), key, args...)
}

// TL is T without a request, for the few places that format a message outside a
// handler — the mailer, mostly.
func TL(lang Lang, key string, args ...any) string {
	return translate(lang, key, args...)
}

func translate(lang Lang, key string, args ...any) string {
	table := zh
	if lang == EN {
		table = en
	}
	format, ok := table[key]
	if !ok {
		if format, ok = zh[key]; !ok {
			return key
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Keys reports every key in the Chinese catalogue, which is the canonical one.
// Used by the test that keeps the two in step.
func Keys() []string {
	out := make([]string, 0, len(zh))
	for k := range zh {
		out = append(out, k)
	}
	return out
}

// Has reports whether the English catalogue carries a key.
func Has(lang Lang, key string) bool {
	if lang == EN {
		_, ok := en[key]
		return ok
	}
	_, ok := zh[key]
	return ok
}
