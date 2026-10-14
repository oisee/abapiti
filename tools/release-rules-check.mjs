#!/usr/bin/env node
// Fresh pinned Node oracle vs the actual standalone release clients.
// Invoke under the caller's heavy-job lock; never contacts SAP.
import {buildUpstream} from './statements-upstream.mjs';
import {createRequire} from 'node:module';
import {mkdirSync, readFileSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {spawnSync} from 'node:child_process';
const [upstreamArg, outArg, ...clients] = process.argv.slice(2);
if (!clients.length) throw new Error('usage: release-rules-check.mjs UPSTREAM OUTPUT CLIENT...');
const upstream=resolve(upstreamArg), out=resolve(outArg), require=createRequire(import.meta.url);
mkdirSync(out,{recursive:true});
const built=buildUpstream(upstream);
try {
 const ts=require(join(upstream,'node_modules/typescript'));
 const source=readFileSync(new URL('../tsfront/testdata/registrycorpus/harness/registry_run.ts',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText.replace(/require\("\.\.\/src\/([^" ]+)"\)/g,(_,p)=>`require(${JSON.stringify(join(built.core,p+'.js'))})`);
 const harness=join(out,'node-harness.cjs');writeFileSync(harness,js);
 const {RegistryRun}=require(harness);
 const filename='zsample.prog.abap', raw="report zsample.\nwrite 'hello'\n.\n\n\n\n\n\n";
 writeFileSync(join(out,filename),raw);
 const results=[];
 for(const rule of ['keyword_case','line_only_punc','sequential_blank']) {
  const config=JSON.stringify({global:{files:'**/*.abap'},syntax:{version:'v750'},rules:{[rule]:true}});
  const configPath=join(out,rule+'.json');writeFileSync(configPath,config);
  const oracle=new RegistryRun();oracle.addFile(filename,raw);
  const expected=oracle.run(config)+'\n';
  if(Number(expected.split('\n')[0])<=0) throw new Error(rule+': sample must exercise a real issue');
  writeFileSync(join(out,rule+'-node.txt'),expected);
  for(const clientArg of clients) {
   const client=resolve(clientArg);
   const r=spawnSync(client,['--file',join(out,filename),'--config',configPath,'-allow-read',out],{encoding:'utf8',maxBuffer:16*1024*1024});
   const actual=(r.stdout||'').replace(/[ \t]+$/gm,'');
   writeFileSync(join(out,rule+'-'+clients.indexOf(clientArg)+'.txt'),actual);
   if(r.error||r.status!==0||actual!==expected||/zlnt_exception_un_|refused|trap/i.test(r.stderr||'')) throw new Error(`${rule}: ${client} differs/refuses (exit ${r.status})\n${r.stdout}\n${r.stderr}\nNode:\n${expected}`);
   results.push({rule,client:client.split('/').pop(),issues:Number(expected.split('\n')[0]),equal:true});
  }
 }
 writeFileSync(join(out,'results.json'),JSON.stringify(results,null,2)+'\n');
 console.log(JSON.stringify(results,null,2));
} finally {built.dispose()}
