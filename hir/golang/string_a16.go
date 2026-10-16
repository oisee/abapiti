package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/jsregex"
)

// New helpers are usage-gated to preserve the default deployment sources.
func (e *emitter) stringA16Runtime(runtime string) string {
	ops := hir.UsedRuntimeOps(e.p)
	if _, ok := ops["string.padEnd"]; ok {
		runtime += padEndSource
	}
	if _, ok := ops["string.relativeAt"]; ok {
		runtime += relativeAtSource
	}
	_, iterator := ops["regexp.clone"]
	_, groups := ops["array.matchGroups"]
	if iterator || groups || e.regexA16 {
		runtime = strings.Replace(runtime, "type jsRegExp struct {", "type jsRegExp struct { labelGuard bool;", 1)
		runtime = strings.Replace(runtime, "denied:denied}", "denied:denied,labelGuard:p=="+strconv.Quote(jsregex.IteratorLabelPattern)+"}", 1)
		runtime = strings.Replace(runtime, "HasMatchIndex bool; Missing", "HasMatchIndex bool; MatchGroups *dynamic; Missing", 1)
		start := strings.Index(runtime, "func (r *jsRegExp) exec(s jsString)")
		end := strings.Index(runtime[start:], "func (r *jsRegExp) test(") + start
		runtime = runtime[:start] + regexpExecA16Source + runtime[end:]
		start = strings.Index(runtime, "func (s jsString) match(r *jsRegExp)")
		end = strings.Index(runtime[start:], "func (r *jsRegExp) toString()") + start
		runtime = runtime[:start] + regexpMatchA16Source + runtime[end:]
		start = strings.Index(runtime, "func (s jsString) replaceRegex(")
		end = strings.Index(runtime[start:], "func (s jsString) match(") + start
		runtime = runtime[:start] + regexpReplaceA16Source + runtime[end:]
		if !e.regexSplit {
			runtime = strings.Replace(runtime, `"unicode/utf8"`, "", 1)
		}
		runtime += matchAllSource
	}
	return runtime
}

const padEndSource = `
func (s jsString) padEnd(n float64, fill jsString) jsString {
 if math.IsNaN(n)||n<=0||math.Trunc(n)<=float64(s.length())||fill.length()==0{return s}
 n=math.Trunc(n)
 if math.IsInf(n,0)||n>1048576{panic(newTrap("padEnd allocation exceeds the supported UTF-16 length domain"))}
 needed:=int32(n)-s.length()
 count:=(int64(needed)+int64(fill.length())-1)/int64(fill.length())
 padding:=jsString(strings.Repeat(string(fill),int(count)))
 return s+padding[:2*needed]
}
`
const relativeAtSource = `
func (s jsString) relativeAt(n float64) optional[jsString] {
 if math.IsNaN(n){n=0};n=math.Trunc(n)
 if n<0{n+=float64(s.length())}
 if n<0||n>=float64(s.length()){return optional[jsString]{}}
 return present(s.charAt(int32(n)))
}
`

