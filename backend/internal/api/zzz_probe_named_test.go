package api

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type probeDialector struct{}

func (probeDialector) Name() string                                 { return "postgres" }
func (probeDialector) Initialize(*gorm.DB) error                    { return nil }
func (probeDialector) Migrator(db *gorm.DB) gorm.Migrator           { return nil }
func (probeDialector) DataTypeOf(*schema.Field) string              { return "" }
func (probeDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (probeDialector) QuoteTo(w clause.Writer, str string)          { w.WriteString(str) }
func (probeDialector) Explain(s string, vars ...interface{}) string { return s }
func (probeDialector) BindVarTo(w clause.Writer, stmt *gorm.Statement, v interface{}) {
	w.WriteString("$" + strconv.Itoa(len(stmt.Vars)))
}

func TestZZZProbeNamedMix(t *testing.T) {
	db, err := gorm.Open(probeDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlStr := `SELECT t.id FROM quota_transactions t
	WHERE (u.email ILIKE @q ESCAPE '\' OR t.reason ILIKE @q ESCAPE '\')
	ORDER BY t.created_at DESC LIMIT ? OFFSET ?`
	args := []any{sql.Named("q", "%foo%")}
	tx := db.Raw(sqlStr, append(append([]any{}, args...), 50, 100)...)
	fmt.Printf("SQL >>>%s<<<\n", tx.Statement.SQL.String())
	for i, v := range tx.Statement.Vars {
		fmt.Printf("VAR[%d] %T = %v\n", i, v, v)
	}
}
