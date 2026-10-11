package golang

import "strings"

// The legacy dense runtime stays byte-identical for programs that don't use
// presence-sensitive callbacks/deletion. In the presence-aware ABI, a nil hole
// map is dense; undefined is a value, never a hole.
func sparseRuntime(source string) string {
	// Regex absent captures and sparse-array holes share membership semantics.
	source = strings.ReplaceAll(source, "Missing map[int32]bool", "holes map[int]bool")
	source = strings.ReplaceAll(source, "out.Missing", "out.holes")
	source = strings.ReplaceAll(source, "out.holes=map[int32]bool{}", "out.holes=map[int]bool{}")
	source = strings.ReplaceAll(source, "out.holes[int32(len(out.Items))]", "out.holes[len(out.Items)]")
	replace := func(start, end, body string) {
		i := strings.Index(source, start)
		j := strings.Index(source[i:], end) + i
		source = source[:i] + body + "\n" + source[j:]
	}
	replace("func (a *array[T]) get(", "type entry[", `func (a *array[T]) has(i int32) bool {
  return i>=0 && int(i)<len(a.Items) && !a.holes[int(i)]
 }
 func (a *array[T]) get(i int32) optional[T] {
  if !a.has(i) { return optional[T]{} };return present(a.Items[i])
 }
 func (a *array[T]) delete(i int32) bool {
  if i>=0 && int(i)<len(a.Items) {
   if a.holes==nil {a.holes=map[int]bool{}}
   a.holes[int(i)]=true;var zero T;a.Items[i]=zero
  };return true
 }
 func (a *array[T]) put(i int32, v T) {
  if i<0 {panic(newRangeFault())}
  if int(i)>len(a.Items) && a.holes==nil {a.holes=map[int]bool{}}
  for len(a.Items)<=int(i) {a.holes[len(a.Items)]=true;var zero T;a.Items=append(a.Items,zero)}
  a.Items[i]=v;delete(a.holes,int(i))
 }
 func copyHoles(holes map[int]bool, lo, hi int) map[int]bool {
  if len(holes)==0 {return nil};out:=map[int]bool{}
  for i:=range holes {if i>=lo && i<hi {out[i-lo]=true}}
  return out
 }
 `)
	// put at the tail must not assign into a nil map.
	source = strings.Replace(source, "a.holes[len(a.Items)]=true;var zero T", "if a.holes!=nil {a.holes[len(a.Items)]=true};var zero T", 1)
	replace("func (a *array[T]) reverse()", "func (a *array[T]) indexOf", `func (a *array[T]) reverse() *array[T] {
  n:=len(a.Items);holes:=map[int]bool{}
  for i:=range a.holes {holes[n-1-i]=true};a.holes=holes
  for i,j:=0,n-1;i<j;i,j=i+1,j-1 {a.Items[i],a.Items[j]=a.Items[j],a.Items[i]};return a
 }
 func (a *array[T]) unshift(v T) int32 {
  holes:=map[int]bool{};for i:=range a.holes {holes[i+1]=true};a.holes=holes
  a.Items=append([]T{v},a.Items...);return int32(len(a.Items))
 }
 func (a *array[T]) concat(b *array[T]) *array[T] {
  out:=a.slice0();n:=len(out.Items);out.Items=append(out.Items,b.Items...)
  if len(b.holes)>0 && out.holes==nil {out.holes=map[int]bool{}}
  for i:=range b.holes {out.holes[n+i]=true};return out
 }
 func (a *array[T]) slice0() *array[T] {return a.slice2(0,int32(len(a.Items)))}
 func (a *array[T]) slice1(i int32) *array[T] {return a.slice2(i,int32(len(a.Items)))}
 func (a *array[T]) slice2(i,j int32) *array[T] {
  n:=int32(len(a.Items));i=sliceIndex(i,n);j=max(i,sliceIndex(j,n))
  return &array[T]{Items:append([]T(nil),a.Items[i:j]...),holes:copyHoles(a.holes,int(i),int(j))}
 }
 func (a *array[T]) splice1(i int32) *array[T] {return a.splice2(i,int32(len(a.Items)))}
 func (a *array[T]) splice1_view(i int32) *array[T] {return a.splice1(i)}
 func (a *array[T]) splice2(i,n int32) *array[T] {
  i=sliceIndex(i,int32(len(a.Items)));n=max(0,min(n,int32(len(a.Items))-i))
  out:=a.slice2(i,i+n);holes:=map[int]bool{}
  for k:=range a.holes {if k<int(i) {holes[k]=true} else if k>=int(i+n) {holes[k-int(n)]=true}}
  a.Items=append(a.Items[:i],a.Items[i+n:]...);a.holes=holes;return out
 }
 func (a *array[T]) splice3(i,n int32,v T) *array[T] {
  i=sliceIndex(i,int32(len(a.Items)));out:=a.splice2(i,n);holes:=map[int]bool{}
  for k:=range a.holes {if k<int(i) {holes[k]=true} else {holes[k+1]=true}};a.holes=holes
  a.Items=append(a.Items,v);copy(a.Items[i+1:],a.Items[i:len(a.Items)-1]);a.Items[i]=v;return out
 }
 func (a *array[T]) pop() optional[T] {
  if len(a.Items)==0 {return optional[T]{}};i:=len(a.Items)-1;out:=a.get(int32(i));delete(a.holes,i);a.Items=a.Items[:i];return out
 }
 func (a *array[T]) shift() optional[T] {
  if len(a.Items)==0 {return optional[T]{}};out:=a.get(0);a.holes=copyHoles(a.holes,1,len(a.Items));a.Items=a.Items[1:];return out
 }
 `)
	source = strings.Replace(source, "if equal(x, v) {", "if a.has(int32(i)) && equal(x, v) {", 1)
	source = strings.Replace(source, "func (a *array[T]) includes(v T) bool { return a.indexOf(v) >= 0 }", `func (a *array[T]) includes(v T) bool {
 for i,x:=range a.Items {
  if !a.has(int32(i)) {r:=reflect.ValueOf(v);if nilRef(v) || r.Kind()==reflect.Struct && r.FieldByName("Has").IsValid() && !r.FieldByName("Has").Bool() {return true};continue}
  if equal(x,v) {return true}
 };return false
}`, 1)
	source = strings.Replace(source, "out += primitiveString(v)", "if a.has(int32(i)) {out += primitiveString(v)}", 1)
	// Both view conversions must preserve membership (including boxed undefined).
	source = strings.Replace(source, "out:=&array[T]{Items:make([]T,len(a.Items))}", "out:=&array[T]{Items:make([]T,len(a.Items)),holes:copyHoles(a.holes,0,len(a.Items))}", 1)
	source = strings.Replace(source, "func referenceArray[T any](a *array[T]) *array[any] {\n\tout := &array[any]{}", "func referenceArray[T any](a *array[T]) *array[any] {\n\tif a==nil {return nil};out := &array[any]{holes:copyHoles(a.holes,0,len(a.Items))}", 1)
	source = strings.Replace(source, "for i,v:=range a.Items {out.Items[i]=unwrap(v)}", "for i,v:=range a.Items {if a.has(int32(i)) {out.Items[i]=unwrap(v)}}", 1)
	return source
}
