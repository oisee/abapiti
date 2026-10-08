import {strict as assert} from "node:assert";
import {mkdtempSync, writeFileSync, rmSync} from "node:fs";
import {join} from "node:path";
import {tmpdir} from "node:os";
import {test} from "node:test";
import {compareDirectories, firstDifference, validateObservations} from "./registry-compare.mjs";

const hash='a'.repeat(64);
export function fixture() {
 return {
 manifest:{upstreamPin:'a'.repeat(40),depsPin:'b'.repeat(40),configSHA256:hash,main:[{filename:'zprobe.prog.abap',bytes:14,sha256:hash}],dependencies:[],types:{INTF:1},objectCount:{total:1,normal:1,dependencies:0},abapFiles:1,registeredABAPFiles:1},
 config:{config:{global:{files:"src/*"},syntax:{version:"v702"},rules:{}},release:{name:'v702',ordinal:2},language:'Normal',rules:[]},
 dumps:[{filename:'zprobe.prog.abap',tokens:2,statements:1,structures:1,issues:0,lexerSHA256:hash,statementsSHA256:hash,structuresSHA256:hash,registered:true}],
 negative:{
 duplicate_default:{outcome:'throw',error:'Error',message:'probe'},duplicate_strict:{outcome:'throw',error:'Error',message:'probe'},missing_xml:{outcome:'throw',error:'Error',message:'probe'},malformed_xml:{outcome:'throw',error:'Error',message:'probe'},malformed_config:{outcome:'throw',error:'SyntaxError',message:'probe'},json5:{outcome:'return',value:{global:{files:'src/*'},syntax:{version:'v702'},rules:{}}},case_sensitivity:{outcome:'throw',error:'Error',message:'probe'},dependency_replaced:{outcome:'throw',error:'Error',message:'probe'},include:{outcome:'throw',error:'Error',message:'probe'}},
 inventory:[{name:'IF_HTTP_CLIENT',type:'INTF',dependency:true,files:['if_http_client.intf.abap'],xml:null,description:null}]
 };
}
function write(dir,data) {for(const [name,value] of Object.entries(data)) writeFileSync(join(dir,name+'.json'),JSON.stringify(value));}
test('empty or incomplete equal observations fail',()=>{
 const root=mkdtempSync(join(tmpdir(),'registry-empty-'));
 try {
  write(root,Object.fromEntries(['manifest','inventory','config','dumps','negative'].map(k=>[k,{}])));
  assert.throws(()=>compareDirectories(root,root),/incomplete/);
  for(const key of ['manifest','inventory','config','dumps','negative']) {
   const data=fixture(); data[key]=key==='inventory'||key==='dumps'?[]:{};
   assert.throws(()=>validateObservations(data),/incomplete/);
  }
  const data=fixture();delete data.inventory[0].dependency;
  assert.throws(()=>validateObservations(data),/incomplete/);
 } finally {rmSync(root,{recursive:true,force:true});}
});
test("dependency flag mutation fails even with unchanged supplied hashes", () => {
 const root = mkdtempSync(join(tmpdir(),"registry-compare-"));
 try {
  const original = {description:null,name:"IF_HTTP_CLIENT",type:"INTF",dependency:true,files:["if_http_client.intf.abap"],xml:null};
  write(root,fixture());
  writeFileSync(join(root,"inventory.json"),JSON.stringify([original]));
  assert.equal(compareDirectories(root,root).verdict,"OK");
  assert.deepEqual(firstDifference([original],[{...original,dependency:false}]),{path:"$[0].dependency",expected:true,actual:false});
  // The full directory comparison also reads the mutated observations.
  const candidate = mkdtempSync(join(tmpdir(),"registry-mutant-"));
  try {
   write(candidate,fixture());
   writeFileSync(join(candidate,"inventory.json"),JSON.stringify([{...original,dependency:false}]));
   writeFileSync(join(candidate,"checksums.json"),"{}\n");
   const result = compareDirectories(root,candidate);
   assert.equal(result.verdict,"MISMATCH");
   assert.equal(result.comparisons[1].difference.path,"$[0].dependency");
  } finally { rmSync(candidate,{recursive:true,force:true}); }
 } finally { rmSync(root,{recursive:true,force:true}); }
});
test("metadata key order is immaterial; filename case and file order are observable", () => {
 assert.equal(firstDifference({a:1,b:2},{b:2,a:1}),null);
 assert.notEqual(firstDifference(["a.abap","b.abap"],["b.abap","a.abap"]),null);
 assert.notEqual(firstDifference("ZCASE.PROG.ABAP","zcase.prog.abap"),null);
 assert.notEqual(firstDifference({xml:null},{}),null);
});

test('documented subset divergences require the specified loud failures', () => {
 const root=mkdtempSync(join(tmpdir(),'registry-scope-'));
 const candidate=mkdtempSync(join(tmpdir(),'registry-subset-'));
 try {
  const original=fixture();
  original.negative.malformed_xml={outcome:'return',value:original.inventory};
  write(root,original);
  const translated=structuredClone(original);
  translated.negative.json5={outcome:'throw',error:'RegistryJSONSubsetError',message:'JSON5-only syntax'};
  translated.negative.malformed_config={outcome:'throw',error:'RegistryJSONSubsetError',message:'invalid JSON'};
  translated.negative.malformed_xml={outcome:'throw',error:'RegistryXMLSubsetError',message:'mismatched end tag'};
  write(candidate,translated);
  assert.equal(compareDirectories(root,candidate).verdict,'MISMATCH');
  const result=compareDirectories(root,candidate,{translated:true});
  assert.equal(result.verdict,'OK');
  assert.equal(result.divergences.length,3);
  assert(result.divergences.every(d=>d.pass && d.reason));
  for (const name of ['json5','malformed_config','malformed_xml']) {
   for (const bad of [original.negative[name],{outcome:'throw',error:'Error',message:'unrelated failure'},
    {...translated.negative[name],message:''}]) {
    const mutant=structuredClone(translated);mutant.negative[name]=bad;write(candidate,mutant);
    assert.equal(compareDirectories(root,candidate,{translated:true}).verdict,'MISMATCH');
   }
  }
  translated.inventory[0].dependency=false;
  write(candidate,translated);
  assert.equal(compareDirectories(root,candidate,{translated:true}).verdict,'MISMATCH');
 } finally {rmSync(root,{recursive:true,force:true});rmSync(candidate,{recursive:true,force:true});}
});
