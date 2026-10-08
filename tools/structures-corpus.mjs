#!/usr/bin/env node
// Extract literal case arrays from upstream tests using its locked TS parser.
// Only literals and concatenation are accepted; no test/source code executes.
import {readFileSync, readdirSync, writeFileSync, mkdirSync} from "node:fs";
import {join} from "node:path";
import {createRequire} from "node:module";
import {verifyUpstream} from "./statements-upstream.mjs";
const [out, upstream = "/home/alice/dev/abaplint", benchmark] = process.argv.slice(2);
if (!out) throw new Error("usage: structures-corpus.mjs output-dir [upstream] [benchmark.abap]");
const require = createRequire(import.meta.url);
const ts = require(verifyUpstream(upstream));
function literal(n) {
 if (ts.isStringLiteral(n) || ts.isNoSubstitutionTemplateLiteral(n)) return n.text;
 if (ts.isBinaryExpression(n) && n.operatorToken.kind === ts.SyntaxKind.PlusToken) return literal(n.left)+literal(n.right);
 throw new Error("nonliteral ABAP test input");
}
const cases = JSON.parse(readFileSync("tsfront/testdata/stmtscorpus/cases.json","utf8")).map(c => ({...c, filename:"ztest.prog.abap"}));
const testdir = join(upstream,"packages/core/test/abap/structures");
for (const f of readdirSync(testdir).sort().filter(f=>f.endsWith(".ts") && f!=="_combi.ts")) {
 const sf = ts.createSourceFile(f, readFileSync(join(testdir,f),"utf8"),ts.ScriptTarget.Latest,true);
 let i=0;
 function visit(n) {
  if (ts.isPropertyAssignment(n) && n.name.getText(sf)==="abap") {
   const filename = f==="class_global.ts" ? "ztest.clas.abap" : f==="interface_global.ts" ? "ztest.intf.abap" : f==="dynpro_logic.ts" || f==="chain.ts" ? "ztest.screen_0100.abap" : "ztest.prog.abap";
   cases.push({name:`upstream/${f}/${i++}`,filename,abap:literal(n.initializer)});
  }
  ts.forEachChild(n,visit);
 }
 visit(sf);
}
// Kinds absent from standalone upstream test files: exercise through parents.
const extra = [
 ["class-data", "CLASS lcl DEFINITION. PUBLIC SECTION. CLASS-DATA: BEGIN OF foo, bar TYPE i, END OF foo. ENDCLASS."],
 ["events", "AT FIRST. ENDAT. AT LAST. ENDAT. AT NEW foo. ENDAT."],
 ["case-type", "CASE TYPE OF ref. WHEN TYPE cl_foo. ENDCASE."],
 ["enhancement", "ENHANCEMENT 1 zfoo. ENDENHANCEMENT. ENHANCEMENT-SECTION foo SPOTS zfoo. END-ENHANCEMENT-SECTION."],
 ["function", "FUNCTION zfoo. ENDFUNCTION. MODULE foo OUTPUT. ENDMODULE."],
 ["provide", "PROVIDE * FROM itab BETWEEN a AND b. ENDPROVIDE."],
 ["statics", "STATICS: BEGIN OF foo, bar TYPE i, END OF foo."],
 ["seams", "TEST-SEAM foo. END-TEST-SEAM. TEST-INJECTION foo. END-TEST-INJECTION."],
 ["mesh", "TYPES: BEGIN OF MESH foo, bar TYPE STANDARD TABLE OF i WITH DEFAULT KEY, END OF MESH foo."],
 ["extract-loop", "LOOP. ENDLOOP."],
 ["try-branches", "TRY. CATCH cx_root. CLEANUP. ENDTRY."],
 ["broken-if", "IF 1 = 1. WRITE 'x'."],
 ["unexpected-end", "ENDIF."],
 ["filtered", "* comment\n@ garbage.\nWRITE 'x'."],
];
for(const [name,abap] of extra) cases.push({name:`supplement/${name}`,filename:"ztest.prog.abap",abap});
mkdirSync(out,{recursive:true});
writeFileSync(join(out,"cases.json"),JSON.stringify(cases,null,1)+"\n");
if (benchmark) writeFileSync(join(out,"benchmark.json"),JSON.stringify([{name:"zabapgit",filename:"zabapgit_standalone.prog.abap",abap:readFileSync(benchmark,"utf8")}],null,1)+"\n");
console.error(`${cases.length} cases`);
