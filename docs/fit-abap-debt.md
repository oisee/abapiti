The ABAP emitter now covers `object.get`, `object.hasOwn`, `object.has` and
`xml.validate`, closing the four emitter gaps introduced by the fit-rules stack.
The branch starts at `origin/feat/fit-rules-2`, commit
`b4b9b43d5ad141a2664142156a7e1019168a6c57`; the default-output reference is
`origin/main`, commit `7659ba6305cc300558b41b8d163f4f93a259de5c`.

Property tables distinguish own fields (including inherited and absent optional
fields) from prototype members, dispatch on the actual class, preserve primitive
values and stable prototype identities, and return absence for unknown names.
Tables and prototype tokens initialize on first use, avoiding module import
cycles in OSG-JS while preserving ABAP class initialization behavior. The usage
scan also visits `Seq` statement bodies before declaring class metadata.
The validator retains fast-xml-parser 5.10.1 default-option behavior, error codes,
messages and UTF-16 line/column positions. Its ABAP runtime reads high code units
through the same UTF-16 converter strategy as the string emitter.

The shared HIR fixtures contain 444 XML inputs, 30 property cases, and one isolated `Seq` property case. XML inputs
are the complete corpus assembled by `TestRulesXMLValidatorGoMatchesNode`:
51 edge cases, 50 pinned rule cases, and 343 deterministic mutations. Each ABAP
case is a separate unit method so failures identify the input's case number.
`TestXMLOracleMatchesPinnedNode` checks the committed expected results against
the vendored Node validator; `TestRuntimeDebtFixtures` executes the same HIR on
Go and exports ABAP through `.github/ci/hir-unit.sh`.

`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` lowers the actual pruned
zabapgit build during the normal short gate. It checks every backend's explicit
operation catalogue and a frozen default operation allowlist. New operations
must be reviewed before that allowlist changes. `TestSelectedTargetRuntimeOpsFailLoudly`
checks the selected-target diagnostic, including source location and operations
inside a constructor's `Seq`. The CLI checks selected-target coverage before
writing generated sources; the ABAP emitter also checks coverage directly.

Full-closure evidence is in [fit-abap-debt-gap.json](fit-abap-debt-gap.json),
including complete blocking-message histograms. Commands:

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOFLAGS=-p=2
# Before: binary built at the fit-rules-2 base; after: feature branch.
go build -o /tmp/fit-abap-debt/before ./cmd/abapiti
/tmp/fit-abap-debt/before abaplint --full -o /tmp/fit-abap-debt/before-full
go build -o /tmp/fit-abap-debt/final ./cmd/abapiti
/tmp/fit-abap-debt/final abaplint --full -o /tmp/fit-abap-debt/after-full

go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
TRANSPILER="$PWD/.local/fit-abap-debt-transpiler" \
  bash .github/ci/hir-unit.sh /tmp/fit-abap-debt/runtime-green
```

| Measurement | Before | After |
| --- | ---: | ---: |
| Missing ABAP emitters in this category | 4 | 0 |
| Full-closure blocking diagnostics | 434 | 434 |
| Root blocking diagnostics | 185 | 185 |
| Cascade blocking diagnostics | 249 | 249 |
| Blocking diagnostics in rules | 283 | 283 |
| HIR verification errors | 347 | 347 |

The frontend counts and every blocking-message count are unchanged because this
work implements operations that were already lowered by TS→HIR. Remaining
closure failures belong to other lowering categories; no additional lowering
roots were changed here.

For the complete default-output comparison, both binaries emit to the same
pathname in turn. This keeps generated README paths and Go binary source paths
identical, allowing `diff -r` to compare every byte without exclusions:

```sh
# main-bin was built in a detached worktree at the reference main commit.
/tmp/fit-abap-debt/main-bin abaplint --target all,go \
  -o /tmp/fit-abap-debt/default-stable
mv /tmp/fit-abap-debt/default-stable /tmp/fit-abap-debt/default-stable-main
/tmp/fit-abap-debt/final abaplint --target all,go \
  -o /tmp/fit-abap-debt/default-stable
diff -r /tmp/fit-abap-debt/default-stable-main /tmp/fit-abap-debt/default-stable
```

Validation of the final implementation: all 444 XML and 31 property cases pass
on Go, osgo and OSG-JS. The four core HIR suites total 484/484 passing methods
on each ABAP runtime, with no hir-lint issues. The new runtime helpers also pass
an ABAP v702 syntax check using the pinned abaplint dependency. The full vet and
short-test commands above pass, and the complete default-output diff is empty.

The complete `hir-unit.sh` run exits 0, including all 22 lexer subchecks on
both runtimes (three deliberate failures must be detected). The final
475-method debt suite, including the isolated `Seq` case, also passes a
focused generation/lint/execution run on both runtimes:

```sh
ABAPITI_TEST_OUT=/tmp/fit-abap-debt/seq-hir \
  go test -p 2 ./hir/abap ./hir/golang -run TestRuntimeDebtFixtures -count=1
node .github/ci/hir-lint.mjs /tmp/fit-abap-debt/seq-hir/TestRuntimeDebtFixtures \
  /tmp/fit-abap-debt/runtime/open-steamgate
npm --prefix /tmp/fit-abap-debt/runtime/open-steamgate run -s osgo:unit -- \
  /tmp/fit-abap-debt/seq-hir/TestRuntimeDebtFixtures --json --jobs 2
npm --prefix /tmp/fit-abap-debt/runtime/open-steamgate run -s osgjs:unit -- \
  /tmp/fit-abap-debt/seq-hir/TestRuntimeDebtFixtures --json
```
