A17 lowers the typed Node platform calls on gogen's step-4 path (Go target;
ABAP gives a located catalogue refusal for every new operation).

| Site | Call | Result |
| --- | --- | --- |
| frontend-artifact-digests.mjs:11:25 | named `createHash("sha256").update(s).digest("hex")` | A, lowered (`string.sha256Hex`) |
| frontend-inventory.mjs:13:21 | same, untyped `s` (checked string unboxing) | A, lowered |
| frontend-artifact-digests.mjs:9:65 | `basename(p)` (node:path, one argument) | A, lowered (`path.basename`, POSIX) |
| host-io.ts:42:10 | `path.basename(file)` (namespace) | A, lowered |
| host-io.ts:11:10 | `readFileSync(file, "utf8")` | A, lowered (`fs.readText`) |
| host-io.ts:36:10 | `readdirSync(dir, {withFileTypes: true})` + `name`/`isDirectory()` | A, lowered (`fs.readdirEntries`, `builtin.Dirent`) |
| host-io.ts:25 | `readdirSync(dir)` (not reached) | lowered (`fs.readdirNames`) |
| frontend-provenance-hooks.mjs:4:18 | `fileURLToPath(new URL("../../", import.meta.url))` | G, located refusal |
| frontend-provenance.mjs:455:44 | `import.meta.url === pathToFileURL(process.argv[1]).href` | G, located refusal |
| frontend-provenance.mjs:8:14 | `coreRequire` | G, unchanged refusal |

Semantics follow Node 26: UTF-8 decoding replaces ill-formed subsequences with
U+FFFD per maximal subpart and keeps a BOM; lone surrogates hash as U+FFFD;
directory entries are in raw-name byte order (libuv scandir sort, verified
against Node) and Dirent types do not follow symlinks. Filesystem failures
throw a catchable `Error` with Node's message (`ENOENT: no such file or
directory, open '<p>'`, `EISDIR: illegal operation on a directory, read`,
`ENOTDIR: not a directory, scandir '<p>'`); `error.code` is not modelled and no
caller on the path inspects it. Other overloads (basename suffix, Buffer
results, non-string paths, other options, other algorithms/digests) keep
located `unsupported-node-platform`/`unsupported-crypto` refusals.

G reasons: `coreRequire` loads `@abaplint/core/build/src/${path}` modules at
run time to patch prototypes (provenance hooks); node:url is used only to
derive module locations (`import.meta.url`, the `process.argv` entry guard),
which a compiled program does not have. OSG should move these out of the
compiled closure.

Pilot (OSG 779bb4f5, measure-step4.sh, MemoryMax=16G, ref 61618e3b unchanged):

```
base (integ/gogen-stack3 ecca937): full: 380 blocking (250 root, 130 cascade), 1734 HIR errors in 38 files
A17:                               full: 369 blocking (239 root, 130 cascade), 1729 HIR errors in 38 files
```

The step-4 driver's `readFileSync(new URL(...), "utf8")` (kit file, not OSG)
is now refused at the call as a non-string path.

Gates: gofmt, go vet, short suite (tsfront, hir/..., cmd/abapiti,
internal/abapfixture), Grace full 1927 classes / 64 interfaces with identical
method/virtual/static facts (only catalogue-wide op records grow by the five
new ops), TestAbaplintCoreTrappedBodies, trap gate 2, abaplint Go sources
byte-identical to base, M1 osg-check-kit clean.
