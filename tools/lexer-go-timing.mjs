#!/usr/bin/env node
// One tool, one machine; excludes compilation/startup and token-dump formatting.
import {readFileSync,mkdtempSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createRequire} from 'node:module';
import {spawnSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const core=resolve(process.env.TSFRONT_ABAPLINT || '/home/alice/dev/abaplint/packages/core');
const iterations=Number(process.argv[2] || 100);
if (!Number.isInteger(iterations)||iterations<1) throw new Error('iterations must be positive');
const require=createRequire(import.meta.url);
const {Lexer}=require(join(core,'build/src/abap/1_lexer/lexer.js'));
const {MemoryFile}=require(join(core,'build/src/files/memory_file.js'));
const cases=JSON.parse(readFileSync(join(root,'tsfront/testdata/lexercorpus/cases.json'),'utf8')).filter(c=>c.name.startsWith('real_'));
const oracle=JSON.parse(readFileSync(join(root,'tsfront/testdata/lexercorpus/tokens.json'),'utf8'));
if(cases.length!==3) throw new Error('expected three real corpus files');
const dir=mkdtempSync(join(tmpdir(),'abapiti-lexer-timing-'));
try {
 const binary=join(dir,'lexer');
 const build=spawnSync('go',['test','./tsfront','-run','^TestPrepareGoLexerTiming$','-count=1'],{cwd:root,env:{...process.env,ABAPITI_GO_LEXER_BENCH:binary},encoding:'utf8'});
 if(build.status!==0)throw new Error(build.stdout+build.stderr);
 const run=spawnSync(binary,[],{input:JSON.stringify({Cases:cases,Iterations:iterations}),encoding:'utf8'});
 if(run.status!==0)throw new Error(run.stdout+run.stderr);
 const go=JSON.parse(run.stdout);
 console.log(`30 warmups; ${iterations} measured runs/file; milliseconds/run; compilation and startup excluded`);
 console.log('file\tGo\tNode\tGo/Node\ttokens');
 for(const [i,c] of cases.entries()) {
  const lex=()=>new Lexer().run(new MemoryFile('zcorpus.prog.abap',c.abap)).tokens.length;
  for(let j=0;j<30;j++)lex();
  const start=performance.now();let tokens=0;
  for(let j=0;j<iterations;j++)tokens=lex();
  const node=(performance.now()-start)/iterations;
  const expected=oracle.find(o=>o.name===c.name).tokens;
  if(tokens!==expected||go[i].tokens!==expected)throw new Error(`${c.name}: token count mismatch`);
  console.log(`${c.name}\t${go[i].milliseconds.toFixed(3)}\t${node.toFixed(3)}\t${(go[i].milliseconds/node).toFixed(2)}\t${tokens}`);
 }
} finally {rmSync(dir,{recursive:true,force:true})}
