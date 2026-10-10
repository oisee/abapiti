package grace_test

import (
	"fmt"
	"github.com/oisee/abapiti/grace"
)

func ExampleEvaluate() {
	type function struct {
		name   string
		calls  []string
		throws bool
	}
	ir := []function{{"main", []string{"read"}, false}, {"read", nil, true}}
	db := grace.NewDB()
	for _, f := range ir {
		if err := db.Add("method", f.name); err != nil {
			panic(err)
		}
		for _, callee := range f.calls {
			if err := db.Add("calls", f.name, callee); err != nil {
				panic(err)
			}
		}
		if f.throws {
			if err := db.Add("throws", f.name); err != nil {
				panic(err)
			}
		}
	}
	_, rules, err := grace.ParseRules(`
 (rule throws 0 (head (may_throw ?m)) (base (throws ?m))
  (tail (calls ?m ?c) (may_throw ?c)))`)
	if err != nil {
		panic(err)
	}
	selected, _ := grace.SelectDemand([]*grace.Rules{rules}, []string{"may_throw"})
	if err := grace.Evaluate(db, selected); err != nil {
		panic(err)
	}
	for _, fact := range db.Facts("may_throw") {
		fmt.Println(fact[0])
	}
	// Output:
	// main
	// read
}
