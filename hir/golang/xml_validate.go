package golang

// Default-options port of fast-xml-parser 5.10.1 src/validator.js and its
// name grammar. Retains upstream acceptance quirks and UTF-16 positions.
// Copyright (c) 2017 Amit Kumar Gupta; MIT (see testdata/xml-validator/LICENSE).
const xmlValidationRuntimeSource = `
type xmlValidation struct { input jsString }
type xmlOpenTag struct { name jsString; pos int32 }
func (v xmlValidation) at(i int32) int32 { if i<0 || i>=v.input.length() {return -1};return v.input.charCodeAt(i) }
func xmlWhite(c int32) bool {return c==32 || c==9 || c==10 || c==13}
func xmlJSWhite(c int32) bool {return xmlWhite(c)||c==11||c==12||c==160||c==0x1680||c>=0x2000&&c<=0x200a||c==0x2028||c==0x2029||c==0x202f||c==0x205f||c==0x3000||c==0xfeff}
func xmlNameStart(c int32) bool {
 return c==58||c==95||c>=65&&c<=90||c>=97&&c<=122||c>=0xc0&&c<=0xd6||c>=0xd8&&c<=0xf6||c>=0xf8&&c<=0x2ff||c>=0x370&&c<=0x37d||c>=0x37f&&c<=0x1fff||c>=0x200c&&c<=0x200d||c>=0x2070&&c<=0x218f||c>=0x2c00&&c<=0x2fef||c>=0x3001&&c<=0xd7ff||c>=0xf900&&c<=0xfdcf||c>=0xfdf0&&c<=0xfffd
}
func xmlValidName(s jsString) bool {
 if s.length()==0 || !xmlNameStart(s.charCodeAt(0)) {return false}
 for i:=int32(1);i<s.length();i++ {c:=s.charCodeAt(i);if !xmlNameStart(c)&&c!=45&&c!=46&&!(c>=48&&c<=57)&&c!=0xb7&&!(c>=0x300&&c<=0x36f)&&!(c>=0x203f&&c<=0x2040) {return false}}
 return true
}
func (v xmlValidation) position(index int32) (int32,int32) {
 if index>v.input.length() {index=v.input.length()};line,col:=int32(1),int32(1)
 for i:=int32(0);i<index;i++ {if v.at(i)==10 {line++;col=1} else {col++}}
 return line,col
}
func xmlValidationError(code,msg string,line,col int32) *dynamic {
 details:=&orderedMap[jsString,*dynamic]{}
 details.set(str("code"),box(str(code)));details.set(str("msg"),box(str(msg)));details.set(str("line"),box(float64(line)))
 if col>0 {details.set(str("col"),box(float64(col)))} else {details.set(str("col"),nil)}
 result:=&orderedMap[jsString,*dynamic]{};result.set(str("err"),&dynamic{Tag:tagObject,Value:details});return &dynamic{Tag:tagObject,Value:result}
}
func (v xmlValidation) failure(code,msg string,pos int32) *dynamic {line,col:=v.position(pos);return xmlValidationError(code,msg,line,col)}
func (v xmlValidation) pi(i int32) (int32,*dynamic) {
 start:=i
 for ;i<v.input.length();i++ {if v.at(i)==63||v.at(i)==32 {
  name:=v.input[2*start:2*i]
  if i>5&&name==str("xml") {return i,v.failure("InvalidXml","XML declaration allowed only at the start of the document.",i)}
  if v.at(i)==63&&v.at(i+1)==62 {return i+1,nil}
 }}
 return i,nil
}
func (v xmlValidation) comment(i int32) int32 {
 length:=v.input.length()
 if length>i+5&&v.at(i+1)==45&&v.at(i+2)==45 {
  for i+=3;i<length;i++ {if v.at(i)==45&&v.at(i+1)==45&&v.at(i+2)==62 {return i+2}}
 } else if length>i+8&&v.input[2*(i+1):2*(i+8)]==str("DOCTYPE") {
  depth:=1;for i+=8;i<length;i++ {if v.at(i)==60 {depth++} else if v.at(i)==62 {depth--;if depth==0 {return i}}}
 } else if length>i+9&&v.input[2*(i+1):2*(i+8)]==str("[CDATA[") {
  for i+=8;i<length;i++ {if v.at(i)==93&&v.at(i+1)==93&&v.at(i+2)==62 {return i+2}}
 }
 return i
}
type xmlAttrError struct {msg string;pos int32}
// Scanner for (\s*)([^\s=]+)(\s*=)?(\s*(['"])([\s\S]*?)\5)?.
// Like the upstream unanchored global regexp, unmatched '=' is skipped.
func xmlValidateAttrs(s jsString) *xmlAttrError {
 names:=map[jsString]bool{};v:=xmlValidation{s}
 for i:=int32(0);i<s.length(); {
  start:=i;for xmlJSWhite(v.at(i)) {i++};spaces:=i-start
  if i>=s.length() {break};if v.at(i)==61 {i++;continue}
  nameStart:=i;for i<s.length()&&!xmlJSWhite(v.at(i))&&v.at(i)!=61 {i++};name:=s[2*nameStart:2*i]
  afterName:=i;for xmlJSWhite(v.at(i)) {i++};equal:=v.at(i)==61
  if equal {i++} else {i=afterName}
  afterEqual:=i;for xmlJSWhite(v.at(i)) {i++};quote:=v.at(i);value:=false
  if quote==34||quote==39 {j:=i+1;for j<s.length()&&v.at(j)!=quote {j++};if j<s.length() {value=true;i=j+1} else {i=afterEqual}} else {i=afterEqual}
  msg:="";n:=name.String()
  if spaces==0 {msg="Attribute '"+n+"' has no space in starting."} else if equal&&!value {msg="Attribute '"+n+"' is without value."} else if !equal {msg="boolean attribute '"+n+"' is not allowed."} else if !xmlValidName(name) {msg="Attribute '"+n+"' is an invalid name."} else if names[name] {msg="Attribute '"+n+"' is repeated."}
  if msg!="" {return &xmlAttrError{msg,nameStart}};names[name]=true
 }
 return nil
}
func (v xmlValidation) amp(i int32) int32 {
 i++;if v.at(i)==59 {return -1}
 if v.at(i)==35 {i++;hex:=v.at(i)==120;if hex {i++};for ;i<v.input.length();i++ {c:=v.at(i);if c==59 {return i};if !(c>=48&&c<=57||hex&&(c>=97&&c<=102||c>=65&&c<=70)) {break}};return -1}
 count:=0;for ;i<v.input.length();i++ {c:=v.at(i);if (c>=48&&c<=57||c>=65&&c<=90||c>=97&&c<=122||c==95)&&count<20 {count++;continue};if c==59 {break};return -1};return i
}
func validateXML(input jsString) *dynamic {
 if input.length()>0&&input.charCodeAt(0)==0xfeff {input=input[2:]}
 v:=xmlValidation{input};var tags []xmlOpenTag;found,root:=false,false
 for i:=int32(0);i<input.length();i++ {
  if v.at(i)==60&&v.at(i+1)==63 {var err *dynamic;i,err=v.pi(i+2);if err!=nil {return err}} else if v.at(i)==60 {
   tagStart:=i;i++
   if v.at(i)==33 {i=v.comment(i);continue}
   closing:=v.at(i)==47;if closing {i++};start:=i
   for i<input.length()&&v.at(i)!=62&&!xmlWhite(v.at(i)) {i++}
   name:=input[2*start:2*i].trim()
   if name.length()>0&&name.charCodeAt(name.length()-1)==47 {name=name[:len(name)-2];i--}
   if !xmlValidName(name) {msg:="Tag '"+name.String()+"' is an invalid name.";if name.trim().length()==0 {msg="Invalid space after '<'."};return v.failure("InvalidTag",msg,i)}
   attrStart:=i;quote:=int32(0);closed:=false
   for ;i<input.length();i++ {c:=v.at(i);if c==34||c==39 {if quote==0 {quote=c} else if quote==c {quote=0}} else if c==62&&quote==0 {closed=true;break}}
   if quote!=0 {return v.failure("InvalidAttr","Attributes for '"+name.String()+"' have open quote.",attrStart)}
   attrs:=input[2*attrStart:2*i]
   if attrs.length()>0&&attrs.charCodeAt(attrs.length()-1)==47 {
    attrs=attrs[:len(attrs)-2];if err:=xmlValidateAttrs(attrs);err!=nil {return v.failure("InvalidAttr",err.msg,attrStart+err.pos)};found=true
   } else if closing {
    if !closed {return v.failure("InvalidTag","Closing tag '"+name.String()+"' doesn't have proper closing.",i)}
    if attrs.trim().length()>0 {return v.failure("InvalidTag","Closing tag '"+name.String()+"' can't have attributes or invalid starting.",tagStart)}
    if len(tags)==0 {return v.failure("InvalidTag","Closing tag '"+name.String()+"' has not been opened.",tagStart)}
    open:=tags[len(tags)-1];tags=tags[:len(tags)-1]
    if name!=open.name {line,col:=v.position(open.pos);msg:="Expected closing tag '"+open.name.String()+"' (opened in line "+strconv.Itoa(int(line))+", col "+strconv.Itoa(int(col))+") instead of closing tag '"+name.String()+"'.";return v.failure("InvalidTag",msg,tagStart)}
    if len(tags)==0 {root=true}
   } else {
    if err:=xmlValidateAttrs(attrs);err!=nil {return v.failure("InvalidAttr",err.msg,attrStart+err.pos)}
    if root {return v.failure("InvalidXml","Multiple possible root nodes found.",i)}
    tags=append(tags,xmlOpenTag{name,tagStart});found=true
   }
   for i++;i<input.length();i++ {
    if v.at(i)==60 {if v.at(i+1)==33 {i=v.comment(i+1);continue} else if v.at(i+1)==63 {var err *dynamic;i,err=v.pi(i+1);if err!=nil {return err}} else {break}} else if v.at(i)==38 {after:=v.amp(i);if after==-1 {return v.failure("InvalidChar","char '&' is not expected.",i)};i=after} else if root&&!xmlWhite(v.at(i)) {return v.failure("InvalidXml","Extra text at the end",i)}
   }
   if v.at(i)==60 {i--}
  } else if !xmlWhite(v.at(i)) {return v.failure("InvalidChar","char '"+input.charAt(i).String()+"' is not expected.",i)}
 }
 if !found {return xmlValidationError("InvalidXml","Start tag expected.",1,0)}
 if len(tags)==1 {return v.failure("InvalidTag","Unclosed tag '"+tags[0].name.String()+"'.",tags[0].pos)}
 if len(tags)>0 {names:=make([]string,len(tags));for i,t:=range tags {raw,_:=json.Marshal(t.name.String());names[i]="    "+string(raw)};return xmlValidationError("InvalidXml","Invalid '["+strings.Join(names,",")+"]' found.",1,1)}
 return box(true)
}
`
