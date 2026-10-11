package golang

// Over the certified non-u regular language, full-subject leftmost-first
// enumeration is equivalent to @@split's private sticky clone: failed sticky
// positions advance by one UTF-16 unit, and empty matches at the preceding
// separator end are suppressed. unitText makes every rune one UTF-16 unit.
// Unlike exec's start-prefix wrapper, this has no bounded-repeat cursor limit.
const regexSplitRuntimeSource = `
func (s jsString) splitRegex(pattern *jsRegExp, limit float64) *array[jsString] {
 out:=&array[jsString]{}
 lim:=float64(0)
 if !math.IsNaN(limit) && !math.IsInf(limit,0) {
  lim=math.Mod(math.Trunc(limit),4294967296)
  if lim<0 {lim+=4294967296}
 }
 if lim==0 {return out}
 // Older exec reviews do not certify retained repeated captures for split.
 if pattern.compiled.NumSubexp()>0 {
  if _,ok:=TranslateSubset(pattern.Source.String(),pattern.Flags.String());!ok {
   panic(newTrap("regexp split captures outside the checked capture-safe grammar"))
  }
 }
 input:=unitText(s)
 denied:=pattern.denied!=nil && pattern.denied.MatchString(input)
 if s.length()==0 {
  if denied || !pattern.compiled.MatchString(input) {out.Items=append(out.Items,s)}
  return out
 }
 p:=int32(0)
 if !denied {
  for _,m:=range pattern.compiled.FindAllStringSubmatchIndex(input,-1) {
   q:=int32(utf8.RuneCountInString(input[:m[0]]))
   end:=int32(utf8.RuneCountInString(input[:m[1]]))
   // JS never starts a sticky split attempt at the final unit boundary.
   if q==s.length() {break}
   if end==p {continue}
   out.Items=append(out.Items,s[2*p:2*q])
   if float64(len(out.Items))==lim {return out}
   p=end
   for i:=2;i<len(m);i+=2 {
    if m[i]<0 {panic(newTrap("regexp split unmatched capture requires a nullable string-element ABI"))}
    a:=int32(utf8.RuneCountInString(input[:m[i]]));b:=int32(utf8.RuneCountInString(input[:m[i+1]]))
    out.Items=append(out.Items,s[2*a:2*b])
    if float64(len(out.Items))==lim {return out}
   }
  }
 }
 out.Items=append(out.Items,s[2*p:])
 return out
}
`
