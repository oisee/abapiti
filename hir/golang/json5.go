package golang

// JSON5 is separate from strict JSON: neither operation silently widens the other.
// The callback exposes the pinned package's diagnostic; the emitted operation
// refuses SyntaxError payload ABI rather than throwing a fake native Error.
const json5RuntimeSource = `
type json5Parser struct {
 input jsString
 cursor, line, column int32
 nonfinite bool
 onSyntaxError func(jsString)
}
func (p *json5Parser) peek() int32 {
 if p.cursor >= p.input.length() {return -1}; return p.input.charCodeAt(p.cursor)
}
func (p *json5Parser) read() int32 {
 c:=p.peek(); if c==10 {p.line++;p.column=0} else {p.column++}
 if c>=0 {p.cursor++}; return c
}
func (p *json5Parser) errorMessage(message string) {
 p.onSyntaxError(str(message+" at "+strconv.Itoa(int(p.line))+":"+strconv.Itoa(int(p.column))))
 panic(newTrap("JSON5 syntax callback returned"))
}
func (p *json5Parser) invalid() {
 c:=p.read()
 if c<0 {p.errorMessage("JSON5: invalid end of input")}
 text:=unit(c).String()
 switch c {
 case 39:text="\\'"
 case 34:text="\\\""
 case 92:text="\\\\"
 case 8:text="\\b"
 case 12:text="\\f"
 case 10:text="\\n"
 case 13:text="\\r"
 case 9:text="\\t"
 case 11:text="\\v"
 case 0:text="\\0"
 case 0x2028:text="\\u2028"
 case 0x2029:text="\\u2029"
 default:
  if c<32 {h:=strconv.FormatInt(int64(c),16);if len(h)<2 {h="0"+h};text="\\x"+h}
  if c>=0xd800 && c<=0xdbff && p.peek()>=0xdc00 && p.peek()<=0xdfff {text=(unit(c)+unit(p.read())).String()}
 }
 p.errorMessage("JSON5: invalid character '"+text+"'")
}
func (p *json5Parser) take(c int32) {if p.peek()!=c {p.invalid()};p.read()}
func json5Space(c int32) bool {
 switch c {case 9,10,11,12,13,32,0xa0,0x1680,0x2028,0x2029,0x202f,0x205f,0x3000,0xfeff:return true}
 return c>=0x2000 && c<=0x200a
}
func (p *json5Parser) space() {
 for {
  if json5Space(p.peek()) {p.read();continue}
  if p.peek()!=47 {return};p.read()
  switch p.peek() {
  case 47:
   p.read();for p.peek()>=0 && p.peek()!=10 && p.peek()!=13 && p.peek()!=0x2028 && p.peek()!=0x2029 {p.read()}
  case 42:
   p.read();for {if p.peek()<0 {p.invalid()}; c:=p.read();if c==42 && p.peek()==47 {p.read();break}}
  default:p.invalid()
  }
 }
}
func json5Digit(c int32) bool {return c>=48 && c<=57}
func json5Hex(c int32) int32 {
 switch {case c>=48 && c<=57:return c-48;case c>=65 && c<=70:return c-55;case c>=97 && c<=102:return c-87};return -1
}
func (p *json5Parser) hex(count int) int32 {
 v:=int32(0);for j:=0;j<count;j++ {d:=json5Hex(p.peek());if d<0 {p.invalid()};p.read();v=v*16+d};return v
}
func (p *json5Parser) text() jsString {
 quote:=p.read();out:=jsString("")
 for {
  c:=p.peek();if c==quote {p.read();return out}
  if c<0 || c==10 || c==13 {p.invalid()}
  p.read()
  if c!=92 {out+=unit(c);continue}
  c=p.peek()
  switch c {
  case -1:p.invalid()
  case 48:
   p.read();if json5Digit(p.peek()) {p.invalid()};out+=unit(0)
  case 49,50,51,52,53,54,55,56,57:p.invalid()
  case 98:p.read();out+=unit(8)
  case 102:p.read();out+=unit(12)
  case 110:p.read();out+=unit(10)
  case 114:p.read();out+=unit(13)
  case 116:p.read();out+=unit(9)
  case 118:p.read();out+=unit(11)
  case 120:p.read();out+=unit(p.hex(2))
  case 117:p.read();out+=unit(p.hex(4))
  case 10,0x2028,0x2029:p.read()
  case 13:p.read();if p.peek()==10 {p.read()}
  default:out+=unit(p.read())
  }
 }
}
func json5ID(c int32, first bool) bool {
 if c==36 || c==95 || c>=65 && c<=90 || c>=97 && c<=122 {return true}
 if !first && (json5Digit(c) || c==0x200c || c==0x200d) {return true}
 ranges:=json5IDStart;if !first {ranges=json5IDContinue}
 lo,hi:=0,len(ranges)
 for lo<hi {mid:=lo+(hi-lo)/2;r:=ranges[mid];if c<r[0] {hi=mid} else if c>r[1] {lo=mid+1} else {return true}}
 return false
}
func (p *json5Parser) codePoint() int32 {
 c:=p.peek();if c>=0xd800 && c<=0xdbff && p.cursor+1<p.input.length() {low:=p.input.charCodeAt(p.cursor+1);if low>=0xdc00 && low<=0xdfff {return 0x10000+(c-0xd800)*1024+low-0xdc00}};return c
}
func (p *json5Parser) key() jsString {
 if p.peek()==34 || p.peek()==39 {return p.text()}
 out:=jsString("");first:=true
 for {
  c:=p.peek()
  if c==92 {
   p.read();p.take(117);c=p.hex(4)
   if !json5ID(c,first) {p.column-=5;p.errorMessage("JSON5: invalid identifier character")}
  } else {
   c=p.codePoint()
   if !json5ID(c,first) {if first {p.invalid()};return out};p.read();if c>0xffff {p.read()}
  }
  if c>0xffff {out+=str(string(rune(c)))} else {out+=unit(c)};first=false
 }
}
func (p *json5Parser) literal(s string) {for _,c:=range s {p.take(int32(c))}}
func (p *json5Parser) number() *dynamic {
 sign:=1.0;if p.peek()==43 || p.peek()==45 {if p.read()==45 {sign=-1}}
 if p.peek()==73 {p.literal("Infinity");p.nonfinite=true;return box(float64(0))}
 if p.peek()==78 {p.literal("NaN");p.nonfinite=true;return box(float64(0))}
 start:=p.cursor;hex:=false
 if p.peek()==48 {
  p.read();if p.peek()==120 || p.peek()==88 {hex=true;p.read();if json5Hex(p.peek())<0 {p.invalid()};for json5Hex(p.peek())>=0 {p.read()}}
 } else if json5Digit(p.peek()) {for json5Digit(p.peek()) {p.read()}} else if p.peek()!=46 {p.invalid()}
 if !hex {
  if p.peek()==46 {
   leading:=p.cursor==start;p.read();if leading && !json5Digit(p.peek()) {p.invalid()};for json5Digit(p.peek()) {p.read()}
  }
  if p.peek()==101 || p.peek()==69 {
   p.read();if p.peek()==43 || p.peek()==45 {p.read()};if !json5Digit(p.peek()) {p.invalid()};for json5Digit(p.peek()) {p.read()}
  }
 }
 text:=p.input[2*start:2*p.cursor].String();if hex {text+="p0"}
 v,err:=strconv.ParseFloat(text,64)
 if math.IsInf(v,0) || math.IsNaN(v) {p.nonfinite=true;return box(float64(0))}
 if err!=nil {panic(newTrap("JSON5 numeric conversion domain is unsupported"))}
 return box(sign*v)
}
func (p *json5Parser) value() *dynamic {
 p.space()
 switch p.peek() {
 case 34,39:return box(p.text())
 case 110:p.literal("null");return dynNull()
 case 116:p.literal("true");return box(true)
 case 102:p.literal("false");return box(false)
 case 123,91:
  arrayMode:=p.read()==91;close:=int32(125);if arrayMode {close=93}
  d:=&dynamic{Tag:tagObject,Value:&orderedMap[jsString,*dynamic]{}};a:=&array[any]{}
  if arrayMode {d=&dynamic{tagArray,a}}
  p.space();if p.peek()==close {p.read();return d}
  for {
   if arrayMode {a.push(p.value())} else {p.space();k:=p.key();p.space();p.take(58);d.put(k,p.value())}
   p.space();if p.peek()==close {p.read();return d};p.take(44);p.space();if p.peek()==close {p.read();return d}
  }
 default:return p.number()
 }
}
func parseJSON5(input jsString,onSyntaxError func(jsString)) *dynamic {
 p:=&json5Parser{input:input,line:1,onSyntaxError:onSyntaxError}
 d:=p.value();p.space();if p.peek()!=-1 {p.invalid()}
 if p.nonfinite {panic(newTrap("JSON5 nonfinite Number representation is unsupported"))}
 return d
}
`
