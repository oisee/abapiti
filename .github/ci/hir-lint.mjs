// Syntax and generation gate; dependency sources are read, never downloaded.
import {createRequire} from "node:module";
import {readFileSync, readdirSync} from "node:fs";
import {join, resolve} from "node:path";
const [dir, osg] = process.argv.slice(2);
const require = createRequire(join(resolve(osg || "."), "package.json"));
const abaplint = require("@abaplint/core");
const config = abaplint.Config.getDefault();
config.get().syntax.version = abaplint.Version.v750;
config.get().syntax.errorNamespace = ".*";
config.get().rules = {parser_error: true, check_syntax: true};
const ruleNames = new Set(abaplint.ArtifactsRules.getRules().map(rule => rule.getMetadata().key));
for (const name of Object.keys(config.get().rules)) {
  if (!ruleNames.has(name)) throw new Error(`Unknown abaplint rule: ${name}`);
}
const reg = new abaplint.Registry(config);
const constructs = {inline: false, instance: false};
const files = readdirSync(dir).filter(n => n.endsWith(".abap"));
for (const n of files) {
  const src = readFileSync(join(dir, n), "utf8");
  // The only comment allowed is the TypeScript origin as the first line.
  src.split("\n").forEach((line, i) => {
    const origin = i === 0 && /^\* TS: \S+ \S+$/.test(line);
    if (Buffer.byteLength(line) > 255 || (/^\s*\*/.test(line) && !origin)) throw new Error(`${n}: generation rule`);
  });
  const code = src.replace(/'(?:''|[^'])*'|`(?:``|[^`])*`|\|(?:\\.|[^|])*\|/g, "");
  if (code.includes('"')) throw new Error(`${n}: comment`);
  if (n.endsWith(".clas.abap")) {
    for (const section of ["PUBLIC", "PROTECTED", "PRIVATE"]) {
      if (!new RegExp(`\\b${section}\\s+SECTION\\s*\\.`, "i").test(code)) {
        throw new Error(`${n}: global class must contain ${section} SECTION`);
      }
    }
  }
  constructs.inline ||= /\bDATA\s*\(/i.test(code);
  constructs.instance ||= /\bIS INSTANCE OF\b/i.test(code);
  reg.addFile(new abaplint.MemoryFile(n, src));
}
if (!constructs.inline || !constructs.instance) throw new Error("HIR corpus must contain inline DATA and IS INSTANCE OF");
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
console.log(`HIR ABAP v750: ${files.length} files, ${issues.length} issues`);
process.exitCode = issues.length ? 1 : 0;
