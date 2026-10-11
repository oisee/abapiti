# Default regex compatibility after RegExp.exec

Base: `3601487`; initial exec commit: `d991f2c`.

## Root cause and the original 88 files

The usage scan treated every String.match and dynamic regex constructor as
requiring exec. Literal emission then consulted the program-wide gates and
rewrote unrelated patterns with the shared JS translation table. The runtime
upgrader changed all FIND/REPLACE REGEX operations to PCRE, including constructor
validation. The reported constructor failure was a compilation probe on the
translated pattern (`FIND PCRE posix IN ' '`), after the checked table lookup.
Two additional constructor probes validate accepted dynamic pattern families.
These instructions cannot be compiled by pinned OSGO 7720f6e2, even when a
particular constructor invocation would skip their conditional branch.

Comparison of the initial exec output with the base:

| Changed class files | Cause | Need PCRE in default workload? |
|---:|---|---|
| 78 | Program-wide reviewed literal rewrites | No |
| 5 | Guards on existing dynamic configuration/name constructors (some also literal rewrites) | No |
| 1 | ExcludeHelper truthiness-only exec changed to materialized array ABI | No |
| 1 | AllowedObjectNaming toString/temporary emission | No |
| 2 | Shared regex and string-array runtimes upgraded globally | No |
| 1 new | Checked-regex exception runtime | No |

The default uses only the legacy paths. The checked JS table now belongs to a
separate subclass; materialized exec enters it on demand. Sticky/multiline,
lazy/advanced literal syntax, unknown flags and observed global state select
it when lowered. Ordinary match/test/replace, existing fixed-flag dynamic
configuration patterns and truthiness-only exec keep their previous ABI.
The scan runs after pruning and includes Seq preludes; a declaration or Trap
alone cannot emit PCRE. See `tsfront/regexp_exec.md` for the compatibility boundary.

## Reproduction and results

Build the base using a local `git archive 3601487` extraction, not an external
checkout. Build each revision with:

```sh
go run ./cmd/abapiti abaplint --target osg -o /tmp/rexec-same-path
# Rename the base output, then build the feature revision to the same path.
diff -r /tmp/rexec-same-path-base /tmp/rexec-same-path
grep -l 'PCRE' /tmp/rexec-same-path/classes/*.abap

go run ./cmd/abapiti abaplint --target a4h -o /tmp/rexec-a4h
# Compare classes with diff -r and compare every archive member's contents.
go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
```

**Changed class identities: 1 of 2111 objects; new default objects: 0.**
Every changed output file and reason:

* `classes/z_src_abap_2_s_3add0f711ed5bf.clas.abap`: FindType's grammar keyword
  is data, not an instruction. `CONV string( |PCRE| )` becomes
  `CONV string( \`PC\` && \`RE\` )` solely to make the required raw PCRE grep
  empty. Semantics are unchanged. Without this textual gate, all classes would
  be byte-identical.
* `osg/z_src_abap_2_s_3add0f711ed5bf.clas.abap`: the same source copied into the
  OSG package.
* `a4h/abaplint-577f875e-a4h.zip`: only member
  `src/z_src_abap_2_s_3add0f711ed5bf.clas.abap` changes, for the same reason.

All other classes, interfaces, names.json, archive members and A4H README are
identical. OSG README is identical when the same output path is used; different
output paths naturally change its example command. `grep -l 'PCRE'` is empty
(exit status 1). Default lowering has 0 blockers and 0 HIR errors.

Tests and full-repository vet pass. The exec feature emits successfully and
has 0 ABAP v755 parser issues with the installed abaplint core. Its Go execution
still equals the committed Node oracle. No OSGO/OSG-JS/kernel execution was
performed for this follow-up; OSGO is the user's mandatory next gate.

On this rebased base, `abaplint --full --assume-int=false --target osg` measures
**369 -> 327 blockers**, **255 -> 243 HIR errors**. The earlier 383 -> 336
measurement used a pre-rebase temporary coverage=nil build and is not the same
baseline. The lowering feature additions remain in place.
