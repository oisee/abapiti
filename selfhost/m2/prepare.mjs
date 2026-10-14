// Release-shaped paired emission from a frozen compiler archive, never a download.
import {existsSync, mkdirSync, writeFileSync, openSync, closeSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {execFileSync, spawnSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';
import {verifyUpstream, upstreamPin} from '../../tools/statements-upstream.mjs';
export const compilerPin = '953eae439f7fe5d39fcf6a4f4d8f4fb8d6a262ab';
export const work = resolve('.tmp/selfhost-m2');
if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
 const upstream = resolve(process.argv[2] || join(work,'abaplint'));
 verifyUpstream(upstream);
 const source=join(work,'compiler');
 if(existsSync(source)) throw new Error('remove .tmp/selfhost-m2/compiler and a4h before a fresh emission');
 mkdirSync(source,{recursive:true});
 execFileSync('tar',['-x','-C',source],{input:execFileSync('git',['archive',compilerPin],{maxBuffer:128*1024*1024})});
 const env={...process.env,GOFLAGS:'-buildvcs=false',CGO_ENABLED:'0',GOMAXPROCS:'8',NODE_OPTIONS:'--max-old-space-size=24576'};
 // Release toolchain; caller places Go 1.26.0 on PATH (no host home paths).
 const go=execFileSync('go',['version'],{encoding:'utf8'}).trim();
 if(!go.includes('go1.26.0 ')) throw new Error('release-shaped emission requires Go 1.26.0');
 execFileSync('npm',['ci','--ignore-scripts','--no-audit','--no-fund'],{cwd:source,env,stdio:'inherit'});
 const log=openSync(join(work,'emission.log'),'w'), start=performance.now();
 const p=spawnSync(process.execPath,['tools/a4h-tests-build.mjs',upstream,join(work,'a4h')],{cwd:source,env,stdio:['ignore',log,log],timeout:30*60*1000});
 closeSync(log);
 const record={schema:'selfhost-emission/1',compilerCommit:compilerPin,abaplintPin:upstreamPin,go,node:process.version,exitCode:p.status,signal:p.signal,error:p.error?.code,wallSeconds:(performance.now()-start)/1000,command:'node tools/a4h-tests-build.mjs UPSTREAM OUTPUT',log:'.tmp/selfhost-m2/emission.log'};
 writeFileSync(join(work,'emission.json'),JSON.stringify(record,null,2)+'\n');
 if(p.status!==0)throw new Error('paired emission failed; see .tmp/selfhost-m2/emission.log');
 console.log(JSON.stringify(record,null,2));
 execFileSync(process.execPath,[join(source,'tools/a4h-tests-parts.mjs'),join(work,'a4h'),join(work,'test-parts')],{stdio:'inherit'});
 execFileSync('python3',['selfhost/m2/inventory.py'],{stdio:'inherit'});
}
