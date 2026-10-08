#!/usr/bin/env node
import {readFileSync,writeFileSync} from "node:fs";
import {createRequire} from "node:module";
import {buildUpstream} from "./statements-upstream.mjs";
const [cases,out,upstream] = process.argv.slice(2);
if (!out) throw new Error("usage: memory-file-oracle.mjs cases.json dumps.json [upstream]");
const {core,dispose} = buildUpstream(upstream);
try {
 const {MemoryFile} = createRequire(import.meta.url)(core+"/files/memory_file.js");
 const dumps = JSON.parse(readFileSync(cases,"utf8")).map(c => {
  const file = new MemoryFile(c.filename,c.raw);
  const rows = file.getRawRows();
  return {...c,dump:file.getFilename()+"\n"+file.getObjectName()+"\n"+(file.getObjectType() ?? "<undefined>")+"\n"+file.getRaw()+"\n"+rows.length+rows.map(row => "\n"+row.length+":"+row).join("")};
 });
 writeFileSync(out,JSON.stringify(dumps,null,2)+"\n");
 console.error(JSON.stringify({cases:dumps.length}));
} finally { dispose(); }
