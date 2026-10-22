package tsfront

import (
	"os"
	"strings"
	"testing"
)

const externABAPProbe = `import * as host from './host';class Probe {
 static run():string {
  let out=host.readText("a.txt")+"|"+host.flip(true)+"|"+host.add(40,2)+"|"+host.strings(["a","b"]).join(",");
  out+="|"+host.join("a",...["b"])+"|"+host.join()+"|"+(host.maybe(false)===undefined);
  out+="|"+host.tuples(host.listEntries("."));
  try {host.writeStdout("x");} catch(e) {out+="|"+e.message;}
  return out;
 }}`

const externABAPHost = `export function readText(file:string):string{return file;}
 export function flip(x:boolean):boolean{return x;}
 export function add(x:number,y:number):number{return x+y;}
 export function strings(x:string[]):string[]{return x;}
 export function join(...parts:string[]):string{return "";}
 export function maybe(x:boolean):string|undefined{return undefined;}
 export function listEntries(dir:string):[string,boolean][]{return [];}
 export function tuples(x:[string,boolean][]):string{return "";}
 export function writeStdout(text:string):void{}`

// externABAPStub implements externABAPHost like the Go host of
// TestExternTypedABI, so the compile gate (and an OSGo run) sees the ABI.
const externABAPStub = `CLASS zcl_abapiti_probe_host DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    TYPES: BEGIN OF ty_pair,
             f0 TYPE string,
             f1 TYPE abap_bool,
           END OF ty_pair.
    TYPES ty_pairs TYPE STANDARD TABLE OF ty_pair WITH DEFAULT KEY.
    CLASS-METHODS read_text IMPORTING file TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS flip IMPORTING x TYPE abap_bool RETURNING VALUE(result) TYPE abap_bool.
    CLASS-METHODS add IMPORTING x TYPE f y TYPE f RETURNING VALUE(result) TYPE f.
    CLASS-METHODS strings IMPORTING x TYPE string_table RETURNING VALUE(result) TYPE string_table.
    CLASS-METHODS join IMPORTING parts TYPE string_table RETURNING VALUE(result) TYPE string.
    CLASS-METHODS maybe IMPORTING x TYPE abap_bool RETURNING VALUE(result) TYPE REF TO string.
    CLASS-METHODS list_entries IMPORTING dir TYPE string RETURNING VALUE(result) TYPE ty_pairs.
    CLASS-METHODS tuples IMPORTING x TYPE ty_pairs RETURNING VALUE(result) TYPE string.
    CLASS-METHODS write_stdout IMPORTING text TYPE string.
ENDCLASS.
CLASS zcl_abapiti_probe_host IMPLEMENTATION.
  METHOD read_text.
    result = file.
  ENDMETHOD.
  METHOD flip.
    result = xsdbool( x = abap_false ).
  ENDMETHOD.
  METHOD add.
    result = x + y.
  ENDMETHOD.
  METHOD strings.
    result = x.
  ENDMETHOD.
  METHOD join.
    result = concat_lines_of( table = parts sep = '/' ).
  ENDMETHOD.
  METHOD maybe.
    IF x = abap_true.
      CREATE DATA result.
    ENDIF.
  ENDMETHOD.
  METHOD list_entries.
    APPEND VALUE #( f0 = dir && 'entry' f1 = abap_true ) TO result.
  ENDMETHOD.
  METHOD tuples.
    DATA pair TYPE ty_pair.
    READ TABLE x INDEX 1 INTO pair.
    IF pair-f1 = abap_true.
      result = pair-f0.
    ENDIF.
  ENDMETHOD.
  METHOD write_stdout.
    RAISE EXCEPTION TYPE cx_sy_file_open EXPORTING filename = text.
  ENDMETHOD.
ENDCLASS.
`

// The golden pins the extern host ABI on ABAP: static calls on the bound
// class, snake-case names, primitive carriers, host exceptions as Error.
func TestExternABAPHostCall(t *testing.T) {
	r, _, _ := externProjectABAP(t, externABAPProbe, externABAPHost, "ZCL_ABAPITI_PROBE_HOST")
	requireExternClean(t, r)
	files, names, err := emitABAPNamed(t, r.Prog)
	if err != nil {
		t.Fatal(err)
	}
	files["zcl_abapiti_probe_host.clas.abap"] = externABAPStub
	src := files[names.Get("probe.ts.Probe")+".clas.abap"]
	start, end := strings.Index(src, "METHOD "+names.Get("member.run")+"."), strings.Index(src, "ENDMETHOD.")
	if start < 0 {
		t.Fatalf("no run method:\n%s", src)
	}
	end = start + strings.Index(src[start:], "ENDMETHOD.")
	got := src[start:end]
	golden := "testdata/extern/abap-host-call.golden"
	if os.Getenv("ABAPITI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("ABAP host call drifted (ABAPITI_UPDATE_GOLDEN=1 to accept):\n%s", got)
	}
	for _, call := range []string{"zcl_abapiti_probe_host=>read_text( file = ", "zcl_abapiti_probe_host=>join( parts = ", "zcl_abapiti_probe_host=>list_entries( dir = ", "zcl_abapiti_probe_host=>write_stdout( text = ", "CATCH cx_static_check cx_dynamic_check INTO "} {
		if !strings.Contains(got, call) {
			t.Errorf("missing %q", call)
		}
	}
}

