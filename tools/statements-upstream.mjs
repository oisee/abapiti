// Always compile the original pinned source into a disposable directory.
// Existing build output is never trusted or imported by the oracle.
import {execFileSync} from "node:child_process";
import {mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync} from "node:fs";
import {tmpdir} from "node:os";
import {join, resolve} from "node:path";

export const upstreamPin = "577f875ebec44cfaf64841cfe71c8ab8dc32622e";
export function verifyUpstream(repository) {
 const git = (...args) => execFileSync("git", ["-C", repository, ...args], {encoding:"utf8"}).trim();
 if (git("rev-parse", "HEAD") !== upstreamPin) throw new Error(`oracle upstream must be ${upstreamPin}`);
 if (git("status", "--porcelain", "--untracked-files=all")) throw new Error("oracle upstream has source changes");
 const compiler = join(repository,"node_modules/typescript");
 const lock = JSON.parse(readFileSync(join(repository,"package-lock.json"),"utf8"));
 const installed = JSON.parse(readFileSync(join(compiler,"package.json"),"utf8"));
 if (lock.packages["node_modules/typescript"].version !== installed.version) throw new Error("oracle TypeScript version does not match upstream lockfile");
 return compiler;
}

export function buildUpstream(repositoryArg = "/home/alice/dev/abaplint") {
 const repository = resolve(repositoryArg);
 const compiler = verifyUpstream(repository);
 const temporary = mkdtempSync(join(tmpdir(),"statements-oracle-"));
 const dispose = () => rmSync(temporary,{recursive:true,force:true});
 try {
  const source = join(temporary,"source");
  mkdirSync(source);
  const archive = execFileSync("git",["-C",repository,"archive",upstreamPin],{maxBuffer:64*1024*1024});
  execFileSync("tar",["-x","-C",source],{input:archive});
  symlinkSync(join(repository,"node_modules"),join(source,"node_modules"),"dir");
  symlinkSync(join(repository,"packages/core/node_modules"),join(source,"packages/core/node_modules"),"dir");
  symlinkSync(join(repository,"node_modules"),join(temporary,"node_modules"),"dir");
  execFileSync(process.execPath,[join(compiler,"bin/tsc"),"--project",join(source,"packages/core/tsconfig.json"),"--outDir",join(temporary,"build"),"--incremental","false"],{stdio:["ignore","pipe","pipe"]});
  symlinkSync(join(repository,"packages/core/node_modules"),join(temporary,"build/node_modules"),"dir");
  console.error(`oracle: fresh original upstream build ${upstreamPin}, TypeScript ${JSON.parse(readFileSync(join(compiler,"package.json"),"utf8")).version}`);
  return {core:join(temporary,"build/src"),dispose};
 } catch (e) { dispose(); throw e; }
}
