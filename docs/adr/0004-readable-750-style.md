# 0004. Plain, readable 7.50 style

Status: accepted, 2026-10-07 (Alice).

## Context

7.50 offers constructor expressions (`FOR`, `REDUCE`, `LET`, `FILTER`) that can
make code dense. abaplint and its transpiler use `push` 1,830 times, `concat`
132, `join` 113, `map` 86, `includes` 79, `some` 62, `find` 61, `filter` 29 and
`reduce` 6 times.

## Decision

Prefer plain statements: `DATA(lt_copy) = lt_orig.`, `APPEND`, `LOOP AT`,
`DO`, `WHILE`. `FOR` only for `map` and `filter` whose callback is a single
pure expression, e.g. `VALUE string_table( FOR lo_n IN lt_nodes
( lo_n->get_name( ) ) )`. No `REDUCE`, `LET` or the `FILTER` operator (it
needs a sorted or hashed key and differs from JavaScript `filter`). Built-ins
where they are exact: `concat_lines_of` for `join`, `line_exists` for a simple
`includes`.

## Consequences

- Output reads like hand-written ABAP; callbacks with statements become loops.
- The `FOR` rule lands with closures (phase 2).
