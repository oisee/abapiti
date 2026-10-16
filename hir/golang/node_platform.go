package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var nodePlatformGoOps = []string{"string.sha256Hex", "path.basename", "fs.readText", "fs.readdirNames", "fs.readdirEntries"}

func init() {
	for _, op := range nodePlatformGoOps {
		supportedOps[op] = true
	}
}

func (b *body) nodePlatformRuntime(x *hir.Expr) (string, bool) {
	switch x.Op {
	case "string.sha256Hex":
		return "sha256Hex(" + b.expr(x.X) + ")", true
	case "path.basename":
		return "posixBasename(" + b.expr(x.X) + ")", true
	case "fs.readText":
		return "fsReadText(" + b.expr(x.X) + "," + b.nodeErrorThrower() + ")", true
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
	if used["string.sha256Hex"] {
		imports["crypto/sha256"], imports["encoding/hex"] = true, true
		parts = append(parts, nodeSHA256Source)
	}
	if used["path.basename"] {
		parts = append(parts, nodeBasenameSource)
	}
	if used["fs.readText"] || used["fs.readdirNames"] || used["fs.readdirEntries"] {
		for _, i := range []string{"errors", "os", "strings", "syscall", "unicode/utf8"} {
			imports[i] = true
		}
		parts = append(parts, nodeFSSource)
	}
	var list []string
	for _, i := range []string{"crypto/sha256", "encoding/hex", "errors", "os", "strings", "syscall", "unicode/utf8"} {
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
