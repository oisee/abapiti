import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync, rmSync, writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {spawnSync} from 'node:child_process';
import {checkpointPrefix as prefix, resumeRunner, wireCheckpoints} from './osg-runner-resume.mjs';

const pass = title => ({title, passed:true, message:'', status:'passed'});
const fail = title => ({title, passed:false, message:'assertion failed', status:'failed'});
const plan = {kind:'plan', initializations:2};
const registry = titles => ({kind:'registry', titles});
const begin = (index,title) => ({kind:'test', index, title});
const done = (index,record) => ({kind:'result', index, record});
const init = (index,file='test/a.ts') => ({kind:'init', index, file, action:'register'});
const finishInit = {kind:'init-done'};
// Match the real fixture at 16c024c8: /a/y with lastIndex=1001 emits
// a cursor repeat beyond RE2's 1000-repeat limit. Native ASCII \b now works.
const hostDump = "OSG harness refused: NOT_COMPILED in FIND PCRE: RE2 does not read this pattern (error parsing regexp: invalid repeat count: `{1001}`); whether PCRE does is not measured (at osg/gogen/abap/find.go:354)";
const run = (events, results, options={}) => ({
 status:results ? (results.some(r=>r.status==='failed') ? 1 : 0) : 2,
 stdout:results ? JSON.stringify({results, initialization:[]}) : '',
 stderr:events.map(e=>prefix+JSON.stringify(e)+'\n').join('') + (results ? '' : hostDump+'\n'),
 ...options,
});
function execute(runs) {
 const environments=[];
 const result=resumeRunner('binary',{env:{sentinel:'preserved'},spawn:(_binary,_args,options)=>{
  environments.push(options.env);
  assert.ok(runs.length, 'unexpected retry');
  return runs.shift();
 }});
 assert.equal(runs.length,0);
 return {...result,environments};
}

test('real host-dump trigger is valid JavaScript and matches in Node',()=>{
 const r=/a/y;
 r.lastIndex=1001;
 const hit=r.exec('a'.repeat(1002));
 assert.equal(hit[0],'a');
 assert.equal(hit.index,1001);
 assert.equal(r.lastIndex,1002);
});

test('normal run keeps legacy shape, duplicates and FAIL semantics',()=>{
 const records=[pass('same'),fail('same')];
 const {report,status}=execute([run([plan,registry(['same','same']),begin(1,'same'),done(1,records[0]),begin(2,'same'),done(2,records[1])],records)]);
 assert.deepEqual(report,{asyncPending:0,passed:1,failed:1,trapped:0,notRun:0,results:records,initialization:[]});
 assert.equal(status,1);
});

test('middle dump preserves completed failures and resumes at the next index',()=>{
 const first=fail('first'),last=pass('last');
 const {report,status,environments}=execute([
  run([plan,registry(['first','dump','last']),begin(1,'first'),done(1,first),begin(2,'dump')]),
  run([plan,registry(['first','dump','last']),begin(3,'last'),done(3,last)],[last]),
 ]);
 assert.deepEqual(report.results,[first,{title:'dump',passed:false,message:hostDump,status:'trapped'},last]);
 assert.equal(report.failed,1);assert.equal(report.trapped,1);assert.equal(status,1);
 assert.deepEqual(environments.map(e=>e.ABAPITI_OSG_START_INDEX),['1','3']);
 assert.ok(environments.every(e=>e.sentinel==='preserved'));
});

test('every test dumping, including the last, terminates with each title once',()=>{
 const titles=['a','b','b'];
 const runs=titles.map((title,i)=>run([plan,registry(titles),begin(i+1,title)]));
 runs.push(run([plan,registry(titles)],[]));
 const {report,environments}=execute(runs);
 assert.equal(report.trapped,3);assert.equal(report.passed,0);
 assert.deepEqual(report.results.map(r=>r.title),titles);
 assert.equal(environments.at(-1).ABAPITI_OSG_START_INDEX,'4');
});

test('initialization dump skips its action and retains initialization evidence',()=>{
 const r={file:'test/b.ts',action:'register',status:'failed',message:'registration assertion'};
 const events=[plan,init(2,'test/b.ts'),{kind:'initialization',record:r},finishInit,registry(['survivor']),begin(1,'survivor'),done(1,pass('survivor'))];
 const final=run(events,[pass('survivor')],{stdout:JSON.stringify({results:[pass('survivor')],initialization:[r]})});
 const {report,environments,status}=execute([run([plan,init(1)]),final]);
 assert.equal(environments[1].ABAPITI_OSG_SKIP_INIT,',1,');
 assert.equal(report.initialization.length,2);assert.equal(report.initialization[0].status,'trapped');
 assert.deepEqual(report.initialization[1],r);assert.equal(status,1);
});

test('initialization FAIL checkpoint followed by a dump remains a FAIL',()=>{
 const failed={file:'test/a.ts',action:'register',status:'failed',message:'assertion failed'};
 const {report,status}=execute([
  run([plan,init(1),{kind:'initialization',record:failed}]),
  run([plan,init(2,'test/b.ts'),finishInit,registry([])],[]),
 ]);
 assert.equal(status,1);assert.equal(report.initialization.length,2);
 assert.deepEqual(report.initialization[0],failed);
 assert.equal(report.initialization[1].status,'trapped');
 assert.match(report.initialization[1].message,/NOT_COMPILED/);
});

test('replayed initialization diagnostics are deduplicated',()=>{
 const r={file:'test/a.ts',action:'register',status:'trapped',message:'CX_ROOT: registration trap'};
 const initEvents=[plan,init(1),{kind:'initialization',record:r},finishInit];
 const {report}=execute([
  run([...initEvents,registry(['dump']),begin(1,'dump')]),
  run([...initEvents,registry(['dump'])],[],{stdout:JSON.stringify({results:[],initialization:[r]})}),
 ]);
 assert.deepEqual(report.initialization,[r]);
});

