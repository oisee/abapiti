# Batch D: registration roots

Base: `2169c5d` (`origin/feat/fit-traps-b`). Implementation: `789bc0d`,
with proof-name guards and their regression in `5d7ba01`.
The complete before/after runs and remaining refusals are summarized in
[abaplint-tests.md](abaplint-tests.md). No genuine upstream assertion mismatch
was encountered; no failed record was reclassified.

## Immutable module callables

The biggest registration blocker was not a Node module import: it was ordinary
JavaScript function values such as `const str = Combi.str`. The previous lowering
could call the exported function directly but could not initialize its alias.
`combi.ts` and `combi_size.ts` then lost their entire registration tables.
`pretty_printer.ts` also used a module-level arrow to generate test titles.

`tsfront/module_callables.go` registers immutable aliases and module arrows as
static callees, using the existing function signatures and argument adapters.
This is not a general function-object ABI. Function identity comparisons,
arbitrary escaping function values and unmodeled receiver-dependent functions
still refuse. A whole-program, binding-identity write screen rejects replaced
bindings, including erased, destructuring and loop-assignment targets. Unrelated
shadowed names do not invalidate the proof.

Readiness flags preserve const initialization boundaries. Alias initialization
checks the target binding; calls check their callee **before evaluating argument
preludes**. Named higher-order callbacks also check readiness before traversal.
Ordinary modules using the new feature retain source-order initialization rather
than moving all constants ahead of module statements. Test modules keep the
existing independent, source-ordered recovery actions. A refused function body
is deferred to invocation; it does not prevent creation of the callable binding.
TDZ cases explicitly refuse outside JavaScript catch rather than pretending to
implement a catchable ReferenceError. ABAP uses existing static fields, methods
and explicit traps; no new runtime operation or backend ABI was introduced.
Coverage-pruned deployment lowering is unchanged.

## Complete literal registration-row contexts

Fresh object rows with different optional fields previously became an array of
`Object`, even when every row carried the title/data fields needed for
registration. This lost `n`/`top` in `nesting.ts`, and `input` in pretty-printer
suites with optional configuration objects.

`testModuleArrayContext` now recovers the complete checker structural context
for **fresh object-literal arrays in test files**, including local tables inside
describe callbacks. For unions it obtains all possible properties and sorts them
for deterministic schema construction. It verifies every source property is
represented; spreads, computed properties, getters and nonliteral elements do
not receive this proof. Existing shape construction preserves absent optional
fields, nested options and the original row values. Module signatures and
initializers use the same context; the global nominal union/constructor ABI is
not changed. Local tables do not repeatedly scan the whole test file for helper
argument contexts.

## Failure guard

`tools/abaplint-tests-all.mjs` now treats genuine failed **initialization records**
as stop conditions too. It also checks raw results before borrowed-title
accounting, so a genuine borrowed test failure cannot be hidden by ownership
filtering. Reporting still distinguishes failures, typed refusals and not-run
cases; no oracle titles or synthetic registrations are supplied to Go.

## Retained regression tests

- `TestModuleConstCallablesNodeOracle`: alias chains, defaults/rest arguments,
  module arrows, newline-derived titles and named map callbacks; independent
  Node output and Go execution, plus ABAP emission.
- `TestModuleCallableTDZNodeOracle`: Node establishes that TDZ callee lookup
  precedes argument effects; Go explicitly refuses with no argument effect.
  ABAP emission is checked as well.
- `TestModuleCallableInitializationBoundaries`: unavailable forward aliases,
  independent later suites, and deferred body refusals.
- `TestModuleCallableMutationRefuses`: replaced and erased target bindings
  remain unsupported, rather than becoming incorrect direct calls.
- `TestRegistrationLiteralRowsNodeOracle`: unchanged registration source on
  Node/Go, top-level and local heterogeneous rows, absent scalar/nested fields,
  preserved exclusive properties, titles and body assertions.
- `TestRegistrationProofRejectsComputedNames`: computed-name tables remain
  explicit refusals; the new proof does not assume identifier-shaped names.
  The callable refusal tests also cover destructured bindings outside test
  initialization (the pre-existing test-action binding-pattern panic is outside
  this batch's ownership).
- `tools/abaplint-tests-all-test.mjs`: initialization assertion failures stop
  the runner, alongside the existing body-failure stop regression.

All test closures and generated builds are temporary. No upstream source was modified,
no transpiler was cloned, and nothing was pushed.
