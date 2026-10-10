// Package rewrite adapts Grace analysis and bounded rewrites to HIR.
package rewrite

import "github.com/oisee/abapiti/grace"

// These aliases preserve the existing HIR adapter API.
type DB = grace.DB
type Rules = grace.Rules
type Tuple = grace.Tuple

func NewDB() *DB                            { return grace.NewDB() }
func Parse(src string) (*DB, *Rules, error) { return grace.ParseRules(src) }
func Evaluate(db *DB, rules *Rules) error   { return grace.Evaluate(db, rules) }
func Report(db *DB) string                  { return grace.Report(db) }
