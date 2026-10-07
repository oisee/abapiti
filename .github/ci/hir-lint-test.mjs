// A parser-valid call on abap_bool must fail the same gate used for generated HIR.
import assert from "node:assert/strict";
import {mkdtempSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";
import {spawnSync} from "node:child_process";

const dir = mkdtempSync(join(tmpdir(), "hir-lint-test-"));
const gate = new URL("./hir-lint.mjs", import.meta.url);
const [osg] = process.argv.slice(2);
assert.ok(osg, "usage: node hir-lint-test.mjs <open-steamgate-dir>");
const source = statement => `CLASS z_hir_lint_test DEFINITION PUBLIC.
  PUBLIC SECTION.
    CLASS-METHODS run.
  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.
CLASS z_hir_lint_test IMPLEMENTATION.
  METHOD run.
    DATA(t1) = abap_true.
    DATA ref TYPE REF TO object.
    t1 = xsdbool( ref IS INSTANCE OF object ).
    ${statement}
  ENDMETHOD.
ENDCLASS.
`;
function run(statement) {
  writeFileSync(join(dir, "z_hir_lint_test.clas.abap"), source(statement));
  const result = spawnSync(process.execPath, [gate.pathname, dir, osg], {encoding: "utf8"});
  assert.ifError(result.error);
  return result;
}
try {
  const valid = run("t1 = abap_true.");
  assert.equal(valid.status, 0, valid.stderr + valid.stdout);
  const original = source("t1 = abap_true.");
  writeFileSync(join(dir, "z_hir_lint_test.clas.abap"), original.replace("DATA(t1) = abap_true.", "DATA t1 TYPE abap_bool.").replace("t1 = xsdbool( ref IS INSTANCE OF object ).", "CLEAR t1."));
  const old = spawnSync(process.execPath, [gate.pathname, dir, osg], {encoding: "utf8"});
  assert.ifError(old.error);
  assert.equal(old.status, 1);
  assert.match(old.stderr, /must contain inline DATA and IS INSTANCE OF/);
  for (const section of ["PUBLIC", "PROTECTED", "PRIVATE"]) {
    writeFileSync(join(dir, "z_hir_lint_test.clas.abap"), original.replace(`  ${section} SECTION.\n`, ""));
    const missing = spawnSync(process.execPath, [gate.pathname, dir, osg], {encoding: "utf8"});
    assert.ifError(missing.error);
    assert.equal(missing.status, 1, missing.stderr + missing.stdout);
    assert.match(missing.stderr, new RegExp(`global class must contain ${section} SECTION`));
  }
  const invalid = run("CALL METHOD t1->missing.");
  assert.equal(invalid.status, 1, invalid.stderr + invalid.stdout);
  assert.match(invalid.stdout, /[1-9]\d* issues/);
  const newer = run("FINAL(t2) = abap_true.");
  assert.equal(newer.status, 1, newer.stderr + newer.stdout);
  assert.match(newer.stdout, /[1-9]\d* issues/);
  console.log("HIR lint regression: valid v750 accepted; missing class sections, old corpus, invalid call and FINAL rejected");
} finally {
  rmSync(dir, {recursive: true, force: true});
}
