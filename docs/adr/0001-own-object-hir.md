# 0001. The TypeScript track uses its own typed object HIR

Status: accepted, 2026-10-07 (Alice). Plan: [TS-HIR-PLAN.md](../TS-HIR-PLAN.md).

## Context

We translate the TypeScript subset of abaplint and the abaplint transpiler into
ABAP classes. That code is object-oriented: 941 `extends`, 913 `implements`,
3,047 `instanceof`, classes passed as values. MinZ (`minzc/pkg/hir`) has a
typed HIR with structured control flow and a MIR2 below it with Go and ABAP
back ends. Its type interface `mir2.Ty` is sealed and has no strings, doubles
or objects; fields carry byte offsets; methods are free functions; interfaces
are monomorphised without vtables; the node set carries Z80 details
(register classes, self-modifying code). `mir2abap` emits FORMs over register
variables and GOTO.

## Decision

A new HIR owned by abapiti (`hir/`): semantic types (Number, I32, I64, String,
Optional, ClassRef, InterfaceRef, ClassValue, Array, OrderedMap, OrderedSet),
classes with single inheritance, interfaces, virtual and super calls,
exceptions, a type on every expression. From MinZ we take the layering (typed
AST, names resolved, structured control flow), the text dump and the oracle
tests, not the types or nodes. Two independent reviews (codex gpt-6-astra,
Claude Fable 5.1) chose this over reusing or extending MinZ's HIR.

## Consequences

- abapiti and MinZ evolve independently; no veto of one over the other's IR.
- A shared, versioned dump subset can be added later if both want it.
- The ABAP emitter keeps classes, inheritance and interfaces one to one; a Go
  emitter can be added on the same HIR (virtual dispatch through interfaces).
