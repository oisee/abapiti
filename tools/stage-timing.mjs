#!/usr/bin/env node
// Generate a separate benchmark next to the full, unmodified differential.
// Usage: node tools/stage-timing.mjs GENERATED WORK ALLBACKENDS [COUNT=500]
// Emit GENERATED with STMTS_EMIT=1 ABAPITI_TEST_OUT=GENERATED go test
// ./tsfront -run ^TestEmitStatementsClosure$ -count=1. WORK is the session
// stagetime directory; its parent contains corpus-lex/unique.tsv and the
// existing allb/work-500/{set.txt,abap} artifacts. ALLBACKENDS is the sibling
// worktree tools/allbackends directory. COUNT=100 uses every fifth 500 input.
import {readFileSync, writeFileSync, readdirSync, mkdirSync, copyFileSync, unlinkSync, rmSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {buildUpstream} from './statements-upstream.mjs';
const [generatedArg, workArg, helpersArg, countArg = '500'] = process.argv.slice(2);
if (!helpersArg) throw new Error('usage: stage-timing.mjs GENERATED WORK ALLBACKENDS [COUNT=500]');
const generated = resolve(generatedArg), work = resolve(workArg), helpers = resolve(helpersArg);
const count = Number(countArg);
if (![100, 500].includes(count)) throw new Error('corpus count must be 100 or 500');
mkdirSync(work, {recursive:true});
const {corpusZip} = await import(pathToFileURL(join(helpers, 'corpus.mjs')));
const {abapgit} = await import(pathToFileURL(join(helpers, 'benchmark.mjs')));
const {core, dispose} = buildUpstream();
const require = createRequire(import.meta.url);
const {Lexer} = require(join(core,'abap/1_lexer/lexer.js'));
const {StatementParser} = require(join(core,'abap/2_statements/statement_parser.js'));
const {MemoryFile} = require(join(core,'files/memory_file.js'));
const {Release} = require(join(core,'version.js'));
const root = resolve(import.meta.dirname, '..');
const scratch = resolve(work, '..');
// Exactly tools/allbackends/run.mjs's sorted, evenly spaced TSV selection.
const rows = readFileSync(join(scratch,'corpus-lex/unique.tsv'),'utf8').trim().split('\n').map(l=>l.split('\t')).sort((a,b)=>a[0].localeCompare(b[0]));
const paths = Array.from({length:count},(_,i)=>rows[Math.floor(i*rows.length/count)][1]);
const sets = [
 {name:'cases', inputs:JSON.parse(readFileSync(join(root,'tsfront/testdata/stmtscorpus/cases.json'),'utf8'))},
 {name:'corpus', inputs:paths.map(path=>({name:path, abap:readFileSync(path,'utf8')}))}
];
for(const set of sets)for(const input of set.inputs)if(Buffer.from(input.abap,'utf8').toString('utf8')!==input.abap)throw new Error('input cannot round-trip through UTF-8: '+input.name);
writeFileSync(join(work,'set.txt'),paths.join('\n')+'\n');
function once(s, old, next) {
 if (!s.includes(old) || s.indexOf(old)!==s.lastIndexOf(old)) throw new Error('unsupported generated harness: '+old);
 return s.replace(old,next);
}
const harnessFile = readdirSync(generated).find(f=>f.startsWith('z_harness_stat') && f.endsWith('.clas.abap'));
const original = readFileSync(join(generated,harnessFile),'utf8');
const originalOwner = original.match(/CLASS (\w+) DEFINITION/)[1];
const owner = 'zcl_stage_driver';
const [,dump,raw] = original.match(/CLASS-METHODS (\w+) IMPORTING (\w+) TYPE string RETURNING/);
const attrs = [...original.matchAll(/CLASS-DATA (\w+) TYPE (?:f|i|int8)\./g)].map(m=>m[1]);
const dumpBody = original.slice(original.indexOf(`METHOD ${dump}.`),original.indexOf('ENDMETHOD.',original.indexOf(`METHOD ${dump}.`))+10);
const calls = [...dumpBody.matchAll(/^t\d+ = t\d+->z_member_run_\w+\( .* \)\.$/gm)].map(m=>m[0]);
if(calls.length!==2 || attrs.length!==2) throw new Error('unsupported stage layout');
let timed = once(dumpBody, `METHOD ${dump}.`, `METHOD ${dump}.\nDATA stage_start TYPE i.\nDATA stage_finish TYPE i.`);
for (const [i,call] of calls.entries()) timed=once(timed,call,`GET RUN TIME FIELD stage_start.\n${call}\nGET RUN TIME FIELD stage_finish.\n${i===0?'lexer_us':'parser_us'} = stage_finish - stage_start.`);
// Stop after parser count: dumping trees is outside both measured stages and
// would add unnecessary wall time. The original differential stays untouched.
const lastCount = timed.indexOf(`${originalOwner}=>${attrs[1]} =`);
const endCount = timed.indexOf('\n',lastCount);
if(lastCount<0) throw new Error('missing statement count');
timed=timed.slice(0,endCount)+'\nCLEAR result.\nENDMETHOD.';
let harness=once(original,dumpBody,timed);
harness=once(harness,'PUBLIC SECTION.','PUBLIC SECTION.\nCLASS-DATA lexer_us TYPE i.\nCLASS-DATA parser_us TYPE i.');
harness=harness.replaceAll(originalOwner,owner);
// Same 180-character base64 literals and 290 KiB data-class budget as
// tools/allbackends/benchmark.mjs. The 500-file classes are reused verbatim.
function dataClasses(dest,payload) {
 let number=1, body='', files=[];
 const header=n=>`CLASS zcl_allb_data_${String(n).padStart(2,'0')} DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS append CHANGING cv_data TYPE string.\nENDCLASS.\nCLASS zcl_allb_data_${String(n).padStart(2,'0')} IMPLEMENTATION.\nMETHOD append.\n`;
 const footer='ENDMETHOD.\nENDCLASS.\n';
 const flush=()=>{const file=`zcl_allb_data_${String(number).padStart(2,'0')}.clas.abap`;writeFileSync(join(dest,file),header(number)+body+footer);files.push(file);number++;body='';};
 for(let offset=0;offset<payload.length;offset+=180){
  const line=`cv_data = cv_data && \`${payload.slice(offset,offset+180)}\`.\n`;
  if(Buffer.byteLength(header(number)+body+line+footer)>290*1024) flush();
  body+=line;
 }
 flush();return files;
}
try {
 for (const set of sets) {
  const dir=join(work,set.name);rmSync(dir,{recursive:true,force:true});mkdirSync(dir,{recursive:true});
  for(const f of readdirSync(generated).filter(f=>f.endsWith('.abap'))) copyFileSync(join(generated,f),join(dir,f));
  writeFileSync(join(dir,owner+'.clas.abap'),harness);
  let lexerMs=0,parserMs=0;
  const expected=[];
  for(const [index,input] of set.inputs.entries()){
   if(index % 25 === 0) console.error(`${set.name}: ${index}/${set.inputs.length}`);
   const file=new MemoryFile('zcorpus.prog.abap',input.abap);
   const lexer=new Lexer(),parser=new StatementParser(Release.v758);
   let start=performance.now();const lexed=lexer.run(file);lexerMs+=performance.now()-start;
   start=performance.now();const parsed=parser.run([{file,tokens:lexed.tokens}],[]);parserMs+=performance.now()-start;
   expected.push({tokens:lexed.tokens.length,statements:parsed[0].statements.length});
  }
  set.expected=expected;set.lexerMs=lexerMs;set.parserMs=parserMs;
  set.tokens=expected.reduce((n,e)=>n+e.tokens,0);set.statements=expected.reduce((n,e)=>n+e.statements,0);
  if(set.name==='cases'&&(set.tokens!==5229||set.statements!==926))throw new Error('64-case Node oracle changed');
  set.tokenChecksum=expected.reduce((n,e,i)=>n+(i+1)*e.tokens,0);set.statementChecksum=expected.reduce((n,e,i)=>n+(i+1)*e.statements,0);
  const payload=corpusZip(set.inputs.map(i=>i.abap),set.inputs.map(i=>i.name)).toString('base64');
  let dataFiles;
  const existing=join(scratch,'allb/work-500/abap');
  if(set.name==='corpus'&&count===500){
   const previous=readFileSync(join(scratch,'allb/work-500/set.txt'),'utf8');
   if(previous!==paths.join('\n')+'\n') throw new Error('500-file selection differs from allbackends');
   dataFiles=readdirSync(existing).filter(f=>/^zcl_allb_data_\d+\.clas\.abap$/.test(f)).sort();
   const previousPayload=dataFiles.map(f=>[...readFileSync(join(existing,f),'utf8').matchAll(/cv_data = cv_data && `([^`]*)`\./g)].map(m=>m[1]).join('')).join('');
   // Original allbackends used default ZIP names; preserve those exact classes.
   const originalPayload=corpusZip(set.inputs.map(i=>i.abap)).toString('base64');
   if(previousPayload!==originalPayload) throw new Error('existing corpus data does not match selection');
   for(const f of dataFiles) copyFileSync(join(existing,f),join(dir,f));
  } else dataFiles=dataClasses(dir,payload);
  writeFileSync(join(dir,'zcl_stage_timing.clas.abap'),`CLASS zcl_stage_timing DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-DATA lexer_us TYPE int8.
CLASS-DATA parser_us TYPE int8.
CLASS-DATA tokens TYPE i.
CLASS-DATA statements TYPE i.
CLASS-DATA token_checksum TYPE int8.
CLASS-DATA statement_checksum TYPE int8.
CLASS-METHODS run.
CLASS-METHODS read_le IMPORTING bytes TYPE xstring offset TYPE i size TYPE i RETURNING VALUE(value) TYPE i.
CLASS-METHODS result RETURNING VALUE(value) TYPE string.
ENDCLASS.
CLASS zcl_stage_timing IMPLEMENTATION.
METHOD run.
DATA base64 TYPE string.
DATA zip TYPE xstring.
DATA inputs TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
DATA cursor TYPE i.
DATA compressed_size TYPE i.
DATA name_size TYPE i.
DATA extra_size TYPE i.
DATA compressed TYPE xstring.
DATA content TYPE xstring.
DATA raw TYPE string.
DATA ignored TYPE string.
DATA index TYPE i.
DATA expected_tokens TYPE STANDARD TABLE OF i WITH DEFAULT KEY.
DATA expected_statements TYPE STANDARD TABLE OF i WITH DEFAULT KEY.
DATA expected_token TYPE i.
DATA expected_statement TYPE i.
CLEAR lexer_us.
CLEAR parser_us.
CLEAR tokens.
CLEAR statements.
CLEAR token_checksum.
CLEAR statement_checksum.
${expected.map(e=>`APPEND ${e.tokens} TO expected_tokens.\nAPPEND ${e.statements} TO expected_statements.`).join('\n')}
${dataFiles.map(f=>f.replace('.clas.abap','')+'=>append( CHANGING cv_data = base64 ).').join('\n')}
zip = cl_http_utility=>decode_x_base64( base64 ).
WHILE cursor + 30 <= xstrlen( zip ).
IF zip+cursor(4) <> '504B0304'.
EXIT.
ENDIF.
cl_abap_unit_assert=>assert_equals( act = read_le( bytes = zip offset = cursor + 6 size = 2 ) exp = 0 msg = 'ZIP flags' ).
cl_abap_unit_assert=>assert_equals( act = read_le( bytes = zip offset = cursor + 8 size = 2 ) exp = 8 msg = 'ZIP deflate' ).
compressed_size = read_le( bytes = zip offset = cursor + 18 size = 4 ).
name_size = read_le( bytes = zip offset = cursor + 26 size = 2 ).
extra_size = read_le( bytes = zip offset = cursor + 28 size = 2 ).
cursor = cursor + 30 + name_size + extra_size.
compressed = zip+cursor(compressed_size).
cl_abap_gzip=>decompress_binary( EXPORTING gzip_in = compressed IMPORTING raw_out = content ).
raw = cl_abap_codepage=>convert_from( content ).
APPEND raw TO inputs.
cursor = cursor + compressed_size.
ENDWHILE.
LOOP AT inputs INTO raw.
index = sy-tabix.
ignored = ${owner}=>${dump}( ${raw} = raw ).
lexer_us = lexer_us + ${owner}=>lexer_us.
parser_us = parser_us + ${owner}=>parser_us.
tokens = tokens + ${owner}=>${attrs[0]}.
statements = statements + ${owner}=>${attrs[1]}.
token_checksum = token_checksum + index * ${owner}=>${attrs[0]}.
statement_checksum = statement_checksum + index * ${owner}=>${attrs[1]}.
READ TABLE expected_tokens INDEX index INTO expected_token.
cl_abap_unit_assert=>assert_equals( act = sy-subrc exp = 0 msg = 'input cardinality' ).
READ TABLE expected_statements INDEX index INTO expected_statement.
cl_abap_unit_assert=>assert_equals( act = ${owner}=>${attrs[0]} exp = expected_token msg = 'per-input count' ).
cl_abap_unit_assert=>assert_equals( act = ${owner}=>${attrs[1]} exp = expected_statement msg = 'per-input count' ).
ENDLOOP.
cl_abap_unit_assert=>assert_equals( act = index exp = ${expected.length} msg = 'all inputs completed' ).
ENDMETHOD.
METHOD read_le.
DATA idx TYPE i.
DATA scale TYPE int8 VALUE 1.
DATA byte TYPE int8.
DO size TIMES.
idx = offset + sy-index - 1.
byte = bytes+idx(1).
value = value + byte * scale.
scale = scale * 256.
ENDDO.
ENDMETHOD.
METHOD result.
value = |STAGETIME ${set.name} ${expected.length} { tokens } { statements } { token_checksum } { statement_checksum }|.
value = value && | { lexer_us } { parser_us }|.
ENDMETHOD.
ENDCLASS.
`);
  writeFileSync(join(dir,'zcl_stage_timing.clas.testclasses.abap'),`CLASS ltcl_stage_timing DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS benchmark FOR TESTING.
ENDCLASS.
CLASS ltcl_stage_timing IMPLEMENTATION.
METHOD benchmark.
zcl_stage_timing=>run( ).
cl_abap_unit_assert=>assert_equals( act = zcl_stage_timing=>tokens exp = ${set.tokens} ).
cl_abap_unit_assert=>assert_equals( act = zcl_stage_timing=>statements exp = ${set.statements} ).
DATA expected_checksum TYPE int8.
expected_checksum = ${set.tokenChecksum}.
cl_abap_unit_assert=>assert_equals( act = zcl_stage_timing=>token_checksum exp = expected_checksum ).
expected_checksum = ${set.statementChecksum}.
cl_abap_unit_assert=>assert_equals( act = zcl_stage_timing=>statement_checksum exp = expected_checksum ).
cl_abap_unit_assert=>assert_equals( act = '' exp = 'telemetry' msg = zcl_stage_timing=>result( ) ).
ENDMETHOD.
ENDCLASS.
`);
 }
 const a4h=join(work,'a4h'),src=join(a4h,'src');rmSync(a4h,{recursive:true,force:true});mkdirSync(src,{recursive:true});
 for(const f of readdirSync(join(work,'corpus')).filter(f=>f.endsWith('.abap')))copyFileSync(join(work,'corpus',f),join(src,f));
 // Separate data-class namespace for the 64 inputs in the same SAP package.
 const casesDir=join(work,'cases');
 for(const f of readdirSync(casesDir).filter(f=>f.startsWith('zcl_allb_data_')||f.startsWith('zcl_stage_timing.'))){
  const dest=f.replace('zcl_allb_data_','zcl_case_data_').replace('zcl_stage_timing.','zcl_stage_cases.');
  writeFileSync(join(src,dest),readFileSync(join(casesDir,f),'utf8').replaceAll('zcl_allb_data_','zcl_case_data_').replaceAll('zcl_stage_timing','zcl_stage_cases'));
 }
 // SAP uses the same native ZIP reader as the original allbackends harness.
 // The CI workaround is confined to untimed input loading; stage code is shared.
 for(const name of ['zcl_stage_timing','zcl_stage_cases']) {
  const file=join(src,name+'.clas.abap');
  let source=readFileSync(file,'utf8');
  source=once(source,'DATA inputs TYPE STANDARD TABLE OF string WITH DEFAULT KEY.',
   'DATA inputs TYPE STANDARD TABLE OF string WITH DEFAULT KEY.\nDATA archive TYPE REF TO cl_abap_zip.\nDATA zip_file LIKE LINE OF archive->files.');
  const start=source.indexOf('WHILE cursor + 30 <= xstrlen( zip ).');
  const end=source.indexOf('LOOP AT inputs INTO raw.',start);
  if(start<0||end<0)throw new Error('unsupported ZIP loader layout');
  const load=`CREATE OBJECT archive.
archive->load( zip ).
LOOP AT archive->files INTO zip_file.
archive->get( EXPORTING name = zip_file-name IMPORTING content = content ).
raw = cl_abap_codepage=>convert_from( content ).
APPEND raw TO inputs.
ENDLOOP.
`;
  writeFileSync(file,source.slice(0,start)+load+source.slice(end));
 }
 writeFileSync(join(src,'zstagetime.prog.abap'), 'REPORT zstagetime.\nzcl_stage_timing=>run( ).\nWRITE / zcl_stage_timing=>result( ).\nzcl_stage_cases=>run( ).\nWRITE / zcl_stage_cases=>result( ).\n');
 abapgit(src,{reportName:'zstagetime',description:'Lexer and statement parser stage timing'});
 writeFileSync(join(a4h,'.abapgit.xml'),'<?xml version="1.0" encoding="utf-8"?><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values><DATA><MASTER_LANGUAGE>E</MASTER_LANGUAGE><STARTING_FOLDER>/src/</STARTING_FOLDER><FOLDER_LOGIC>FULL</FOLDER_LOGIC></DATA></asx:values></asx:abap>');
 writeFileSync(join(a4h,'package.devc.xml'),readFileSync(join(src,'package.devc.xml')));
 for(const f of readdirSync(src).filter(f=>f.endsWith('.xml')))writeFileSync(join(src,f),readFileSync(join(src,f),'utf8').replaceAll('All-backend lexer benchmark','Lexer and statement parser timing'));
 unlinkSync(join(src,'.abapgit.xml'));
 unlinkSync(join(src,'README.md'));
 writeFileSync(join(a4h,'README.md'), 'Import this folder into a disposable package with abapGit. Run ABAP Unit for ZCL_STAGE_TIMING (corpus) and ZCL_STAGE_CASES (64 cases). Each benchmark deliberately ends with one telemetry assertion failure after all correctness assertions pass. The message is STAGETIME set files tokens statements token_checksum statement_checksum lexer_us parser_us. Any earlier failure is invalid. Run ZSTAGETIME for spool output without the final telemetry assertion. ZIP decoding and checks are outside both stage timers. The original statement differential is also included and must pass.\n');
 for(const set of sets) for(const f of readdirSync(join(work,set.name)).filter(f=>f.endsWith('.abap')))if(readFileSync(join(work,set.name,f),'utf8').split('\n').some(l=>l.length>255))throw new Error('oversized ABAP line: '+f);
 writeFileSync(join(work,'node.json'),JSON.stringify(sets.map(({inputs,...s})=>({...s,files:inputs.length})),null,2));
 console.log(JSON.stringify(sets.map(({inputs,expected,...s})=>({...s,files:inputs.length})),null,2));
} finally {dispose();}
