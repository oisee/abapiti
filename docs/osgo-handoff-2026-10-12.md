# OSGO handoff — 12 October 2026

Work is paused at the user's request. The goal remains all 11,060 Node-active
abaplint tests on TS-HA@OSGO, with zero wrong answers, traps, async-pending or
unregistered cases, then the same suite on TS-HA@A4H. Do not execute or upload
the suite to A4H before the complete OSGO suite passes.

TS-HG@Go already reached 11,060/11,060 in main (PR #129, 67c187b), with two
independent full runs. The celebration README/release work followed. This
achievement does not certify the separate ABAP emitter and runtime.

## Saved branches and review state

All heads below were pushed. OSGO integration changes remain unmerged.

| Branch | Head | Worktree under `.local/worktrees/` | Result / review |
| --- | --- | --- | --- |
| `integ/o1` | `63b91b7` | `pin2` | OSG pin bump and class ABI; constructor reference-cast compile defect repaired. Rules registration gaps remain; not certified for merge. |
| `feat/osgo-chai` | `5822a07` | `ochai` | Six Chai operations emitted in ABAP; 43 host unit assertions pass. Lexer 200/200. Four sparse-array statements regressions remain; guards retained. |
| `feat/osgo-json5-subset` | `fa6efed` | `ojson` | Includes O1 and Chai dependencies. Regex-free JSON5 subset, strict-parser delimiter repair, host evidence. [Draft abapiti PR #132](https://github.com/oisee/abapiti/pull/132), base main. |
| `feat/osgo-nested-views` | `db9fbd3` | `onested` | Built on JSON5 branch. Shared backing storage for nested activated class layouts; native nominal checks retained. [Draft abapiti PR #133](https://github.com/oisee/abapiti/pull/133), base JSON5 branch. |
| OSG `fix/gogen-character-set-fdpos` | `fd07c7928` | `osg-fdpos` | CO/CN and CA/NA update `sy-fdpos` on Go and JS hosts. [Draft OSG PR #719](https://github.com/oisee/open-steamgate/pull/719). Required by the JSON5 pin. |

## Measured achievements

JSON5 normalization is a seven-state scanner over 128-unit windows, using
CO/CN and CA with immediate `sy-fdpos` capture. It removes comments and trailing
commas, then calls the existing strict JSON parser. Unsupported JSON5 syntax
is explicitly refused. No regex was added to normalization. Real json5@2.2.3
supplied answers for 50 supported valid inputs, 22 invalid inputs and 11 valid
unsupported inputs. The ABAP host unit passes; default Go source and executable
are byte-identical to the Chai baseline.

Nested views now preserve original child identity, unknown keys and mutations
through aliases. Both structural and explicitly activated data-only layouts
pass executed OSG units. Replacement keeps old aliases attached to their old
graph; constructors are not called. Missing optional children and incompatible
nominal layouts are covered. Short tests, vet and lint passed.

| Complete measured shard | Passed / Node-active | FAIL | Traps | Unregistered |
| --- | ---: | ---: | ---: | ---: |
| lexer, JSON5 integration | 200 / 200 | 0 | 0 | 0 |
| statements, JSON5 integration | 429 / 4,962 | 0 | 4,533 | 0 |
| root, exact nested-view commit `db9fbd3` | 8 / 94 | 0 | 86 | 0 |

These are selected-shard results, not a whole-suite percentage. Statements
was not rerun after the nested-view change. All 429 prior statements passes
survive the JSON5 change; all eight prior root passing titles survive the
exact nested-view run. Async-pending is zero in these measurements.

JSON5 removed the emitter refusal from 4,528 statements tests, which then
reached a native cast boundary. Nested views remove the reproduced child-object
cast defect, but the root pass count stays at eight: subsequent runtime gaps
still block the same tests. The original Config JSON5 test remains trapped on
`json.stringifySubset` and `json.indent`; do not report it as passing.

## Resume here

1. Implement presence-aware array storage and shared live array views. Parsed
   arrays are still refused by ABAP field views. Preserve alias mutations,
   holes versus present `undefined`, null and element identity. Fix the four
   sparse-preallocation Chai regressions without weakening guards or making
   detached snapshots. No array foundation implementation has been committed.
2. Complete JSON serialization/indentation and dynamic mutation. Other remaining
   emitter gaps include dynamic calls, LSP guards, numeric/string operations
   and XML. Static catalogue coverage alone does not certify execution.
3. Run affected shards, retain title-level pass comparisons, then measure all
   17 pinned shards. Merge only after semantic gates and required CI are green.
   Dependency PR #719 must be handled before relying on its OSG pin in main.
4. Recheck default Go compatibility for the final integration. Keep A4H gated
   on the complete OSGO pass. Update README only with measured host achievements.

Evidence is linked from [JSON5](osgo-json5-subset.md),
[nested views](osgo-nested-views.md) and [Chai](osgo-chai.md).
The final root summary is [saved here](evidence/osgo-nested-views-final-root.json).
Local raw artifacts are in `/tmp/abapiti-json5-cert` and
`/tmp/abapiti-nested-cert`; committed evidence is the durable record.

Keep working in isolated worktrees. The root checkout has unrelated staged
changes/deletions and must not be reset or checked out. The original shared OSG
checkout is untouched. The named i3 colleague had no discoverable Herdr agent
at the last check, so no message was delivered; re-identify a real agent before
sending. User authorization covers coordination with that colleague.
