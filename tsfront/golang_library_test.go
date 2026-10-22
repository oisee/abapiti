package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

// Compile the emitted library wrapper against a small core double and the real
// generated Go runtime. No full closure build is needed for this lifecycle gate.
func TestRegistryGoLibrary(t *testing.T) {
	files, err := gohir.EmitPackage(&hir.Program{}, "lintlib")
	if err != nil {
		t.Fatal(err)
	}
	files["library.go"] = RegistryGoLibrary("lintlib")
	files["go.mod"] = "module zabaplint\n\ngo 1.26.0\n"
	files["stub.go"] = registryGoLibrarySymbols("lintlib").Replace(`package lintlib
 import("path";"strings")
 type @Registry@ = *registryStub
 type @Object@ interface {
  @getFiles@() *array[any]
  @getName@() jsString
  @setDirty@()
  @isDirty@() bool
 }
 type @File@ interface { @getRaw@() jsString; @getObjectName@() jsString }
 type @Issue@ = *issueStub
 type fileStub struct { name, raw jsString }
 func @newFile@(name,raw jsString) *fileStub { return &fileStub{name,raw} }
 func(f *fileStub) @getRaw@() jsString { return f.raw }
 func(f *fileStub) @getObjectName@() jsString {return str(strings.ToUpper(strings.Split(path.Base(f.name.String()),".")[0]))}
 type objectStub struct {name jsString; files *array[any]; dirty bool}
 func(o *objectStub) @getFiles@() *array[any] {return o.files}
 func(o *objectStub) @getName@() jsString {return o.name}
 func(o *objectStub) @setDirty@() {o.dirty=true}
 func(o *objectStub) @isDirty@() bool {return o.dirty}
 type registryStub struct {files map[string]*fileStub; objects map[string]*objectStub}
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
  issues:=c.Check("/zbase.clas.abap")
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
