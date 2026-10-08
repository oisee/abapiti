# 0009 — Source-pinned workload reachability

Status: proposed (provenance, body and declaration pruning implemented; Registry acceptance pending).

## Context

Registry's static import closure includes tools, object handlers and rules beyond
any one check. Treating all their bodies as reachable obscures the blockers that
matter for a chosen deployment. V8 coverage is empirical evidence, not a proof
that an API cannot be called on a different input.

## Decision

Collect function-level V8 coverage from a fresh, verified original upstream
build. Record per-span positive execution provenance as `DEPLOYMENT` (the north-star
check), `NEGATIVE` (the six negative variants), `OBSERVATION` (the complete Registry oracle inventory/config/dumps and negative observations), and `UPSTREAM` (original upstream
rule and ABAP syntax tests). Schema 2 keeps `executed` as the informational union
of all four; pruning roots are **DEPLOYMENT ∪ NEGATIVE ∪ OBSERVATION**. UPSTREAM-only
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

A body without DEPLOYMENT, NEGATIVE or OBSERVATION execution lowers to `hir.Trap`. The ABAP emitter raises a
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
DEPLOYMENT/NEGATIVE/OBSERVATION-executed and unmapped callable declarations plus module
initializers, follows resolved signature (including inferred returns), heritage,
namespace and class-value references, and skips excluded bodies. Type-only
classes retain signatures and trapped constructors, without evaluating their
initializers. The source AST is kept intact for checker queries. Only retained
statements enter lowering. Report selected file/declaration counts separately
from the original pinned static closure.

## Observation workloads and input validation

Anything the north-star check or our acceptance tests execute is a root. The
collector runs the same exported oracle used for comparisons against the same
fresh build under OBSERVATION coverage. Description extraction and the negative
oracle cases therefore contribute roots, while upstream-only APIs remain trapped.
Every recorded input/config/negative-variant fingerprint is validated against
independently supplied current workload paths before any pruning. Added, removed,
changed or unbound inputs fail loudly. Function identity requires a unique source
range and declaration kind; ambiguous mappings are omitted and stay live.

## Strict JSON acceptance conflict

Decision 1 requires JSON5-only syntax to raise. The required oracle's `json5`
negative observation instead returns a resolved `v702` configuration for comments,
unquoted keys, single-quoted strings and trailing commas. Fresh OBSERVATION
execution reproduces this result. Strict JSON parsing raises for the same bytes,
so the required negative observation cannot remain equal under that decision.
The original differential remains intact; no strict JSON adapter is substituted
until this acceptance conflict is resolved.

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
REGISTRY_INPUT="$INPUT" REGISTRY_DEPENDENCIES="$DEPS_SRC" \
  REGISTRY_CONFIG="$CONFIG" REGISTRY_NEGATIVES="$NEGATIVES" \
  REGISTRY_CLOSURE="$CLOSURE" REGISTRY_REACHABILITY="$OUT/reachability.json" \
  ABAPITI_TEST_OUT="$DIAGNOSTICS" go test ./tsfront -run '^TestRegistryClosureGate$' -v
```
