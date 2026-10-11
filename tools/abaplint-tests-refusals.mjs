#!/usr/bin/env node
// Apply to the generated module of a LOCAL harness combination, then rebuild.
// Runtime domain refusals are compiler guards, not TypeScript assertion errors.
import {readFileSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
export function fixRefusalStatus(source) {
 const pattern = /if _,ok:=x\.\(trap\);ok\s*\{(status|r\.Status)="trapped"\}/g;
 const matches = [...source.matchAll(pattern)];
 if (matches.length !== 2) throw new Error(`expected two harness recovery boundaries, found ${matches.length}`);
 return source.replace(pattern, (_, status) => `switch x.(type) {case trap, rangeFault: ${status}="trapped"}`);
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
 const [module] = process.argv.slice(2);
 if (!module) throw new Error('usage: abaplint-tests-refusals.mjs GENERATED_GO_MODULE');
 const runtime = readFileSync(join(module, 'runtime.go'), 'utf8');
 for (const type of ['trap', 'rangeFault']) if (!runtime.includes(`type ${type} struct`)) throw new Error(`missing runtime refusal type ${type}`);
 const path = join(module, 'main.go');
 writeFileSync(path, fixRefusalStatus(readFileSync(path, 'utf8')));
}
