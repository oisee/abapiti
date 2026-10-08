import {spawn} from "node:child_process";
import {closeSync, mkdtempSync, openSync, readFileSync, rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";

export function pinnedTranspilerRef(osg) {
  const lock = JSON.parse(readFileSync(join(osg, "libs.lock.json"), "utf8"));
  if (!/^[0-9a-f]{40}$/.test(lock.transpiler?.ref ?? "")) {
    throw new Error(`libs.lock.json has no full transpiler commit: ${lock.transpiler?.ref}`);
  }
  return lock.transpiler.ref;
}

export function parseTranspilerDescription(description, expectedRef) {
  const line = description.split("\n")[0] ?? "";
  const pinned = line.match(/^transpiler: the pinned build of \S+ ([0-9a-f]{8,40}) \(libs\.lock\.json\)/);
  const local = line.match(/^transpiler: a LOCAL BUILD, .*\((?:HEAD )?([0-9a-f]{8,40})(?:, uncommitted changes)?\)?(?:,|$)/);
  if (!pinned && !local) return null;
  const commit = (pinned ?? local)[1];
  const clean = !line.includes(", uncommitted changes");
  return {commit, clean, matches: clean && expectedRef.startsWith(commit) && commit.length >= 8};
}

export async function assertPinnedTranspiler(osg, env = process.env) {
  const expected = pinnedTranspilerRef(osg);
  const run = (args, options = {}) => new Promise((resolve, reject) => {
    const child = spawn("npm", args, {cwd: osg, env, ...options});
    child.on("error", reject);
    if (child.stdout) child.stdout.on("data", chunk => process.stdout.write(chunk));
    if (child.stderr) child.stderr.on("data", chunk => process.stderr.write(chunk));
    child.on("close", status => resolve(status));
  });
  if (!env.TRANSPILER) {
    const childEnv = {...env};
    delete childEnv.TRANSPILER;
    const status = await run(["run", "-s", "transpiler:pin"], {env: childEnv, stdio: "inherit"});
    if (status !== 0) throw new Error(`transpiler:pin failed for ${expected}`);
  }
  const directory = mkdtempSync(join(tmpdir(), "abapiti-osd-transpiler-"));
  const descriptionPath = join(directory, "description.txt");
  let description;
  try {
    const output = openSync(descriptionPath, "w");
    try {
      const status = await new Promise((resolve, reject) => {
        const child = spawn("node", ["tools/osd-transpiler.mjs"], {cwd: osg, env, stdio: ["ignore", output, "inherit"]});
        child.on("error", reject);
        child.on("close", resolve);
      });
      if (status !== 0) throw new Error(`osd-transpiler.mjs failed with exit ${status}`);
      description = readFileSync(descriptionPath, "utf8");
    } finally {
      closeSync(output);
    }
  } finally {
    rmSync(directory, {recursive: true, force: true});
  }
  const found = parseTranspilerDescription(description, expected);
  if (!found?.matches) {
    console.error(description);
    throw new Error(`OSG-JS must use the pinned transpiler ${expected}, not a published or different local build`);
  }
  console.log(`OSG-JS transpiler: pinned ${found.commit} (expected ${expected})`);
  return expected;
}

if (process.argv[1] && process.argv[1].endsWith("osg-transpiler.mjs") && process.argv[1] === import.meta.filename) {
  const [osg] = process.argv.slice(2);
  if (!osg) {
    console.error("usage: node osg-transpiler.mjs <open-steamgate>");
    process.exit(2);
  }
  try {
    await assertPinnedTranspiler(osg);
  } catch (error) {
    console.error(`osg-transpiler: ${error.message}`);
    process.exit(4);
  }
}
