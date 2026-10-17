package golang

// dynamicSearchSource implements indexOf/includes on any[] receivers. Boxes
// compare by JavaScript value, not by box identity: indexOf uses strict
// equality and skips absent slots; includes uses SameValueZero, so NaN
// matches NaN and an absent slot reads as undefined.
const dynamicSearchSource = `
func dynArrayIndexOf(a *array[any], v any) int32 {
 for i:=int32(0);i<LENGTH;i++ {x:=a.get(i);if x.Has && dynEqual(box(x.Value),box(v)) {return i}};return -1
}
func dynArrayIncludes(a *array[any], v any) bool {
 for i:=int32(0);i<LENGTH;i++ {if dynSameValueZero(box(a.get(i).Value),box(v)) {return true}};return false
}
func dynSameValueZero(a, b *dynamic) bool {
 if dynEqual(a,b) {return true}
 return a!=nil && b!=nil && a.Tag==6 && b.Tag==6 && math.IsNaN(a.Value.(float64)) && math.IsNaN(b.Value.(float64))
}
`
