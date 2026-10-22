import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {execFileSync} from 'node:child_process';
import {adaptTestRegex} from './osgo-test-regex.mjs';
const start='(?<![\\s\\S])', end='(?![\\s\\S])';
const patterns=[start+'foo'+end,start+'foo|bar'+end,start+'foo', 'bar'+end];
const str=value=>({e:'str',value,type:{k:'string'}});
const constants=patterns.map(str);
const split={e:'concat',l:str(start.slice(0,5)),r:str(start.slice(5)+'foo'+end),type:{k:'string'}};
const unknown=str('(?<![A-Za-z0-9_])foo');
const allUnit=str('<DD03P>(?s:.)*?</DD03P>');
const foreign=str(patterns[0]);
const program={classes:[{name:'RX',methods:[{body:[...constants,split,unknown,allUnit]}]},{name:'USER',methods:[{body:[foreign]}]}]};
const names={RX:{id:'runtime.regexp_js'},USER:{id:'user'}};
assert.equal(adaptTestRegex(program,names).literalSites,6);
assert.equal(split.e,'str');
assert.equal(split.value,'\\Afoo\\z');
assert.equal(unknown.value,'(?<![A-Za-z0-9_])foo');
assert.equal(foreign.value,patterns[0]);
assert.equal(allUnit.value,'<DD03P>(?:[^\\n]|\\n)*?</DD03P>');
assert.equal(adaptTestRegex(program,names).literalSites,0);
const inputs=['','foo','foox','bar','xbar','\nfoo','foo\nbar','foo\n','\u2028foo','bar\n','😀bar','foo😀'];
const cases=patterns.flatMap((pattern,i)=>inputs.map(input=>({pattern:constants[i].value,input,
 expected:(()=>{const m=new RegExp(pattern,'m').exec(input);return m ? {text:m[0],index:m.index}:null})()})));
for(const input of ['<DD03P>a\nb</DD03P>','😀<DD03P>\r\n\u2028\u2029x</DD03P>','<DD03P></DD03P>','<DD03P>a</DD03P><DD03P>b</DD03P>','none']) {
 const m=/<DD03P>[\s\S]*?<\/DD03P>/.exec(input);
 cases.push({pattern:allUnit.value,input,expected:m ? {text:m[0],index:m.index}:null});
}
const dir=mkdtempSync(join(tmpdir(),'osgo-regex-oracle-'));
try {
 const file=join(dir,'oracle.go');
 writeFileSync(file,`package main
import("encoding/json";"os";"regexp";"unicode/utf16")
func main(){
 var cases []struct{Pattern,Input string;Expected *struct{Text string;Index int}}
 if err:=json.Unmarshal([]byte(os.Args[1]),&cases);err!=nil{panic(err)}
 for _,c:=range cases {
  re,err:=regexp.Compile(c.Pattern);if err!=nil{panic(err)}
  m:=re.FindStringIndex(c.Input)
  if (m==nil)!=(c.Expected==nil){panic("presence mismatch")}
  if m!=nil && (c.Input[m[0]:m[1]]!=c.Expected.Text || len(utf16.Encode([]rune(c.Input[:m[0]])))!=c.Expected.Index){panic("match mismatch")}
 }
}
`);
 execFileSync('go',['run',file,JSON.stringify(cases)],{env:{...process.env,GOTOOLCHAIN:'go1.26.0',GOMAXPROCS:'4'},stdio:'pipe'});
} finally {rmSync(dir,{recursive:true,force:true})}
console.log('OSGO-only absolute assertions: Node (implicit m) / RE2 oracle, literal chunks, and source isolation: ok');
