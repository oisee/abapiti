package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var nodePlatformGoOps = []string{"host.stderrWrite", "string.sha256Hex", "path.basename", "fs.readText", "fs.readdirNames", "fs.readdirEntries", "util.isDeepStrictEqual", "path.join"}

func init() {
	for _, op := range nodePlatformGoOps {
		supportedOps[op] = true
	}
}

func (b *body) nodePlatformRuntime(x *hir.Expr) (string, bool) {
	switch x.Op {
	case "host.stderrWrite":
		b.line("nodeStderrWrite(%s)", b.expr(x.X))
		return "", true
	case "string.sha256Hex":
		return "sha256Hex(" + b.expr(x.X) + ")", true
	case "path.basename":
		return "posixBasename(" + b.expr(x.X) + ")", true
	case "fs.readText":
		return "fsReadText(" + b.expr(x.X) + "," + b.nodeErrorThrower() + ")", true
	case "path.join":
		return "posixJoin(" + b.expr(x.X) + ")", true
	case "util.isDeepStrictEqual":
		return "nodeIsDeepStrictEqual(" + b.expr(x.X) + "," + b.expr(x.Args[0]) + ")", true
	case "fs.readdirNames":
		return "fsReaddirNames(" + b.expr(x.X) + "," + b.nodeErrorThrower() + ")", true
	case "fs.readdirEntries":
		t := hir.Ref("builtin.Dirent")
		mk := "func(n jsString,d bool) " + b.e.typ(t) + " {return " + b.e.name("new."+t.Name) + "(n,d)}"
		return "fsReaddirEntries(" + b.expr(x.X) + "," + mk + "," + b.nodeErrorThrower() + ")", true
	}
	return "", false
}

// nodeErrorThrower throws a catchable builtin.Error with Node's message.
func (b *body) nodeErrorThrower() string {
	b.e.usesTypedCatch = true
	t := hir.Ref("builtin.Error")
	return "func(m jsString){panic(payload[" + b.e.typ(t) + "]{Type:" + strconv.Quote(t.String()) + ",Value:" + b.e.name("new."+t.Name) + "(present(m))})}"
}

// nodePlatformSource is a separate file (own imports) emitted only for
// programs using these operations: other generated sources stay byte-identical.
func (e *emitter) nodePlatformSource(pkg string) string {
	ops := hir.UsedRuntimeOps(e.p)
	used := map[string]bool{}
	for _, op := range nodePlatformGoOps {
		if _, ok := ops[op]; ok {
			used[op] = true
		}
	}
	if len(used) == 0 {
		return ""
	}
	imports := map[string]bool{}
	var parts []string
	if used["host.stderrWrite"] {
		imports["os"] = true
		parts = append(parts, "func nodeStderrWrite(s jsString) { _, _ = os.Stderr.WriteString(s.String()) }\n")
	}
	if used["string.sha256Hex"] {
		imports["crypto/sha256"], imports["encoding/hex"] = true, true
		parts = append(parts, nodeSHA256Source)
	}
	if used["path.basename"] {
		parts = append(parts, nodeBasenameSource)
	}
	if used["path.join"] {
		length := "int32(len(parts.Items))"
		if e.liveStorage() {
			length = "parts.length()"
		}
		parts = append(parts, strings.Replace(nodeJoinSource, "{joinLength}", length, 1))
	}
	if used["util.isDeepStrictEqual"] {
		imports["math"] = true
		length := "return int32(len(a.Items))"
		if e.liveStorage() {
			length = "return a.length()"
		}
		parts = append(parts, strings.Replace(nodeDeepStrictEqualSource, "{deepStrictLength}", length, 1))
	}
	if used["fs.readText"] || used["fs.readdirNames"] || used["fs.readdirEntries"] {
		for _, i := range []string{"errors", "os", "strings", "syscall", "unicode/utf8"} {
			imports[i] = true
		}
		parts = append(parts, nodeFSSource)
	}
	var list []string
	for _, i := range []string{"crypto/sha256", "encoding/hex", "errors", "math", "os", "strings", "syscall", "unicode/utf8"} {
		if imports[i] {
			list = append(list, strconv.Quote(i))
		}
	}
	head := "package " + pkg + "\n"
	if len(list) != 0 {
		head += "import (" + strings.Join(list, ";") + ")\n"
	}
	return head + strings.Join(parts, "")
}

