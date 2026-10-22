# Self-hosting step 4b: gogen as ABAP on OSGo

Goal: OSG's gogen, compiled by abapiti to ABAP and run on OSGo (OSG pin
`.github/ci/osgo.ref` = `0171e92b`), prints the same Go as Node gogen on the
step-4 reference inputs (sha256 `b90648781967b2cf…`, `--expect-ref b9064878`).

## State on this branch

Measured by the Go-side owner on i10 and by the abapiti builders on u7.

| Stage | Result |
| --- | --- |
| `tools/selfhost-step4.py --abap` | emits about 4,600 ABAP files; `step4b-traps: 11 reached`, unattributable 0; guards counted separately |
| Union ABI | representation C: one payload interface with `kind`/`sig`; `union.*` gaps 0 |
| Cast probes | `IS INSTANCE OF`; `CATCH cx_sy_move_cast_error` count 0 |
| OSG frontend over the ABAP | 0 diagnostics |
| `go build` of the generated Go (`-p 1 -gcflags=-N -l`) | passes, about 210 s, peak about 21 GB |
| Run (`ZCL_ABAPITI_MAIN=>RUN`) | passes module initialization; first runtime stop below |

Step 4 (Go) is unchanged by every commit here: `hir.go` of the step-4 emission
has the same sha256 as on `selfhost`.

## What remains

1. **Class to structural interface.** A class value assigned to a TypeScript
   interface that it satisfies only structurally (gogen's
   `frontend-session-types.ts` `Registry` from abaplint's `Registry` class)
   becomes a nominal `?=` in ABAP and fails with `CX_SY_MOVE_CAST_ERROR`.
   Short term: type the gogen side as abaplint's `IRegistry`. General fix:
   when a class conforms structurally, emit `INTERFACES` plus delegating
   implementations; otherwise a view adapter.
2. Further runtime stops after (1), one at a time; each is classified A
   (abapiti), G (gogen) or O (OSG runtime).
3. Size: generated Go is about 167–200 MB; the remaining levers are listed in
   the bloat analysis (static method-level reachability, initializers as data,
   descriptor tables).
4. TS-HA output changes on purpose in this branch (initializer chunks, native
   dispatch and delete, cast probes, union C for programs with unions). Before
   merging to `selfhost`, the TS-HA and TS-HG finals (11,060 each), M1 and M2
   must pass on the branch head.

## Resume

```sh
# emit, list traps (reached / guards / unattributable)
python3 tools/selfhost-step4.py --abap --expect-ref b9064878 \
  <abapiti-bin> <osg-0b925177-with-step4-driver> <abaplint> <transpiler> <out>
# OSGo frontend + Go build + run (clean OSG checkout at 0171e92b)
OSD_DATASET_READ=<osg>:<lib folders>:<out> OSD_DATASET_WRITE=<out2> \
  node tools/selfhost-step4b-osg.mjs --build --main <out>/abap/src <out2> <osg-0171e92b>
```

`--main` configures `ZCL_ABAPITI_HOSTIO` with absolute paths for the
`fs-snapshot.json` that the reference run records and for the output file. The
run prints the full exception chain (class, text, position, previous) when it
stops.
