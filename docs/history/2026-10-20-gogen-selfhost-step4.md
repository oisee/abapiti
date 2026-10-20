# gogen compiled by abapiti emits the same Go as Node gogen

Self-hosting step 4 is reached. OSG's Go generator, gogen, is JSDoc-typed
JavaScript and TypeScript. abapiti compiles it to Go. On OSG's captured
reference inputs, that Go program emits byte-for-byte the same Go source as
the original gogen running under Node. The unit test kit then passes on the
generated program.

| | Pin |
| --- | --- |
| abapiti (A) | `3115e569b2e05e469f0a1a650f7a2101ea029f19` (`integ/step4-abapiti`); first equal on `e054a615` |
| abapiti binary | sha256 `a9eeeaa14f3e34834c028780d69660e67f48675c039da6216d705ddfe2a641c9` |
| OSG gogen (G) | `1dbd12c76c1284bcae9deb9cac6fe506e9d0a959` (open-steamgate #817), merged to OSG main as `0b925177a19574a5f127da9c9ba355e344515b5c` |
| OSG runtime pin (`.github/ci/osgo.ref`) | `0171e92b2974854153a5ab4d645f0e53b6f27092` |
| Node reference output | sha256 `b90648781967b2cf3eaa4787f6dc9d41618682c26d18092e7608db18cfff5261` |

Result of `tools/selfhost-step4.py --expect-ref b9064878` on i10:

```text
step4-traps: 0 reached (0/11 unknown) 322 unreached
step4: build=ok run=ok cmp=equal ref=b90648781967b2cf out=b90648781967b2cf
```

The same run on the merge commit, OSG main `0b925177` with abapiti
`e054a615`, gave the same result. Both `gogen.go` and `ref.go` have sha256
`b906487…`, and `run-unit.sh` passes.

The final confirmation used the published pins, OSG main `0b925177` with
abapiti `3115e569`. It gave the same result: zero reached traps, `cmp=equal`
and `run-unit` PASS.

The generated module then ran the step-4 unit kit (`run-unit.sh`, s4kit2):
`PASS: ZCL_GOGEN_T_RUNTIME/LTCL_RUNTIME/ELAPSED`.

## How it got there

Each full run stopped at the first runtime trap. Every trap was classified
before it was fixed:

- **A:** abapiti mis-lowered ordinary typed TypeScript. These were fixed in the
  compiler, each with a failing test first.
- **G:** gogen told the type checker something that was not true at runtime.
  These were fixed in gogen. The Node output had to stay at the same reference
  SHA-256.

A fixes on the way to `3115e569`:

| Change | Cause |
| --- | --- |
| Union admission dispatches a typed carrier to its own member | foreign members of a typed value trapped instead of returning a range fault |
| `?:`, `??` and `||` joins of a nullable reference with `null`/`undefined` stay typed | a `null` join went through a dynamic value and reached a class view |
| One shared argument conversion for direct and stored-closure calls | structurally compatible arguments were refused in closure calls |
| Out-of-range array reads yield `undefined` | `body[i + 1]` at the end of a statement list trapped |
| Live read-only views for covariant `ReadonlyMap`, `ReadonlySet` and readonly arrays | two IR type families share container fields |
| Capture groups of `exec`, `match` and `matchAll` are `string \| undefined` | unmatched optional groups are holes, whatever `lib.d.ts` says |
| `object`-typed values keep plain records, arrays and class values by identity | opaque `{}` loop tokens |
| Class-to-shape assertions and prototype-bearing sources read through structural views | `{type: {row}}` assertions on attribute objects |
| Without range facts (inside `async` bodies) integer expressions keep their lowered type | the TS-HG final found 6 traps in `method_length_stats` |
| Arrays of nominal subclasses and of class-union interface refs stay off readonly views | the TS-HA@OSGO final found 432 traps: no ABAP emitter for the view |

`tools/selfhost-step4.py --list-traps` classifies a body trap by entry into its
owning function, so a single pass lists every trap the Node reference reaches.
List-traps equal to 0 on the current gogen is a required gate for every
step-4 compiler binary. Other required gates:

- short tests;
- the abaplint Go target with exactly two known assignment traps;
- M1 byte-equal output.

## Final gates on `3115e569`

| Gate | Result |
| --- | --- |
| M1 (TS-HG@Go on OSG's own ABAP) | PASS, byte-equal `95d37bc6…`, 27.0 s, 1.9 GB peak |
| M2 (frozen 4,676 files + 716 deps) | PASS, byte-equal `9a271f2a…`, 264.5 s on u7 (`b59f47d0`: 270.4 s), 10.7 GB peak; an idle-host A/B of `e054a615` against `b59f47d0` showed no regression (227–228 s against 230 s) |
| TS-HG@Go, 11,060 tests | PASS: 17/17 shards, 11,060 passed, 0 failed, 0 trapped; strict audit 12/12 |
| TS-HA@OSGO, 11,060 tests | PASS: 11,060 passed, 0 failed, 0 trapped. 16 shards ran in the full registry. The `rules` shard (3,149 tests) ran separately with a 12 GiB Node heap, because the registry's 8 GiB cap ran out of memory in its OSG build. That shard took 20.5 min with a 10.8 GB peak. |
| abaplint Go target | exactly 2 known assignment traps |

## CI follow-up on top of the measured pin

Two follow-up commits fix the repository CI. `7a73c482` emits ABAP named
regexp group fields only when the program includes the dynamic runtime.
`c2f41e69` updates the Grace lexer-facts golden for four new runtime catalogue
entries. Generated output is byte-identical to `3115e569` for:

- the step-4 gogen Go emit (only the run directory path in `go.mod` differs);
- every `abaplint --target go` source file;
- the full TS-HA ABAP of the abap/syntax and rules closure.

The finals above therefore carry over.

## Cache-domain sign-off

On OSG main `0b925177`, the cache-domain owner approved gogen's cache. The
re-proof ran on the #817 head `61f25917`:

- all five byte comparisons are equal;
- every row selects the same 3,055 classes;
- the emission is identical to main `2cb360e3`.

Between `61f25917` and `0b925177`, two lines change in the cache domain, both
reviewed and type-only or equivalent. `tools/gogen` on the merge commit is
byte-identical to the measured PR head `1dbd12c7`. Leak scan and size budget
both exit 0.

## Open follow-ups

- The TS-HA@OSGO `rules` shard needs more than 8 GiB of Node heap now. The final gate recipe should use 12 GiB, or the generated ABAP for that closure should shrink.

- abapiti should accept more legitimate TypeScript forms, so that gogen does
  not need to rewrite them:
  - intersection aliases with data members;
  - form-B probes through `unknown` on typed structs;
  - `Map.has` with an optional key;
  - function carriers for dynamic method-name reads.

  The list is in the tscheck backlog.
- `internal/tsgo/checker/exports.go` is a local addition to the vendored
  checker. The vendoring script must keep it.
