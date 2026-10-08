#!/usr/bin/env node
import {orderingDomains} from './registry-scope.mjs';
const root=new Intl.Collator('und',{numeric:false,sensitivity:'variant',ignorePunctuation:false,caseFirst:'false'});
const evidence=[];
for (const domain of orderingDomains) {
 const chars=[...domain.alphabet];
 const words=['',...chars,...chars.flatMap(a=>chars.map(b=>a+b))];
 const weights=new Map(chars.map((char,index)=>[char,index]));
 const compare=(a,b)=>{
  for(let i=0;i<Math.min(a.length,b.length);i++){
   const delta=weights.get(a[i])-weights.get(b[i]);
   if(delta)return Math.sign(delta);
  }
  return Math.sign(a.length-b.length);
 };
 let pairs=0;
 for(const a of words)for(const b of words){
  const expected=compare(a,b);
  if(Math.sign(root.compare(a,b))!==expected || Math.sign(a.localeCompare(b))!==expected)
   throw new Error(`ordering mismatch: ${domain.name} ${JSON.stringify(a)} / ${JSON.stringify(b)}`);
  pairs++;
 }
 evidence.push({...domain,pairs});
}
console.log(JSON.stringify({node:process.version,icu:process.versions.icu,collator:root.resolvedOptions(),domains:evidence},null,2));
