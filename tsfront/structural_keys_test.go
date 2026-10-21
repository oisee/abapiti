package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// ValueExpr adds constraints already present on Expr. The resolved variants
// must reuse Expr's payload layouts even though their intersections differ.
func TestStructuralKeysValueExprNodeOracle(t *testing.T) {
	gogenTSOracle(t, `type Type = {readonly k:'num'} | {readonly k:'list';readonly item:Type};
type Meta = {readonly arith?:boolean};
type Expr = Meta & ({readonly e:'num';readonly value:number;readonly type:Type} | {readonly e:'wrap';readonly x:Expr;readonly type:Type} | {readonly e:'flag';readonly value:boolean;readonly type?:Type} | {readonly e:'none'} | {readonly e:'call';readonly args:readonly {readonly value:Expr}[];readonly type:Type});
type TypedExpr = Expr & {readonly type:Type};
type ValueExpr = TypedExpr & {readonly arith?:boolean};
function wrap(x:ValueExpr):Expr {const args=[{value:x}];return {e:'call',args,type:{k:'num'}};}
class Probe {static run():string {const x:ValueExpr={e:'num',value:7,type:{k:'num'}};const y=wrap(x);return y.e==='call' && y.args[0].value.e==='num' ? ''+y.args[0].value.value : 'bad';}}
`, "union.project")
}

// Instantiated Omit/Mutable interfaces retain their own fields, including when
// an initializer's narrower inferred type also fits a public Program view.
func TestStructuralKeysFrontendProgramNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Mutable<T>={-readonly [K in keyof T]:T[K]};
export interface Class {readonly name:string}
export interface FrontendClass extends Mutable<Class> {cache?:string}
export interface Program {readonly name:string;readonly reg:string;readonly classes:readonly Class[];readonly cache?:Map<string,string>}
export interface FrontendProgram extends Omit<Mutable<Program>,'classes'> {classes:FrontendClass[];reg:string;pending?:string}
`, `/** @param {import('./types.ts').Program} p @returns {string} */ function publicName(p) {return p.name;}
/** @param {import('./types.ts').FrontendProgram} p @returns {string} */ function finish(p) {p.pending='done';return p.reg+':'+p.pending;}
/** @param {()=>import('./types.ts').FrontendProgram} fn @returns {()=>import('./types.ts').FrontendProgram} */ function wrap(fn) {return ()=>fn();}
const compile=wrap(function() {
 /** @type {import('./types.ts').FrontendProgram} */ const program={name:'n',classes:[],reg:'r'};
 publicName(program);const run=()=>finish(program);run();return program;
});
export class Probe {static run() {return compile().pending??"";}}
`, true, true)
}

// Constructor argument objects use the Param view while callbacks read the
// additional optional fields of FrontendParam.
func TestStructuralKeysFrontendParamNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Param {readonly name:string;readonly type:string;readonly optional?:boolean;readonly default?:string}
export interface FrontendParam extends Param {readonly defaultOwner?:string;readonly defOwner?:string}
export interface Argument {readonly value:string;readonly type:string}
`, `/** @param {import('./types.ts').FrontendParam & {default:string}} p @returns {string} */ function defaultValue(p) {return (p.defaultOwner??p.name)+':'+p.default;}
/** @param {readonly import('./types.ts').FrontendParam[]} params @returns {readonly import('./types.ts').Argument[]} */
function args(params) {return params.map(/** @returns {import('./types.ts').Argument} */ p=>({type:p.type,value:p.default!==undefined?defaultValue(p):p.name}));}
export class Probe {static run() {return args([{name:'n',type:'s',defaultOwner:'owner',default:'v'}]).map(p=>p.value).join(',');}}
`, true, true)
}

