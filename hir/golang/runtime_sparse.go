package golang

import "strings"

// The legacy dense runtime stays byte-identical for programs that don't use
// presence-sensitive callbacks/deletion. In the presence-aware ABI, a nil hole
// map is dense; undefined is a value, never a hole.
func sparseRuntime(source string) string {
	// Unmatched regex captures are present undefined values, not holes.
	// Preserve their separate Missing metadata when enabling sparse arrays.
	source = strings.Replace(source, "type arrayExtra[T any] struct{", "type arrayExtra[T any] struct{ holes map[int]bool;", 1)
	replace := func(start, end, body string) {
		i := strings.Index(source, start)
		j := strings.Index(source[i:], end) + i
		source = source[:i] + body + "\n" + source[j:]
	}
	replace("func (a *array[T]) get(", "type entry[", `func (a *array[T]) has(i int32) bool {
  return i>=0 && int(i)<len(a.Items) && !a.holeSet()[int(i)]
 }
 func (a *array[T]) get(i int32) optional[T] {
  if !a.has(i) || a.missing()[i] { return optional[T]{} };return present(a.Items[i])
 }
 func (a *array[T]) delete(i int32) bool {
  if i>=0 && int(i)<len(a.Items) {
   x:=a.ext();if x.holes==nil {x.holes=map[int]bool{}}
   x.holes[int(i)]=true;var zero T;a.Items[i]=zero
  };return true
 }
 func (a *array[T]) put(i int32, v T) {
  if i<0 {panic(newRangeFault())}
  if int(i)>len(a.Items) && a.holeSet()==nil {a.ext().holes=map[int]bool{}}
  holes:=a.holeSet()
  for len(a.Items)<=int(i) {if holes!=nil {holes[len(a.Items)]=true};var zero T;a.Items=append(a.Items,zero)}
  a.Items[i]=v;if a.extra!=nil {delete(a.extra.holes,int(i));delete(a.extra.Missing,i)}
 }
 func copyHoles(holes map[int]bool, lo, hi int) map[int]bool {
  if len(holes)==0 {return nil};out:=map[int]bool{}
  for i:=range holes {if i>=lo && i<hi {out[i-lo]=true}}
  return out
 }
 `)
	replace("func (a *array[T]) reverse()", "func (a *array[T]) indexOf", `func (a *array[T]) reverse() *array[T] {
  n:=len(a.Items);holes:=map[int]bool{}
  for i:=range a.holeSet() {holes[n-1-i]=true};a.setHoles(holes)
  for i,j:=0,n-1;i<j;i,j=i+1,j-1 {a.Items[i],a.Items[j]=a.Items[j],a.Items[i]};return a
 }
 func (a *array[T]) unshift(v T) int32 {
  holes:=map[int]bool{};for i:=range a.holeSet() {holes[i+1]=true};a.setHoles(holes)
  a.Items=append([]T{v},a.Items...);return int32(len(a.Items))
 }
 func (a *array[T]) concat(b *array[T]) *array[T] {
  out:=a.slice0();n:=len(out.Items);out.Items=append(out.Items,b.Items...)
  if bh:=b.holeSet();len(bh)>0 {x:=out.ext();if x.holes==nil {x.holes=map[int]bool{}};for i:=range bh {x.holes[n+i]=true}}
  return out
 }
 func (a *array[T]) slice0() *array[T] {return a.slice2(0,int32(len(a.Items)))}
 func (a *array[T]) slice1(i int32) *array[T] {return a.slice2(i,int32(len(a.Items)))}
 func (a *array[T]) slice2(i,j int32) *array[T] {
  n:=int32(len(a.Items));i=sliceIndex(i,n);j=max(i,sliceIndex(j,n))
  out:=&array[T]{Items:append([]T(nil),a.Items[i:j]...)};out.setHoles(copyHoles(a.holeSet(),int(i),int(j)));return out
 }
 func (a *array[T]) splice1(i int32) *array[T] {
  if len(a.holeSet())>0 {return a.splice2(i,int32(len(a.Items)))}
  i=sliceIndex(i,int32(len(a.Items)));out:=&array[T]{}
  if int(i)<len(a.Items) {out.Items=a.Items[i:]}
  if i==0 {a.Items=nil} else {a.Items=a.Items[:i:i]}
  return out
 }
 func (a *array[T]) splice1_view(i int32) *array[T] {return a.splice1(i)}
 func (a *array[T]) splice2(i,n int32) *array[T] {
  i=sliceIndex(i,int32(len(a.Items)));n=max(0,min(n,int32(len(a.Items))-i))
  out:=a.slice2(i,i+n);holes:=map[int]bool{}
  for k:=range a.holeSet() {if k<int(i) {holes[k]=true} else if k>=int(i+n) {holes[k-int(n)]=true}}
  a.Items=append(a.Items[:i],a.Items[i+n:]...);a.setHoles(holes);return out
 }
 func (a *array[T]) spliceDrop2(i,n int32) *array[T] {
  if len(a.holeSet())>0 {a.splice2(i,n);return nil}
  i=sliceIndex(i,int32(len(a.Items)));n=max(0,min(n,int32(len(a.Items))-i))
  a.Items=append(a.Items[:i],a.Items[i+n:]...);return nil
 }
 func (a *array[T]) splice3(i,n int32,v T) *array[T] {
  i=sliceIndex(i,int32(len(a.Items)));out:=a.splice2(i,n);holes:=map[int]bool{}
  for k:=range a.holeSet() {if k<int(i) {holes[k]=true} else {holes[k+1]=true}};a.setHoles(holes)
  a.Items=append(a.Items,v);copy(a.Items[i+1:],a.Items[i:len(a.Items)-1]);a.Items[i]=v;return out
 }
 func (a *array[T]) pop() optional[T] {
  if len(a.Items)==0 {return optional[T]{}};i:=len(a.Items)-1;out:=a.get(int32(i));if a.extra!=nil {delete(a.extra.holes,i)};a.Items=a.Items[:i];return out
 }
 func (a *array[T]) shift() optional[T] {
  if len(a.Items)==0 {return optional[T]{}};out:=a.get(0);a.setHoles(copyHoles(a.holeSet(),1,len(a.Items)));a.Items=a.Items[1:];return out
 }
 `)
	source = strings.Replace(source, "if equal(x, v) {", "if a.has(int32(i)) && equal(x, v) {", 1)
	source = strings.Replace(source, "func (a *array[T]) includes(v T) bool { return a.indexOf(v) >= 0 }", `func (a *array[T]) includes(v T) bool {
 for i,x:=range a.Items {
  if !a.has(int32(i)) {r:=reflect.ValueOf(v);if nilRef(v) || r.Kind()==reflect.Struct && r.FieldByName("Has").IsValid() && !r.FieldByName("Has").Bool() {return true};continue}
  if equal(x,v) {return true}
 };return false
}`, 1)
	source = strings.Replace(source, "b.WriteString(string(primitiveString(v)))", "if a.has(int32(i)) {b.WriteString(string(primitiveString(v)))}", 1)
	// Both view conversions must preserve membership (including boxed undefined).
	source = strings.Replace(source, "out:=&array[T]{Items:make([]T,len(a.Items))}", "out:=&array[T]{Items:make([]T,len(a.Items))};out.setHoles(copyHoles(a.holeSet(),0,len(a.Items)))", 1)
	source = strings.Replace(source, "func referenceArray[T any](a *array[T]) *array[any] {\n\tout := &array[any]{}", "func referenceArray[T any](a *array[T]) *array[any] {\n\tif a==nil {return nil};out := &array[any]{};out.setHoles(copyHoles(a.holeSet(),0,len(a.Items)))", 1)
	source = strings.Replace(source, "for i,v:=range a.Items {out.Items[i]=unwrap(v)}", "for i,v:=range a.Items {if a.has(int32(i)) {out.Items[i]=unwrap(v)}}", 1)
	return source + `
func (a *array[T]) holeSet() map[int]bool {if a.extra==nil {return nil};return a.extra.holes}
// An empty hole set is dense, so it never forces the extra allocation.
func (a *array[T]) setHoles(h map[int]bool) {if a.extra==nil && len(h)==0 {return};a.ext().holes=h}
// Sized construction creates absent slots, not present zero/undefined values.
func sizedArray[T any](n float64) *array[T] {
 if n<0 || n>2147483646 || n!=math.Trunc(n) {panic(newRangeFault())}
 out:=&array[T]{Items:make([]T,int(n)),extra:&arrayExtra[T]{holes:map[int]bool{}}}
 for i:=range out.Items {out.extra.holes[i]=true}
 return out
}
`
}
