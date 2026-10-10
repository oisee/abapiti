# Generated ABAP indentation

`hir/abap.Format` is the final text pass in `EmitNamed`, after cast tracing,
peephole optimizations and all runtime helpers. It follows abaplint's default
`Indent` settings (two spaces per block, four for TRY bodies, two for CATCH,
class sections and WHEN at the saved class/CASE level). The statement scanner
ignores periods inside literals, templates and comments. Statement continuation
lines receive the statement's indentation. `wrap` retains that prefix on every
split and includes it in the 255-byte limit.

The production ABAP generators outside `e.files` are the lexer/statements
unit-test drivers, structures benchmark class/test/report, and registry
class/test/corpus/report/native-CLI drivers in `tsfront/*_driver.go`. All run
`Format` on return. Their existing string-only APIs treat an unwrappable
source token as a generator bug. `golang_driver.go` emits Go, not ABAP.
Hand-written ABAP snippets in Go tests are test fixtures, not production
emission APIs.

`cmd/abapiti/abaplint.go` writes those formatted sources to `classes/`, copies
them to `native/lib/`, and writes the already formatted native CLI report.
`writeAbapGitZip` packages sources verbatim and only generates XML metadata;
it does not create additional ABAP statements. The independent LLVM/WASM/TS
legacy compile commands use their own existing indented printers; this change
is to the HIR/abaplint emission pipeline and its auxiliary drivers, not those
backends.

## Validation

Using Go 1.26.0, `GOFLAGS=-buildvcs=false` and the requested GOCACHE:

- `go test ./hir/... ./tsfront/... ./cmd/...` passed.
- `go vet ./hir/...` passed.
- A4H and OSG generation passed. With the supplied indentation-only config,
  abaplint 2.120.73 reported **20,781 → 0** issues per target (2,111 files
  analyzed). The issue count uses abaplint's default per-file cap.
- Every generated ABAP line was at most **255 bytes**, including native copies.
- `tools/compare-abap-format.py` confirmed identical normalized ABAP against
  `origin/main` (`69c79ee`): **2,111 A4H files**, **4,222 OSG files** (including
  native copies). The script preserves literal/template and comment contents.
- The local `main` ref (`1362df0`) is stale relative to this worktree's starting
  commit (`12d79da`): it lacks many already-existing compiler changes. Its
  baseline has 2,110 class files and 20,770 indentation issues. Literal equality
  to that old output is not possible without reverting unrelated changes;
  the current upstream-main baseline above is the semantics-preservation gate.

Tests cover nested IF/LOOP/TRY/CASE, ELSEIF/ELSE, class sections, interfaces,
events, FORM, chained declarations, literals, multiple statements per line,
column-zero star comments, and a wrapped statement inside a nested block.
The global-definition golden file now includes indentation.