func TestExternABAPHostIdentifiers(t *testing.T) {
	r, _, _ := externProjectABAP(t, `import {f} from './host';class Probe {static run():string {return f("a");}}`, `export function f(result:string):string{return result;}`, "ZCL_H")
	requireExternClean(t, r)
	_, err := emitABAP(t, r.Prog)
	if err == nil || !strings.Contains(err.Error(), `"result" is not an ABAP host identifier`) || !strings.Contains(err.Error(), "probe.ts:") {
		t.Fatal(err)
	}
}

// Every function of gogen's host-io.ts plus the step-4 driver's writeStdout,
// bound to the reference host class. The fixture carries the class, so the
// compile gate checks both sides of the ABI.
func TestExternABAPReferenceHost(t *testing.T) {
	host, err := os.ReadFile("../extern/hostio/abap/zcl_abapiti_hostio.clas.abap")
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := externProjectABAP(t, `import * as io from './host';class Probe {
 static run():string {
  let out=io.readText("in.txt")+"|"+io.listDir(".").join(",")+"|"+io.listEntries(".").length;
  out+="|"+io.basename("/a/b/")+"|"+io.dirname("a/b")+"|"+io.join("a","..","b",...["c"])+"|"+io.gitHead(".")+"|"+(io.env("HOME")??"none");
  io.writeText("out.txt",out);io.writeStdout(out);
  return out;
 }}`, `export function readText(file:string):string{return file;}
 export function writeText(file:string,text:string):void{}
 export function writeStdout(text:string):void{}
 export function listDir(dir:string):string[]{return [];}
 export function listEntries(dir:string):[string,boolean][]{return [];}
 export function basename(file:string):string{return file;}
 export function dirname(file:string):string{return file;}
 export function join(...parts:string[]):string{return "";}
 export function gitHead(cwd:string):string|undefined{return undefined;}
 export function env(name:string):string|undefined{return undefined;}`, "ZCL_ABAPITI_HOSTIO")
	requireExternClean(t, r)
	files, _, err := emitABAPNamed(t, r.Prog)
	if err != nil {
		t.Fatal(err)
	}
	files["zcl_abapiti_hostio.clas.abap"] = string(host)
	for _, line := range strings.Split(string(host), "\n") {
		if len(line) > 255 {
			t.Errorf("host line over 255 bytes: %q", line)
		}
	}
}

// Exported to the ABAP fixture compile gate with the real host class. Ambient
// process.env uses the env extern capability through host.getenv; exercise
// that path as well as ordinary optional-string extern calls.
func TestExternABAPOptionalStringConsumers(t *testing.T) {
	host, err := os.ReadFile("../extern/hostio/abap/zcl_abapiti_hostio.clas.abap")
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := externProjectABAP(t, `import * as io from './host';
 declare const process:{env:{[key:string]:string|undefined}};
 class Probe {
  static pass(value:string|undefined):string {return value??"unset";}
  static run():string {
   let out="";
   for (const name of ["ABAPITI_SMOKE","ABAPITI_SMOKE_UNSET"]) {
    const value:string|undefined=io.env(name);
    out+=Probe.pass(value)+"|"+(io.env(name)??"unset");
    out+="|"+(io.env(name)===undefined)+"|"+Probe.pass(io.env(name));
    out+="|"+io.basename(io.env(name)??"unset");
    out+="|"+Probe.pass(io.gitHead(name))+"|"+(io.gitHead(name)??"no-git");
    const ambient:string|undefined=process.env[name];
    out+="|"+Probe.pass(ambient)+"|"+(process.env[name]??"unset");
    out+="|"+(process.env[name]===undefined)+"|"+Probe.pass(process.env[name]);
    out+="|"+io.basename(process.env[name]??"unset");
   }
   return out+"|"+(io.env("ABAPITI_SMOKE")==="on");
  }
 }`, `export function env(name:string):string|undefined{return undefined;}
 export function gitHead(cwd:string):string|undefined{return undefined;}
 export function basename(file:string):string{return file;}`, "ZCL_ABAPITI_HOSTIO")
	requireExternClean(t, r)
	files, names, err := emitABAPNamed(t, r.Prog)
	if err != nil {
		t.Fatal(err)
	}
	files["zcl_abapiti_hostio.clas.abap"] = string(host)
	src := files[names.Get("probe.ts.Probe")+".clas.abap"]
	for _, want := range []string{"=>env( name = ", "=>git_head( cwd = ", "IS BOUND.", "->* TO <", "->has = abap_true."} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(src, "CAST string(") {
		t.Fatal("optional extern string cast as an object")
	}
}
