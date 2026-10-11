#!/usr/bin/env node
// Original Node observations using the pinned upstream TypeScript compiler.
import {readFileSync, writeFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {join} from 'node:path';
import {homedir} from 'node:os';
import {verifyUpstream} from './statements-upstream.mjs';
const require = createRequire(import.meta.url);
const ts = require(verifyUpstream(process.argv[2] ?? join(homedir(), 'dev/abaplint')));
const directory = new URL('../tsfront/testdata/switchgrouped/', import.meta.url);
const source = readFileSync(new URL('probe.ts', directory), 'utf8');
const compiled = ts.transpileModule(source, {compilerOptions:{target:ts.ScriptTarget.ES2020, module:ts.ModuleKind.CommonJS}}).outputText;
const module = {exports:{}};
new Function('exports', 'module', compiled)(module.exports, module);
writeFileSync(new URL('oracle.json', directory), JSON.stringify({expected:module.exports.SwitchProbe.run()}, null, 2) + '\n');