func TestStructuralKeysResolvedFields(t *testing.T) {
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": `
interface Param {readonly name:string;readonly type:string;readonly optional?:boolean}
interface FrontendParam extends Param {readonly defaultOwner?:string;readonly defOwner?:string}
interface Program {readonly name:string;readonly classes:readonly string[];readonly cache?:Map<string,string>}
type Mutable<T>={-readonly [K in keyof T]:T[K]};
interface FrontendProgram extends Omit<Mutable<Program>,'classes'> {classes:string[];reg:string;pending?:string}
type A=Param; type B=FrontendParam; type C=Program; type D=FrontendProgram;
type E={readonly x:number}&{readonly y:string};type F={readonly x:number;readonly y:string};
type G={readonly x:number;readonly y?:string};type H={x:number;readonly y:string};
interface Box<T> {readonly value:T}
type BoxString=Box<string>; type FlatBox={readonly value:string}; type BoxNumber=Box<number>;
type R={readonly x:number};type M=Mutable<R>;type W={x:number};type Mixed=R & {x:number};
type Leaf={x:number};type NestedA={readonly v:{x:number}};type NestedB={readonly v:Pick<Leaf,"x">};type NestedC={readonly v:Mutable<R>};type NestedD={readonly v:{x:string}};type NestedAlias={readonly v:Leaf};
`})
	l := p.lowerer()
	for _, name := range []string{"NestedB", "NestedC", "NestedAlias"} {
		if l.familyTypeKey(p.types[name], false, nil) != l.familyTypeKey(p.types["NestedA"], false, nil) {
			t.Errorf("resolved member types split: %s", name)
		}
	}
	for _, name := range []string{"M", "Mixed"} {
		if l.familyTypeKey(p.types[name], false, nil) != l.familyTypeKey(p.types["W"], false, nil) {
			t.Errorf("resolved mutable field differs from W: %s", name)
		}
	}
	for _, pair := range [][2]string{{"A", "B"}, {"C", "D"}, {"F", "G"}, {"F", "H"}, {"BoxString", "BoxNumber"}, {"R", "M"}, {"NestedA", "NestedD"}} {
		if l.familyTypeKey(p.types[pair[0]], false, nil) == l.familyTypeKey(p.types[pair[1]], false, nil) {
			t.Errorf("different resolved fields merged: %v", pair)
		}
	}
	if l.familyTypeKey(p.types["BoxString"], false, nil) != l.familyTypeKey(p.types["FlatBox"], false, nil) {
		t.Error("instantiated generic fields split from equivalent record")
	}
	if l.familyTypeKey(p.types["E"], false, nil) != l.familyTypeKey(p.types["F"], false, nil) {
		t.Error("equivalent intersection and record fields split")
	}
}

func TestStructuralKeysRequiredFieldGuard(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"if", `if(p.default!==undefined) return use(p);return '';`, true},
		{"conditional", `return p.default!==undefined?use(p):'';`, true},
		{"and", `return p.default!==undefined && use(p);`, true},
		{"absent", `return use(p);`, false},
		{"else", `if(p.default!==undefined) return '';else return use(p);`, false},
		{"different-object", `if(q.default!==undefined) return use(p);return '';`, false},
		{"call-invalidates", `if(p.default!==undefined) {mutate(p);return use(p);}return '';`, false},
		{"later-argument", `if(p.default!==undefined) return use(p,mutate(p));return '';`, false},
		{"write-invalidates", `if(p.default!==undefined) {p.default=undefined;return use(p);}return '';`, false},
		{"deferred", `if(p.default!==undefined) return ()=>use(p);return '';`, false},
		{"condition-effect", `if(p.default!==undefined && mutate(p)) return use(p);return '';`, false},
		{"delete-invalidates", `if(p.default!==undefined) {delete p.default;return use(p);}return '';`, false},
		{"shadowed-undefined", `let undefined='v';if(p.default!==undefined) return use(p);return '';`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := loadFamilySCCProbe(t, map[string]string{"probe.ts": `interface Param {readonly default?:string} declare function use(p:Param & {readonly default:string}):string;declare function mutate(p:Param):void;function f(p:Param,q:Param){` + tc.body + `}`})
			l := p.lowerer()
			found := false
			var walk func(*ast.Node) bool
			walk = func(n *ast.Node) bool {
				if n.Kind == ast.KindCallExpression && n.Expression().Kind == ast.KindIdentifier && n.Expression().Text() == "use" {
					found = true
					if got := l.guardedPropertyPresent(n.Arguments()[0], "default"); got != tc.want {
						t.Errorf("guard = %t, want %t", got, tc.want)
					}
				}
				return n.ForEachChild(walk)
			}
			walk(p.file.AsNode())
			if !found {
				t.Fatal("missing use call")
			}
		})
	}
}
