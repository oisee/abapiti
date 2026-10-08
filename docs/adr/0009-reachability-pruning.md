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
Promise signatures now use an opaque nominal ABI. Only coverage-trapped async
bodies can emit; live async bodies remain blocking.
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

## Supported subset

Strict JSON is the chosen adapter contract. The north-star config is strict JSON.
JSON5-only syntax, object types outside CLAS/INTF/PROG/TYPE/XSLT, XML outside the
supported fast-xml-parser subset, and async APIs must raise loudly. The oracle
must retain the original outcomes, and the translation comparator must expect
these exceptions as explicit documented divergences, each with a reason. A
missing observation, successful translated return, or different exception is a
failure. These cases do not authorize ignoring an in-scope mismatch. Adapter
and comparator integration remain pending; the earlier strict-JSON conflict
is resolved by this scope rule.

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

## Round 3 adapter and telemetry continuation

Named namespace property accesses follow the resolved member edge; only a
namespace used as a value retains all exports. Dynamic/computed/reflection
uses remain conservative. No source declaration or coverage span is rewritten.

The XML boundary is a source-fingerprinted replacement of
`AbstractObject.parseRaw2`, sharing its HIR builder with the differential
fixture. Missing XML returns undefined. The ABAP runtime adapter preserves
strings, ordered object entries, arrays, empty values, repeated children,
namespace names, XML declarations, whitespace text, and the pinned parser's
CR/CRLF and document-root text behavior. Attributes are ignored. The supported
XML grammar has ordinary start/end/self-closing tags with quoted attributes,
`?xml` declarations, and amp/lt/gt/quot/apos entities. DTDs, comments, CDATA,
other processing instructions, numeric/unknown entities, malformed nesting or
headers, and prototype-sensitive names raise `RegistryXMLSubsetError`.

Tagged graph nodes distinguish undefined (an initial reference) from null,
strings, booleans, numbers, objects and arrays. Primitive truthiness, strict
comparison, optional/nullish access and boxing preserve those distinctions.
Opaque boxed native references retain identity and have explicit unboxing;
graph property access does not silently pretend those references are parsed
JSON/XML objects. Loose tagged coercion remains blocking. This XML adapter is
not a strict JSON/config adapter or a completed Registry translation.

Addendum 5 classifies parser/rule times as telemetry. Fingerprinted Date.now
sites use a shared GET RUN TIME helper accumulating microseconds and returning
truncated milliseconds. Signed-i rollover is accumulated in binary64; readings
must be separated by less than one full microsecond-counter cycle. This is a
monotonic duration clock, not an epoch clock. General Date remains unsupported.
Tests require integer, nonnegative, nondecreasing readings on both runtimes.
Times, durations and elapsed-work counters are listed in registry-scope.mjs as
nondeterministic, not compared; inventory/dependency counts remain compared.

Addendum 4's generator replacements are source-pinned snapshots rather than a
library-wide `Generator`/`Iterable` erasure. `Registry.getObjects` snapshots the
outer name keys and inner type keys, retaining the same object references and
order. `getObjectsByType` does the same for the selected type, including an empty
result for an absent type. The reached consumers enumerate immediately: object
counts, filename lookup, dirtiness/config invalidation, parsing, global definition
and macro discovery, include graph collection, DDIC/rule lookup, and the oracle's
inventory/dump extraction. Registry membership mutations occur during input
loading and between complete checks. The parser, syntax, macro and definition
consumers mutate object contents/reference indexes, not Registry membership.
The downport rule's edits load a distinct high-version Registry; rename and
unsupported proxy APIs are not reached by the accepted workload. The dependency
flag mutation happens between complete observations, so each new snapshot reads
the current state. None of these consumers retains a yielded iterator across a
membership mutation. Other iterator APIs remain blocking.

`MethodDefinitions.getAll` snapshots its private `all` map. Only construction's
parse/add methods populate that map; the reached method-name, inheritance,
implementation and parameter checks read it after construction. `getFiles` is
unreached and retains a located coverage trap, with an array-shaped opaque
signature solely for ABI verification. Fingerprinted interface result signatures
and the two RulesRunner parameter annotations agree with these replacements;
changing an unrelated Generator/Iterable annotation does not enable lowering.
The shared RulesRunner span pins both its clock expressions and its reviewed
parameter annotation. Every adaptation remains override debt to generalise later.
