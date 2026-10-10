// Analysis fixture syntax check, using the repository's locked @abaplint/core.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const abap = require('@abaplint/core');
const root = 'docs/history/2026-10-10-peephole-mining/a4h';
const config = abap.Config.getDefault(abap.Version.v758).get();
config.rules = {parser_error: true, syntax_check: true};
let issues = 0;
const files = fs.readdirSync(root).filter(f => f.endsWith('.prog.abap')).sort();
const fingerprints = {};
for (const file of files) {
  fingerprints[file] = crypto.createHash('sha256').update(fs.readFileSync(path.join(root,file))).digest('hex');
  const registry = new abap.Registry(new abap.Config(JSON.stringify(config)));
  registry.addFile(new abap.MemoryFile('zpeephole_bench.prog.abap', fs.readFileSync(path.join(root,file),'utf8')));
  registry.parse();
  for (const issue of registry.findIssues()) {
    console.log(`${file}:${issue.getStart().getRow()}: ${issue.getMessage()}`);
    issues++;
  }
}
console.log(`${files.length} fixture reports checked with @abaplint/core ${abap.Registry.abaplintVersion()}, v758: ${issues} issues`);
fs.writeFileSync(path.join(root,'..','a4h-syntax.json'),JSON.stringify({checker:'@abaplint/core',version:abap.Registry.abaplintVersion(),abap_version:'v758',reports:files.length,issues,files:fingerprints},null,2)+'\n');
process.exitCode = issues ? 1 : 0;
