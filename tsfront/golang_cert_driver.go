package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// RegistryGoPilotCLI isolates the entire guarded attempt in a child process.
// Snapshot bytes are read once; the sequential child receives the same snapshot.
// Child stdout is published only after successful completion. No workers run.
func RegistryGoPilotCLI() string {
	n := hir.NewNames()
	return strings.NewReplacer("@new@", n.Get("new.harness/registry_run.ts.RegistryRun"), "@add@", n.Get("member.addFile"), "@dep@", n.Get("member.addDependency"), "@parse@", n.Get("member.parse"), "@report@", n.Get("member.report"), "@times@", n.Get("member.timings")).Replace(`package main
import("bufio";"bytes";"encoding/json";"flag";"fmt";"os";"os/exec";"path/filepath";"strings")
type pilotInput struct{Name string;Raw []byte}
type pilotSnapshot struct{Files,Dependencies []pilotInput;Config []byte;Times bool}
func main(){
 mode:=os.Getenv("ABAPITI_CERT_CHILD")
 if mode!=""{pilotChild(mode);return}
 file:=flag.String("file","","ABAP file");files:=flag.String("files-list","","ordered ABAP path list");config:=flag.String("config","","abaplint.json");deps:=flag.String("deps","","dependency path list");times:=flag.Bool("times",false,"print stage milliseconds");cert:=flag.Bool("cert-pilot",false,"warm and freeze structures caches in quarantine");flag.Parse()
 defer func(){if x:=recover();x!=nil{fmt.Fprintf(os.Stderr,"refused: %v\n",x);os.Exit(1)}}()
 if (*file=="")==(*files=="")||*config==""{panic("supply --file or --files-list and --config")}
 read:=func(path string)string{b,err:=os.ReadFile(path);if err!=nil{panic(err)};return string(b)}
 paths:=func(path string)[]string{var out []string;s:=bufio.NewScanner(strings.NewReader(read(path)));for s.Scan(){if p:=strings.TrimSuffix(s.Text(),"\r");p!=""{out=append(out,p)}};if err:=s.Err();err!=nil{panic(err)};return out}
 snapshot:=pilotSnapshot{Config:[]byte(read(*config)),Times:*times}
 add:=func(path string,dependency bool){i:=pilotInput{Name:filepath.Base(strings.ReplaceAll(path,"\\","/")),Raw:[]byte(read(path))};if dependency{snapshot.Dependencies=append(snapshot.Dependencies,i)}else{snapshot.Files=append(snapshot.Files,i)}}
 if *file!=""{add(*file,false)}else{for _,p:=range paths(*files){add(p,false)}}
 if *deps!=""{for _,p:=range paths(*deps){add(p,true)}}
 raw,err:=json.Marshal(snapshot);if err!=nil{panic(err)}
 executable,err:=os.Executable();if err!=nil{panic(err)}
 attempt:=func(mode string)([]byte,[]byte,error){child:=exec.Command(executable);child.Env=append(os.Environ(),"ABAPITI_CERT_CHILD="+mode);child.Stdin=bytes.NewReader(raw);var output,diagnostics bytes.Buffer;child.Stdout=&output;child.Stderr=&diagnostics;err:=child.Run();return output.Bytes(),diagnostics.Bytes(),err}
 mode="vanilla";if *cert{mode="guarded"}
 output,diagnostics,err:=attempt(mode)
 if err!=nil {if e,ok:=err.(*exec.ExitError);ok&&*cert&&e.ExitCode()==86{
  fmt.Fprintln(os.Stderr,"certificate quarantine: guarded results discarded; sequential rerun from unchanged input snapshot")
  os.Stderr.Write(diagnostics)
  output,diagnostics,err=attempt("vanilla")
 }}
 os.Stderr.Write(diagnostics)
 if err!=nil{panic(err)}
 if _,err:=os.Stdout.Write(output);err!=nil{panic(err)}
}
func pilotChild(mode string){
 defer func(){if x:=recover();x!=nil{if v,ok:=x.(pilotViolation);ok{fmt.Fprintln(os.Stderr,v.Error());os.Exit(86)};fmt.Fprintf(os.Stderr,"refused: %v\n",x);os.Exit(1)}}()
 if mode!="guarded"&&mode!="vanilla"{panic("invalid certificate child mode")}
 pilotEnabled=mode=="guarded"
 var snapshot pilotSnapshot;if err:=json.NewDecoder(os.Stdin).Decode(&snapshot);err!=nil{panic(err)}
 h:=@new@();for _,f:=range snapshot.Files{h.@add@(str(f.Name),str(string(f.Raw)))};for _,f:=range snapshot.Dependencies{h.@dep@(str(f.Name),str(string(f.Raw)))}
 reg:=h.@parse@(str(string(snapshot.Config)));dump:=h.@report@(reg)
 fmt.Println(dump.String());if snapshot.Times{fmt.Println("ms: "+h.@times@().String())}
}
`)
}
