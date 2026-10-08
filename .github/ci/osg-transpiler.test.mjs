import assert from "node:assert/strict";
import test from "node:test";

import {parseTranspilerDescription} from "./osg-transpiler.mjs";

const ref = "753230cb696480cc4c7f25c9e3876bf2a43a555e";

test("accepts the old local-build wording at the pinned commit", () => {
  const description = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD 753230cb), calling itself 2.13.96. A clean clone will not build this way.\nruntime: ...";
  assert.equal(parseTranspilerDescription(description, ref).matches, true);
});

test("accepts the pinned-build wording at the pinned commit", () => {
  const description = "transpiler: the pinned build of oisee/transpiler 753230cb (libs.lock.json), at /cache/transpiler, calling itself 2.13.96";
  assert.equal(parseTranspilerDescription(description, ref).matches, true);
});

test("rejects published, wrong-commit, and dirty local builds", () => {
  const published = "transpiler: @abaplint/transpiler 2.13.96, published";
  const wrongCommit = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD e2a459b1), calling itself 2.13.96";
  const dirty = "transpiler: a LOCAL BUILD, /cache/transpiler (HEAD 753230cb, uncommitted changes), calling itself 2.13.96";
  assert.equal(parseTranspilerDescription(published, ref), null);
  assert.equal(parseTranspilerDescription(wrongCommit, ref).matches, false);
  assert.equal(parseTranspilerDescription(dirty, ref).matches, false);
});
