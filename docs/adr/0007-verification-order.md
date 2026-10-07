# 0007. Verification order for the HIR track

Status: accepted, 2026-10-07 (Alice).

## Context

OSG-JS and osgo run generated ABAP in seconds without SAP, but both are more
permissive than the kernel (open-steamgate issues #537–#543). OSG-JS runs on
the abaplint transpiler, which is one of the programs we translate, so its
agreement alone is not independent.

## Decision

1. Static gate on every build: abaplint (`v750`, `check_syntax`, unknown rule
   names fail, a negative test proves the gate rejects invalid code) and our
   generation rules.
2. Differential runs: the same ABAP corpus through the original TypeScript and
   through the generated ABAP on osgo and OSG-JS; outputs compared as JSON.
3. A4H after every phase, not only at the end.
4. Tests that do not fail when the code under test is broken do not count: the
   critic mutates code (for example, replaces a map `get` with `CLEAR result`)
   and a test must catch it.

## Consequences

- Kernel differences show up one phase at a time.
- osgo and OSG-JS gaps go to the open-steamgate inbox.
