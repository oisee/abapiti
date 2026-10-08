package overrides

import "github.com/oisee/abapiti/hir"

// StableProjectedSort is a package-adapter builder. Every comparator is pure
// under its source pin; it is not a general Array.sort callback implementation.
// Receiver evaluation occurs once, aliasing and element references are retained.
func StableProjectedSort(receiver *hir.Expr, compare func(*hir.Expr, *hir.Expr) *hir.Expr) *hir.Expr {
	array := receiver.Type
	element := array.Args[0]
	i32, boolean := hir.T(hir.I32), hir.T(hir.Bool)
	saved := hir.V("adapterSortArray", array)
	limit := hir.V("adapterSortLength", i32)
	index := hir.V("adapterSortIndex", i32)
	position := hir.V("adapterSortPosition", i32)
	moving := hir.V("adapterSortMoving", element)
	previous := hir.V("adapterSortPrevious", element)
	binary := func(op string, left, right *hir.Expr, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Op: op, Type: typ, X: left, Y: right}
	}
	slot := func(at *hir.Expr) *hir.Expr { return &hir.Expr{Kind: hir.IndexGet, Type: element, X: saved, Y: at} }
	read := func(at *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Narrow, Type: element, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.get", Type: hir.T(hir.Optional, element), X: saved, Args: []*hir.Expr{at}}}
	}
	variable := func(value, initial *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.VarDecl, Name: value.Name, Type: value.Type, X: initial}
	}
	assign := func(left, right *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: left, Y: right} }
	minusOne := binary("-", position, hir.L(i32, 1), i32)
	body := hir.B(
		variable(saved, receiver),
		variable(limit, &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: i32, X: saved}),
		variable(index, hir.L(i32, 1)),
		&hir.Stmt{Kind: hir.While, X: binary("<", index, limit, boolean), Body: hir.B(
			variable(moving, read(index)), variable(position, index),
			&hir.Stmt{Kind: hir.While, X: binary(">", position, hir.L(i32, 0), boolean), Body: hir.B(
				variable(previous, read(minusOne)),
				&hir.Stmt{Kind: hir.If, X: binary("<=", compare(previous, moving), hir.L(i32, 0), boolean), Body: hir.B(&hir.Stmt{Kind: hir.Break})},
				assign(slot(position), previous), assign(position, minusOne),
			)},
			assign(slot(position), moving), assign(index, binary("+", index, hir.L(i32, 1), i32)),
		)},
	)
	return &hir.Expr{Kind: hir.Seq, Type: array, Stmt: body, Y: saved}
}

func StringKeyComparator(operation string, key func(*hir.Expr) *hir.Expr) func(*hir.Expr, *hir.Expr) *hir.Expr {
	return func(left, right *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: operation, Type: hir.T(hir.I32), X: key(left), Args: []*hir.Expr{key(right)}}
	}
}

func stableNamedSort(receiver *hir.Expr, operation, getter string) *hir.Expr {
	return StableProjectedSort(receiver, StringKeyComparator(operation, func(value *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.VirtualCall, Name: getter, Type: hir.T(hir.String), X: value}
	}))
}

// FileSequenceComparator reproduces the four pure endsWith/findIndex keys;
// an unrecognized filename keeps upstream's -1 key and equal keys stay stable.
func FileSequenceComparator(left, right *hir.Expr) *hir.Expr {
	key := func(file *hir.Expr) *hir.Expr {
		name := &hir.Expr{Kind: hir.VirtualCall, Name: "getFilename", Type: hir.T(hir.String), X: file}
		var result *hir.Expr = hir.L(hir.T(hir.I32), -1)
		suffixes := []string{".clas.locals_def.abap", ".clas.locals_imp.abap", ".clas.abap", ".clas.testclasses.abap"}
		for index := len(suffixes) - 1; index >= 0; index-- {
			result = &hir.Expr{Kind: hir.Conditional, Type: hir.T(hir.I32), X: &hir.Expr{Kind: hir.RuntimeOp, Op: "string.endsWith", Type: hir.T(hir.Bool), X: name, Args: []*hir.Expr{hir.L(hir.T(hir.String), suffixes[index])}}, Y: hir.L(hir.T(hir.I32), index), Z: result}
		}
		return result
	}
	return &hir.Expr{Kind: hir.Binary, Op: "-", Type: hir.T(hir.I32), X: key(left), Y: key(right)}
}

