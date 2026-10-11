package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestCallArityUnionOptionalSlot(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 class A { onlyA(): string { return "a"; } parseType(reg: string): string { return reg; } }
 class B { onlyB(): string { return "b"; } parseType(reg: string, name?: string): string { return reg; } }
 class C { onlyC(): string { return "c"; } parseType(reg: string): string { return reg; } }
 class D { onlyD(): string { return "d"; } parseType(reg: string, name?: string, description?: string): string { return reg; } }
 export class Probe {
 static read(o: A | C): string { return o.parseType("ok"); }
 static widen(flag: boolean): string { const o = flag ? new A() : new B(); return o.parseType("ok"); }
 static widenAgain(flag: boolean): string { const o = flag ? new A() : new D(); return o.parseType("ok"); }
 static run(): string { return Probe.read(new A())+Probe.read(new C())+Probe.widen(false)+Probe.widenAgain(false); }
 }`}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != "okokokok\n" {
		t.Fatal(got)
	}
}

func TestCallAritySemantics(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"optional", `export class Probe { static f(x?: string): string { if (x === undefined) { return "absent"; } return x; } static run(): string { return Probe.f()+":"+Probe.f("yes"); } }`, "absent:yes\n"},
		{"defaults", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static f(a: number = Probe.next(), b: number = a + Probe.next()): number { return a*10+b; } static run(): number { const a=Probe.f(); const b=Probe.f(7); const c=Probe.f(undefined, 9); return a*10000+b*100+c+Probe.count; } }`, "138053\n"},
		{"extra", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static f(x: number): number { return x*10+Probe.count; } static run(): number { return Probe.f(Probe.next(), Probe.next(), Probe.next()); } }`, "13\n"},
		{"extraPrelude", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static rest(...xs: number[]): number { return xs[0]; } static f(a: number, b: number): number { return a*100+b*10+Probe.count; } static run(): number { return Probe.f(Probe.next(),Probe.rest(Probe.next()),Probe.next()); } }`, "123\n"},
		{"restPrelude", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static nested(...xs: number[]): number { return xs[0]; } static f(a: number, b: number, ...xs: number[]): number { return a*100+b*10+xs[0]; } static run(): number { return Probe.f(Probe.next(),Probe.nested(Probe.next()),Probe.next()); } }`, "123\n"},
		{"rest", `export class Probe { static f(a: number = 1, ...xs: number[]): number { let n=a; for (const x of xs) { n+=x; } return n; } static run(): number { const xs: number[]=[3,4]; return Probe.f()+Probe.f(2,...xs); } }`, "10\n"},
		{"lifted", `export class Probe { static run(): number { const base=4; const f=(x: number = base, y: number = x+1): number => x*10+y; const rest=(a: number, ...xs: number[]): number => a+xs.length; return f()+f(2)+rest(1,2,3); } }`, "71\n"},
		{"receiver", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static make(): Probe { Probe.count += 10; return new Probe(); } f(x: number): number { return x*100+Probe.count; } static run(): number { return Probe.make().f(Probe.next(),Probe.next()); } }`, "1112\n"},
		{"inlineZeroCallback", `export class Probe { static run(): number { const xs: number[]=[2,4]; const ys=xs.map((): number => 7); return ys[0]+ys[1]; } }`, "14\n"},
		{"namedCallback", `function f(x: number, i: number, a: number[], missing?: string): number { if (missing === undefined) { return x+i+a.length; } return 99; } function zero(): number { return 7; } export class Probe { static run(): number { const xs: number[]=[2,4]; const a=xs.map(f); const b=xs.map(zero); return a[0]+a[1]+b[0]+b[1]; } }`, "25\n"},
		{"callbackDefaults", `export class Probe { static run(): number { const extra=5; const xs: number[]=[2,4]; const a=xs.map((x: number, i: number, rows: number[], missing: number = extra): number => x+i+rows.length+missing); const b=xs.map((x: number, ...rest: any[]): number => x+rest.length); return a[0]+a[1]+b[0]+b[1]; } }`, "31\n"},
		{"namedReduce", `function sum(acc: number, x: number, i: number, a: number[], missing?: string): number { return acc+x+i+a.length; } export class Probe { static run(): number { const xs: number[]=[2,4]; return xs.reduce(sum,0); } }`, "11\n"},
		{"namedSort", `function cmp(a: number,b: number, missing?: string): number { return a-b; } export class Probe { static run(): number { const xs: number[]=[4,2]; xs.sort(cmp); return xs[0]*10+xs[1]; } }`, "24\n"},
		{"thunkExtra", `export class Probe { static count: number = 0; static next(): number { Probe.count += 1; return Probe.count; } static call(d: () => void): void { d(Probe.next()); } static run(): number { Probe.call(() => { Probe.count += 10; }); return Probe.count; } }`, "11\n"},
		{"tupleEffects", `export class Probe { static xs: [number, number] = [1,2]; static change(): number { Probe.xs[0]=9; return 8; } static f(x: number, y: number): number { return x*10+y; } static run(): number { return Probe.f(...Probe.xs,Probe.change()); } }`, "12\n"},
		{"overloadMethodConstructor", `class Value { x: string; constructor(); constructor(x: string); constructor(x: string = "d") { this.x=x; } f(): string; f(y: string): string; f(y: string = "e"): string { return this.x+y; } } export class Probe { static run(): string { return new Value().f()+new Value("x").f("y"); } }`, "dexy\n"},
		{"overload", `function f(x: string): string; function f(x: string, y: string): string; function f(x: string, y?: string): string { if (y === undefined) { return x; } return x+y; } export class Probe { static run(): string { return f("a")+f("b","c"); } }`, "abc\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": tc.source}, []string{"probe.ts"})
			if _, err := abap.Emit(p); err != nil {
				t.Fatal(err)
			}
			if got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"())"); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
