package golang

const profileOffSource = `//go:build !profile_sites

package main

const siteProfileEnabled = false
func siteProfileStart(path string) {if path!="" {panic("--profile-sites requires a binary built with -tags profile_sites")}}
func siteProfileRead(name, raw string) {}
func siteProfileFinish() {}
`

const profileRuntimeSource = `//go:build profile_sites

package main

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "hash"
 "os"
 "reflect"
 "sync"
)

const siteProfileEnabled = true

type siteCounter struct {
 Kind string ` + "`json:\"kind\"`" + `
 Calls uint64 ` + "`json:\"calls,omitempty\"`" + `
 Receivers map[string]uint64 ` + "`json:\"receivers,omitempty\"`" + `
 Allocations uint64 ` + "`json:\"allocations,omitempty\"`" + `
 Invocations uint64 ` + "`json:\"invocations,omitempty\"`" + `
 Trips uint64 ` + "`json:\"trips,omitempty\"`" + `
 Histogram map[string]uint64 ` + "`json:\"histogram,omitempty\"`" + `
}
var siteProfile struct {
 sync.Mutex
 counters map[int]*siteCounter
 path string
 input hash.Hash
}
func siteProfileStart(path string) {
 siteProfile.Lock();defer siteProfile.Unlock()
 siteProfile.path=path;siteProfile.input=sha256.New();siteProfile.counters=map[int]*siteCounter{}
}
func siteProfileRead(name,raw string) {
 siteProfile.Lock();defer siteProfile.Unlock()
 if siteProfile.input!=nil {fmt.Fprintf(siteProfile.input,"%d:%s%d:%s",len(name),name,len(raw),raw)}
}
func siteEntry(index int) *siteCounter {
 if siteProfile.counters==nil {siteProfile.counters=map[int]*siteCounter{}}
 c:=siteProfile.counters[index]
 if c==nil {c=&siteCounter{Kind:siteDefinitions[index].Kind};siteProfile.counters[index]=c}
 return c
}
func siteHit(index int,receiver any) {
 siteProfile.Lock();defer siteProfile.Unlock()
 c:=siteEntry(index)
 switch c.Kind {
 case "new":c.Allocations++
 case "call","virtual_call":c.Calls++
 }
 if c.Kind=="virtual_call" {
  name:="<nil>"
  if t:=reflect.TypeOf(receiver);t!=nil {name=siteClasses[t.String()];if name==""{name=t.String()}}
  if c.Receivers==nil {c.Receivers=map[string]uint64{}}
  c.Receivers[name]++
 }
}
func siteLoop(index int,trips uint64) {
 siteProfile.Lock();defer siteProfile.Unlock()
 c:=siteEntry(index);c.Invocations++;c.Trips+=trips
 bucket:="8+"
 switch {case trips==0:bucket="0";case trips==1:bucket="1";case trips<4:bucket="2-3";case trips<8:bucket="4-7"}
 if c.Histogram==nil {c.Histogram=map[string]uint64{"0":0,"1":0,"2-3":0,"4-7":0,"8+":0}}
 c.Histogram[bucket]++
}
func siteProfileFinish() {
 siteProfile.Lock();defer siteProfile.Unlock()
 if siteProfile.path=="" {return}
 path,err:=os.Executable();if err!=nil{panic(err)}
 binary,err:=os.ReadFile(path);if err!=nil{panic(err)};binarySHA:=sha256.Sum256(binary)
 sites:=map[string]*siteCounter{}
 for i,c:=range siteProfile.counters {sites[siteDefinitions[i].ID]=c}
 inputSHA:="";if siteProfile.input!=nil{inputSHA=hex.EncodeToString(siteProfile.input.Sum(nil))}
 report:=struct {
  Schema string ` + "`json:\"schema\"`" + `
  InputSHA string ` + "`json:\"input_sha256\"`" + `
  BinarySHA string ` + "`json:\"binary_sha256\"`" + `
  Sites map[string]*siteCounter ` + "`json:\"sites\"`" + `
 }{"site-profile/1",inputSHA,hex.EncodeToString(binarySHA[:]),sites}
 data,err:=json.MarshalIndent(report,"","  ");if err!=nil{panic(err)}
 if err:=os.WriteFile(siteProfile.path,append(data,'\n'),0644);err!=nil{panic(err)}
}
`