func registrySorts() []Entry {
	rule := hir.Type{Kind: hir.InterfaceRef, Name: "src/rules/_irule.ts.IRule"}
	candidates := hir.Type{Kind: hir.InterfaceRef, Name: "union.b0b6a58b84dc8bc589d0501b1bd3651c334e7360f1d90cb261317c96490bb097"}
	file := hir.Ref("src/abap/abap_file.ts.ABAPFile")
	return []Entry{
		{ID: "abaplint-rule-key-stable-sort", Key: Key{"src/config.ts", "Config.getDefault", "KindMethodDeclaration"}, SHA256: "cc2a04c8855f50d10ba44f37a30af3ca8963d83a222bb4117b59f67c44dc9c40", Rationale: "pure metadata key comparator; stable in-place ordering on ICU-root [a-z0-9_] with guard; evaluation and alias identity retained; generalise later; `rules: any = {}` is a string-keyed bag of rule configs (fable)", Patterns: &Patterns{Annotations: map[string]hir.Type{"any": hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic))}, Expressions: map[string]func() *hir.Expr{
			"ArtifactsRules.getRules().sort((a, b) => {\n      return a.getMetadata().key.localeCompare(b.getMetadata().key);\n    })": func() *hir.Expr {
				receiver := &hir.Expr{Kind: hir.DirectCall, Owner: "src/artifacts_rules.ts.ArtifactsRules", Name: "getRules", Type: hir.T(hir.Array, rule)}
				return StableProjectedSort(receiver, StringKeyComparator("string.compareRegistryKey", func(value *hir.Expr) *hir.Expr {
					metadata := &hir.Expr{Kind: hir.VirtualCall, Name: "getMetadata", Type: hir.Ref("src/rules/_irule.ts.IRuleMetadata"), X: value}
					return &hir.Expr{Kind: hir.FieldGet, Name: "key", Type: hir.T(hir.String), X: metadata}
				}))
			},
		}}},
		{ID: "abaplint-object-name-stable-sort", Key: Key{"src/abap/5_syntax/global_definitions/find_global_definitions.ts", "FindGlobalDefinitions.run", "KindMethodDeclaration"}, SHA256: "b483f60a676a128f3a478f237e8fef250a0526649a1af1c384bb12f9c1d83b3d", Rationale: "getName is a pure stored uppercase name getter; stable ICU-root [A-Z0-9_/] ordering with guard; generalise later", Expressions: map[string]func() *hir.Expr{
			"candidates.sort((a, b) => {return a.getName().localeCompare(b.getName());})": func() *hir.Expr {
				return stableNamedSort(hir.V("candidates", hir.T(hir.Array, candidates)), "string.compareObjectName", "getName")
			},
		}},
		{ID: "abaplint-class-file-stable-sort", Key: Key{"src/objects/class.ts", "Class.getSequencedFiles", "KindMethodDeclaration"}, SHA256: "6826130908b1dd6e07c9dbc5e64fcdcfd815aae390551674e30d01a70f0a3078", Rationale: "four pure filename suffix keys reproduce findIndex; copy before stable in-place sort; unknown suffix remains -1; generalise later", References: []Key{{"src/abap/abap_file.ts", "ABAPFile", "KindClassDeclaration"}}, Method: func() *hir.Method {
			receiver := &hir.Expr{Kind: hir.VirtualCall, Name: "getABAPFiles", Type: hir.T(hir.Array, file), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("src/objects/class.ts.Class")}}
			copy := &hir.Expr{Kind: hir.RuntimeOp, Op: "array.slice0", Type: receiver.Type, X: receiver}
			return &hir.Method{Name: "getSequencedFiles", Virtual: true, Result: receiver.Type, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: StableProjectedSort(copy, FileSequenceComparator)})}
		}},
	}
}