// Build a byte-to-UTF-16 offset table, including the end position. In u mode
// pairs form one code point; unmatched surrogate units remain one unit. The
// certified ASCII pattern grammar never compares surrogate literal values.
const regexpExecA16Source = `
func (r *jsRegExp) exec(s jsString) *array[jsString] {
 stateful:=r.Flags.indexOf(str("g"))>=0||r.Flags.indexOf(str("y"))>=0
 start:=int32(0);if stateful{start=max(0,r.LastIndex)}
 if start>s.length(){if stateful{r.LastIndex=0};return nil}
 unicodeMode:=r.Flags.indexOf(str("u"))>=0
 var text strings.Builder
 offsets:=map[int]int32{}
 cursor:=0
 for pos:=int32(0);pos<s.length();pos++ {
  offsets[text.Len()]=pos
  unit:=rune(s.charCodeAt(pos))
  if unicodeMode&&unit>=0xd800&&unit<=0xdbff&&pos+1<s.length(){low:=rune(s.charCodeAt(pos+1));if low>=0xdc00&&low<=0xdfff{unit=utf16.DecodeRune(unit,low);if pos<start&&start==pos+1{start=pos};pos++}}
  if pos<start{cursor++}
  text.WriteRune(unit)
 }
 offsets[text.Len()]=s.length()
 input:=text.String()
 if r.denied!=nil&&r.denied.MatchString(input){if stateful{r.LastIndex=0};return nil}
 re:=r.compiled
 wrapped:=start>0||r.Flags.indexOf(str("y"))>=0
 if wrapped {
  // RE2 bounds counted repetitions at 1000. Concatenated fixed blocks
  // retain anchors against the original input without that cursor limit.
  prefix:="";for count:=cursor;count>0;{n:=min(count,1000);prefix+="(?s:.{"+strconv.Itoa(n)+"})";count-=n}
  scan:="(?s:.*?)";if r.Flags.indexOf(str("y"))>=0{scan=""}
  re=regexp.MustCompile("\\A"+prefix+scan+"("+re.String()+")")
 }
 m:=re.FindStringSubmatchIndex(input)
 if m==nil{if stateful{r.LastIndex=0};return nil}
 if wrapped{m=m[2:]}
 if r.labelGuard{m=m[2:]}
 if r.Source==str("^(-?)0+(?=\\d)"){m[1]--;m=m[:len(m)-2]}
 out:=&array[jsString]{HasMatchIndex:true,MatchIndex:offsets[m[0]]}
 if stateful{r.LastIndex=offsets[m[1]]}
 names:=r.compiled.SubexpNames()
 if r.labelGuard{names=names[1:]}
 var groups *orderedMap[jsString,*dynamic]
 for j,name:=range names {if j>0&&name!=""{if groups==nil{groups=&orderedMap[jsString,*dynamic]{}};var value *dynamic;if m[2*j]>=0{value=box(s[2*offsets[m[2*j]]:2*offsets[m[2*j+1]]])};groups.set(str(name),value)}}
 if groups!=nil{out.MatchGroups=&dynamic{Tag:tagObject,Value:groups}}
 for j:=0;j<len(m);j+=2{
  if m[j]<0{if out.Missing==nil{out.Missing=map[int32]bool{}};out.Missing[int32(len(out.Items))]=true;out.Items=append(out.Items,str(""));continue}
  out.Items=append(out.Items,s[2*offsets[m[j]]:2*offsets[m[j+1]]])
 }
 return out
}
`
const matchAllSource = `
func cloneRegexp(r *jsRegExp) *jsRegExp {clone:=*r;clone.LastIndex=max(0,clone.LastIndex);return &clone}
func (r *jsRegExp) matchAllNext(s jsString) *array[jsString] {
 m:=r.exec(s)
 if m!=nil&&m.Items[0].length()==0 {
  index:=r.LastIndex
  if r.Flags.indexOf(str("u"))>=0&&index+1<s.length(){a,b:=s.charCodeAt(index),s.charCodeAt(index+1);if a>=0xd800&&a<=0xdbff&&b>=0xdc00&&b<=0xdfff{index++}}
  r.LastIndex=index+1
 }
 return m
}
`

// All consumers of newly admitted Unicode patterns share exec's code-point
// offsets and empty-match advancement, rather than applying RE2 to UTF-16 units.
const regexpMatchA16Source = `
func (s jsString) match(r *jsRegExp) *array[jsString] {
 if r.Flags.indexOf(str("g"))<0{return r.exec(s)}
 r.LastIndex=0
 var out *array[jsString]
 for m:=r.matchAllNext(s);m!=nil;m=r.matchAllNext(s){if out==nil{out=&array[jsString]{}};out.Items=append(out.Items,m.Items[0])}
 return out
}
`
const regexpReplaceA16Source = `
func (s jsString) replaceRegex(r *jsRegExp,v jsString) jsString {
 global:=r.Flags.indexOf(str("g"))>=0
 if global{r.LastIndex=0}
 var names map[string]int
 for i,name:=range r.compiled.SubexpNames(){if name!=""{if names==nil{names=map[string]int{}};names[name]=i}}
 out:=jsString("");last:=int32(0)
 for {
  var m *array[jsString];if global{m=r.matchAllNext(s)}else{m=r.exec(s)}
  if m==nil{break}
  start:=m.MatchIndex;end:=start+m.Items[0].length()
  out+=s[2*last:2*start]+getSubstitution(s,v,start,end,m.Items,names);last=end
  if !global{break}
 }
 return out+s[2*last:]
}
`
