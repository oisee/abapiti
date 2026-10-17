package tsfront

import "testing"

// gogen's literals use dotAll (frontend.mjs /^'.*'$/s), control escapes
// (osd-narrow-submit.mjs /\n/g) and a later-alternative anchor
// (frontend-owned.ts /\bFRIENDS\b|^ASSIGN COMPONENT\b|.../).
func TestStep4RegexDotAllControlAnchorNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Probe {
  /** @returns {string} */ static run() {
    const out = [];
    for (const s of ["'a\nb'", "'x'", "x'y'", "'unterminated"]) out.push(String(/^'.*'$/s.test(s)));
    for (const s of ["WRITE '@KERNEL lines.\nmore'.", "write  '@kernel x' ", "WRITE 'X'."]) { const m = /^WRITE\s+'@KERNEL(.*)'\s*\.?$/is.exec(s); out.push(m ? JSON.stringify(m[1]) : "null"); }
    for (const s of ["CLASS x DEFINITION FRIENDS y", "ASSIGN COMPONENT a OF STRUCTURE s", "  ASSIGN COMPONENT", "ASSIGN ( name ) TO <f>", "x->( m )", "y=>  (n)", "FRIENDSHIP", "ASSIGNx"]) out.push(String(/\bFRIENDS\b|^ASSIGN COMPONENT\b|^ASSIGN\s*\(|(?:->|=>)\s*\(/.test(s)));
    out.push("a\nb\n\nc".replace(/\n/g, "|"), String("x\ty".split(/\t/).length));
    return out.join(",");
  }
}
`, false)
}
