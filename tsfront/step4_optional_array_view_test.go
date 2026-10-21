package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// OSG convert(): map produces ordinary records, while the recursive family
// lays out fields as an optional array of family-local structural records.
// The source carries an extra data field to force distinct compatible layouts;
// identical layouts can be interned and would miss the covariance boundary.
func TestStep4OptionalArrayViewNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `interface Meta { readonly conv?: boolean }
export type Expr = Meta & (
  | {readonly e: "lit"; readonly text: string}
  | {readonly e: "conv"; readonly kind: string; readonly x: Expr;
     readonly fields?: readonly {readonly name: string; readonly len: number}[]});
export type ValueExpr = Expr & {readonly arith?: boolean};
`, `/** @param {{name: string, type: {len: number}}[]} rows @returns {import("./types.ts").ValueExpr} */
function convert(rows) {
  return {e: "conv", kind: "char_to_struct", x: {e: "lit", text: "x"},
    fields: rows.map(f => ({name: f.name, len: f.type.len, extra: "source"}))};
}
/** @param {{name: string, len: number, extra: string}[] | undefined} fields @returns {import("./types.ts").ValueExpr} */
function wrap(fields) { return {e: "conv", kind: "flat_struct_string", x: {e: "lit", text: "y"}, fields}; }
/** @param {import("./types.ts").Expr} value @returns {string} */
function show(value) {
  if (value.e !== "conv") return value.text;
  if (value.fields === undefined) return "absent";
  const view = value.fields;
  return value.kind + ":" + view.length + ":" + view.map(f => f.name + f.len).join(",");
}
export class Probe {
  /** @returns {string} */ static run() {
    const first = {name: "a", len: 1, extra: "first"};
    const later = {name: "b", len: 2, extra: "later"};
    const fields = [first];
    const value = wrap(fields);
    if (value.e !== "conv" || value.fields === undefined) return "unexpected";
    const view = value.fields;
    const before = view.length;
    fields.push(later);
    first.len = 7;
    return show(convert([{name: "c", type: {len: 3}}])) + ":" + show(wrap(undefined)) + ":" +
      (view === fields) + ":" + before + ":" + view.length + ":" + (view[0] === first) + ":" +
      (view[1] === later) + ":" + show(value);
  }
}
`, false, true)
}

// Structural array adaptation must retain the existing refusal for an element
// with a missing or incompatible required field, and for native class layouts.
func TestStep4OptionalArrayViewRejectsUnrelated(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []hir.Field
		native bool
	}{
		{name: "wrong scalar", fields: []hir.Field{{Name: "len", Type: hir.T(hir.String)}}},
		{name: "missing field"},
		{name: "native class", fields: []hir.Field{{Name: "len", Type: hir.T(hir.Number)}}, native: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst := hir.T(hir.Optional, hir.T(hir.Array, hir.Ref("Row")))
			for _, optional := range []bool{false, true} {
				src := hir.T(hir.Array, hir.Ref("Source"))
				if optional {
					src = hir.T(hir.Optional, src)
				}
				p := &hir.Program{Classes: []*hir.Class{
					{Name: "Row", Structural: true, Fields: []hir.Field{{Name: "len", Type: hir.T(hir.Number), Readonly: true}}},
					{Name: "Source", Structural: !tc.native, Fields: tc.fields},
				}}
				l := &lowerer{out: p, unprunedFeatures: true, shapeLike: map[string]bool{"Row": true, "Source": !tc.native}}
				x := hir.V("source", src)
				got := l.coerce(x, dst)
				if got != x {
					t.Fatal("unrelated array was adapted")
				}
				p.Classes = append(p.Classes, &hir.Class{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "source", Type: src}}, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "view", Type: dst, X: got})}}})
				if errs := hir.Verify(p); len(errs) == 0 {
					t.Fatal("unrelated array passed verification")
				}
			}
		})
	}
}

// OSG's mapped len reads a partially shared property of a typed union. The
// optional array destination must still contextualize the callback's record;
// its len uses the ordinary checked scalar coercion, as a present array does.
func TestStep4OptionalArrayMapContextNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Ty = {readonly k: "c"; readonly len: number} | {readonly k: "s"};
export interface Field {readonly name: string; readonly type: Ty}
export type Expr =
  | {readonly e: "lit"; readonly text: string}
  | {readonly e: "conv"; readonly x: Expr;
     readonly fields?: readonly {readonly name: string; readonly len: number}[]};
`, `/** @param {import("./types.ts").Field[]} fields @returns {import("./types.ts").Expr} */
function convert(fields) {
  if (fields.every(f => f.type.k === "c" && Number.isInteger(f.type.len))) {
    return {e: "conv", x: {e: "lit", text: "x"}, fields: fields.map(f => ({name: f.name, len: f.type.len}))};
  }
  return {e: "conv", x: {e: "lit", text: "y"}};
}
export class Probe {
  /** @returns {string} */ static run() {
    const value = convert([{name: "a", type: {k: "c", len: 3}}]);
    if (value.e !== "conv" || value.fields === undefined) return "unexpected";
    return value.fields.map(f => f.name + (f.len + 1)).join(",") + ":" + JSON.stringify(convert([{name: "s", type: {k: "s"}}]));
  }
}
`, false, true)
}
