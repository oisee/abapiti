import {checkpointPrefix} from './osg-runner-resume.mjs';

// Keep the executable's verdict aligned with reportStatus, including refusals
// during initialization even when every registered test passes.
export const runnerMain = `package main
import("encoding/json";"fmt";"os";"strconv";"runtime";"strings";"osg/gogen/abap")
var driverNames map[string]struct{ID string \x60json:"id"\x60}
func driverExceptionName(class string) string {
 if entry, ok := driverNames[class]; ok { return entry.ID }
 return class
}
func abapitiCheckpoint(value string){fmt.Fprintln(os.Stderr,${JSON.stringify(checkpointPrefix)}+value)}
func abapitiHostLocation() string {
 pcs:=make([]uintptr,256);n:=runtime.Callers(2,pcs)
 frames:=runtime.CallersFrames(pcs[:n]);location:="unknown host location";afterPanic:=false
 for {f,more:=frames.Next()
  if f.Function=="runtime.gopanic" {afterPanic=true} else if afterPanic && !strings.HasPrefix(f.Function,"runtime.") {
   location=fmt.Sprintf("%s:%d",f.File,f.Line);afterPanic=false
  }
  if !more {break}
 }
 return location
}
func main(){
 // Caller-supplied names keep cached binaries independent of fixture paths.
 if len(os.Args)!=2{panic("usage: abaplint-tests-osg NAMES_JSON")}
 names,err:=os.ReadFile(os.Args[1]);if err!=nil{panic(err)}
 if err=json.Unmarshal(names,&driverNames);err!=nil{panic(err)}
 defer func(){if x:=recover();x!=nil {fmt.Fprintf(os.Stderr,"OSG harness refused: %s (at %s)\\n",strings.SplitN(fmt.Sprint(x),"\\n",2)[0],abapitiHostLocation());os.Exit(2)}}()
 start,err:=strconv.ParseInt(os.Getenv("ABAPITI_OSG_START_INDEX"),10,32)
 if err!=nil || start<1 {panic("invalid OSG start index")}
 text:=ZCL_ABAPITI_TEST_RUNNER_RUN(&abap.Session{},int32(start),os.Getenv("ABAPITI_OSG_SKIP_INIT"))
 var report struct {Failed int \x60json:"failed"\x60; Trapped int \x60json:"trapped"\x60; AsyncPending int \x60json:"asyncPending"\x60; Initialization []struct{Status string \x60json:"status"\x60} \x60json:"initialization"\x60}
 if err:=json.Unmarshal([]byte(text),&report);err!=nil{panic(err)}
 fmt.Println(text)
 refusedInit:=false;for _,r:=range report.Initialization{if r.Status=="failed" || r.Status=="trapped"{refusedInit=true}}
 if report.Failed+report.Trapped+report.AsyncPending>0 || refusedInit{os.Exit(1)}
}
`;
