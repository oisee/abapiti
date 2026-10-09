# abapiti abaplint

Translates the core of [abaplint](https://github.com/abaplint/abaplint) from TypeScript into ABAP. Generation is pure Go: no Node, npm or SAP system is needed.

```sh
abapiti abaplint -o out                      # download the pinned abaplint into the user cache
abapiti abaplint ~/src/abaplint -o out       # or use a checkout (at 577f875e, after npm ci in packages/core)
abapiti abaplint -o out --target native      # one target only: all (default), a4h, osg, native
```

## Input

abaplint commit `577f875ebec44cfaf64841cfe71c8ab8dc32622e` (`@abaplint/core` 2.120.56). Nothing else is accepted.

- **No path**: abapiti downloads `https://codeload.github.com/abaplint/abaplint/tar.gz/577f875e…` (about 1.2 MiB) and the three npm packages whose types the translation reads (`fast-xml-parser` 5.10.1, `json5` 2.2.3, `vscode-languageserver-types` 3.18.0). Their URLs come from `packages/core/package-lock.json` in the archive, and each tarball is checked against the lockfile's sha512 integrity before it is unpacked. The result is kept in `<user cache>/abapiti/abaplint-577f875e` (`--cache-dir` sets another place) and reused while it still verifies. `--offline` never downloads.
- **A checkout path**: no downloads. The packages are looked up in `packages/core/node_modules`, then in the root `node_modules`.

In both cases every source file of the closure (1,538 files reachable from `registry.ts`, `config.ts` and `files/memory_file.ts`) and every file of the three npm packages must match its SHA-256 as recorded at the pin. If one does not match, the command stops and names the expected commit and the first file that differs. The fingerprinted overrides and the reachability manifest are recorded against the same pin. abapiti never builds anything from a different abaplint.

## Output

| Directory | Contents | Next step |
|---|---|---|
| `classes/` | the translated classes and interfaces (2,110 objects), shared by all targets | |
| `a4h/` | `abaplint-577f875e-a4h.zip`: an abapGit offline repository with the classes, `ZCL_ABAPITI_REGISTRY_A4H` and the report `ZABAPITI_REGISTRY_RUN` (package `$ZABAPLINT`, set with `--package`) | import with abapGit, then run `ZABAPITI_REGISTRY_RUN` as a background job. The driver reads zabapgit from the `ZABAPITI_CORPUS` table through `ZCL_ABAPITI_CORPUS` and logs through `ZCL_ABAPITI_LOG`, so these must already be installed (they are not part of the zip) |
| `osg/` | the classes; with `--input <dir> --deps <dir> --config <file>` also `ZCL_ABAPITI_REGISTRY_RUN`, which embeds those files and compares the issue dump with `--run-sha` | `npm run osgo:unit -- <out>/osg` (or `osgjs:unit`) in an open-steamgate checkout |
| `native/` | `zabaplint.prog.abap` and `lib/` with the classes | in an open-steamgate checkout: `node tools/gogen/osabap.mjs native/zabaplint.prog.abap --lib native/lib` (set GOOS/GOARCH for cross builds), then `.out/osabap --file zabapgit_standalone.prog.abap --config abaplint.json -allow-read .` |

The output is deterministic. The same pinned input gives byte-identical files on every run and from any directory, and they are the same files that the `TestRegistryClosureGate` test path emits. `--evidence` also writes the lowering evidence (the overrides applied, the trapped bodies, and the blocking diagnostics, of which there are none).

## Limits

The build is pruned to the code paths that checking `zabapgit_standalone` with abapGit's `ci/abaplint.json` executes (release v702, six rules). These are the paths recorded in `tsfront/testdata/registrycorpus/reachability.json`. Function bodies outside them are compiled as traps. Other inputs or rules may reach such a trap. In that case the check is refused, and the refusal names the TypeScript location of the missing code (for example `refused: this build has no code for src/rules/….ts:42`). It never gives a silently wrong answer.
