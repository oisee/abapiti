package golang

// Match vscode-languageserver-types 3.18's predicates (including its integer
// range check, which intentionally admits fractional numbers). Property reads
// use the ordinary dynamic adapters, so native data and live views agree.
const lspGuardsSource = `
func lspDefined(d *dynamic) bool {return d!=nil}
func lspString(d *dynamic) bool {return d!=nil && d.Tag==1}
func lspBool(d *dynamic) bool {return d!=nil && d.Tag==3}
func lspInteger(d *dynamic) bool {if d==nil || d.Tag<4 || d.Tag>6 {return false};n:=dynNumber(d);return n>=-2147483648 && n<=2147483647}
func lspTextDocumentEdit(d *dynamic) bool {
 if !lspDefined(d) {return false}
 td:=d.get(str("textDocument"));if !lspDefined(td) {return false}
 if !lspString(td.get(str("uri"))) {return false}
 v:=td.get(str("version"));if !(v!=nil && v.Tag==tagNull) && !lspInteger(v) {return false}
 return isArray(d.get(str("edits")))
}
func lspRenameFile(d *dynamic, condition bool) bool {
 // The upstream && expression returns its falsy candidate unchanged. A
 // condition consumes only truthiness; value contexts must refuse non-bools.
 falsy:=d==nil || d.Tag==tagNull || d.Tag==1 && dynString(d)==str("")
 if d!=nil && d.Tag>=4 && d.Tag<=6 {n:=dynNumber(d);falsy=n==0 || n!=n}
 if falsy {if condition {return false};panic(newTrap("RenameFile.is returns a non-boolean falsy value"))}
 if d.Tag==3 && !dynBoolean(d) {return false}
 k:=d.get(str("kind"));if !lspString(k) || dynString(k)!=str("rename") {return false}
 if !lspString(d.get(str("oldUri"))) || !lspString(d.get(str("newUri"))) {return false}
 options:=d.get(str("options"));if options!=nil {
  overwrite:=options.get(str("overwrite"));if overwrite!=nil && !lspBool(overwrite) {return false}
  ignore:=options.get(str("ignoreIfExists"));if ignore!=nil && !lspBool(ignore) {return false}
 }
 annotation:=d.get(str("annotationId"));return annotation==nil || lspString(annotation)
}
`
