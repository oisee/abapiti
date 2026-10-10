# Glossary: what runs where

Results are named `<code>@<host>`. The code part spells the pipeline, one letter per stage after `TS`; the host part says what executes it.

## Code

| Name | Pipeline | What it is |
|---|---|---|
| **TS** | TypeScript | abaplint as Lars wrote it, commit `577f875e` (`@abaplint/core` 2.120.56) |
| **TS-HA** | TypeScript → HIR → ABAP | abaplint translated by abapiti: 2,110 ABAP classes and interfaces, the same bytes for every host |
| **TS-HG** | TypeScript → HIR → Go | abaplint translated through the same HIR straight to Go, without ABAP (dell's `hir/golang`) |

HIR is abapiti's typed object IR, on the TypeScript side. **GIR** is open-steamgate's IR on the ABAP side (gogen: ABAP → GIR → Go for OSGO/OSGB, or JS for OSGI). Grace is the rule engine over it; since #70 its inline rules are the default (`ABAPITI_INLINE=classic` runs the older `hir.Inline`, `0` disables inlining).

## Hosts

| Name | What executes the code |
|---|---|
| **Node** | Node.js; `TS@Node` is the reference, also called **vanilla** |
| **A4H** | a SAP kernel 7.58: our ABAP system A4H, background job of report `ZABAPITI_REGISTRY_CLEAN` (timings) or `ZABAPITI_REGISTRY_RUN` (profiles under a trace request) |
| **OSGO** | open-steamgate's Go runtime: ABAP compiled to Go by gogen and run as ABAP Unit (`npm run osgo:unit`) |
| **OSGJ** | open-steamgate's JS runtime: ABAP translated to JS by the abaplint transpiler, `@abaplint/runtime` (`npm run osgjs:unit`) |
| **OSGB** | open-steamgate's Go runtime in one executable: `osabap` compiles the ABAP plus a report into a binary. For abaplint that binary is the command **`zabaplint`** |
| **Go** | a plain Go build of TS-HG |
| **OSGI** | open-steamgate's IR-based JS backend: ABAP → GIR → synchronous JS on Node (in development, not released) |

**osabap** is the compiler (open-steamgate `tools/gogen/osabap.mjs`): ABAP → one Go executable; its output is the host OSGB.

`zabaplint` is the name of the command we ship (TS-HA@OSGB), not of a host. "Native" is not used: it read as "vanilla".

## Results, 2026-10-10

Checking `zabapgit_standalone.prog.abap` with abapGit's `ci/abaplint.json`:

| Run | Time | Equal to vanilla |
|---|---|---|
| TS@Node (vanilla) | 12.6–13.5 s | reference |
| TS-HA@A4H | 205.9 s | yes |
| TS-HA@OSGO | 60.2 s | yes |
| TS-HA@OSGJ | 85 min (heap 24 GB) | yes, first full run on open-steamgate 6e10128e |
| TS-HA@OSGB | 62 s (48 s with open-steamgate #701) | yes |
| TS-HG@Go | 5.3 s | yes (branch `proto/hir-golang-splice`) |

So: **TS-HA equals vanilla on A4H, OSGO, OSGJ and OSGB.**

## Checks

| Term | Meaning |
|---|---|
| **equal to vanilla** | the issue list is byte-identical to Node's (SHA-256 over all issues), `ok=X` in the driver line |
| **clean** | zabapgit as published; 0 issues (`5feceb66…`) |
| **seeded** | the same file with five seeded errors, one per rule; 5 issues (`b1a890fb…`) |
| **scope** | the build holds the code the zabapgit check runs: syntax v702 and six rules. Anything else stops with a refusal naming the TypeScript location, never with a different answer |
| **driver** | the harness that feeds the input to TS-HA and prints the `REGISTRY …` line: `ZCL_ABAPITI_REGISTRY_RUN` (OSGO/OSGJ, zabapgit embedded), `ZCL_ABAPITI_REGISTRY_A4H` + reports on A4H |
| **pin** | the open-steamgate commit CI uses (`.github/ci/osgo.ref`) |
