import assert from "node:assert/strict";
import test from "node:test";
import {spawn} from "node:child_process";
import {mkdirSync, rmSync, symlinkSync, writeFileSync, chmodSync} from "node:fs";
import {tmpdir} from "node:os";
import {join} from "node:path";

import {assertLinkedPackages, assertPinnedBuild, parseRuntimeDescription, parseTranspilerDescription} from "./osg-transpiler.mjs";

const ref = "753230cb696480cc4c7f25c9e3876bf2a43a555e";

test("accepts the old local-build wording at the pinned commit", () => {
  const description = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD 753230cb), calling itself 2.13.96. A clean clone will not build this way.\nruntime: ...";
  assert.equal(parseTranspilerDescription(description, ref).matches, true);
});

test("accepts the pinned-build wording at the pinned commit", () => {
  const description = "transpiler: the pinned build of oisee/transpiler 753230cb (libs.lock.json), at /cache/transpiler/packages/transpiler, calling itself 2.13.96\n"
    + "runtime: the pinned build of oisee/transpiler 753230cb (libs.lock.json), at /cache/transpiler/packages/runtime, calling itself 2.13.96";
  assert.equal(parseTranspilerDescription(description, ref).matches, true);
  assert.equal(parseTranspilerDescription(description, ref).where, "/cache/transpiler/packages/transpiler");
  const runtime = parseRuntimeDescription(description, ref);
  assert.equal(runtime.matches, true);
  assert.equal(runtime.where, "/cache/transpiler/packages/runtime");
  const other = description.replace("runtime: the pinned build of oisee/transpiler 753230cb", "runtime: the pinned build of oisee/transpiler e2a459b1");
  assert.equal(parseRuntimeDescription(other, ref).matches, false);
});

test("matches a mode-dirty pinned local build and rejects different commits", () => {
  const published = "transpiler: @abaplint/transpiler 2.13.96, published";
  const wrongCommit = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD e2a459b1), calling itself 2.13.96";
  const dirty = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD 753230cb, uncommitted changes), calling itself 2.13.96";
  assert.equal(parseTranspilerDescription(published, ref), null);
  assert.equal(parseTranspilerDescription(wrongCommit, ref).matches, false);
  assert.equal(parseTranspilerDescription(dirty, ref).matches, true);
  assert.equal(parseTranspilerDescription(dirty, ref).clean, false);
});

test("rejects published and missing runtimes", () => {
  const description = line => `transpiler: a LOCAL BUILD, /cache/transpiler (HEAD 753230cb)\n${line}`;
  const published = description("runtime: @abaplint/runtime 2.13.96, published");
  const missing = description("runtime: none installed at /osg/node_modules/@abaplint/runtime");
  assert.equal(parseRuntimeDescription(published, ref).matches, undefined);
  assert.equal(parseRuntimeDescription(missing, ref).matches, undefined);
});

test("accepts only the build's executable-bit change", async () => {
  const build = join(tmpdir(), `abapiti-build-status-${process.pid}`);
  rmSync(build, {recursive: true, force: true});
  mkdirSync(join(build, "packages/cli"), {recursive: true});
  const cli = join(build, "packages/cli/abap_transpile");
  writeFileSync(cli, "#!/usr/bin/env node\n");
  const git = (...args) => new Promise((resolve, reject) => {
    const child = spawn("git", args, {cwd: build, stdio: ["ignore", "pipe", "inherit"]});
    let stdout = "";
    child.stdout.on("data", chunk => stdout += chunk);
    child.on("error", reject);
    child.on("close", status => status === 0 ? resolve(stdout) : reject(new Error(`git ${args.join(" ")} exited ${status}`)));
  });
  await git("init");
  await git("config", "user.email", "test@example.invalid");
  await git("config", "user.name", "Test");
  await git("add", "packages/cli/abap_transpile");
  await git("commit", "-m", "test");
  const ref = (await git("rev-parse", "HEAD")).trim();
  chmodSync(cli, 0o755);
  await assert.doesNotReject(assertPinnedBuild(build, ref));
  writeFileSync(cli, "#!/usr/bin/env node\nchanged\n");
  await assert.rejects(assertPinnedBuild(build, ref), /only the build's CLI executable-bit change/);
  writeFileSync(cli, "#!/usr/bin/env node\n");
  writeFileSync(join(build, "untracked.ts"), "changed");
  await assert.rejects(assertPinnedBuild(build, ref), /only the build's CLI executable-bit change/);
  rmSync(build, {recursive: true, force: true});
});

test("requires all OSG package links to the pinned build", () => {
  const build = join(tmpdir(), `abapiti-build-links-${process.pid}`);
  const osg = `${build}-osg`;
  rmSync(build, {recursive: true, force: true});
  rmSync(osg, {recursive: true, force: true});
  for (const path of ["packages/transpiler", "packages/transpiler/node_modules/@abaplint/core", "packages/cli", "packages/runtime"]) {
    mkdirSync(join(build, path), {recursive: true});
  }
  mkdirSync(join(osg, "node_modules/@abaplint"), {recursive: true});
  const links = {
    transpiler: "packages/transpiler",
    "transpiler-cli": "packages/cli",
    runtime: "packages/runtime",
    core: "packages/transpiler/node_modules/@abaplint/core",
  };
  for (const [name, target] of Object.entries(links)) {
    symlinkSync(join(build, target), join(osg, "node_modules/@abaplint", name), "dir");
  }
  assert.doesNotThrow(() => assertLinkedPackages(osg, build));
  rmSync(join(osg, "node_modules/@abaplint/runtime"), {recursive: true, force: true});
  assert.throws(() => assertLinkedPackages(osg, build), /@abaplint\/runtime is not linked/);
  rmSync(build, {recursive: true, force: true});
  rmSync(osg, {recursive: true, force: true});
});
