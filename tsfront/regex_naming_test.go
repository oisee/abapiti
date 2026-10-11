package tsfront

import "testing"

func TestGoRegExpBoundedNamingNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run():string {
 const include=/^((\/[A-Z_\d]{3,8}\/)|[a-zA-Z0-9]{3}|CI_)\w+$/;
 const prefix=new RegExp('^[a-zA-Z]{1,3}_.*$','i');
 const inputs:string[]=['','abc','abcX','ab_X','ABCfoo','012x','CI_foo','CI_','/AB/x','/ABC/x','/A_1/x','/ABCDEFGH/x','/ABCDEFGHI/x','/abc/x','/ABC/','/ABC/_','/ABC/x\n','/ABC/x\r','/ABC/x\u2028','/ABC/x\u2029','/ABC/é','/ABC/K','/ABC/😀','a_','A_x','AbC_x','abcd_x','_x','1_x','a_K😀','K_x','ſ_x','a_x\n','a_x\r','a_x\u2028','a_x\u2029'];
 let out='';for(const s of inputs){out+=include.test(s)+':'+prefix.test(s)+';';const m=include.exec(s);if(m!==null){out+=m[0]+':'+m[1]+':'+(m[2]===undefined?'U':m[2])+':'+m.index+';';}}
 return out;
 }}`)
}
