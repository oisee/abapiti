# 0005. number stays binary64 unless an integer range is proven

Status: accepted, 2026-10-07.

## Context

In abaplint core 3,708 number expressions are provably integer and 4,501 are
not proven; explicit fractions are rare (core and transpiler together: 2
divisions, 1 `parseFloat`), but
`Math.ceil((n + 1) / 2)` exists. ABAP `f` raises on overflow and division by
zero where JavaScript gives Infinity or NaN.

## Decision

The HIR keeps `Number` as binary64 (ABAP `f`); `I32`/`I64` (`i`/`int8`) only
where the range is proven. Phase 0 rejects `Number` division (integer division allows any divisor) unless the divisor is a
non-zero literal and rejects NaN and Infinity literals, with a source
location. Results beyond the binary64 range raising in ABAP instead of giving
Infinity is an accepted, documented divergence (`hir/README.md`) until
exceptional numbers get run-time handling.

## Consequences

- Behaviour changes are visible (a diagnostic), not silent.
- The critic's overflow finding on PR #33 was overruled for this reason; the
  general case is a later task.
