#!/usr/bin/env node
import assert from 'node:assert/strict';
import {checkPortableTestObject, identifiers, checkIndependence, promote, initialAssignment, balanceRules, adaptRunner, dispatchPlan, adaptDispatch, chaiPlan, adaptChai, groups, split, verifyParts} from './a4h-tests-parts.mjs';
import {mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, readdirSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
const sha = b => createHash('sha256').update(b).digest('hex');

checkPortableTestObject('zcl_lnt_test_log.clas.abap', "TRY. CALL METHOD ('ZCL_LNT_TEST_LOG_BAL')=>('EVENT') EXPORTING value = value. CATCH cx_sy_dyn_call_error. ENDTRY.");
checkPortableTestObject('test.clas.abap', 'CLASS lcl_global_func DEFINITION. ENDCLASS.');
for (const token of ['BAL_LOG_CREATE', 'balloghndl', 'bal_s_log', 'bal_t_logh', 'symsgty']) {
 assert.throws(() => checkPortableTestObject('test.clas.abap', token), /SAP BAL dependency/);
}
assert.throws(() => checkPortableTestObject('zcl_lnt_test_log_bal.clas.xml', '<class/>'), /SAP BAL dependency/);

const refs = identifiers(`CLASS zcl_a DEFINITION INHERITING FROM zcl_parent.
 INTERFACES zif_a.
 METHODS work RAISING zcx_a.
 DATA r TYPE REF TO zcl_b.
 DATA t TYPE ztype.
 DATA zlnt_local TYPE string.
ENDCLASS.
METHOD work.
 r = NEW zcl_c( ).
 r = CAST zcl_d( r ).
 zcl_e=>run( ). r->znot_an_object( ).
 CATCH zcx_b.
 text = |literal ZFAKE { NEW zcl_f( ) }|.
 text = 'ZFAKE'' ZFAKE'. text = \`ZFAKE\`\`ZFAKE\`.
 text = \`embedded fixture
 DATA zcl_c TYPE string.
\`.
 " ZCOMMENT
* ZCOMMENT
 zlnt_local = zlnt_local.
ENDMETHOD.`);
assert.deepEqual([...refs].sort(), ['ZCL_A','ZCL_B','ZCL_C','ZCL_D','ZCL_E','ZCL_F','ZCL_PARENT','ZCX_A','ZCX_B','ZIF_A','ZTYPE'].sort());
assert(identifiers('DATA zcl_shadow TYPE string. DATA r TYPE REF TO zcl_shadow.').has('ZCL_SHADOW'), 'local names must not hide explicit global type references');
const assignment = new Map([['ZCL_A','statements'],['ZCL_B','syntax'],['ZCL_C','syntax'],['ZCL_BASE','base']]);
const core = new Set(['ZCL_CORE']);
const edges = new Map([['ZCL_A', identifiers('DATA r TYPE REF TO zcl_b. zcl_base=>run( ). zcl_core=>run( ).')], ['ZCL_B', identifiers('r = NEW zcl_c( ).')]]);
assert.equal(checkIndependence(edges, assignment, core).length, 1, 'synthetic cross-part reference must fail');
assert.equal(promote(edges, assignment, core).length, 2, 'promotion must iterate through base dependencies');
assert.equal(checkIndependence(edges, assignment, core).length, 0);
assert.throws(() => promote(new Map([['ZCL_A', new Set(['ZMISSING'])]]), assignment, core), /unresolved Z object/);
assert.equal(checkIndependence(new Map([['ZCL_BASE', new Set(['ZCL_A'])]]), assignment, core).length, 1, 'base must not statically depend on shards');

const shardNames = [...new Set(Object.values(groups).flat())];
const closure = {shards: shardNames.map(name => ({name, initialization: name === 'rules' ? ['test/rules/a.ts','test/rules/b.ts'] : ['test/shared.ts']}))};
const objects = new Map([['ZCL_A',[]],['ZCL_B',[]],['ZCL_SHARED',[]],['ZCL_SYNTHETIC',[]]]);
const names = {ZCL_A:{source:'test/rules/a.ts:2:1'}, ZCL_B:{id:'test/rules/b.ts.module'},ZCL_SHARED:{source:'test/shared.ts:1:1'}};
const mapped = initialAssignment(objects, names, closure);
assert.equal(mapped.assignment.get('ZCL_SHARED'), 'base');
assert.equal(mapped.assignment.get('ZCL_SYNTHETIC'), 'base');
const roots = balanceRules(mapped.assignment, mapped.provenance, new Map([['ZCL_A','a'.repeat(100)],['ZCL_B','b'.repeat(80)]]));
assert.deepEqual(roots, {rules_1:['test/rules/a.ts'],rules_2:['test/rules/b.ts']});
assert.throws(() => initialAssignment(objects, names, {shards:closure.shards.slice(1)}), /17-shard/);
const runner = 'METHOD host_init_1.\n  zcl_a=>initialize( ).\nENDMETHOD.\nMETHOD host_init_2.\n  zcl_core=>initialize( ).\nENDMETHOD.\n';
const adaptedRunner = adaptRunner(runner, objects);
assert.match(adaptedRunner.source, /CALL METHOD \('ZCL_A'\)=>\('INITIALIZE'\)/);
assert.match(adaptedRunner.source, /zcl_core=>initialize/);
assert.equal(adaptedRunner.calls.length, 1);

const factory = `CLASS zcl_lnt_test_factory DEFINITION PUBLIC CREATE PRIVATE.
 PUBLIC SECTION.
 CLASS-METHODS new IMPORTING p0 TYPE REF TO zlnt_runtime_clas_a39833db3295 RETURNING VALUE(result) TYPE REF TO object.
 CLASS-METHODS new_part_0 IMPORTING class_name TYPE string p0 TYPE REF TO zlnt_runtime_clas_a39833db3295 RETURNING VALUE(result) TYPE REF TO object.
ENDCLASS.
CLASS zcl_lnt_test_factory IMPLEMENTATION.
  METHOD new.
    result = new_part_0( class_name = p0->emitted p0 = p0 ).
  ENDMETHOD.
  METHOD new_part_0.
    CASE class_name.
      WHEN \`ZCL_A\`.
        CREATE OBJECT result TYPE zcl_a.
      WHEN \`ZCL_B\`.
        CREATE OBJECT result TYPE zcl_b.
      WHEN OTHERS.
        RETURN.
    ENDCASE.
  ENDMETHOD.
ENDCLASS.
`;
const deletion = `CLASS zcl_lnt_test_delete DEFINITION PUBLIC CREATE PRIVATE.
 PUBLIC SECTION.
 CLASS-METHODS run IMPORTING p0 TYPE REF TO object key TYPE string EXPORTING handled TYPE abap_bool result TYPE abap_bool.
ENDCLASS.
CLASS zcl_lnt_test_delete IMPLEMENTATION.
  METHOD run.
    IF p0 IS INSTANCE OF zcl_a.
      handled = abap_true.
      result = abap_true.
      RETURN.
    ENDIF.
    IF p0 IS INSTANCE OF zcl_b.
      handled = abap_true.
      result = abap_true.
      RETURN.
    ENDIF.
  ENDMETHOD.
ENDCLASS.
`;
const plan = dispatchPlan(factory, deletion);
assert.equal(plan.branches.length, 2); assert.equal(plan.blocks.length, 2);
const owners = new Map([['ZCL_A','rules_1'],['ZCL_B','rules_2']]);
const dispatch = adaptDispatch(factory, deletion, plan, owners);
assert(!identifiers(dispatch.factory).has('ZCL_A'));
assert(!identifiers(dispatch.deletion).has('ZCL_B'));
assert(dispatch.deletion.indexOf('\\CLASS=ZCL_A') < dispatch.deletion.indexOf('\\CLASS=ZCL_B'), 'first-match order must survive regrouping');
assert.match(dispatch.helpers.get('ZCL_LNT_P_RULES_1_FAC').source, /CREATE OBJECT result TYPE zcl_a/);
assert(!dispatch.helpers.get('ZCL_LNT_P_RULES_1_FAC').source.includes('TYPE zcl_b'));
const chai = `  METHOD invoke.
    IF p0->oval IS INSTANCE OF zcl_a.
      IF p0 IS BOUND.
        CALL METHOD p0->oval->('CALL').
      ENDIF.
      RETURN.
    ENDIF.
    IF p0->oval IS INSTANCE OF zcl_b.
      RETURN.
    ENDIF.
  ENDMETHOD.
`;
const cb = chaiPlan(chai); assert.equal(cb.length,2); assert.match(cb[0].text, /      ENDIF\./);
const ch = adaptChai(chai, cb, owners);
assert(!identifiers(ch.source).has('ZCL_A'));
assert.match(ch.helpers.get('ZCL_LNT_P_RULES_1_CHAI').source, /IF p0->oval IS INSTANCE OF zcl_a/);

// End-to-end archive fixture, including DEVC, reproducible bytes, object hashes,
// name preservation, generated helpers, and manifest tampering refusal.
const scratch = mkdtempSync(join(tmpdir(), 'a4h-parts-test-'));
try {
 const build = join(scratch,'build'), input = join(build,'input'), output = join(scratch,'output');
 mkdirSync(join(input,'src'),{recursive:true}); mkdirSync(join(build,'closure'));
 const support = {ZCL_LNT_TEST_RUNNER:runner,ZCL_LNT_TEST_FACTORY:factory,ZCL_LNT_TEST_DELETE:deletion,
 ZCL_LNT_TEST_LOG: "CLASS zcl_lnt_test_log IMPLEMENTATION. METHOD append. TRY. CALL METHOD ('ZCL_LNT_TEST_LOG_BAL')=>('EVENT') EXPORTING value = value. CATCH cx_sy_dyn_call_error. ENDTRY. ENDMETHOD. ENDCLASS.\n",
 ZLNT_RUNTIME_CHAI_63495E43F558:chai,ZCL_A:'CLASS zcl_a DEFINITION PUBLIC. ENDCLASS.\n',ZCL_B:'CLASS zcl_b DEFINITION PUBLIC. ENDCLASS.\n'};
 writeFileSync(join(input,'.abapgit.xml'), '<DATA><STARTING_FOLDER>/src/</STARTING_FOLDER><FOLDER_LOGIC>PREFIX</FOLDER_LOGIC></DATA>');
 writeFileSync(join(input,'src/package.devc.xml'), '<DEVC><CTEXT>fixture</CTEXT></DEVC>');
 for (const [name,source] of Object.entries(support)) {
  const file = name.toLowerCase(); writeFileSync(join(input,'src',`${file}.clas.abap`),source);
  writeFileSync(join(input,'src',`${file}.clas.xml`),`<VSEOCLASS><CLSNAME>${name}</CLSNAME></VSEOCLASS>`);
 }
 const manifest = {algorithm:'sha256-length-framed-path-and-content-v1',objects:{}};
 for (const file of readdirSync(join(input,'src')).sort()) {
  const key = file.split('.').slice(0,2).join('.').toUpperCase();
  (manifest.objects[key] ??= {files:[]}).files.push(`src/${file}`);
 }
 for (const obj of Object.values(manifest.objects)) {
  const h = createHash('sha256');
  for(const file of obj.files){const raw=readFileSync(join(input,file));h.update(`${file.length}:${file}${raw.length}:`);h.update(raw);}
  obj.sha256=h.digest('hex');
 }
 writeFileSync(join(build,'zlnt_tests.manifest.json'),JSON.stringify(manifest));
 writeFileSync(join(build,'zlnt.manifest.json'),JSON.stringify({objects:Object.fromEntries(['ZCL_CORE','ZLNT_RUNTIME_CLAS_A39833DB3295','ZLNT_RUNTIME_ARRA_C22708FAF2E3','ZLNT_RUNTIME_DYNA_A429568D5BE8'].map(n=>[`${n}.CLAS`,{}]))}));
 writeFileSync(join(build,'names.json'),JSON.stringify(names));writeFileSync(join(build,'closure/closure.json'),JSON.stringify(closure));
 execFileSync('python3',['-c',`import sys,pathlib,zipfile
p=pathlib.Path(sys.argv[1])
with zipfile.ZipFile(sys.argv[2],'w') as z:
 for f in sorted(p.rglob('*')):
  if f.is_file(): z.write(f,f.relative_to(p).as_posix())`,input,join(build,'zlnt_tests.zip')]);
 const first=split(build,output), again=split(build,output);
 assert.deepEqual(first,again,'archives must be reproducible');
 const result=JSON.parse(readFileSync(join(output,'parts.json')));
 assert.equal(result.independence.violations,0);
 assert.equal(result.assignments.ZCL_LNT_TEST_LOG.part,'base');
 assert(!result.assignments.ZCL_LNT_TEST_LOG_BAL, 'optional BAL adapter must remain outside tests layers');
 assert.equal(result.independence.originalObjects,7);
 assert.equal(verifyParts(build,output).checkedObjects,result.independence.checkedObjects);
 for(const info of Object.values(first)) {
  assert.equal(info.sha256,sha(readFileSync(join(output,info.zip))));
  const m=JSON.parse(readFileSync(join(output,info.zip.replace('.zip','.manifest.json'))));
  assert(m.objects['PACKAGE.DEVC']);assert.equal(Object.keys(m.objects).length,info.objectCount+1);
 }
 assert(!readFileSync(join(output,'parts.json'),'utf8').includes(scratch),'no absolute build paths');
 const zipFile=join(output,first.base.zip),zipBytes=readFileSync(zipFile);
 writeFileSync(zipFile,Buffer.concat([zipBytes,Buffer.from('tampered')]));
 assert.throws(()=>verifyParts(build,output),/zip hash\/size mismatch/);
 writeFileSync(zipFile,zipBytes);
 manifest.objects['ZCL_A.CLAS'].sha256='invalid';writeFileSync(join(build,'zlnt_tests.manifest.json'),JSON.stringify(manifest));
 assert.throws(()=>split(build,output),/hash mismatch/);
} finally {rmSync(scratch,{recursive:true,force:true});}
console.log('a4h-tests-parts: scanner, cross-part failure, fixed point, mapping, dispatch, reproducible archives OK');
