package golang

// Delimiter-only String.split: captures and zero-width matches require a
// separate JS splitting algorithm and are deliberately refused, not omitted.
// Source is usage-gated so default deployment output does not gain helpers.
const splitRegexRuntimeSource = `
func (s jsString) splitRegex(r *jsRegExp) *array[jsString] {
 if r.compiled.NumSubexp()!=0 {panic(newTrap("regexp split capturing delimiters are unsupported"))}
 input:=unitText(s)
 out:=&array[jsString]{}
 if r.denied!=nil && r.denied.MatchString(input) {out.Items=append(out.Items,s);return out}
 matches:=r.compiled.FindAllStringIndex(input,-1)
 // Check the complete match list before constructing a partial answer.
 for _,m:=range matches {if m[0]==m[1] {panic(newTrap("regexp split empty-match delimiters are unsupported"))}}
 last:=int32(0)
 for _,m:=range matches {
  a:=int32(utf8.RuneCountInString(input[:m[0]]));b:=int32(utf8.RuneCountInString(input[:m[1]]))
  out.Items=append(out.Items,s[2*last:2*a]);last=b
 }
 out.Items=append(out.Items,s[2*last:])
 return out
}
`