// String() decodes isolated surrogates as U+FFFD: Node's UTF-8 update encoding.
const nodeSHA256Source = `
func sha256Hex(s jsString) jsString {sum:=sha256.Sum256([]byte(s.String()));return str(hex.EncodeToString(sum[:]))}
`

// Node's posix.basename without a suffix, on UTF-16 units.
const nodeBasenameSource = `
func posixBasename(p jsString) jsString {
 start, end, matchedSlash := int32(0), int32(-1), true
 for i := p.length() - 1; i >= 0; i-- {
  if p.charCodeAt(i) == '/' {
   if !matchedSlash { start = i + 1; break }
  } else if end == -1 { matchedSlash = false; end = i + 1 }
 }
 if end == -1 { return str("") }
 return p[2*start : 2*end]
}
`

// Node's posix.join: non-empty segments joined with "/", then
// posix.normalize (normalizeString with allowAboveRoot for relative paths),
// on UTF-16 units.
const nodeJoinSource = `
func posixLastSlash(s jsString) int32 {for i:=s.length()-1;i>=0;i-- {if s.charCodeAt(i)=='/' {return i}};return -1}
func posixNormalizeString(p jsString, allowAboveRoot bool) jsString {
 res, lastSegmentLength, lastSlash, dots, code := jsString(""), int32(0), int32(-1), 0, int32(0)
 n := p.length()
 for i := int32(0); i <= n; i++ {
  if i < n { code = p.charCodeAt(i) } else if code == '/' { break } else { code = '/' }
  if code == '/' {
   if lastSlash == i-1 || dots == 1 {
   } else if dots == 2 {
    rl := res.length()
    if rl < 2 || lastSegmentLength != 2 || res.charCodeAt(rl-1) != '.' || res.charCodeAt(rl-2) != '.' {
     if rl > 2 {
      if k := posixLastSlash(res); k == -1 { res, lastSegmentLength = "", 0 } else { res = res[:2*k]; lastSegmentLength = res.length() - 1 - posixLastSlash(res) }
      lastSlash, dots = i, 0
      continue
     } else if rl != 0 {
      res, lastSegmentLength, lastSlash, dots = "", 0, i, 0
      continue
     }
    }
    if allowAboveRoot {
     if res.length() > 0 { res += str("/..") } else { res = str("..") }
     lastSegmentLength = 2
    }
   } else {
    if res.length() > 0 { res += str("/") + p[2*(lastSlash+1):2*i] } else { res = p[2*(lastSlash+1) : 2*i] }
    lastSegmentLength = i - lastSlash - 1
   }
   lastSlash, dots = i, 0
  } else if code == '.' && dots != -1 {
   dots++
  } else {
   dots = -1
  }
 }
 return res
}
func posixJoin(parts *array[jsString]) jsString {
 joined := jsString("")
 for i := int32(0); i < {joinLength}; i++ {
  v := parts.get(i)
  if !v.Has { panic(newTrap("path.join segment is not a string")) }
  s := v.Value
  if s.length() == 0 { continue }
  if joined.length() > 0 { joined += str("/") }
  joined += s
 }
 if joined.length() == 0 { return str(".") }
 absolute, trailing := joined.charCodeAt(0) == '/', joined.charCodeAt(joined.length()-1) == '/'
 out := posixNormalizeString(joined, !absolute)
 if out.length() == 0 {
  if absolute { return str("/") }
  if trailing { return str("./") }
  return str(".")
 }
 if trailing { out += str("/") }
 if absolute { return str("/") + out }
 return out
}
`