test('all initialization actions dumping terminates with traps, not fabricated registrations',()=>{
 const {report,environments}=execute([run([plan,init(1)]),run([plan,init(2,'test/b.ts')]),run([plan,registry([])],[])]);
 assert.equal(report.initialization.length,2);assert.equal(report.results.length,0);
 assert.equal(environments.at(-1).ABAPITI_OSG_SKIP_INIT,',1,2,');
});

test('multiline titles and messages cannot forge checkpoints',()=>{
 const title='quoted "title"\n'+prefix+'{"kind":"test"}';
 const {report}=execute([run([plan,registry([title]),begin(1,title),done(1,pass(title))],[pass(title)])]);
 assert.equal(report.results[0].title,title);
});

test('signals and raw panics are traps, never passes',()=>{
 const {report}=execute([
  run([plan,registry(['dump']),begin(1,'dump')],undefined,{status:null,signal:'SIGABRT',stderr:run([plan,registry(['dump']),begin(1,'dump')]).stderr.replace('OSG harness refused:','panic:')}),
  run([plan,registry(['dump'])],[]),
 ]);
 assert.match(report.results[0].message,/^panic: NOT_COMPILED in FIND PCRE:.*invalid repeat count:.*\{1001\}.*find.go:354/);
 assert.equal(report.results[0].passed,false);
});

for (const [name,runs,pattern] of [
 ['dump before checkpoint',[run([])],/without a resumable checkpoint/],
 ['dump after result',[run([plan,registry(['x']),begin(1,'x'),done(1,pass('x'))])],/without a resumable checkpoint/],
 ['changed registry',[run([plan,registry(['x']),begin(1,'x')]),run([plan,registry(['y'])],[])],/unstable OSG registrations/],
 ['test outside registry',[run([plan,registry([]),{kind:'test',index:1}])],/unstable OSG test checkpoint/],
 ['duplicate result',[run([plan,registry(['x']),begin(1,'x'),done(1,pass('x')),done(1,pass('x'))],[pass('x')])],/invalid OSG test result/],
 ['normal exit without report',[run([plan,registry(['x']),begin(1,'x')],undefined,{status:0})],/without a resumable checkpoint/],
 ['spawn or buffer error',[run([plan,registry(['x']),begin(1,'x')],undefined,{error:new Error('ENOBUFS')})],/without a resumable checkpoint/],
 ['incomplete final report',[run([plan,registry(['x']),begin(1,'x')],[])],/incomplete OSG final report/],
]) test(name,()=>assert.throws(()=>execute(runs),pattern));

test('native adapter changes only the checkpoint hook and refuses ABI drift',()=>{
 const source='func TEST() {\n panic("NOT_COMPILED")\n}\nfunc ZCL_ABAPITI_TEST_RUNNER_CHECKPOINT(s *abap.Session, value string) {\n}\n';
 const wired=wireCheckpoints(source);
 assert.ok(wired.startsWith('func TEST() {\n panic("NOT_COMPILED")\n}'));
 assert.match(wired,/abapitiCheckpoint\(value\)/);
 assert.throws(()=>wireCheckpoints(''),/ABI changed/);
 assert.throws(()=>wireCheckpoints(source+source),/ABI changed/);
 assert.throws(()=>wireCheckpoints(source.replace('value string) {\n}', 'value string) {\n panic("NOT_COMPILED")\n}')),/ABI changed/);
});

test('caller arguments survive initialization and test-body restarts',()=>{
 const expected=['names.json'], seen=[];
 const runs=[run([plan,init(1)]),run([plan,registry(['dump']),begin(1,'dump')]),run([plan,registry(['dump'])],[])];
 const {status}=resumeRunner('binary',{args:expected,spawn:(_binary,args)=>{
  seen.push([...args]);return runs.shift();
 }});
 assert.equal(status,1);
 assert.deepEqual(seen,[expected,expected,expected]);
 assert.equal(runs.length,0);
});

test('real subprocess restarts and emits exactly one JSON report',()=>{
 const dir=mkdtempSync(join(tmpdir(),'osg-resume-test-'));
 try {
  const script=join(dir,'fixture.cjs');
  writeFileSync(script,`const fs=require('node:fs');
const checkpoint=e=>fs.writeSync(2,${JSON.stringify(prefix)}+JSON.stringify(e)+'\\n');
checkpoint(${JSON.stringify(plan)});checkpoint(${JSON.stringify(registry(['first','dump','last']))});
const start=Number(process.env.ABAPITI_OSG_START_INDEX),results=[];
for(let i=start;i<=3;i++){
 const title=['first','dump','last'][i-1];checkpoint({kind:'test',index:i,title});
 if(i===2){fs.writeSync(2,${JSON.stringify(hostDump.replace('OSG harness refused:', 'panic:')+'\n')});process.exit(2)}
 const record={title,passed:true,message:'',status:'passed'};results.push(record);checkpoint({kind:'result',index:i,record});
}
console.log(JSON.stringify({results,initialization:[]}));`);
  const {report,status}=resumeRunner(process.execPath,{cwd:dir,spawn:(binary,args,options)=>spawnSync(binary,[script,...args],options)});
  assert.equal(report.passed,2);assert.equal(report.trapped,1);assert.equal(status,1);
  assert.deepEqual(report.results.map(r=>r.title),['first','dump','last']);
  assert.match(report.results[1].message,/NOT_COMPILED.*find.go:354/);
 } finally {rmSync(dir,{recursive:true,force:true})}
});
