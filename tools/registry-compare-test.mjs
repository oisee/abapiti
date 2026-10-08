import {strict as assert} from "node:assert";
import {mkdtempSync, writeFileSync, rmSync} from "node:fs";
import {join} from "node:path";
import {tmpdir} from "node:os";
import {test} from "node:test";
import {compareDirectories, firstDifference} from "./registry-compare.mjs";

test("dependency flag mutation fails even with unchanged supplied hashes", () => {
 const root = mkdtempSync(join(tmpdir(),"registry-compare-"));
 try {
  const original = {name:"IF_HTTP_CLIENT",type:"INTF",dependency:true,files:["if_http_client.intf.abap"],xml:null};
  for (const name of ["manifest","config","dumps","negative"]) writeFileSync(join(root,name+".json"),"{}\n");
  writeFileSync(join(root,"inventory.json"),JSON.stringify([original]));
  assert.equal(compareDirectories(root,root).verdict,"OK");
  assert.deepEqual(firstDifference([original],[{...original,dependency:false}]),{path:"$[0].dependency",expected:true,actual:false});
  // The full directory comparison also reads the mutated observations.
  const candidate = mkdtempSync(join(tmpdir(),"registry-mutant-"));
  try {
   for (const name of ["manifest","config","dumps","negative"]) writeFileSync(join(candidate,name+".json"),"{}\n");
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
