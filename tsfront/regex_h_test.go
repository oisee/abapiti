package tsfront

import "testing"

func TestGoRegExpLegacyNamingAndEmptyNodeOracle(t *testing.T) {
	regexNodeOracle(t, `class Probe {static run(): string {
  const namespace=/^(\/[A-Z_\d]{3,8}\/)?[A-Z_-\d<> ]+$/i;
  const slash=new RegExp('^[A-Z_-\\d/<> ]+$','i');
  const empty=new RegExp('', 'i');
  const inputs:string[]=['',' ','-','_','0','a','Z','<>','A-Z','/ABC/NAME','/A_1/NAME','/AB/NAME','/ABCDEFGH/NAME','/ABCDEFGHI/NAME','/abc/name','/ABC/-','/ABC/','A/B','/','?','é','K','ſ','😀','NAME\n','NAME\r','NAME\u2028','NAME\u2029'];
  let out=empty.source+':'+empty.toString()+';';
  for(const s of inputs){out+=namespace.test(s)+':'+slash.test(s)+':'+empty.test(s)+';';const m=namespace.exec(s);if(m!==null){out+=m[0]+':'+(m[1]===undefined?'U':m[1])+':'+m.index+';';}empty.lastIndex=3;const z=empty.exec(s);if(z!==null){out+=z[0]+':'+z.index+':'+empty.lastIndex+';';}}
  return out;
 }}`)
}
