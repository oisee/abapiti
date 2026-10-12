package golang

// Tagged records have certified ordinary Object storage. Own coercion hooks or
// prototype-affecting keys leave this domain; no native class uses this fallback.
const recordStringSource = `
func recordString(d *dynamic) jsString {
 if d!=nil && d.Tag==tagObject {
  bag:=d.Value.(*orderedMap[jsString,*dynamic])
  for _,key:=range []string{"toString","valueOf","__proto__"} {
   if bag.has(str(key)) {panic(newTrap("record string conversion requires the original default Object coercion hooks"))}
  }
  return str("[object Object]")
 }
 return dynToString(d)
}
`
