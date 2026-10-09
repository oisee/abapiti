import {spawn} from "node:child_process";
import {closeSync, lstatSync, mkdtempSync, openSync, readFileSync, realpathSync, rmSync} from "node:fs";
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
  const pinned = line.match(/^transpiler: the pinned build of \S+ ([0-9a-f]{8,40}) \(libs\.lock\.json\)(?:, at (.+?), calling itself )?/);
  const local = line.match(/^transpiler: a LOCAL BUILD, (.+?) \((?:HEAD )?([0-9a-f]{8,40})(?:, uncommitted changes)?\)/);
  if (!pinned && !local) return null;
  const commit = pinned ? pinned[1] : local[2];
  const clean = !line.includes(", uncommitted changes");
  return {commit, clean, where: pinned ? pinned[2] : local[1], matches: expectedRef.startsWith(commit) && commit.length >= 8};
}

export function parseRuntimeDescription(description, expectedRef) {
  const line = description.split("\n")[1] ?? "";
  if (line.startsWith("runtime: none installed at ")) return {kind: "missing"};
  if (line.startsWith("runtime: @abaplint/runtime ")) return {kind: "published"};
  const pinned = line.match(/^runtime: the pinned build of \S+ ([0-9a-f]{8,40}) \(libs\.lock\.json\), at (.+?), calling itself /);
  if (pinned) return {kind: "linked", where: pinned[2], clean: true, matches: expectedRef.startsWith(pinned[1])};
  const local = line.match(/^runtime: a LOCAL BUILD, (.+?) \((?:HEAD )?([0-9a-f]{8,40})(?:, uncommitted changes)?\)/);
  if (!local) return {kind: "invalid"};
  return {
    kind: "linked",
    where: local[1],
    clean: !line.includes(", uncommitted changes"),
    matches: expectedRef.startsWith(local[2]) && local[2].length >= 8,
  };
}

async function git(directory, ...args) {
  return new Promise((resolve, reject) => {
    const child = spawn("git", ["-C", directory, ...args], {stdio: ["ignore", "pipe", "inherit"]});
    let stdout = "";
    child.stdout.on("data", chunk => stdout += chunk);
    child.on("error", reject);
    child.on("close", status => {
      if (status === 0) resolve(stdout);
      else reject(new Error(`git ${args.join(" ")} failed with exit ${status}`));
    });
  });
}

export async function assertPinnedBuild(build, expectedRef) {
  const commit = (await git(build, "rev-parse", "HEAD")).trim();
  if (commit !== expectedRef) throw new Error(`pinned transpiler checkout is ${commit}, expected ${expectedRef}`);
  const status = await git(build, "status", "--porcelain=v1", "--untracked-files=all");
  const summary = await git(build, "diff", "--summary");
  const content = await git(build, "diff", "--numstat");
  if (status !== " M packages/cli/abap_transpile\n" ||
      summary !== " mode change 100644 => 100755 packages/cli/abap_transpile\n" ||
      content !== "0\t0\tpackages/cli/abap_transpile\n") {
    throw new Error(`pinned transpiler checkout must have only the build's CLI executable-bit change:\n${status}${summary}${content}`);
  }
}

export function assertLinkedPackages(osg, build) {
  const links = [
    ["transpiler", join(build, "packages/transpiler")],
    ["transpiler-cli", join(build, "packages/cli")],
    ["runtime", join(build, "packages/runtime")],
    ["core", join(build, "packages/transpiler/node_modules/@abaplint/core")],
  ];
  for (const [name, target] of links) {
    const link = join(osg, "node_modules/@abaplint", name);
    if (!lstatSync(link, {throwIfNoEntry: false})?.isSymbolicLink() || realpathSync(link) !== realpathSync(target)) {
      throw new Error(`@abaplint/${name} is not linked to the pinned transpiler build`);
    }
  }
}

export async function assertPinnedTranspiler(osg, env = process.env) {
  const lock = JSON.parse(readFileSync(join(osg, "libs.lock.json"), "utf8"));
  const expected = lock.transpiler.ref;
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
  const runtime = parseRuntimeDescription(description, expected);
  const transpilerBuild = found?.where && realpathSync(join(found.where, "../.."));
  const runtimeBuild = runtime?.where && realpathSync(join(runtime.where, "../.."));
  if (!found?.matches || !runtime?.matches || !transpilerBuild || transpilerBuild !== runtimeBuild) {
    console.error(description);
    throw new Error(`OSG-JS must use the pinned transpiler and runtime ${expected}, not published, missing, or different local builds`);
  }
  const buildDirectory = transpilerBuild ?? (env.TRANSPILER && realpathSync(env.TRANSPILER));
  await assertPinnedBuild(buildDirectory, expected);
  assertLinkedPackages(osg, buildDirectory);
  const status = await new Promise((resolve, reject) => {
    const child = spawn("bash", ["tools/osd-ci-transpiler-build.sh", "verify"], {
      cwd: osg,
      env: {...env, TRANSPILER: buildDirectory, OSD_TRANSPILER_REPO: lock.transpiler.repo, OSD_TRANSPILER_REF: expected},
      stdio: "inherit",
    });
    child.on("error", reject);
    child.on("close", resolve);
  });
  if (status !== 0) throw new Error("OSG's pinned transpiler build verification failed");
  console.log(`OSG-JS transpiler and runtime: pinned ${expected}`);
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
