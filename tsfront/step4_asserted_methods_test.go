package tsfront

import "testing"

func TestStep4AssertedMethodReceiverNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export class Registry {value:number;constructor(value:number){this.value=value;}parse():number{return this.value;}}`, `import {Registry} from "./types.ts";
/** @param {Registry} reg @param {unknown[]} args @returns {number} */
function parse(reg,args){return /** @type {{parse(...args:unknown[]):number}} */(reg).parse(...args);}
export class Probe{/** @returns {string} */ static run(){return String(parse(new Registry(7),[]));}}`, false, true)
}
