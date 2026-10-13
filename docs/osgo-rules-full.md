# Full rules shard on TS-HA@OSGO

**5/3,149 passed; FAIL 0; 3,132 traps; 12 unregistered active titles.**
Original Node: **3,149/3,149**, FAIL 0, plus 10 pending titles outside the active
denominator. OSG skipped-active 0, async-pending 0; all returned
reports have zero unmatched titles and zero infrastructure failures.
[Result JSON](evidence/osgo-rules-full.json) retains the partition roots,
registration/initialization evidence, trap groups, lowering gates and host timings.

The focused measurement registered **256/268** active titles; this full measurement registers **3,137/3,149** active titles (3,147/3,159 including pending). The missing-title multiset is exactly the same 12 titles as the focused result.
Initialization traps are separate from body traps. Registration is not a test pass.

The full invocation exhausted the requested 16-GiB V8 heap before returning a
report. A three-part attempt then hit the 48-GiB container limit during Go build.
The completed measurement uses **12 sequential file partitions**, selected with
`ABAPITI_TESTS_FILES`; no test source or assertion changed. Every original file
is selected once. Fresh partition Node title multisets equal the independent
full rules oracle, separately for active and pending titles and preserving
multiplicity. Failed attempts contribute no test credits or registrations.

Top-20 body-trap groups (4 observed):

| Trap group | Count |
| --- | ---: |
| An exception was raised. | 1478 |
| runtime operation array.has has no ABAP emitter | 1331 |
| runtime operation json.stringifySubset has no ABAP emitter | 322 |
| runtime operation json.stringifySubset has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 1 |

Owning-body refusals do not prove execution of the named operation. Native
`An exception was raised.` messages remain unattributed.

| Successful host phases, summed over 12 partitions | Seconds |
| --- | ---: |
| Frontend | 1573.455 |
| Go emission | 148.823 |
| Go build | 1569.892 |
| Execution | 76.025 |
| Total | 3368.195 |

Successful partition wall time: **4784.842 s**,
including oracle, compiler build, lowering and ABAP emission. Peak process-tree
RSS: **34.286 GiB** (sum sampled every 250 ms);
largest child high-water RSS: **26.282 GiB**
(`wait4`/`getrusage`). Host phase totals repeat the shared closure in each partition
and are not a single-build full-shard timing. Per-partition and failed-attempt
times/RSS remain in the JSON. Core dumps were disabled for partitioned runs.

Parts 01–09 retain abapiti `fc4035a677bcf8feeffe38ee16ee9fd9f41d64f2`.
Parts 10–12 were rebuilt and run at `904d34c4cf3f3accc244371cd1ef98c81e076b03`,
which cherry-picks fix `342d9f33cd994bd70d5ecf930c7a7b7e4bdb21a3`.
This is a mixed-revision measurement: parts 01–09 were preserved, not rerun.

The first part-10 run stopped on a genuine FAIL in `sequential_blank.ts`:
`blank line matching WRITE:\t\t / 'abc'.\n` reported
`assertion failed: expected true to equal false`. ABAP POSIX `^` and `$`
match line boundaries, so `/^\s*$/` incorrectly matched after the trailing
newline. JavaScript without `m` requires input boundaries. Fix `342d9f3`
maps unescaped anchors outside character classes to `\A` and `\z` when
`m` is absent. The failed attempt is retained as history and contributes no
credits; the replacement part-10 and parts 11–12 have FAIL 0.

Pins: OSG
`368baab28111a072c3f5779a0e07d259305f836f`, abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, transpiler
`ee5c2cd1a45aaac6be503a46fc903ce95a946877`. Node v26.9.0, Go 1.26.0.
OSG and abaplint remained clean. All scratch, dependency and build caches are
under `.cache`. The harness used `GOMAXPROCS=6`, a 16-GiB Node heap, one worker,
`GOFLAGS=-buildvcs=false` (its build wrapper appends `-p=2 -buildvcs=false`).
**`GOGEN_NOLINE=1` was honored: zero line directives in all 12 generated Go files.**
Use the requested rules invocation with each JSON `partitions[].roots` joined by
commas as `ABAPITI_TESTS_FILES` to reproduce the partitions.
