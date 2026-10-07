# 0002. The front end is tsgo, vendored into internal/tsgo

Status: accepted, 2026-10-07.

## Context

The old `ts/` prototype read only the syntax tree (`ts.createSourceFile`), with
no type checker. A translator needs resolved types, symbols, heritage and
narrowing. Options: the TypeScript compiler in Node, tsc-rs (Rust), or tsgo
(Microsoft's TypeScript 7 compiler in Go). abapiti and osgo are Go. tsgo's
packages are under `internal/`, so they cannot be imported; ramune solves this
by copying them and rewriting import paths.

## Decision

`tools/sync-tsgo.sh` copies the `internal/` packages that a Program and the
checker need from microsoft/typescript-go at a pinned commit into
`internal/tsgo`, rewrites import paths, and keeps LICENSE and NOTICE.txt. The
copy is committed (about 14 MB, 3 MB in git). `tsfront/` builds a Program from
the project's real tsconfig and dumps classes, members, signatures and the
checker type of every expression. tsc-rs stays a reference only.

## Consequences

- No network or Node needed to build or test the front end; the vendored code
  is visible in diffs.
- Apache-2.0 obligations: LICENSE and NOTICE.txt kept, the change (import paths)
  stated in `internal/tsgo/README.md` and in the root NOTICE.
- Updating tsgo means changing the pin and re-running the script; it refuses a
  dirty source tree or another commit.
- The linter excludes the vendored files; the leak scan allowlists the known
  upstream findings file by file.
