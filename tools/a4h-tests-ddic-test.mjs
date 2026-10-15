#!/usr/bin/env node
// Check the emitted TABL using real standard DTEL/DOMA dependencies.
import assert from 'node:assert/strict';
import {readFileSync, readdirSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {createRequire} from 'node:module';

const [build, osg = process.env.ABAPITI_OSG_HOME] = process.argv.slice(2);
if (!build || !osg) throw new Error('usage: a4h-tests-ddic-test.mjs BUILD OSG');
const lint = createRequire(join(resolve(osg), 'package.json'))('@abaplint/core');
const xml = readFileSync(join(build, 'tests/zlnt_testrun.tabl.xml'), 'utf8');
// Freeze the previous generator's primitive definitions, including field order.
const fields = [
 ['MANDT', 'MANDT', 'CLNT', '000003', '000006', true],
 ['RUN_ID', 'CHAR32', 'CHAR', '000032', '000064', true],
 ['SEQ', 'INT4', 'INT4', '000010', '000004', true],
 ['CHUNK_NO', 'INT4', 'INT4', '000010', '000004', true],
 ['SHARD', 'CHAR40', 'CHAR', '000040', '000080', false],
 ['NEXT_IDX', 'INT4', 'INT4', '000010', '000004', false],
 ['USED', 'INT4', 'INT4', '000010', '000004', false],
 ['PAYLOAD', 'CHAR255', 'CHAR', '000255', '000510', false],
];
const blocks = [...xml.matchAll(/    <DD03P>\n[\s\S]*?    <\/DD03P>\n/g)].map(m => m[0]);
assert.deepEqual(blocks, fields.map(([name, roll, , , , key]) =>
 `    <DD03P>\n     <FIELDNAME>${name}</FIELDNAME>\n${key ? '     <KEYFLAG>X</KEYFLAG>\n' : ''}` +
 `     <ROLLNAME>${roll}</ROLLNAME>\n     <ADMINFIELD>0</ADMINFIELD>\n` +
 `${key ? '     <NOTNULL>X</NOTNULL>\n' : ''}     <COMPTYPE>E</COMPTYPE>\n    </DD03P>\n`));
const oldFields = fields.map(([name, , datatype, length, internal, key]) =>
 `<DD03P><FIELDNAME>${name}</FIELDNAME><KEYFLAG>${key ? 'X' : ''}</KEYFLAG>` +
 `<NOTNULL>${key ? 'X' : ''}</NOTNULL><ADMINFIELD>0</ADMINFIELD>` +
 `<INTTYPE>${datatype === 'INT4' ? 'I' : 'C'}</INTTYPE><INTLEN>${internal}</INTLEN>` +
 `<DATATYPE>${datatype}</DATATYPE><LENG>${length}</LENG><MASK>  ${datatype}</MASK></DD03P>`).join('\n');
// The old COMPTYPE E without ROLLNAME cannot resolve in abaplint. Parse its
// explicit DATATYPE/LENG as built-in fields to recover the intended old shape.
const before = xml.replace(/<DD03P_TABLE>[\s\S]*?<\/DD03P_TABLE>/,
 `<DD03P_TABLE>${oldFields}</DD03P_TABLE>`);
function parse(source) {
 const reg = new lint.Registry();
 reg.addFile(new lint.MemoryFile('zlnt_testrun.tabl.xml', source));
 function dependencies(dir) {
  for (const entry of readdirSync(dir, {withFileTypes:true})) {
   const path = join(dir, entry.name);
   if (entry.isDirectory()) dependencies(path);
   else if (/\.(dtel|doma)\.xml$/.test(entry.name)) reg.addDependency(new lint.MemoryFile(path, readFileSync(path, 'utf8')));
  }
 }
 dependencies(join(osg, '.local/lars/open-abap-core/src'));
 reg.parse();
 const table = reg.getObject('TABL', 'ZLNT_TESTRUN');
 assert.deepEqual(table.listKeys(reg), fields.filter(f => f[5]).map(f => f[0]));
 return table.parseType(reg).getComponents().map(c => [c.name, c.type.toABAP()]);
}
const expected = fields.map(([name, , datatype, length]) => [name, datatype === 'INT4' ? 'i' : `c LENGTH ${Number(length)}`]);
assert.deepEqual(parse(before), expected);
assert.deepEqual(parse(xml), expected);
console.log('A4H TABL: standard ROLLNAME layout; 8 field types/lengths and 4 keys preserved');
