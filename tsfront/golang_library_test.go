package tsfront

import (
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

// Compile the emitted library wrapper against a small core double and the real
// generated Go runtime. No full closure build is needed for this lifecycle gate.
func TestRegistryGoLibrary(t *testing.T) {
	files, names, err := gohir.EmitPackageWithNames(&hir.Program{}, "lintlib")
	if err != nil {
		t.Fatal(err)
	}
	files["library.go"] = RegistryGoLibrary("lintlib", names)
	files["go.mod"] = "module zabaplint\n\ngo 1.26.0\n"
	files["stub.go"] = registryGoLibrarySymbols("lintlib", names).Replace(`package lintlib
 import("path";"strings")
 type @Registry@ = *registryStub
 type @Object@ interface {
  @getType@() jsString
  @getFiles@() *array[any]
  @getName@() jsString
  @setDirty@()
  @isDirty@() bool
 }
 type @File@ interface { @getFilename@() jsString; @getRaw@() jsString; @getObjectName@() jsString }
 type @Issue@ = *issueStub
 type @Position@ = *positionStub
 type @AbstractToken@ interface { @getStr@() jsString; @getStart@() @Position@; @getEnd@() @Position@ }
 type @StatementNode@ interface { @get@() any; @getFirstToken@() @AbstractToken@; @getLastToken@() @AbstractToken@; @getTokens@() *array[@AbstractToken@] }
 type @StructureNode@ interface { @get@() any; @getFirstToken@() @AbstractToken@; @getLastToken@() @AbstractToken@; @getChildren@() *array[any] }
 type @ABAPFile@ interface { @getFilename@() jsString; @getStructure@() @StructureNode@; @getStatements@() *array[@StatementNode@] }
 type @ABAPObject@ interface { @getABAPFiles@() *array[@ABAPFile@]; @getMainABAPFile@() @ABAPFile@; @getSequencedFiles@() *array[@ABAPFile@] }
 var @PragmaDescriptor@ classDescriptor
 func(o *objectStub) @getType@() jsString { return str("CLAS") }
 func(r *registryStub) @getObject@(typ,name optional[jsString]) @Object@ { if r.data!=nil {if !r.parsed{panic("lookup before parse")};r.parsed=false;return r.data[typ.Value.String()+":"+strings.ToUpper(name.Value.String())]}; if !typ.Has || !name.Has || typ.Value.String()!="CLAS" {return nil}; o:=r.objects[strings.ToUpper(name.Value.String())];if o==nil{return nil};return o }

 type fileStub struct { name, raw jsString }
 func @newFile@(name,raw jsString) *fileStub { return &fileStub{name,raw} }
 func(f *fileStub) @getFilename@() jsString {return f.name}
 func(f *fileStub) @getRaw@() jsString { return f.raw }
 func(f *fileStub) @getObjectName@() jsString {return str(strings.ToUpper(strings.Split(path.Base(f.name.String()),".")[0]))}
 type objectStub struct {name jsString; files *array[any]; dirty bool}
 func(o *objectStub) @getFiles@() *array[any] {return o.files}
 func(o *objectStub) @getName@() jsString {return o.name}
 func(o *objectStub) @setDirty@() {o.dirty=true}
 func(o *objectStub) @isDirty@() bool {return o.dirty}
 type registryStub struct {files map[string]*fileStub; objects map[string]*objectStub; data map[string]@Object@; parsed bool; parseCalls int}
 type objectsStub struct{objects []@Object@; at int}
 type objectNext struct{@done@ bool; @value@ @Object@}
 func(r *registryStub) @getObjects@() *objectsStub {
  out:=&objectsStub{};for _,obj:=range r.objects{out.objects=append(out.objects,obj)};return out
 }
 func(g *objectsStub) @next@() *objectNext {
  if g.at==len(g.objects){return &objectNext{@done@:true}}
  obj:=g.objects[g.at];g.at++;return &objectNext{@value@:obj}
 }
 func(g *objectsStub) @close@() {}

 func(r *registryStub) @getFileByName@(name jsString) @File@ {f:=r.files[name.String()];if f==nil{return nil};return f}
 func(r *registryStub) @findObjectForFile@(f @File@) @Object@ {o:=r.objects[f.@getObjectName@().String()];if o==nil{return nil};return o}
 func(r *registryStub) @addFile@(f @File@) {
  file:=f.(*fileStub);name:=f.@getObjectName@().String();obj:=r.objects[name]
  if obj==nil{obj=&objectStub{name:str(name), files:&array[any]{}};r.objects[name]=obj}
  r.files[file.name.String()]=file;obj.files.push(any(file));obj.dirty=true
 }
 func(r *registryStub) @removeFile@(f @File@) {
  file:=f.(*fileStub);obj:=r.objects[f.@getObjectName@().String()]
  for i,item:=range obj.files.Items{if item==any(f){obj.files.Items=append(obj.files.Items[:i],obj.files.Items[i+1:]...);break}}
  delete(r.files,file.name.String());obj.dirty=true
  if len(obj.files.Items)==0{delete(r.objects,f.@getObjectName@().String())}
 }
 func(r *registryStub) @updateFile@(f @File@) {
  file:=f.(*fileStub);old:=r.files[file.name.String()];obj:=r.objects[f.@getObjectName@().String()]
  for i,item:=range obj.files.Items{if item==any(old){obj.files.Items[i]=any(f)}}
  r.files[file.name.String()]=file;obj.dirty=true
 }
 func(r *registryStub) @findIssuesObject@(o @Object@) *array[any] {
  obj:=o.(*objectStub);file:=obj.files.Items[0].(*fileStub)
  if file.raw.String()=="throw"{panic("refused check")}
  for _,object:=range r.objects{object.dirty=false}
  issue:=&issueStub{file:file}
  return &array[any]{Items:[]any{issue,issue}}
 }
 type harnessStub struct{ files,deps []*fileStub }
 func @newRun@() *harnessStub{return &harnessStub{}}
 func(h *harnessStub) @addFile@(name,raw jsString){h.files=append(h.files,@newFile@(name,raw))}
 func(h *harnessStub) @addDependency@(name,raw jsString){h.deps=append(h.deps,@newFile@(name,raw))}
 func(r *registryStub) @parse@() { r.parsed=true;r.parseCalls++; for _,o:=range r.objects{o.dirty=false} }
 func(h *harnessStub) @parse@(cfg jsString) @Registry@ {
  if cfg.String()=="throw"{panic("refused prime")}
  r:=&registryStub{files:map[string]*fileStub{},objects:map[string]*objectStub{}}
  for _,files:=range [][]*fileStub{h.files,h.deps}{for _,f:=range files{r.@addFile@(f)}}
  for _,o:=range r.objects{o.dirty=false};return r
 }
 type issueStub struct{file *fileStub}
 func(i *issueStub) @getFilename@() jsString{return i.file.name}
 func(i *issueStub) @getMessage@() jsString{return i.file.raw}
 func(i *issueStub) @getSeverity@() jsString{return str("warning")}
 func(i *issueStub) @getKey@() jsString{return str("syntax")}
 func(i *issueStub) @getStart@() *positionStub{return &positionStub{12,5}}
 func(i *issueStub) @getEnd@() *positionStub{return &positionStub{13,6}}
 type positionStub struct{row,col float64}
 func(p *positionStub) @getRow@() float64{return p.row}
 func(p *positionStub) @getCol@() optional[float64]{return optional[float64]{Has:true,Value:p.col}}
 `)
	for _, pair := range [][2]string{{"stub.go.txt", "data_stub.go"}, {"unit_test.go.txt", "data_test.go"}} {
		raw, err := os.ReadFile(filepath.Join("testdata", "library-data", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		files[pair[1]] = registryGoLibrarySymbols("lintlib", names).Replace(string(raw))
	}
	files["library_test.go"] = `package lintlib
 import("math";"strings";"testing")
 func refused(t *testing.T, part string, fn func()){
  t.Helper();defer func(){v:=recover();if v==nil||!strings.Contains(v.(string),part){t.Fatalf("panic %v, want %s",v,part)}}();fn()
 }
 func TestLifecycle(t *testing.T){
  var c Checker
  refused(t,"prime",func(){c.Check("missing")})
  c.Prime([]File{{"/zbase.clas.abap","ZBASE old"},{"/zsub.clas.abap","ZSUB ZBASE"},{"/zcall.clas.abap","ZCALL ZSUB"},{"/zcall.clas.locals_def.abap","ZBASE"},{"/zother.clas.abap","ZOTHER"}},[]File{{"/zdep.intf.abap","ZDEP"}},"{}")
  if c.Stats().IndexedObjects!=5{t.Fatal(c.Stats())}
  if issues,found:=c.CheckObject("CLAS","missing");found||issues!=nil{t.Fatal(issues,found)}
  if files,found:=c.Tree("CLAS","missing");found||files!=nil{t.Fatal(files,found)}
  if files,found:=c.Tree("CLAS","zbase");!found||len(files)!=0{t.Fatal(files,found)}
  issues,found:=c.CheckObject("CLAS","zbase")
  if !found { t.Fatal("lowercase name lookup") }
  if len(issues)!=2||issues[0]!=issues[1]||issues[0].Message!="ZBASE old"||issues[0].Row!=12||issues[0].Column!=5||issues[0].EndRow!=13||issues[0].EndColumn!=6||issues[0].Severity!="warning"{t.Fatal(issues)}
  raw:="ZBASE old";c.Update("/zbase.clas.abap",&raw)
  if c.Stats().Noops!=1||c.Stats().DirtiedObjects!=0{t.Fatal(c.Stats())}
  raw="ZBASE new";c.Update("/zbase.clas.abap",&raw)
  if c.Stats().DirtiedObjects!=3||c.Stats().ParseRuns!=0{t.Fatal(c.Stats())}
  c.Check("/zcall.clas.abap")
  if c.Stats().ParseRuns!=1||c.Stats().ParseObjects!=3{t.Fatal(c.Stats())}
  // Restore remains lazy; no-op updates must retain pending dirty state.
  raw="ZBASE old";c.Update("/zbase.clas.abap",&raw);c.Update("/zbase.clas.abap",&raw)
  c.Check("/zcall.clas.abap")
  if c.Stats().ParseRuns!=2||c.Stats().ParseObjects!=6{t.Fatal(c.Stats())}
  c.Update("/zcall.clas.locals_def.abap",nil)
  before:=c.Stats();raw="ZSUB";c.Update("/zsub.clas.abap",&raw)
  if c.Stats().DirtiedObjects-before.DirtiedObjects!=2{t.Fatal("surviving include lost reader",c.Stats())}
  c.Check("/zcall.clas.abap");before=c.Stats();raw="ZBASE isolated";c.Update("/zbase.clas.abap",&raw)
  if c.Stats().DirtiedObjects-before.DirtiedObjects!=1{t.Fatal("stale edges",c.Stats())}
  // Re-created identities, cycles and removal of the last file.
  c.Update("/zsub.clas.abap",nil);raw="ZSUB ZBASE ZCALL";c.Update("/zsub.clas.abap",&raw)
  before=c.Stats();raw="ZBASE cycle";c.Update("/zbase.clas.abap",&raw)
  if c.Stats().DirtiedObjects-before.DirtiedObjects!=3{t.Fatal("cycle",c.Stats())}
  c.Check("/zcall.clas.abap")
  // Empty source differs from removal; restore after a failed check works.
  raw="";c.Update("/znew.prog.abap",&raw)
  if c.Check("/znew.prog.abap")[0].Message!=""{t.Fatal("empty update")}
  raw="throw";c.Update("/znew.prog.abap",&raw)
  refused(t,"refused check",func(){c.Check("/znew.prog.abap")})
  c.Update("/znew.prog.abap",nil)
  refused(t,"not in the registry",func(){c.Check("/znew.prog.abap")})
  c.Check("/zcall.clas.abap")
  // Failed prime never exposes the old corpus.
  refused(t,"refused prime",func(){c.Prime(nil,nil,"throw")})
  refused(t,"prime",func(){c.Check("/zcall.clas.abap")})
 }
 func TestConcatTokens(t *testing.T){
  tokens:=[]Token{{Str:"a",Start:Pos{1,1}},{Str:"##x",Start:Pos{1,2},Pragma:true},{Str:".",Start:Pos{1,2}},{Str:"😀",Start:Pos{2,1}},{Str:"x",Start:Pos{2,3}},{Str:"##literal",Start:Pos{3,1}}}
  if got:=ConcatTokens(tokens);got!="a. 😀x ##literal"{t.Fatal(got)}
  if ConcatTokens(nil)!=""||ConcatTokens([]Token{{Str:"",Start:Pos{1,1}},{Str:"x",Start:Pos{3,5}}})!="x"{t.Fatal("empty")}
 }
 func TestCoordinates(t *testing.T){
  if libraryCoordinate(optional[float64]{Has:true,Value:5})!=5{t.Fatal("optional")}
  for _,n:=range []float64{math.NaN(),math.Inf(1),1.5,2147483648}{refused(t,"finite int32",func(){libraryCoordinate(n)})}
  refused(t,"undefined",func(){libraryCoordinate(optional[float64]{})})
 }
 `
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-race", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("emitted library: %v\n%s", err, out)
	}
}

// Use real HIR declarations rather than wrapper-shaped Go stubs. Snapshot the
// emitter's names before rendering: resolving a missing identity while rendering
// must not make a placeholder appear to have been emitted by the package.
func TestRegistryGoLibraryEmittedNames(t *testing.T) {
	registry := &hir.Class{Name: "src/registry.ts.Registry"}
	for _, name := range []string{"addFile", "addDependency", "parse", "getFileByName", "findObjectForFile", "findIssuesObject", "getFiles", "getObjects", "next", "close", "done", "value", "getRaw", "getObjectName", "getName", "setDirty", "isDirty", "updateFile", "removeFile", "getKey", "getSeverity", "getFilename", "getMessage", "getStart", "getEnd", "getRow", "getCol", "getObject", "getType", "getABAPFiles", "getMainABAPFile", "getSequencedFiles", "getStructure", "getStatements", "get", "getChildren", "getFirstStatement", "getFirstToken", "getLastToken", "getTokens", "getStr", "getTree", "getSQLViewName", "concatTokens", "parseType", "listKeys", "getComponents", "name"} {
		registry.Methods = append(registry.Methods, &hir.Method{
			Name: name, Result: hir.T(hir.Void), Body: &hir.Stmt{Kind: hir.Block},
		})
	}
	p := &hir.Program{
		Classes: []*hir.Class{registry,
			{Name: "src/abap/nodes/expression_node.ts.ExpressionNode"},
			{Name: "src/objects/data_definition.ts.DataDefinition"},
			{Name: "src/objects/table.ts.Table"},
			{Name: "src/objects/view.ts.View"},
			{Name: "src/abap/types/basic/structure_type.ts.StructureType"},
			{Name: "src/abap/types/basic/_abstract_type.ts.AbstractType"},
			{Name: "src/issue.ts.Issue"},
			{Name: "src/objects/_abap_object.ts.ABAPObject"},
			{Name: "src/abap/abap_file.ts.ABAPFile"},
			{Name: "src/abap/nodes/statement_node.ts.StatementNode"},
			{Name: "src/abap/nodes/structure_node.ts.StructureNode"},
			{Name: "src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken"},
			{Name: "src/abap/1_lexer/tokens/pragma.ts.Pragma"},
			{Name: "src/position.ts.Position"},
			{Name: "harness/registry_run.ts.RegistryRun"},
			{Name: "src/files/memory_file.ts.MemoryFile"},
		},
		Interfaces: []*hir.Interface{
			{Name: "src/abap/types/basic/structure_type.ts.IStructureComponent"},
			{Name: "src/objects/_iobject.ts.IObject"},
			{Name: "src/files/_ifile.ts.IFile"},
		},
	}
	files, names, err := gohir.EmitPackageWithNames(p, "lintlib")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for name, id := range names.Pairs() {
		byID[id] = name
	}
	// Collect actual Go identifiers so a name-table entry alone is insufficient.
	identifiers := map[string]bool{}
	for filename, source := range files {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		var scan scanner.Scanner
		set := token.NewFileSet()
		scan.Init(set.AddFile(filename, -1, len(source)), []byte(source), nil, 0)
		for {
			_, tok, literal := scan.Scan()
			if tok == token.EOF {
				break
			}
			if tok == token.IDENT {
				identifiers[literal] = true
			}
		}
	}
	special := map[string]string{
		"ExpressionNode":   "ref.src/abap/nodes/expression_node.ts.ExpressionNode",
		"DataDefinition":   "ref.src/objects/data_definition.ts.DataDefinition",
		"Table":            "ref.src/objects/table.ts.Table",
		"View":             "ref.src/objects/view.ts.View",
		"StructureType":    "ref.src/abap/types/basic/structure_type.ts.StructureType",
		"AbstractType":     "ref.src/abap/types/basic/_abstract_type.ts.AbstractType",
		"Component":        "ref.src/abap/types/basic/structure_type.ts.IStructureComponent",
		"Registry":         "ref.src/registry.ts.Registry",
		"ABAPObject":       "ref.src/objects/_abap_object.ts.ABAPObject",
		"ABAPFile":         "ref.src/abap/abap_file.ts.ABAPFile",
		"StatementNode":    "ref.src/abap/nodes/statement_node.ts.StatementNode",
		"StructureNode":    "ref.src/abap/nodes/structure_node.ts.StructureNode",
		"AbstractToken":    "ref.src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken",
		"Position":         "ref.src/position.ts.Position",
		"PragmaDescriptor": "descriptor.src/abap/1_lexer/tokens/pragma.ts.Pragma",
		"Object":           "ref.src/objects/_iobject.ts.IObject",
		"File":             "ref.src/files/_ifile.ts.IFile",
		"Issue":            "ref.src/issue.ts.Issue",
		"newRun":           "new.harness/registry_run.ts.RegistryRun",
		"newFile":          "new.src/files/memory_file.ts.MemoryFile",
	}
	marker := regexp.MustCompile(`@([A-Za-z]+)@`)
	want := marker.ReplaceAllStringFunc(registryGoLibrary, func(placeholder string) string {
		key := strings.Trim(placeholder, "@")
		if key == "package" {
			return "lintlib"
		}
		id := special[key]
		if id == "" {
			id = "member." + key
		}
		name, ok := byID[id]
		if !ok || !identifiers[name] {
			t.Fatalf("placeholder %s: %s has no identifier in emitted package (name %q)", placeholder, id, name)
		}
		return name
	})
	if got := RegistryGoLibrary("lintlib", names); got != want || strings.Contains(got, "@") {
		t.Fatal("library placeholders differ from the package emitter's identifiers")
	}
	// A deliberately different naming policy catches reintroducing NewNames
	// inside the renderer, even while today's default names are deterministic.
	alternate := hir.NewReadableNames(p, "LIB")
	alternateWant := marker.ReplaceAllStringFunc(registryGoLibrary, func(placeholder string) string {
		key := strings.Trim(placeholder, "@")
		if key == "package" {
			return "lintlib"
		}
		id := special[key]
		if id == "" {
			id = "member." + key
		}
		return alternate.Get(id)
	})
	if got := RegistryGoLibrary("lintlib", alternate); got != alternateWant || got == want {
		t.Fatal("library renderer ignored the supplied name table")
	}
}
