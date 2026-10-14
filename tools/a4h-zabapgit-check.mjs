#!/usr/bin/env node
// Existing OSG native CLI + existing kit, with external output and the same
// bounded compiler flags as the test harness. Never edits OSG or contacts SAP.
import {execFileSync} from 'node:child_process';
import {cpSync,mkdirSync,readFileSync,writeFileSync,chmodSync,rmSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {externalPath} from './osg-harness-cache.mjs';

const shellQuote=s=>"'"+s.replaceAll("'","'\"'\"'")+"'";
export function goBuildShim(go) {
 return `#!/bin/sh
if [ "$1" = build ]; then
 shift
 exec ${shellQuote(go)} build -p 1 '-gcflags=osg/gogen/cmd/osabap=-N -l -dwarf=false' "$@"
fi
exec ${shellQuote(go)} "$@"
`;
}

if(process.argv[1] && import.meta.url===pathToFileURL(resolve(process.argv[1])).href) {
 const [buildArg,osgArg,kitArg,outArg]=process.argv.slice(2);
 if(!outArg) throw new Error('usage: a4h-zabapgit-check.mjs BUILD OSG KIT OUTPUT');
 const build=resolve(buildArg),osg=resolve(osgArg),kit=resolve(kitArg),out=externalPath(outArg,osg);
 const pin=readFileSync(new URL('../.github/ci/osgo.ref',import.meta.url),'utf8').trim();
 const git=(...args)=>execFileSync('git',['-C',osg,...args],{encoding:'utf8'}).trim();
 if(git('rev-parse','HEAD')!==pin||git('status','--porcelain')) throw new Error('OSG must be clean at '+pin);
 mkdirSync(out,{recursive:true});
 const bin=join(out,'bin');mkdirSync(bin,{recursive:true});
 const go=execFileSync('which',['go'],{encoding:'utf8'}).trim();
 const shim=join(bin,'go');writeFileSync(shim,goBuildShim(go));chmodSync(shim,0o755);
 try {
  const env={...process.env,PATH:bin+':'+process.env.PATH,OSABAP_BUILD_ROOT:join(out,'native'),GOMAXPROCS:'8',GOGEN_NOLINE:'1',NODE_OPTIONS:'--max-old-space-size=24576'};
  execFileSync(process.execPath,['--stack-size=500000',join(osg,'tools/gogen/osabap.mjs'),join(build,'native/zabaplint.prog.abap'),'--lib',join(build,'native/lib')],{cwd:osg,env,stdio:'inherit'});
  cpSync(kit,join(out,'kit'),{recursive:true});
  execFileSync('sh',[join(out,'kit/check.sh'),join(out,'native/.out/osabap')],{env,stdio:'inherit'});
  if(git('status','--porcelain')) throw new Error('OSG is no longer clean');
 } finally {rmSync(bin,{recursive:true,force:true})}
}
