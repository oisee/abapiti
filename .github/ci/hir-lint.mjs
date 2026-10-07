// Syntax and generation gate; dependency sources are read, never downloaded.
import {createRequire} from "node:module";
import {readFileSync, readdirSync} from "node:fs";
import {join, resolve} from "node:path";
const [dir, osg] = process.argv.slice(2);
const require = createRequire(join(resolve(osg || "."), "package.json"));
const abaplint = require("@abaplint/core");
const config = abaplint.Config.getDefault();
config.get().syntax.version = abaplint.Version.v702;
config.get().syntax.errorNamespace = ".*";
config.get().rules = {parser_error: true, syntax: true};
const reg = new abaplint.Registry(config);
const files = readdirSync(dir).filter(n => n.endsWith(".abap"));
for (const n of files) {
  const src = readFileSync(join(dir, n), "utf8");
  for (const line of src.split("\n")) {
    if (Buffer.byteLength(line) > 255 || /^\s*\*/.test(line)) throw new Error(`${n}: generation rule`);
  }
  const code = src.replace(/'(?:''|[^'])*'|`(?:``|[^`])*`/g, "");
  if (code.includes('"')) throw new Error(`${n}: comment`);
  if (/\b(?:DATA\s*\(|NEW\s|VALUE\s*#|IS INSTANCE OF|CONV\s|COND\s|SWITCH\s)/i.test(code)) throw new Error(`${n}: post-7.02 syntax`);
  reg.addFile(new abaplint.MemoryFile(n, src));
}
function walk(dir) {
  for (const f of readdirSync(dir, {withFileTypes: true})) {
    const path = join(dir, f.name);
    if (f.isDirectory()) walk(path);
    else if (f.name.endsWith(".abap") && !f.name.includes("testclasses")) reg.addDependency(new abaplint.MemoryFile(path, readFileSync(path, "utf8")));
  }
}
walk(join(osg, ".local/lars/open-abap-core/src"));
reg.parse();
const issues = reg.findIssues();
for (const i of issues) console.error(`${i.getFilename()}:${i.getStart().getRow()}: ${i.getMessage()}`);
console.log(`HIR ABAP v702: ${files.length} files, ${issues.length} issues`);
process.exitCode = issues.length ? 1 : 0;
