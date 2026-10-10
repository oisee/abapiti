package tsfront

import (
	_ "embed"
	"github.com/oisee/abapiti/hir"
	"strings"
)

//go:embed golang_read_policy.go.txt
var registryGoReadPolicy string

// RegistryGoCLI is the native host for the same lowered RegistryRun harness
// used by zabaplint. Only file I/O, flags and profiling are host operations.
func RegistryGoCLI() string {
	n := hir.NewNames()
	return strings.NewReplacer("@new@", n.Get("new.harness/registry_run.ts.RegistryRun"), "@addFile@", n.Get("member.addFile"), "@addDependency@", n.Get("member.addDependency"), "@parse@", n.Get("member.parse"), "@report@", n.Get("member.report"), "@timings@", n.Get("member.timings")).Replace(`package main
import("bufio";"encoding/json";"flag";"fmt";"os";"path/filepath";"runtime";"runtime/pprof";rmetrics "runtime/metrics";"strings";"time")
func main(){
 file:=flag.String("file","","ABAP file");config:=flag.String("config","","abaplint.json");deps:=flag.String("deps","","dependency path list");times:=flag.Bool("times",false,"print stage milliseconds");cpu:=flag.String("cpu-profile","","CPU profile path");mem:=flag.String("mem-profile","","allocation profile path");metrics:=flag.Bool("metrics",false,"print resource metrics to stderr");var allowed readPolicy;flag.Func("allow-read","additional directory tree permitted for reads (repeatable)",allowed.allow);flag.Parse()
 if *file==""||*config==""{fmt.Fprintln(os.Stderr,"usage: --file FILE --config CONFIG [--deps LIST] [--times] [-allow-read DIR]");os.Exit(2)}
 defer func(){if x:=recover();x!=nil{fmt.Fprintf(os.Stderr,"refused: %v\n",x);os.Exit(1)}}()
 for _,path:=range []string{*file,*config,*deps}{if path!=""{if err:=allowed.allow(filepath.Dir(path));err!=nil{panic(err)}}}
 read:=func(path string)string{raw,err:=allowed.read(path);if err!=nil{panic(err)};return string(raw)}
 h:=@new@()
 add:=func(path string,dependency bool){name:=filepath.Base(strings.ReplaceAll(path,"\\","/"));raw:=read(path);if dependency{h.@addDependency@(str(name),str(raw))}else{h.@addFile@(str(name),str(raw))}}
 cfg:=read(*config);add(*file,false)
 if *deps!=""{scan:=bufio.NewScanner(strings.NewReader(read(*deps)));for scan.Scan(){p:=strings.TrimSuffix(scan.Text(),"\r");if p!=""{if !filepath.IsAbs(p){p=filepath.Join(filepath.Dir(*deps),p)};if err:=allowed.allow(filepath.Dir(p));err!=nil{panic(err)};add(p,true)}};if err:=scan.Err();err!=nil{panic(err)}}
 if *cpu!=""{f,err:=os.Create(*cpu);if err!=nil{panic(err)};if err=pprof.StartCPUProfile(f);err!=nil{panic(err)};defer func(){pprof.StopCPUProfile();f.Close()}()}
 start:=time.Now();reg:=h.@parse@(str(cfg));dump:=h.@report@(reg);elapsed:=time.Since(start)
 fmt.Println(dump.String())
 if *times {fmt.Println("ms: "+h.@timings@().String())}
 if *metrics {var m runtime.MemStats;runtime.ReadMemStats(&m);samples:=[]rmetrics.Sample{{Name:"/cpu/classes/gc/total:cpu-seconds"}};rmetrics.Read(samples);gcSeconds:=samples[0].Value.Float64();json.NewEncoder(os.Stderr).Encode(struct{Seconds float64;GCCPUFraction float64;GCCPUSeconds float64;HeapSys,TotalAlloc uint64;NumGC uint32;Stages string}{elapsed.Seconds(),m.GCCPUFraction,gcSeconds,m.HeapSys,m.TotalAlloc,m.NumGC,h.@timings@().String()})}
 if *mem!=""{f,err:=os.Create(*mem);if err!=nil{panic(err)};if err=pprof.Lookup("allocs").WriteTo(f,0);err!=nil{panic(err)};f.Close()}
}
`) + registryGoReadPolicy
}
