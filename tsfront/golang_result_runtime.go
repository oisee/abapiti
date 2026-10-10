package tsfront

const valueRuntime = `package main
// Result's persistent chain and tokens remain shared; only the mutable wrapper
// is copied. Neither implementation deep copies ABAP syntax/token nodes.
func resultCopy[T any](v T) T {
 if x,ok:=any(v).(*@OBJ@);ok && x!=nil {c:=*x;return any(&c).(T)}
 return v
}
func resultCopyElements[T any](a *array[T])*array[T]{if a!=nil{for i,v:=range a.Items{a.Items[i]=resultCopy(v)}};return a}
func resultDebugMutation(string,string){}
func resultDebugSite(string){}
`

const debugRuntime = `package main
import("encoding/json";"fmt";"os";"reflect";"sort";"strings";"regexp";"strconv")
type resultRootSlot struct{Name,Created string;Get func()any;Slot reflect.Value;Last int;Scope int}
type resultFrameState struct{ID int;Site string;Tick int;Roots map[string]resultRootSlot}
type resultHolder struct{Path,Created string}
type resultFinding struct{Mutator,Site string;Holders []resultHolder;Published []string;Count int}
var resultFrames []*resultFrameState
var resultFrameID int
var resultSite string
var resultReceiverLocal string
var resultReceiverFrame *resultFrameState
var resultWalkLimit=func()int{if s:=os.Getenv("ABAPITI_RESULT_WALK_LIMIT");s!=""{n,err:=strconv.Atoi(s);if err!=nil||n<1{panic("invalid ABAPITI_RESULT_WALK_LIMIT")};return n};return 200000}()
var resultUnknownPublish bool
var resultChecks int
var resultChecksByMutator=map[string]int{}
var resultChecksBySite=map[string]int{}
var resultIndices=regexp.MustCompile("\\[[0-9]+\\]")
var resultFindings=map[string]*resultFinding{}
var resultPublished=map[uintptr][]string{}
var resultGlobals=map[string]func()any{}
var resultGlobalNames []string
var resultContainerOrigins=map[uintptr]map[int]string{}
var resultFieldOrigins=map[uintptr]string{}
func resultFieldStored(slot any,site string){v:=reflect.ValueOf(slot);if v.Kind()==reflect.Pointer{resultFieldOrigins[v.Pointer()]=site}}
func resultDirect(v any)uintptr{value:=reflect.ValueOf(v);if value.IsValid()&&value.Type()==resultType&&!value.IsNil(){return value.Pointer()};return 0}
func resultStored(container any,site,operation string,args ...any){
 v:=reflect.ValueOf(container);if v.Kind()!=reflect.Pointer||v.IsNil(){return};owner:=v.Pointer();v=v.Elem();if v.Kind()!=reflect.Struct{return};items:=v.FieldByName("Items");if !items.IsValid(){items=v.FieldByName("Entries")};if !items.IsValid()||items.Kind()!=reflect.Slice{return}
 // Append, indexed overwrite and map entry writes update one holder. In
 // particular, lexer token arrays must not be rescanned after every append.
 if operation=="push"||operation=="put"||operation=="set" {
  origins:=resultContainerOrigins[owner];index:=-1;contains:=false
  switch operation {
  case "push":if len(args)>0 {contains=resultDirect(args[0])!=0};index=items.Len()-1
  case "put":if len(args)>1 {contains=resultDirect(args[1])!=0;index=int(reflect.ValueOf(args[0]).Int())}
  case "set":if len(args)>1 {contains=resultDirect(args[0])!=0||resultDirect(args[1])!=0};if !contains&&len(origins)==0{return};keys:=v.FieldByName("index");if keys.IsValid()&&len(args)>0 {key:=reflect.ValueOf(args[0]);if !key.IsValid(){key=reflect.Zero(keys.Type().Key())};position:=keys.MapIndex(key);if position.IsValid(){index=int(position.Int())}}
  }
  if index<0||index>=items.Len(){return};if contains{if origins==nil{origins=map[int]string{};resultContainerOrigins[owner]=origins};origins[index]=site}else if origins!=nil{delete(origins,index)};return
 }
 // Removing/reordering slots cannot introduce a new Result provenance.
 // Literal-born slots without an origin keep the documented root fallback.
 if len(resultContainerOrigins[owner])==0&&operation!="unshift"&&operation!="splice3"&&operation!="add" {return}
 // Provenance records direct Result slots only. Skip Result-free element
 // types before shift/splice reconstruction (especially long token arrays).
 itemType:=items.Type().Elem();mayContain:=resultType.AssignableTo(itemType)
 if itemType.Kind()==reflect.Struct {for i:=0;i<itemType.NumField();i++ {field:=itemType.Field(i);if (field.Name=="Key"||field.Name=="Value")&&resultType.AssignableTo(field.Type){mayContain=true}}}
 if !mayContain {return}
 origins:=map[int]string{};for i:=0;i<items.Len();i++ {item:=items.Index(i);if item.Kind()==reflect.Struct{if value:=item.FieldByName("Value");value.IsValid(){item=value}};for item.Kind()==reflect.Interface&&!item.IsNil(){item=item.Elem()};if item.IsValid()&&item.Type()==resultType&&!item.IsNil(){origins[i]=site}}
 if len(origins)>0{resultContainerOrigins[owner]=origins}else{delete(resultContainerOrigins,owner)}
}
func resultDebugSite(site string){resultSite=site}
func resultDebugMutation(site,local string){resultSite=site;resultReceiverLocal=local;resultReceiverFrame=nil;if len(resultFrames)>0{resultReceiverFrame=resultFrames[len(resultFrames)-1]}}
func resultEnter(site string)*resultFrameState{resultFrameID++;f:=&resultFrameState{ID:resultFrameID,Site:site,Roots:map[string]resultRootSlot{}};resultFrames=append(resultFrames,f);return f}
func resultLeave(f *resultFrameState){for i:=len(resultFrames)-1;i>=0;i--{if resultFrames[i]==f{resultFrames=append(resultFrames[:i],resultFrames[i+1:]...);return}}}
func resultRoot(f *resultFrameState,name string,get func()any,created string,last,scope int){f.Roots[name]=resultRootSlot{Name:name,Created:created,Get:get,Last:last,Scope:scope}}
func resultRootValue(f *resultFrameState,name string,slot any,created string,last,scope int){f.Roots[name]=resultRootSlot{Name:name,Created:created,Slot:reflect.ValueOf(slot),Last:last,Scope:scope}}
func resultAt(f *resultFrameState,tick int){f.Tick=tick}
func resultClear(f *resultFrameState,scope int){for k,r:=range f.Roots{if r.Scope>=scope{delete(f.Roots,k)}}}
func resultPublish(v any,site string){budget:=0;resultWalk(reflect.ValueOf(v),site,site,site,map[uintptr]bool{},func(p uintptr,_ string,_ resultHolder){resultPublished[p]=append(resultPublished[p],site)},&budget);if budget>resultWalkLimit{resultUnknownPublish=true}}
// No unsafe pointer conversion: pointers are retained when they are observed
// through instrumented publication sites. Reflection walk callbacks use IDs.
var resultPointers=map[uintptr]*@OBJ@{}
func resultPointer(p uintptr)*@OBJ@{return resultPointers[p]}
func parseResultSlot(s string)uintptr {var p uintptr;fmt.Sscan(s,&p);return p}
var resultType=reflect.TypeOf((*@OBJ@)(nil))
var resultShapes=func()map[string]map[string]bool{var m map[string]map[string]bool;if err:=json.Unmarshal([]byte(@SHAPES@),&m);err!=nil{panic(err)};return m}()
func resultWalk(v reflect.Value,key,path,created string,seen map[uintptr]bool,visit func(uintptr,string,resultHolder),budget *int){
 if !v.IsValid(){return};*budget++;if *budget>resultWalkLimit{return}
 if v.Kind()==reflect.Interface{if !v.IsNil(){resultWalk(v.Elem(),key,path,created,seen,visit,budget)};return}
 if v.Type()==resultType{if !v.IsNil(){if v.CanInterface(){resultPointers[v.Pointer()]=v.Interface().(*@OBJ@)};if origin:=resultFieldOrigins[parseResultSlot(key)];origin!=""{created=origin};visit(v.Pointer(),key,resultHolder{path,created})};return}
 switch v.Kind(){
 case reflect.Pointer:
  if v.IsNil(){return};p:=v.Pointer();if seen[p]{return};seen[p]=true;resultWalk(v.Elem(),fmt.Sprint(p),path,created,seen,visit,budget)
 case reflect.Struct:
  fields,known:=resultShapes[v.Type().Name()];for i:=0;i<v.NumField()&&*budget<=resultWalkLimit;i++{if known&&!fields[v.Type().Field(i).Name]{continue};field:=v.Field(i);k:=key+"."+v.Type().Field(i).Name;if field.CanAddr(){k=fmt.Sprint(field.Addr().Pointer())};if (v.Type().Field(i).Name=="Items"||v.Type().Field(i).Name=="Entries")&&field.Kind()==reflect.Slice&&v.CanAddr(){origins:=resultContainerOrigins[v.Addr().Pointer()];for j:=0;j<field.Len()&&*budget<=resultWalkLimit;j++ {c:=created;if origin:=origins[j];origin!=""{c=origin};item:=field.Index(j);itemKey:=fmt.Sprint(item.Addr().Pointer());resultWalk(item,itemKey,fmt.Sprintf("%s.%s[%d]",path,v.Type().Field(i).Name,j),c,seen,visit,budget)}
 }else{resultWalk(field,k,path+"."+v.Type().Field(i).Name,created,seen,visit,budget)}}
 case reflect.Slice,reflect.Array:
  for i:=0;i<v.Len()&&*budget<=resultWalkLimit;i++{item:=v.Index(i);k:=fmt.Sprintf("%s[%d]",key,i);if item.CanAddr(){k=fmt.Sprint(item.Addr().Pointer())};resultWalk(item,k,fmt.Sprintf("%s[%d]",path,i),created,seen,visit,budget)}
 case reflect.Map:
  if v.IsNil(){return};owner:=uintptr(v.UnsafePointer());if seen[owner]{return};seen[owner]=true;key=fmt.Sprint(owner)
  iter:=v.MapRange();for *budget<=resultWalkLimit&&iter.Next(){keyValue:=iter.Key();for keyValue.Kind()==reflect.Interface&&!keyValue.IsNil(){keyValue=keyValue.Elem()};keyIdentity:=fmt.Sprint(iter.Key());if keyValue.Kind()==reflect.Pointer{keyIdentity=fmt.Sprint(keyValue.Pointer())};k:=fmt.Sprintf("%s[%s]",key,keyIdentity);label:=fmt.Sprintf("%s[%v]",path,iter.Key());if iter.Key().Type()==resultType {label=path+"[Result key]"};resultWalk(iter.Key(),k+".key",label+".key",created,seen,visit,budget);resultWalk(iter.Value(),k+".value",label+".value",created,seen,visit,budget)}
 }
}
func resultCheck(receiver *@OBJ@,mutator string){
 resultChecks++;resultChecksByMutator[mutator]++;resultChecksBySite[resultSite]++;if receiver==nil{return}
 target:=reflect.ValueOf(receiver).Pointer();holders:=map[string]resultHolder{"receiver":{Path:"call receiver",Created:resultSite}};seen:=map[uintptr]bool{};budget:=0
 visit:=func(p uintptr,k string,h resultHolder){if p==target{holders[k]=h}}
 // Shared collection/field addresses are counted once even when multiple
 // stack roots reach them. Distinct local slots remain distinct holders.
 for frameIndex:=len(resultFrames)-1;frameIndex>=0&&budget<=resultWalkLimit;frameIndex-- {f:=resultFrames[frameIndex];names:=make([]string,0,len(f.Roots));for k:=range f.Roots{names=append(names,k)};sort.Strings(names);for _,k:=range names{if budget>resultWalkLimit{break};r:=f.Roots[k];if r.Last<f.Tick{continue};if f==resultReceiverFrame&&strings.Split(k,"[")[0]==resultReceiverLocal{continue};key:=fmt.Sprintf("frame%d.%s",f.ID,k);value:=r.Slot;if value.IsValid(){value=value.Elem()}else{value=reflect.ValueOf(r.Get())};resultWalk(value,key,f.Site+":"+r.Name,r.Created,seen,visit,&budget)}}
 if resultGlobalNames==nil{for k:=range resultGlobals{resultGlobalNames=append(resultGlobalNames,k)};sort.Strings(resultGlobalNames)};for _,k:=range resultGlobalNames{if budget>resultWalkLimit{break};resultWalk(reflect.ValueOf(resultGlobals[k]()),"static."+k,"static."+k,"static declaration",seen,visit,&budget)}
 published:=append([]string(nil),resultPublished[target]...);if resultUnknownPublish{published=append(published,"publication traversal budget exceeded (untracked capture possible)")};if budget>resultWalkLimit{published=append(published,"root traversal budget exceeded (untracked holders possible)")}
 if len(holders)<=1&&len(published)==0{return}
 hs:=make([]resultHolder,0,len(holders));for _,h:=range holders{hs=append(hs,h)};sort.Slice(hs,func(i,j int)bool{return hs[i].Path<hs[j].Path})
 // Aggregate by source and holder provenance, never by unstable addresses.
 var parts []string;for _,h:=range hs{parts=append(parts,resultIndices.ReplaceAllString(h.Path,"[]")+"@"+h.Created)}
 key:=mutator+"@"+resultSite+"|"+strings.Join(parts,"|")+strings.Join(published,"|")
 if f:=resultFindings[key];f!=nil{f.Count++}else{resultFindings[key]=&resultFinding{mutator,resultSite,hs,published,1}}
}
func resultExit(code int){resultReport();os.Exit(code)}
func resultReport(){
 keys:=make([]string,0,len(resultFindings));for k:=range resultFindings{keys=append(keys,k)};sort.Strings(keys);findings:=make([]*resultFinding,0,len(keys));for _,k:=range keys{findings=append(findings,resultFindings[k])}
 out:=struct{Checks,WalkLimit int;ChecksByMutator,ChecksBySite map[string]int;Violations []*resultFinding;Coverage string}{resultChecks,resultWalkLimit,resultChecksByMutator,resultChecksBySite,findings,"active lexical locals (last-use conservative at loops), return temporaries, current reachable collection elements/object fields/statics; closure captures published; traversal budget fail closed"}
 if path:=os.Getenv("ABAPITI_RESULT_REPORT");path!="" {b,err:=json.MarshalIndent(out,"","  ");if err!=nil{panic(err)};if err:=os.WriteFile(path,append(b,'\n'),0644);err!=nil{panic(err)}}else{b,_:=json.Marshal(out);fmt.Fprintln(os.Stderr,string(b))}
}
`
