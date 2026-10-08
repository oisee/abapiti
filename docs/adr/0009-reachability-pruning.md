# 0009 — Source-pinned workload reachability

Status: proposed (provenance, body and declaration pruning implemented; Registry acceptance blocked by the observation boundary below).

## Context

Registry's static import closure includes tools, object handlers and rules beyond
any one check. Treating all their bodies as reachable obscures the blockers that
matter for a chosen deployment. V8 coverage is empirical evidence, not a proof
that an API cannot be called on a different input.

## Decision

Collect function-level V8 coverage from a fresh, verified original upstream
build. Record per-span positive execution provenance as `DEPLOYMENT` (the north-star
check), `NEGATIVE` (the six negative variants), and `UPSTREAM` (original upstream
rule and ABAP syntax tests). Schema 2 keeps `executed` as the informational union
of all three; pruning roots are **DEPLOYMENT ∪ NEGATIVE only**. UPSTREAM-only
spans trap just like unexecuted spans. In particular `Registry.parseAsync` is
UPSTREAM-only and must use an opaque async signature with a trapped body.
That signature implementation is pending.
Schema 1 remains supported for older callers, with its original union policy;
the Registry workload must use schema 2.
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

A body without DEPLOYMENT or NEGATIVE execution lowers to `hir.Trap`. The ABAP emitter raises a
dedicated `cx_no_check` subclass with a `source_location` attribute containing
the TS file and line. It is distinct from the exceptions used for translated TS
payloads, so a translated catch cannot silently consume an excluded execution.
Do not evaluate default parameters or the excluded body. Keep the declaration's
signature; unsupported signature types still block emission. The normal lowering
API does not use coverage implicitly.

## Declaration pruning boundary

Schema 2 uses a checker-resolved conservative reference graph
including signatures, inheritance, module initializers, namespace/class-value
registries and reflective factories. A zero method count alone is insufficient:
implicit constructors and field initializers can execute without a mapped TS
constructor. In particular, dropping exported classes before resolving
`artifacts_objects.ts` would change its reflective factory inventory.

The opt-in Registry deployment adds fingerprinted factory selection for
CLAS/INTF/PROG/TYPE/XSLT, trapping other types and the old reflective map builder.
Replacement dependencies are explicit in the override registry. The graph roots
DEPLOYMENT/NEGATIVE-executed and unmapped callable declarations plus module
initializers, follows resolved signature (including inferred returns), heritage,
namespace and class-value references, and skips excluded bodies. Type-only
classes retain signatures and trapped constructors, without evaluating their
initializers. The source AST is kept intact for checker queries. Only retained
statements enter lowering. Report selected file/declaration counts separately
from the original pinned static closure.

## Observed acceptance conflict

The required round-1 inventory oracle calls `getDescription()` for every object.
Fresh schema-2 evidence excludes all five object handlers' description methods;
`Class` and `Interface` are UPSTREAM-only, and `Program`, `TypePool` and
`Transformation` are unexecuted in the collected classes of workload. Following
the chosen policy therefore makes the required inventory throw on its first
object at `src/objects/program.ts:27`, where the original returns
`abapGit (standalone version)`. The same failure occurs on a one-file Program
inventory. The compiler's located trap agrees with an independent fresh-original
JS policy experiment. The explicit five-type factory alone preserves all 188
original inventory entries.

This is a conflict between the executed check workloads and required observation
APIs, not an async policy question. No observation-only span is promoted to a
root without a coordinator decision; remaining ABI/runtime work stops here.

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
