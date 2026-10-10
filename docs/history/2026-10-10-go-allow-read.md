# TS-HG@Go read access and executable naming evidence

Base: `origin/main` at `619cfb42ef7835267580f3e780471f4c1093c59a`
(includes #74). Implementation: `351dcf9`, branch `feat/go-allow-read`.

The generation command builds `out/go/zabaplint-go`. The emitted host accepts
repeatable `-allow-read` and `--allow-read`, grants the directory trees containing
explicit input/config/deps paths and each listed dependency, and resolves list
entries relative to the list file. Canonical containment rejects parent traversal,
sibling-prefix confusion and symlink escape. Missing outside files also receive
an access refusal. All host reads, including the dependency list, use this policy.

## Validation

Commands ran in this clone with `GOCACHE=$HOME/.cache/abapiti-go-cache`,
`GOFLAGS=-buildvcs=false` and `TMPDIR=$PWD/.local/go-allow-read`.

- `go test ./tsfront -run '^TestRegistryGoReadPolicy$' -v`: passed. Compiles the
  actual generated CLI with a small harness and tests distinct default roots,
  relative dependency entries outside the list's directory, a different cwd,
  repeatable single/double-dash flags, refusal without panic output and explicit
  permission for a symlink target. Separately compiles and exercises the actual
  policy for outside reads, `../`, symlinks, sibling prefixes and missing files.
- `go vet ./...`: passed.
- `go test -short ./...`: passed.
- `./.github/ci/lint.sh gate origin/main`: passed with golangci-lint 2.13.2;
  zero new issues and all five canaries detected.
- `abapiti abaplint --target go`: generated all 1,927 classes and 73 interfaces,
  with zero blocking diagnostics, and built `zabaplint-go`.
- `python3 tools/go-target-check.py
  $HOME/dev/dell-work/kits/zabapgit-check-kit
  .local/go-allow-read/out/go/zabaplint-go`: passed. Runs from the generated
  module directory, with absolute input flags and **no allow-read flag**.
  Clean: 0 issues, byte-identical (12.293 s); seeded: 5 issues, byte-identical
  (11.364 s). These are single observations, not a performance claim.
- Unchanged `check.sh`, copied with the kit into this clone to confine its writes,
  invoked from the generated module directory with the absolute binary path:
  passed for clean and seeded. Both actual files also passed bytewise `cmp`.
  The script's existing `-allow-read .` works.
- `check.ps1`: not executed; PowerShell is absent. Downloading/executing portable
  PowerShell was rejected by automatic approval review because third-party binary
  execution lacked explicit user authorization. Approval was requested. The
  script's existing flags are covered by the generated CLI tests, but this does
  not constitute a PowerShell script execution result.
- Main was archived under `.local/go-allow-read/main-source` and built there.
  Main and branch both ran `abapiti abaplint -o` against the same output pathname
  (the first result was moved aside before the second run). `diff -r` compared
  all 6,336 output files: exit 0, no differences. Using the same pathname also
  preserves the absolute command examples in generated README files.

## Observed full-binary refusal

Exit status 1, stderr only, no Go panic traceback:

```text
refused: read access denied for ".local/go-allow-read/refusal/input/escape.abap" (resolved to "/home/alice/dev/dell-work/kits/zabapgit-check-kit/zabapgit_standalone.prog.abap"); use -allow-read "/home/alice/dev/dell-work/kits/zabapgit-check-kit" to allow it
```

## Draft PR description (not opened)

Title: TS-HG@Go: limit read access and name the binary zabaplint-go

The standalone Go host now grants read access only to directory trees containing
explicit inputs and listed dependencies. Additional roots require repeatable
`-allow-read DIR` (also `--allow-read`). Paths resolve symlinks before containment
checks, and refused reads identify the requested path and the flag needed to
grant access. Dependency entries resolve relative to `deps.txt`, allowing kit
checks from another working directory without extra permission flags.

`abapiti abaplint --target go` now builds `out/go/zabaplint-go`; CLI messages,
documentation and the go target CI job use the same name. CI exercises the read
policy and performs clean/seeded bytewise kit comparisons from another cwd.

Validation: vet, short tests and lint gate passed; full clean/seeded kit output
was byte-identical without `-allow-read`; unchanged `check.sh` passed; default
ABAP output matched main across all 6,336 files. PowerShell script execution
remains pending authorization to install/run portable PowerShell.
