package tsfront

import "testing"

func TestStep4EmitProgramStructsConditionalNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Cls{readonly name:string}
 export class Row{readonly name:string;constructor(name:string){this.name=name;}}
 export interface Program{readonly classes:Cls[];readonly structs:Map<string,Row>}
 export type ClassList=Cls[]&Partial<Program>;export type ProgramInput=Program|ClassList;
 export interface Layers{readonly structs?:Map<string,Row>}`, `import {Row} from "./types.ts";
 /** @param {import("./types.ts").ProgramInput} program @param {import("./types.ts").Layers|null} layers @returns {string} */
 function emit(program,layers){/** @type {Map<string,import("./types.ts").Row>} */ const structs=layers?.structs??(Array.isArray(program)?new Map():program.structs);const defs=Array.isArray(program)?structs:program.structs;return defs.has("a")?"a":defs.has("b")?"b":"empty";}
 export class Probe{/** @returns {string} */ static run(){/** @type {import("./types.ts").ClassList} */ const xs=[];return emit({classes:[],structs:new Map([["a",new Row("A")]])},null)+":"+emit(xs,{structs:new Map([["b",new Row("B")]])});}}`, false, true)
}
