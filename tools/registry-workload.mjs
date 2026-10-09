import {readFileSync,readdirSync,statSync} from 'node:fs';
import {join} from 'node:path';
export function validateWorkload(input,deps,negatives) {
 const main = readdirSync(input).sort();
 if (JSON.stringify(main) !== JSON.stringify(['zabapgit_standalone.prog.abap','zabapgit_standalone.prog.xml']) || main.some(n=>!statSync(join(input,n)).isFile() || !readFileSync(join(input,n)).length)) throw new Error('expected populated north-star main inputs');
 function files(dir) {return readdirSync(dir).flatMap(n=>statSync(join(dir,n)).isDirectory()?files(join(dir,n)):[join(dir,n)]);}
 const dependencies = files(deps);
 if (dependencies.length !== 360 || !dependencies.some(n=>n.endsWith('.abap')) || !dependencies.some(n=>n.endsWith('.xml'))) throw new Error('expected 360 dependency files including ABAP and XML');
 const variants = JSON.parse(readFileSync(negatives,'utf8'));
 const targets = ['check_syntax','unknown_types','implement_methods','superclass_final','parser_error','allowed_object_naming'].sort();
 if (!Array.isArray(variants) || JSON.stringify(variants.map(v=>v.target).sort()) !== JSON.stringify(targets) || variants.some(v=>typeof v.filename !== 'string' || !v.filename.endsWith('.abap') || typeof v.edit !== 'string' || (!v.edit.length && v.filename === 'zabapgit_standalone.prog.abap'))) throw new Error('expected six distinct populated negative targets');
}
