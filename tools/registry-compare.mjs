#!/usr/bin/env node
// Compare observations, never trust candidate-supplied checksums as evidence.
import {readFileSync} from "node:fs";
import {join, resolve} from "node:path";
import {pathToFileURL} from "node:url";
import {createHash} from "node:crypto";

export function firstDifference(expected, actual, path = "$") {
 if (expected === actual) return null;
 if (expected === null || actual === null || typeof expected !== typeof actual) return {path,expected,actual};
 if (typeof expected !== "object") return {path,expected,actual};
 if (Array.isArray(expected) !== Array.isArray(actual)) return {path,expected,actual};
 if (Array.isArray(expected)) {
  if (expected.length !== actual.length) return {path:path+".length",expected:expected.length,actual:actual.length};
  for (let i = 0; i < expected.length; i++) {
   const d = firstDifference(expected[i],actual[i],`${path}[${i}]`);
   if (d) return d;
  }
 } else {
  const keys = [...new Set([...Object.keys(expected),...Object.keys(actual)])].sort();
  for (const key of keys) {
   if (!Object.hasOwn(expected,key) || !Object.hasOwn(actual,key)) return {path:path+"."+key,expected:expected[key],actual:actual[key],missing:true};
   const d = firstDifference(expected[key],actual[key],path+"."+key);
   if (d) return d;
  }
 }
 return null;
}
export function canonical(value) {
 if (Array.isArray(value)) return "["+value.map(canonical).join(",")+"]";
 if (value !== null && typeof value === "object") return "{"+Object.keys(value).sort().map(k => JSON.stringify(k)+":"+canonical(value[k])).join(",")+"}";
 return JSON.stringify(value);
}
export function compareDirectories(expectedDir, actualDir) {
 const observations = ["manifest","inventory","config","dumps","negative"];
 const result = {verdict:"OK", comparisons:[]};
 for (const name of observations) {
  const expected = JSON.parse(readFileSync(join(expectedDir,name+".json"),"utf8"));
  const actual = JSON.parse(readFileSync(join(actualDir,name+".json"),"utf8"));
  const difference = firstDifference(expected,actual);
  const sha = data => createHash("sha256").update(canonical(data)).digest("hex");
  result.comparisons.push({name,expectedSHA256:sha(expected),actualSHA256:sha(actual),difference});
  if (difference) result.verdict = "MISMATCH";
 }
 return result;
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [expected,actual] = process.argv.slice(2);
 if (!actual) throw new Error("usage: registry-compare.mjs original-observations translated-observations");
 const result = compareDirectories(expected,actual);
 console.log(JSON.stringify(result,null,2));
 if (result.verdict !== "OK") process.exitCode = 1;
}
