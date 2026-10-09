#!/usr/bin/env node
import {createRequire} from 'node:module';
import {buildUpstream} from './statements-upstream.mjs';
const original=buildUpstream(process.argv[2]);
try {
 const require=createRequire(import.meta.url);
 const types=require(original.core+'/abap/types/basic/index.js');
 const observations=['CGenericType','CLikeType','PGenericType','SimpleType','XGenericType'].map(name=>{
  const constructor=types[name];
  const first=constructor.get();
  return {name,identity:first===constructor.get(),instance:first instanceof constructor,text:first.toText(0),generic:first.isGeneric()};
 });
 console.log(JSON.stringify(observations,null,2));
} finally {original.dispose();}
