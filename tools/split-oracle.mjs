#!/usr/bin/env node
// JavaScript built-in behavior; does not import the translated harness.
import {readFileSync,writeFileSync} from "node:fs";
const [cases,out] = process.argv.slice(2);
if (!out) throw new Error("usage: split-oracle.mjs cases.json dumps.json");
const data = JSON.parse(readFileSync(cases,"utf8")).map(c => {
 const rows = c.raw.split(c.separator);
 return {...c,dump:String(rows.length)+rows.map(row => "\n"+row.length+":"+row).join("")};
});
writeFileSync(out,JSON.stringify(data,null,2)+"\n");
