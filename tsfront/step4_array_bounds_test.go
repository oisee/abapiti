package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const arrayBoundsTypes = `type Stmt = {kind:"left", name:string, flag?:boolean}|{kind:"right", name:string, flag?:boolean};`
const arrayBoundsTS = arrayBoundsTypes + `
function movePair(st:Stmt,next:Stmt|undefined,previous:Stmt|undefined):string {
 return st.name+":"+(next===undefined)+":"+(previous===undefined)+";";
}
class Kid {name="kid";}
class Probe {
 static run():string {
 const body:Stmt[]=[{kind:"left",name:"L"},{kind:"right",name:"R"}];
 let result="";
 for(let i=0;i<body.length;i++) {const st=body[i],next=body[i+1];result+=movePair(st,next,body[i-1]);}
 const kids=[new Kid()];
 return result+(kids[1] instanceof Kid)+":"+(body[-1]?.name??"absent")+":"+(body[body.length]===undefined);
 }
}`

func TestStep4ArrayBoundsTS(t *testing.T) {
	want := conversionOracle(t, arrayBoundsTS)
	p := structuralUnionProgram(t, arrayBoundsTS)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}

func TestStep4ArrayBoundsJSDoc(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		t.Run(fmt.Sprintf("recursive=%v", recursive), func(t *testing.T) { runArrayBoundsJSDoc(t, recursive) })
	}
}

