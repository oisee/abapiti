#!/usr/bin/env node
// The same byte-pinned static closure as registry-closure.mjs, with test roots.
import {resolve} from "node:path";
import {materializeRegistryClosure} from "./registry-closure.mjs";
const [out, upstream = "/home/alice/dev/abaplint", ...files] = process.argv.slice(2);
if (!out || !files.length) throw new Error("usage: abaplint-tests-closure.mjs OUT UPSTREAM test/rules/7bit_ascii.ts [...files]");
materializeRegistryClosure(out, upstream, {
  roots: files,
  runtime: resolve(import.meta.dirname, "../tsfront/testdata/abaplint-tests/runtime"),
});
