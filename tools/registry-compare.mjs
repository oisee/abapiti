#!/usr/bin/env node
// Compare observations, never trust candidate-supplied checksums as evidence.
import {readFileSync} from "node:fs";
import {join, resolve} from "node:path";
import {pathToFileURL} from "node:url";
import {createHash} from "node:crypto";
import {compareDocumentedDivergences, documentedDivergences} from './registry-scope.mjs';

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
export function validateObservations(o) {
 const fail = detail => {throw new Error('incomplete Registry observations: '+detail);};
 const record = x => x !== null && typeof x === 'object' && !Array.isArray(x);
 const text = x => typeof x === 'string' && x.length > 0;
 const hash = x => typeof x === 'string' && /^[a-f0-9]{64}$/.test(x);
 const count = x => Number.isSafeInteger(x) && x >= 0;
 const m=o.manifest;
 if (!record(m) || !/^[a-f0-9]{40}$/.test(m.upstreamPin) || !/^[a-f0-9]{40}$/.test(m.depsPin) || !hash(m.configSHA256) || !Array.isArray(m.main) || !m.main.length || !Array.isArray(m.dependencies) || !record(m.types) || !record(m.objectCount) || !['total','normal','dependencies'].every(k=>count(m.objectCount[k])) || !count(m.abapFiles) || !count(m.registeredABAPFiles)) fail('manifest');
 for (const f of [...m.main,...m.dependencies]) if (!record(f) || !text(f.filename) || !count(f.bytes) || !hash(f.sha256)) fail('manifest files');
 if (!Array.isArray(o.inventory) || !o.inventory.length || o.inventory.length !== m.objectCount.total) fail('inventory count');
 if (m.objectCount.normal+m.objectCount.dependencies !== m.objectCount.total) fail('dependency counts');
 const types={};
 for (const i of o.inventory) {
  if (!record(i) || !text(i.name) || !text(i.type) || typeof i.dependency !== 'boolean' || !Array.isArray(i.files) || !i.files.length || !i.files.every(text) || !Object.hasOwn(i,'xml') || (i.xml !== null && !record(i.xml)) || (i.description !== null && typeof i.description !== 'string')) fail('inventory entry');
  if (i.type === 'PROG' && (typeof i.include !== 'boolean' || typeof i.modulePool !== 'boolean')) fail('program metadata');
  types[i.type]=(types[i.type]??0)+1;
 }
 if (firstDifference(types,m.types)) fail('manifest type counts');
 const c=o.config;
 if (!record(c) || !record(c.config) || !record(c.config.global) || !record(c.config.syntax) || !record(c.config.rules) || !text(c.config.global.files) || !text(c.config.syntax.version) || !record(c.release) || !text(c.release.name) || !count(c.release.ordinal) || !Object.hasOwn(c,'language') || (c.language !== undefined && typeof c.language !== 'string') || !Array.isArray(c.rules)) fail('config');
 for (const r of c.rules) if (!record(r) || !text(r.key) || !record(r.config)) fail('rules');
 if (!Array.isArray(o.dumps) || !o.dumps.length || o.dumps.length !== m.abapFiles || o.dumps.filter(d=>d.registered).length !== m.registeredABAPFiles) fail('dump counts');
 for (const d of o.dumps) if (!record(d) || !text(d.filename) || !['tokens','statements','structures','issues'].every(k=>count(d[k])) || !['lexerSHA256','statementsSHA256','structuresSHA256'].every(k=>hash(d[k])) || typeof d.registered !== 'boolean') fail('dump entry');
 const names=['duplicate_default','duplicate_strict','missing_xml','malformed_xml','malformed_config','json5','case_sensitivity','dependency_replaced','include','ordering_domain'];
 if (!record(o.negative) || names.some(k=>!Object.hasOwn(o.negative,k))) fail('negative observations');
 for (const [key,n] of Object.entries(o.negative)) {
  if (!record(n) || (n.outcome === 'return' ? !Object.hasOwn(n,'value') : n.outcome !== 'throw' || !text(n.error) || typeof n.message !== 'string')) fail('negative outcome');
  if (n.outcome !== 'return') continue;
  if (['duplicate_default','duplicate_strict','missing_xml','malformed_xml','dependency_replaced'].includes(key) && (!Array.isArray(n.value) || !n.value.length || n.value.some(i=>!record(i) || !text(i.name) || !text(i.type) || typeof i.dependency !== 'boolean' || !Array.isArray(i.files) || !i.files.length || !i.files.every(text) || !Object.hasOwn(i,'description') || !Object.hasOwn(i,'xml')))) fail('negative inventory');
  if (['malformed_config','json5'].includes(key) && (!record(n.value) || !record(n.value.global) || !text(n.value.global.files) || !record(n.value.syntax) || !text(n.value.syntax.version) || !record(n.value.rules))) fail('negative config');
  if (['case_sensitivity','include'].includes(key) && (!record(n.value) || !Array.isArray(n.value.objects) || !n.value.objects.length)) fail('negative objects');
  if (key === 'case_sensitivity' && (!['lowerType','upperType','file'].every(k=>Object.hasOwn(n.value,k) && (n.value[k]===null || typeof n.value[k]==='string')) || !count(n.value.parsed))) fail('negative case sensitivity');
  if (key === 'include' && (!Array.isArray(n.value.files) || !n.value.files.length || n.value.files.some(f=>!text(f.filename) || !Array.isArray(f.statements) || !f.statements.every(s=>record(s) && text(s.kind) && typeof s.text==='string')))) fail('negative include');
 }

}
export function compareDirectories(expectedDir, actualDir, {translated = false} = {}) {
 const observations = ["manifest","inventory","config","dumps","negative"];
 const read = dir => Object.fromEntries(observations.map(name=>[name,JSON.parse(readFileSync(join(dir,name+'.json'),'utf8'))]));
 const expectedObservations=read(expectedDir), actualObservations=read(actualDir);
 validateObservations(expectedObservations); validateObservations(actualObservations);
 const result = {verdict:"OK", comparisons:[]};
 if (translated) {
  result.divergences = compareDocumentedDivergences(expectedObservations.negative, actualObservations.negative);
  if (result.divergences.some(d=>!d.pass)) result.verdict = 'MISMATCH';
 }
 for (const name of observations) {
  const expected = expectedObservations[name];
  const actual = actualObservations[name];
  let difference;
  if (translated && name === 'negative') {
   const inScope = data => Object.fromEntries(Object.entries(data).filter(([key])=>!documentedDivergences.some(d=>d.name===key)));
   difference = firstDifference(inScope(expected),inScope(actual));
  } else difference = firstDifference(expected,actual);
  const sha = data => createHash("sha256").update(canonical(data)).digest("hex");
  result.comparisons.push({name,expectedSHA256:sha(expected),actualSHA256:sha(actual),difference});
  if (difference) result.verdict = "MISMATCH";
 }
 return result;
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [expected,actual,mode] = process.argv.slice(2);
 if (!actual) throw new Error("usage: registry-compare.mjs original-observations translated-observations");
 if (mode && mode !== '--translated') throw new Error('unknown comparison mode: '+mode);
 const result = compareDirectories(expected,actual,{translated:mode==='--translated'});
 console.log(JSON.stringify(result,null,2));
 if (result.verdict !== "OK") process.exitCode = 1;
}