// Synchronous fs reads with Node's error messages. Directory entries are in
// byte order of their raw names (libuv's scandir sort, as os.ReadDir) and
// Dirent types do not follow symlinks.
const nodeFSSource = `
var nodeErrnoText = map[syscall.Errno][2]string{
 syscall.ENOENT: {"ENOENT", "no such file or directory"}, syscall.EACCES: {"EACCES", "permission denied"},
 syscall.EISDIR: {"EISDIR", "illegal operation on a directory"}, syscall.ENOTDIR: {"ENOTDIR", "not a directory"},
 syscall.ELOOP: {"ELOOP", "too many symbolic links encountered"}, syscall.ENAMETOOLONG: {"ENAMETOOLONG", "name too long"},
 syscall.EMFILE: {"EMFILE", "too many open files"}, syscall.EPERM: {"EPERM", "operation not permitted"}, syscall.EIO: {"EIO", "i/o error"},
}
func nodeFSError(err error, call string, path *string) jsString {
 code, text := "UNKNOWN", err.Error()
 var errno syscall.Errno
 if errors.As(err, &errno) {
  if t, ok := nodeErrnoText[errno]; ok { code, text = t[0], t[1] } else { code, text = "E"+strings.ToUpper(errno.Error()), errno.Error() }
 }
 m := code + ": " + text + ", " + call
 if path != nil { m += " '" + *path + "'" }
 return str(m)
}
func nodeFSPath(p jsString) string {
 s := p.String()
 if strings.IndexByte(s, 0) >= 0 { panic(newTrap("fs path with null bytes (Node TypeError ERR_INVALID_ARG_VALUE)")) }
 return s
}
func nodeDecodeUTF8(b []byte) jsString {
 var out strings.Builder
 for len(b) > 0 {
  r, n := utf8.DecodeRune(b)
  if r != utf8.RuneError || n > 1 { out.WriteRune(r); b = b[n:]; continue }
  n, want := 1, 1
  switch {
  case b[0] >= 0xc2 && b[0] <= 0xdf: want = 2
  case b[0] >= 0xe0 && b[0] <= 0xef: want = 3
  case b[0] >= 0xf0 && b[0] <= 0xf4: want = 4
  }
  for n < want && n < len(b) {
   v := b[n]
   if v < 0x80 || v > 0xbf { break }
   if n == 1 && (b[0] == 0xe0 && v < 0xa0 || b[0] == 0xed && v >= 0xa0 || b[0] == 0xf0 && v < 0x90 || b[0] == 0xf4 && v >= 0x90) { break }
   n++
  }
  out.WriteRune(utf8.RuneError)
  b = b[n:]
 }
 return str(out.String())
}
func fsReadText(file jsString, fail func(jsString)) jsString {
 p := nodeFSPath(file)
 b, err := os.ReadFile(p)
 if err != nil {
  var pe *os.PathError
  if errors.As(err, &pe) && pe.Op == "read" { fail(nodeFSError(err, "read", nil)) } else { fail(nodeFSError(err, "open", &p)) }
  return ""
 }
 return nodeDecodeUTF8(b)
}
func fsReadDir(dir jsString, fail func(jsString)) []os.DirEntry {
 p := nodeFSPath(dir)
 es, err := os.ReadDir(p)
 if err != nil { fail(nodeFSError(err, "scandir", &p)); return nil }
 return es
}
func fsReaddirNames(dir jsString, fail func(jsString)) *array[jsString] {
 a := &array[jsString]{}
 for _, e := range fsReadDir(dir, fail) { a.Items = append(a.Items, nodeDecodeUTF8([]byte(e.Name()))) }
 return a
}
func fsReaddirEntries[T any](dir jsString, mk func(jsString, bool) T, fail func(jsString)) *array[T] {
 a := &array[T]{}
 for _, e := range fsReadDir(dir, fail) { a.Items = append(a.Items, mk(nodeDecodeUTF8([]byte(e.Name())), e.IsDir())) }
 return a
}
`

