# 0009 — Source-pinned workload reachability

Status: proposed (body pruning implemented; declaration pruning pending).

## Context

Registry's static import closure includes tools, object handlers and rules beyond
any one check. Treating all their bodies as reachable obscures the blockers that
matter for a chosen deployment. V8 coverage is empirical evidence, not a proof
that an API cannot be called on a different input.

## Decision

Collect function-level V8 coverage from a fresh, verified original upstream
build. Union positive execution counts from the north-star check, its six
negative variants, and upstream unit tests for those rules and ABAP syntax.
Map functions through the fresh compiler's source maps to named TypeScript
methods/functions/constructors/accessors. Use the root function range count;
zero-count branch ranges never authorize removing a called function.
Unmapped, ambiguous and synthetic functions remain conservatively live. Anonymous
nested callbacks are not pruned separately.

Commit the evidence as an input containing the upstream pin, workload names,
input file/config hashes and SHA-256 fingerprints of the exact TS declaration
spans. Store UTF-8 byte offsets so tsgo and the collector use the same coordinate
system. Validate all selected spans before lowering, including executed spans;
a changed or missing span fails loudly instead of falling back to generic code.

An observed-but-unexecuted body lowers to `hir.Trap`. The ABAP emitter raises a
dedicated `cx_no_check` subclass with a `source_location` attribute containing
the TS file and line. It is distinct from the exceptions used for translated TS
payloads, so a translated catch cannot silently consume an excluded execution.
Do not evaluate default parameters or the excluded body. Keep the declaration's
signature; unsupported signature types still block emission. The normal lowering
API does not use coverage implicitly.

## Declaration pruning boundary

This first implementation retains the static closure and its declarations.
Dropping entire classes/modules requires a separate conservative reference graph
including signatures, inheritance, module initializers, namespace/class-value
registries and reflective factories. A zero method count alone is insufficient:
implicit constructors and field initializers can execute without a mapped TS
constructor. In particular, dropping exported classes before resolving
`artifacts_objects.ts` would change its reflective factory inventory.

Proposed next step: add explicit, fingerprinted factory selection, root the
reference graph in executed and unmapped declarations plus initializers, and
retain type-only dependencies as declarations with trapped bodies. Only nodes
outside that graph can leave the lowering input. Until then, report body pruning
and file count separately; never claim a reduced source closure.

## Consequences

Coverage is tied to the recorded workload, not general abaplint compatibility.
New inputs may encounter a visible trap. Recollect and review evidence when the
pin or workload changes. The manifest does not itself guarantee ABAP semantics:
HIR verification, v750 lint and independent differentials remain acceptance
gates. XML dynamic shapes, generators, callbacks and npm adapters still need
explicit semantics; they must not be disguised as unreachable paths.

Reproduction:

```sh
node tools/registry-coverage.mjs "$OUT" "$INPUT" "$DEPS_SRC" "$CONFIG" "$NEGATIVES" "$UPSTREAM"
REGISTRY_CLOSURE="$CLOSURE" REGISTRY_REACHABILITY="$OUT/reachability.json" \
  ABAPITI_TEST_OUT="$DIAGNOSTICS" go test ./tsfront -run '^TestRegistryClosureGate$' -v
```
