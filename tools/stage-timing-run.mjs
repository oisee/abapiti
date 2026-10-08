#!/usr/bin/env node
// Run the clean pinned CI entry points; stdout is a comparison table.
// Each benchmark command has a 30-minute budget, then forced group cleanup.
// Passing assertion messages are discarded by both runners. The benchmark's
// final, deliberately failing assertion exports telemetry only AFTER every
// input and aggregate correctness check has passed. Any other failure is invalid.
import {readFileSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {spawnSync,execFileSync} from 'node:child_process';
const [workArg, ...runtimeArgs] = process.argv.slice(2);
if(!workArg)throw new Error('usage: stage-timing-run.mjs WORK [osgo|osgjs ...]');
const work=resolve(workArg),osg=join(work,'open-steamgate');
const runtimes=runtimeArgs.length?runtimeArgs:['osgo','osgjs'];
if(runtimes.some(r=>!['osgo','osgjs'].includes(r)))throw new Error('unknown runtime');
const pin=readFileSync(new URL('../.github/ci/osgo.ref',import.meta.url),'utf8').trim();
const env={PATH:process.env.PATH,HOME:process.env.HOME,GOCACHE:process.env.GOCACHE||'/tmp/abapiti-stage-go-cache',GOTOOLCHAIN:'go1.26.0',GOFLAGS:'-buildvcs=false'};
function clean(){
 const git=(...args)=>execFileSync('git',args,{cwd:osg,env,encoding:'utf8'}).trim();
 if(git('rev-parse','HEAD')!==pin||git('status','--porcelain','--untracked-files=all'))throw new Error('runtime must be clean at CI pin');
}
clean();
const expected=JSON.parse(readFileSync(join(work,'node.json'),'utf8'));
const rows=[];
for(const runtime of runtimes)for(const set of expected){
 const label=`${runtime}-${set.name}`;
 console.error(`${label}: ${set.files} files`);
 const child=spawnSync('timeout',['--foreground','--kill-after=5s','1800s','npm','run','-s',`${runtime}:unit`,'--',join(work,set.name),'--json','--class','ZCL_STAGE_TIMING'],{cwd:osg,env,encoding:'utf8',maxBuffer:64*1024*1024});
 writeFileSync(join(work,`${label}.json`),child.stdout??'');
 writeFileSync(join(work,`${label}.err`),child.stderr??'');
 let value;
 try{
  if(child.error)throw child.error;
  if(child.status===124||child.status===137)throw new Error('runner exceeded the 1800-second command budget');
  const result=JSON.parse(child.stdout);
  const t=result.totals;
  if(child.status!==1||!t||t.tests!==1||t.failure!==1||t.error||t.not_compiled||t.skipped||result.rows?.length!==1||result.overrides?.length)throw new Error('unexpected runner verdict: '+JSON.stringify(result.rows));
  const row=result.rows[0];
  if(row.status!=='FAILURE'||row.class!=='ZCL_STAGE_TIMING'||row.method!=='BENCHMARK')throw new Error('unexpected telemetry row');
  const m=/STAGETIME (cases|corpus) (\d+) (\d+) (\d+) (\d+) (\d+) (\d+) (\d+)/.exec(row.message);
  if(!m||m[1]!==set.name)throw new Error('missing final telemetry: '+row.message);
  const [files,tokens,statements,tokenChecksum,statementChecksum,lexerUs,parserUs]=m.slice(2).map(Number);
  if([files,tokens,statements,tokenChecksum,statementChecksum].some((v,i)=>v!==[set.files,set.tokens,set.statements,set.tokenChecksum,set.statementChecksum][i]))throw new Error('Node count/checksum mismatch');
  value={runtime,set:set.name,files,tokens,statements,tokenChecksum,statementChecksum,lexerMs:lexerUs/1000,parserMs:parserUs/1000,valid:true};
 }catch(error){value={runtime,set:set.name,files:set.files,valid:false,error:error.message};}
 rows.push(value);console.error(JSON.stringify(value));
}
clean();
writeFileSync(join(work,'runtime-results.json'),JSON.stringify(rows,null,2));
console.log('| runtime | files | lexer ms | parser ms | parser/lexer | lexer vs Node | parser vs Node |');
console.log('|---|---:|---:|---:|---:|---:|---:|');
for(const set of expected){
 const values=[{runtime:`Node (${set.name})`,files:set.files,...set,valid:true},...rows.filter(r=>r.set===set.name)];
 for(const r of values)console.log(r.valid?`| ${r.runtime} | ${r.files} | ${r.lexerMs.toFixed(3)} | ${r.parserMs.toFixed(3)} | ${(r.parserMs/r.lexerMs).toFixed(2)} | ${(r.lexerMs/set.lexerMs).toFixed(2)} | ${(r.parserMs/set.parserMs).toFixed(2)} |`:`| ${r.runtime} | ${r.files} | INVALID | ${r.error.replaceAll('|','/')} | — | — | — |`);
}
if(rows.some(r=>!r.valid))process.exitCode=1;
