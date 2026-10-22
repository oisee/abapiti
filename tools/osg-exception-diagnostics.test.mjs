import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {adaptExceptionDiagnostics} from './osg-exception-diagnostics.mjs';

test('diagnostic capture tolerates only the unavailable CX_ROOT source-position method', () => {
  const dir = mkdtempSync(join(tmpdir(), 'cx-diag-'));
  try {
    writeFileSync(join(dir, 'go.mod'), 'module osg/gogen\n\ngo 1.26.0\n');
    mkdirSync(join(dir, 'abap'));
    writeFileSync(join(dir, 'abap', 'stub.go'), `package abap
 type Session struct { Failure any }
 type ArithmeticError struct { Class, Op string }
 func (e ArithmeticError) Error() string { return e.Class+": "+e.Op }
 func AsError(x any) (ArithmeticError,bool) { e,ok:=x.(ArithmeticError);return e,ok }
`);
    // Use the generated Go ABI, including its unexported attribute names.
    writeFileSync(join(dir, 'diag.go'), adaptExceptionDiagnostics(`package diag
import "osg/gogen/abap"
type Error struct {
\tabap_origin any
\tabap_program string
\tabap_include string
\tabap_line int32
\tabap_position_status string
}
func (me *Error) CAPTURE_ABAP_POSITION(s *abap.Session) {
 if s.Failure!=nil {panic(s.Failure)}
 me.abap_program="program";me.abap_include="include";me.abap_line=42
}
type Exception struct {
\tpayload any
}
type Dynamic struct {
\toval any
}
type Host struct {}
func (me *Host) CAPTURE_ABAP_POSITION(s *abap.Session) {
 panic(s.Failure)
}
func nativePosition(s *abap.Session) { panic(s.Failure) }
`));
    writeFileSync(join(dir, 'diag_test.go'), `package diag
import ("testing";"osg/gogen/abap")
func recovered(f func()) (x any) { defer func(){x=recover()}();f();return }
func TestPosition(t *testing.T) {
 origin:=new(int)
 e:=&Error{abap_origin:origin}
 e.CAPTURE_ABAP_POSITION(&abap.Session{})
 if e.abap_program!="program"||e.abap_include!="include"||e.abap_line!=42 {t.Fatal("available position lost")}
 refusal:=abap.ArithmeticError{Class:"NOT_COMPILED",Op:"CX_ROOT=>GET_SOURCE_POSITION (cx_root.clas.abap:40): unavailable"}
 e.CAPTURE_ABAP_POSITION(&abap.Session{Failure:refusal})
 if e.abap_program!=""||e.abap_include!=""||e.abap_line!=0||e.abap_position_status=="" {t.Fatal("unavailable position not recorded")}
 if e.AbapDiagnosticNext()!=origin {t.Fatal("origin identity lost")}
 if (&Exception{payload:e}).AbapDiagnosticNext()!=e {t.Fatal("payload lost")}
 if (&Dynamic{oval:e}).AbapDiagnosticNext()!=e {t.Fatal("dynamic payload lost")}
 for _,failure:=range []any{
  "host panic",
  abap.ArithmeticError{Class:"CX_SY_ZERODIVIDE",Op:"/"},
  abap.ArithmeticError{Class:"NOT_COMPILED",Op:"OTHER_METHOD: unavailable"},
  abap.ArithmeticError{Class:"NOT_COMPILED",Op:"CX_ROOT=>GET_SOURCE_POSITION_OTHER: unavailable"},
 } {
  got:=recovered(func(){e.CAPTURE_ABAP_POSITION(&abap.Session{Failure:failure})})
  if got!=failure {t.Fatalf("unrelated failure changed: got %v, want %v",got,failure)}
 }
 if got:=recovered(func(){(&Host{}).CAPTURE_ABAP_POSITION(&abap.Session{Failure:refusal})});got!=refusal {t.Fatal("unrelated method with the same name was changed")}
 if got:=recovered(func(){nativePosition(&abap.Session{Failure:refusal})});got!=refusal {t.Fatal("source-program call was changed")}
}
`);
    const result = spawnSync('go', ['test', '-timeout', '30m', '.'], {cwd: dir, encoding: 'utf8'});
    assert.equal(result.status, 0, result.stdout + result.stderr);
  } finally {
    rmSync(dir, {recursive: true, force: true});
  }
});
