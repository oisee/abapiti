# Self-hosting step 4b: gogen as ABAP on OSGo

**Status: paused on 2026-10-22.** Work moved to step 3 speed (the ADT checker on
OSGo uses TS-HG, abaplint compiled to Go, as a library). This branch keeps
everything 4b reached; nothing here is merged into `selfhost`.

Goal: OSG's gogen, compiled by abapiti to ABAP and run on OSGo (OSG pin
`.github/ci/osgo.ref` = `0171e92b`), prints the same Go as Node gogen on the
step-4 reference inputs (sha256 `b90648781967b2cf…`, `--expect-ref b9064878`).

## State at the pause

Branch head `b0d68287` (base `selfhost` `6236a982`), measured on i10 with gogen
G fixes at OSG `81498dd1` and OSGo pin `0171e92b`.

| Stage | Result |
| --- | --- |
| `tools/selfhost-step4.py --abap` | 297 s, 5,676 objects; `step4b-traps`: 11 reached (0/13 unknown), 326 unreached, unattributable 0; guards 648 reached-owner |
| Union ABI | representation C: one payload interface with `kind`/`sig`; `union.*` gaps 0 |
| Cast probes | `IS INSTANCE OF`; `CATCH cx_sy_move_cast_error` count 0 |
| OSG frontend over the ABAP | 5,583 classes, 0 diagnostics (25 missing, 17 stubs), 222 s |
| `go build` of the generated Go | passes, 228 s; whole build 489 s, peak 21.35 GB |
| Run (`ZCL_ABAPITI_MAIN=>RUN`) | rc 3 after 0.54 s, 89 MB, at the first open stop below |

Step 4 (Go) is unchanged by every commit here: `hir.go` of the step-4 emission
is byte-identical to `selfhost` (sha256 `cb5a3504…`).

## Stops fixed on the way (2026-10-21/22)

Each stop was classified A (abapiti), G (gogen) or O (OSG runtime).

1. `union.declarations` — union ABI C (A).
2. Go ICE in `NewBulk` on large initializers — native delete, initializer
   chunks, chunked `JSON_WRITE` (A).
3. `FrontendProgram` constructor with a wide shape (A).
4. `env` extern data reference (A).
5. Harness paths — shared `osg-harness-input.mjs` (A).
6. `Registry` structural cast — gogen typed as abaplint's `IRegistry` (G),
   plus structural interface admission `8f66e547` (A).
7. `localeCompare` — printable ASCII ordering in the ABAP runtime `a3292972`
   (A). The earlier "Object.keys holes" diagnosis was wrong.
8. Interface property lookup on class instances — `b0d68287` (A).
9. Incompatible types in structural interface adapters — `409233e9` (A).

## First open stop

`UNCAUGHT_EXCEPTION Z_VIEW_REFUSAL_…`: "ABAP view refused: live_class_view.go:
dynamic.get: reference does not implement view storage", raised in the dynamic
runtime class. Path: a closure's member call into the member cache of
`tools/gogen/frontend-cache-diagnostics.mjs` (LOAD / HAS / GET / PUT, then a GET
on a dynamic view whose reference has no view storage). Class A: the dynamic
view ABI must cover this carrier, or the emitter must keep the value typed.

The run passed `frontend-inventory.mjs:12` (interface property lookup) and
stopped before `frontend-inventory.mjs:63` (`result?.then`), the next known gap.

## Not done

- TS-HA output changes on purpose in this branch (initializer chunks, native
  dispatch and delete, cast probes, union C). The TS-HA and TS-HG finals
  (11,060 each), M1 and M2 were **not run** on this branch head; they must pass
  before any part is merged into `selfhost`.
- Size: generated Go is about 167–200 MB; levers F3–F5 (method-level
  reachability, initializers as data, descriptor tables) are in the backlog.

## Resume

On i10 with the Go-side owner's runner (kit v2: the closure is packed with
`tar -h`, because `closure/node_modules` is a symlink):

```sh
G_OSG=<gogen/OSG commit with the G fixes> \
  ~/dev/dell-work/step4b-run.sh <abapiti-sha> <label>
```

Locally, step by step:

```sh
# emit, list traps (reached / guards / unattributable)
python3 tools/selfhost-step4.py --abap --expect-ref b9064878 \
  <abapiti-bin> <osg-with-step4-driver> <abaplint> <transpiler> <out>
# OSGo frontend + Go build + run (clean OSG checkout at 0171e92b)
OSD_DATASET_READ=<osg>:<lib folders>:<out> OSD_DATASET_WRITE=<out2> \
  node tools/selfhost-step4b-osg.mjs --build --main <out>/abap/src <out2> <osg-0171e92b>
```

`--main` configures `ZCL_ABAPITI_HOSTIO` with absolute paths for the
`fs-snapshot.json` that the reference run records and for the output file. The
run prints the full exception chain (class, text, position, previous) when it
stops. Needs at least 40 GiB free on the output file system.
