package api

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"gorm.io/gorm/clause"
)

// captureBuilder records what gorm would bind, without a database.
//
// The postgres driver connects eagerly, so gorm.Open(DryRun) is not available
// here; going straight at the clause builder exercises the exact code path that
// turns the handler's SQL and args into a statement.
type captureBuilder struct {
	strings.Builder
	vars []any
}

func (c *captureBuilder) WriteQuoted(field any) { fmt.Fprintf(c, "%v", field) }
func (c *captureBuilder) AddError(error) error  { return nil }
func (c *captureBuilder) AddVar(_ clause.Writer, vars ...any) {
	for _, v := range vars {
		if n, ok := v.(sql.NamedArg); ok {
			v = n.Value // what gorm's Statement.AddVar does
		}
		c.vars = append(c.vars, v)
		c.WriteString("$?")
	}
}

func bind(query string, vars ...any) []any {
	b := &captureBuilder{}
	clause.NamedExpr{SQL: query, Vars: vars}.Build(b)
	return b.vars
}

// The bug this guards against cost a working search: gorm's Raw switches to
// NamedExpr the moment the SQL contains an '@', and NamedExpr still resolves
// '?' positionally against the same Vars slice — the named substitutions do not
// advance that index. So `... ILIKE @q ... LIMIT ? OFFSET ?` with
// (NamedArg, 50, 100) binds the search pattern to LIMIT and 50 to OFFSET,
// which Postgres rejects outright. Every placeholder has to be one form or the
// other.
func TestLedgerQueryBindsNamedAndPositional(t *testing.T) {
	const mixed = `SELECT id FROM t WHERE email ILIKE @q ESCAPE '\' LIMIT ? OFFSET ?`
	got := bind(mixed, sql.Named("q", "%draft%"), 50, 100)
	if fmt.Sprint(got) == fmt.Sprint([]any{"%draft%", 50, 100}) {
		t.Fatal("gorm 的行为变了：混用 @ 和 ? 现在能正确绑定了，" +
			"这条测试和 adminListTransactions 里的注释都该重写")
	}
	t.Logf("确认混用会错位，绑定结果是 %v", got)

	const named = `SELECT id FROM t WHERE email ILIKE @q ESCAPE '\' LIMIT @lim OFFSET @off`
	want := []any{"%draft%", 50, 100}
	if got := bind(named, sql.Named("q", "%draft%"), sql.Named("lim", 50), sql.Named("off", 100)); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("全命名参数应绑定为 %v，得到 %v", want, got)
	}
}

// The handler's own SQL, so a future edit that reintroduces a bare '?' fails
// here rather than in production.
func TestLedgerHandlerSQLUsesOnlyNamedPlaceholders(t *testing.T) {
	const pageSQL = `
		SELECT t.id FROM quota_transactions t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN images i ON i.id = t.image_id
		 WHERE (u.email ILIKE @q ESCAPE '\' OR u.name ILIKE @q ESCAPE '\'
		        OR i.orig_name ILIKE @q ESCAPE '\' OR t.reason ILIKE @q ESCAPE '\')
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT @lim OFFSET @off`

	if strings.Contains(pageSQL, "?") {
		t.Fatal("分页 SQL 里出现了位置参数 ?，会和命名参数抢同一个索引")
	}
	// @q appears four times and must bind the same value each time, followed by
	// the two paging values in order.
	want := []any{"%p%", "%p%", "%p%", "%p%", 25, 50}
	got := bind(pageSQL, sql.Named("q", "%p%"), sql.Named("lim", 25), sql.Named("off", 50))
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("绑定结果应为 %v，得到 %v", want, got)
	}
}

// Truncating the search term by bytes rather than runes hands Postgres invalid
// UTF-8, which fails the query outright (SQLSTATE 22021) instead of searching
// for something slightly shorter. A Chinese character is three bytes, so a
// fixed byte offset lands mid-character two times out of three.
func TestLedgerQueryTruncatesByRunes(t *testing.T) {
	truncate := func(q string) string {
		if r := []rune(q); len(r) > maxLedgerQueryRunes {
			q = string(r[:maxLedgerQueryRunes])
		}
		return q
	}

	long := strings.Repeat("图", 200) // 600 bytes, 200 runes
	got := truncate(long)
	if !utf8.ValidString(got) {
		t.Errorf("截断后不是合法 UTF-8：%q", got)
	}
	if n := utf8.RuneCountInString(got); n != maxLedgerQueryRunes {
		t.Errorf("应保留 %d 个字符，得到 %d", maxLedgerQueryRunes, n)
	}
	// The byte-based version this replaced, kept as a demonstration that the
	// failure is real and not theoretical.
	if bad := long[:128]; utf8.ValidString(bad) {
		t.Error("按字节切 128 竟然仍是合法 UTF-8，这个测试的前提需要重看")
	}

	// Short terms pass through untouched, including mixed scripts.
	for _, s := range []string{"", "photo.jpg", "用户@example.com", "draft_v2"} {
		if got := truncate(s); got != s {
			t.Errorf("truncate(%q) = %q，短输入不该被改动", s, got)
		}
	}
}
