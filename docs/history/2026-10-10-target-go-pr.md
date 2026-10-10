# Draft PR: Add TS-HG@Go as an abaplint CLI target

`abapiti abaplint --target go -o out` now writes a standalone standard-library
Go module and builds `out/go/zabaplint` when Go is on PATH. Without Go, it leaves
buildable sources and reports the build command. This exposes the direct
TypeScript → HIR → Go backend as TS-HG@Go, using main's pinned closure, overrides
and reachability manifest. Existing default and `all` ABAP targets are retained;
`--target all,go` explicitly requests both.

Runtime refusals now include the originating TS method location for bounded
regex, replacement, ClassValue, numeric, JSON/XML and pruned-body contracts.
Static unsupported shapes remain explicit diagnostics. The limits and usage
are documented in the CLI guide and README.

`go vet ./...`, `go test -short ./...` and the lint gate against origin/main
passed (zero new issues, five lint canaries detected). Measured locally:
generation 20.394 seconds + Go build 25.6 seconds = 46.0 seconds;
clean/seeded check medians of three are 12.261/13.053 seconds. Other builds
were active; these are command wall times with the Go cache enabled.

Evidence: default ABAP output on origin/main and this branch is byte-identical
under `diff -r`, using the same output path with no exemptions. The lexer passes
44/44 plus a mutation check; registry oracles pass; clean (0 issues) and seeded
(5 issues) release-kit output is byte-identical. A normal cached **go target**
CI job repeats these checks within a 15-minute timeout. Verification and measured
build/check costs are recorded in [the integration evidence](2026-10-10-target-go.md).

This does not change ABAP generation, inlining, pins, the workload coverage or
release binaries, and does not provide a general JavaScript runtime or make Go
a default target. The maintainer decides whether the new CI job is required.
Follow-ups: the allocation pass, and `run_one` at HIR level (separate from PR #73).
