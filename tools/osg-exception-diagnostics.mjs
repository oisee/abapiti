// The pinned OSGo refuses CX_ROOT=>GET_SOURCE_POSITION with NOT_COMPILED,
// which is a host dump, not a catchable CX. Only diagnostic capture may tolerate
// that specific refusal. Never change CX_ROOT itself or source-program calls.
export function adaptExceptionDiagnostics(go) {
  // Generated ABAP attributes are unexported Go fields. Small host seams
  // let the reporter follow payloads without unsafe reflection or guessed text.
  const diagnosticTypes = new Set();
  go = go.replace(/^type (\w+) struct \{\n([\s\S]*?)^\}/gm, (decl, name, fields) => {
    if (/^\tabap_origin /m.test(fields) && /^\tabap_position_status /m.test(fields)) diagnosticTypes.add(name);
    const field = ['abap_origin', 'payload', 'oval'].find(f => new RegExp(`^\\t${f} `, 'm').test(fields));
    return field ? decl + `\nfunc (me *${name}) AbapDiagnosticNext() any { return me.${field} }\n` : decl;
  });
  return go.replace(/func \(me \*(\w+)\) CAPTURE_ABAP_POSITION\(s \*abap\.Session\) \{\n/g, (signature, name) => !diagnosticTypes.has(name) ? signature : signature + `
 defer func() {
  if x := recover(); x != nil {
   if e, ok := abap.AsError(x); ok && e.Class == "NOT_COMPILED" {
    const method = "CX_ROOT=>GET_SOURCE_POSITION"
    if e.Op == method || (len(e.Op) > len(method) && e.Op[:len(method)] == method && (e.Op[len(method)] == ' ' || e.Op[len(method)] == ':')) {
     me.abap_program = ""
     me.abap_include = ""
     me.abap_line = 0
     me.abap_position_status = "unavailable on OSGo: " + e.Op
     return
    }
   }
   panic(x)
  }
 }()
`);
}