func runArrayBoundsJSDoc(t *testing.T, recursive bool) {
	const source = `/** @typedef {import("./types.ts").Stmt} Stmt */
/** @param {Stmt} st @param {Stmt|undefined} next @param {Stmt|undefined} previous @returns {string} */
function movePair(st,next,previous) {return st.name+":"+(next===undefined)+":"+(previous===undefined)+";";}
class Kid {name="kid";}
export class Probe {
 static run() {
 /** @type {Stmt[]} */ const body=[{kind:"left",name:"L"},{kind:"right",name:"R"}];
 let result="";
 for(let i=0;i<body.length;i++) {const st=body[i],next=body[i+1];result+=movePair(st,next,body[i-1]);}
 const kids=[new Kid()];
 return result+(kids[1] instanceof Kid)+":"+(body[-1]?.name??"absent")+":"+(body[body.length]===undefined);
 }
}`
	dir := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		dir = filepath.Join(base, t.Name())
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	types := arrayBoundsTypes
	if recursive {
		types = strings.ReplaceAll(types, "flag?:boolean", "body?:Stmt[]")
	}
	for name, contents := range map[string]string{"types.ts": "export " + types, "probe.mjs": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"checkJs":true,"allowJs":true,"noEmit":true,"target":"ES2022","module":"nodenext"},"include":["*.ts","*.mjs"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run())`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	front, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, ds, err := front.Lower([]string{"probe.mjs", "types.ts"})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if !recursive {
		storageABAPProbe(t, p, "probe.mjs.Probe", strings.TrimSpace(string(want)), "")
	}
}

// Recursive families use a distinct admission ABI which ABAP currently refuses.
// Keep the original failure on that ABI covered alongside the cross-target probe.
func TestStep4ArrayBoundsRecursive(t *testing.T) {
	source := strings.ReplaceAll(arrayBoundsTS, "flag?:boolean", "body?:Stmt[]")
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStep4ArrayBoundsScalarDestinations(t *testing.T) {
	const source = `function show(value:string|undefined):string {return value===undefined?"absent":value;}
 interface Box {value?:string}
 class Probe {
 static calls=0;static trace="";
 static words():string[] {Probe.trace+="w";return ["present"];}
 static index():number {Probe.calls++;Probe.trace+="i";return 0.5;}
 static optional(words:string[]):string|undefined {return words[-1];}
 static required():string {const words=["present"];return words[1];}
 static run():string {
 const words=["present"],numbers=[0];
 const optional:string|undefined=words[words.length];const inferred=words[-1];
 const erased:unknown=words[-1];const anyValue:any=words[1];
 const box:Box={value:words[-1]};
 return (words[-1]==undefined)+":"+(numbers[-1]==0)+":"+(numbers[-1]<1)+":"+(numbers[1]>=0)+":"+show(words[-1])+":"+show(inferred)+":"+show(optional)+":"+(erased===undefined)+":"+(anyValue===undefined)+":"+show(box.value)+":"+show(Probe.optional(words))+":"+(words[Probe.index()]===undefined)+":"+(words[-0.5]?.length===undefined)+":"+(words[Probe.index()]?.length===undefined)+":"+(Probe.words()[Probe.index()]?.length===undefined)+":"+Probe.calls+":"+Probe.trace;
 }
 }`
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("required absent scalar must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "required"))
	if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "required", "cx_sy_range_out_of_bounds")
}

func TestStep4ArrayBoundsOptionalFamilyWidening(t *testing.T) {
	const source = `type Stmt={kind:"left",name:string,body?:Stmt[]}|{kind:"right",name:string,body?:Stmt[]}|{kind:"last",name:string,body?:Stmt[]};
 type Pair=Extract<Stmt,{kind:"left"|"right"}>;
 function show(st:Stmt|undefined):string {return st===undefined?"absent":st.name;}
 class Probe {static run():string {
 const pairs:Pair[]=[{kind:"left",name:"L"},{kind:"right",name:"R"}];
 return show(pairs[-1])+":"+show(pairs[0])+":"+show(pairs[pairs.length]);
 }}`
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStep4ArrayBoundsStructuralViews(t *testing.T) {
	const source = `interface Left {readonly kind:"left";readonly name:string;readonly left:number}
 interface Right {readonly kind:"right";readonly name:string;readonly right:boolean}
 type Item=Left|Right;
 interface Bag {readonly items:readonly Item[]}
 function visit(st:Item,next:Item|undefined,previous?:Item):string {
 return st.name+":"+(next===undefined)+":"+(previous===undefined)+";";
 }
 class Probe {static run():string {
 const bag=JSON.parse('{"items":[{"kind":"left","name":"L","left":1},{"kind":"right","name":"R","right":true}]}') as Bag;
 const items=bag.items;let result="";
 for(let i=0;i<items.length;i++) {const st=items[i],next=items[i+1];result+=visit(st,next,items[i-1]);}
 const beyond=items[items.length];
 return result+(beyond===undefined)+":"+(items[-1]?.name??"absent");
 }}`
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}

// Required reference elements preserve absence until a property is used. That
// use must throw through the JS catch guard rather than a Go interface panic.
func TestStep4ArrayBoundsRequiredReferenceCatch(t *testing.T) {
	for _, tc := range []struct{ name, element, value string }{
		{"union", `type Item={kind:"left",name:string}|{kind:"right",name:string};`, `{kind:"left",name:"present"}`},
		{"class", `class Item {name="present";}`, `new Item()`},
		{"class-interface", `class Item {name="present";} class Child extends Item {}`, `new Item(),new Child()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.element + `class Probe {static run():string {
    const body:Item[]=[` + tc.value + `];
    try {return body[body.length].name;} catch {return "caught";}
   }}`
			want := conversionOracle(t, source)
			if want != "caught\n" {
				t.Fatalf("Node %q", want)
			}
			p := structuralUnionProgram(t, source)
			if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
				t.Fatalf("Go %q Node %q", got, want)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
		})
	}
}

func TestStep4ArrayBoundsReferenceTypeError(t *testing.T) {
	const source = `class Item {name="present";} class Probe {static run():string {
 const body:Item[]=[new Item()];
 try {return body[body.length].name;} catch(e) {return "caught:"+(e instanceof TypeError)+":"+(e instanceof Error);}
 }}`
	want := conversionOracle(t, source)
	if want != "caught:true:true\n" {
		t.Fatal(want)
	}
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "")
}
