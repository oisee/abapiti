// Run only the clean CI pin. Mutations reproduce critic-r1's false passes.
import {execFileSync, spawnSync} from "node:child_process";
import {cpSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync} from "node:fs";
import {join, resolve} from "node:path";
import {assertPinnedTranspiler} from "./osg-transpiler.mjs";
const [workArg] = process.argv.slice(2);
if (!workArg) throw new Error("usage: lexer-unit.mjs <osgo workdir>");
const work = resolve(workArg);
const osg = join(work, "open-steamgate");
const root = resolve(import.meta.dirname, "../..");
const pin = readFileSync(join(root,".github/ci/osgo.ref"),"utf8").trim();
function clean() {
  if (execFileSync("git",["rev-parse","HEAD"],{cwd:osg,encoding:"utf8"}).trim() !== pin ||
      execFileSync("git",["status","--porcelain","--untracked-files=all"],{cwd:osg,encoding:"utf8"}).trim()) throw new Error("runtime must be clean at CI pin");
}
clean();
await assertPinnedTranspiler(osg);
const gen = join(work,"lexer-r1");
rmSync(gen,{recursive:true,force:true});
execFileSync("go",["test","./tsfront","-run","TestLowerLexerClosure|TestCriticR1Accepted","-count=1"],{
 cwd:root,env:{...process.env, ABAPITI_TEST_OUT:gen},stdio:"inherit"
});
execFileSync("node",[join(root,".github/ci/hir-lint.mjs"),gen,osg],{stdio:"inherit"});
function run(runtime, dir, label, pass, tests = 1) {
  const result = spawnSync("npm",["run","-s",`${runtime}:unit`,"--",dir,"--json"],{
    cwd:osg,encoding:"utf8",maxBuffer:64*1024*1024,
    env:{...process.env,GOTOOLCHAIN:"go1.26.0",GOFLAGS:"-buildvcs=false"}
  });
  writeFileSync(join(work,`lexer-r1-${runtime}-${label}.json`),result.stdout || "");
  writeFileSync(join(work,`lexer-r1-${runtime}-${label}.err`),result.stderr || "");
  if(result.error) throw result.error;
  const d=JSON.parse(result.stdout), t=d.totals;
  if (!d.rows || t.tests !== tests || d.rows.length !== tests || t.error || t.not_compiled || d.overrides?.length) throw new Error(`${runtime} ${label}: invalid runtime report`);
  if (pass ? result.status !== 0 || t.success !== tests || d.rows.some(r=>r.status!=="SUCCESS") : result.status !== 1 || t.failure !== tests || d.rows.some(r=>r.status!=="FAILURE")) throw new Error(`${runtime} ${label}: unexpected verdict`);
  console.log(`${runtime} ${label}: ${pass ? "PASS" : "expected FAIL"}${label === "baseline" ? " (44/44 cases, 4,663 tokens)" : ""}`);
}
for (const runtime of ["osgo","osgjs"]) {
 const corpus = gen;
 const testfile = readdirSync(corpus).find(n=>n.endsWith(".clas.testclasses.abap"));
 const original=readFileSync(join(corpus,testfile),"utf8");
 function mutation(label, edit) {
  const dir=join(work,`lexer-r1-${runtime}-${label}`);
  rmSync(dir,{recursive:true,force:true});mkdirSync(dir,{recursive:true});
  for (const file of readdirSync(corpus).filter(n=>n.endsWith(".abap"))) cpSync(join(corpus,file),join(dir,file));
  const changed=edit(original);
  if(changed===original) throw new Error(`mutation ${label} did not change source`);
  writeFileSync(join(dir,testfile),changed);
  return dir;
 }
 const early=mutation("early",s=>{ let seen=0; return s.replaceAll("executed = executed + 1.",line=>++seen===2 ? line+"\nRETURN." : line); });
 const skip=mutation("skip",s=>{
  const blocks=[...s.matchAll(/CLEAR raw\./g)].map(m=>m.index);
  if(blocks.length!==44) throw new Error("case cardinality in emitted driver");
  const block=s.slice(blocks[1],blocks[2]);
  if(!block.includes("case single_dot")) throw new Error("second block must be critic's single_dot case");
  return s.slice(0,blocks[1])+s.slice(blocks[2]);
 });
 const count=mutation("token-count",s=>s.replace(/(=>\S+ exp = )1( msg = msg )/,"$1999$2"));
 run(runtime,corpus,"baseline",true);
 run(runtime,early,"early",false);
 run(runtime,skip,"skip",false);
 run(runtime,count,"token-count",false);
 for (const probe of readdirSync(join(corpus,"critic-r1"))) {
  run(runtime,join(corpus,"critic-r1",probe),probe,true);
 }
}
clean();
