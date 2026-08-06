package api

import "strings"

// likePattern turns a search term into a LIKE pattern with the wildcards
// escaped. Pair it with `ESCAPE '\'` in the query.
//
// Without this, a filename containing _ or % silently becomes a wildcard.
// Searching "draft_v2.png" also matches "draftXv2.png", which is merely
// confusing — but the same string reaches "delete everything matching this
// search", where a lone "%" selects the entire library while the confirmation
// dialog shows a count the user believes they typed. Backslash is escaped
// first, or it would corrupt the escapes added after it.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(term) + "%"
}
