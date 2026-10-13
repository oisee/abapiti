import assert from 'node:assert/strict';
import {protectTestBoundaries} from './osgo-test-boundaries.mjs';
const program = {classes:[{name:'ZCL_ABAPITI_TEST_RUNNER',methods:[{name:'HOST_BODY'},{name:'HOST_INIT_1'}]}]};
const source = `func ZCL_ABAPITI_TEST_RUNNER_HOST_BODY(s *abap.Session, test *Test, settled **Promise, message *string) {
 callBody()
}
func ZCL_ABAPITI_TEST_RUNNER_HOST_INIT_1(s *abap.Session) (result string) {
 initialize()
 return
}
func ZCL_ABAPITI_TEST_RUNNER_RUN(s *abap.Session) (result string) {
 result = test_error.TextOf(s) + init_error.TextOf(s)
 return
}
func PRODUCT(s *abap.Session) { callBody() }
`;
const protectedSource = protectTestBoundaries(source, program);
assert.equal((protectedSource.match(/defer func/g) ?? []).length,2);
assert.equal((protectedSource.match(/e.Class == "NOT_COMPILED"/g) ?? []).length,2);
assert.match(protectedSource,/\*message = e.Error\(\)/);
assert.match(protectedSource,/\*settled = nil/);
assert.match(protectedSource,/result = e.Error\(\)/);
assert.match(protectedSource,/driverExceptionName\(test_error.Class\)/);
assert.equal((protectedSource.match(/panic\(x\)/g) ?? []).length,2);
assert.ok(protectedSource.endsWith('func PRODUCT(s *abap.Session) { callBody() }\n'));
assert.throws(()=>protectTestBoundaries(source.replace('message *string','message string'),program),/ABI changed/);
assert.throws(()=>protectTestBoundaries(source.replace('(result string)','(answer string)'),program),/ABI changed/);
assert.throws(()=>protectTestBoundaries(source+source,program),/missing or ambiguous/);
assert.throws(()=>protectTestBoundaries('',{classes:[]}),/missing OSG test runner/);
assert.throws(()=>protectTestBoundaries('',{classes:[{name:'ZCL_ABAPITI_TEST_RUNNER',methods:[]}]}),/missing OSG body boundary/);
const resumedSource = protectTestBoundaries(source, program, {resume:true});
assert.equal((resumedSource.match(/defer func/g) ?? []).length,0);
assert.match(resumedSource,/initialize\(\)/);
assert.match(resumedSource,/callBody\(\)/);
assert.match(resumedSource,/driverExceptionName\(init_error.Class\)/);
assert.ok(resumedSource.endsWith('func PRODUCT(s *abap.Session) { callBody() }\n'));
assert.throws(()=>protectTestBoundaries(source.replace('message *string','message string'),program,{resume:true}),/ABI changed/);
console.log('OSGO driver-only native refusal boundaries: ok');