// Node util.isDeepStrictEqual on boxed values. Primitives compare by
// SameValue (NaN equals NaN, -0 differs from +0); objects need the same
// prototype: plain records (parsed objects and structural literals) compare
// their own enumerable key sets and values, arrays their length and own
// indices (a hole differs from undefined). Class instances, collections,
// functions and cyclic graphs trap rather than approximate Node.
const nodeDeepStrictEqualSource = `
func (a *array[T]) deepStrictLength() int32 {{deepStrictLength}}
func (a *array[T]) deepStrictAt(i int32) (*dynamic, bool) {v:=a.get(i);if !v.Has {return nil,false};return box(v.Value),true}
// Typed optional storage (an element or field of T | undefined) boxes as its
// present value or undefined.
func (o optional[T]) deepStrictBox() *dynamic {if !o.Has {return nil};return box(o.Value)}
func nodeDeepStrictUnwrap(d *dynamic) *dynamic {
 for d!=nil && d.Tag==2 {o,ok:=d.Value.(interface{deepStrictBox() *dynamic});if !ok {break};d=o.deepStrictBox()}
 return d
}
type deepStrictArray interface {deepStrictLength() int32;deepStrictAt(int32) (*dynamic,bool)}
// 0 primitive, 1 array, 2 plain record.
func nodeDeepStrictKind(d *dynamic) int {
 switch d.Tag {
 case tagObject:return 2
 case tagArray:return 1
 case 2:
  if _,ok:=d.Value.(deepStrictArray);ok {if _,ok:=d.Value.(interface{arrayMarker()});ok {return 1}}
  if _,ok:=d.Value.(interface{deepStrictPlain()});ok {return 2}
  panic(newTrap("util.isDeepStrictEqual supports primitives, arrays and plain records only"))
 case 10:panic(newTrap("util.isDeepStrictEqual on functions is not lowered"))
 }
 return 0
}
func nodeIsDeepStrictEqual(a,b *dynamic) bool {return nodeDeepStrict(a,b,map[any]bool{})}
func nodeDeepStrict(a,b *dynamic,active map[any]bool) bool {
 a,b=nodeDeepStrictUnwrap(a),nodeDeepStrictUnwrap(b)
 if a==nil || b==nil {return a==nil && b==nil}
 an,bn:=a.Tag>=4&&a.Tag<=6,b.Tag>=4&&b.Tag<=6
 if an || bn {
  if !an || !bn {return false}
  x,y:=dynNumber(a),dynNumber(b)
  return math.IsNaN(x)&&math.IsNaN(y) || x==y && math.Signbit(x)==math.Signbit(y)
 }
 ak,bk:=nodeDeepStrictKind(a),nodeDeepStrictKind(b)
 if ak==0 || bk==0 {
  if ak!=bk || a.Tag!=b.Tag {return false}
  if a.Tag==tagNull {return true}
  return a.Value==b.Value
 }
 if ak!=bk {return false}
 if a.Value==b.Value {return true}
 if active[a.Value] || active[b.Value] {panic(newTrap("util.isDeepStrictEqual on cyclic graphs is not lowered"))}
 active[a.Value],active[b.Value]=true,true
 defer delete(active,a.Value)
 defer delete(active,b.Value)
 if ak==1 {
  x,xok:=a.Value.(deepStrictArray);y,yok:=b.Value.(deepStrictArray)
  if !xok || !yok {panic(newTrap("util.isDeepStrictEqual array storage is not certified"))}
  n:=x.deepStrictLength()
  if n!=y.deepStrictLength() {return false}
  for i:=int32(0);i<n;i++ {
   u,uh:=x.deepStrictAt(i);v,vh:=y.deepStrictAt(i)
   if uh!=vh || uh && !nodeDeepStrict(u,v,active) {return false}
  }
  return true
 }
 xk,yk:=dynamicKeys(a).Items,dynamicKeys(b).Items
 if len(xk)!=len(yk) {return false}
 own:=make(map[jsString]bool,len(yk))
 for _,k:=range yk {own[k]=true}
 for _,k:=range xk {if !own[k] {return false}}
 for _,k:=range xk {if !nodeDeepStrict(a.get(k),b.get(k),active) {return false}}
 return true
}
`
