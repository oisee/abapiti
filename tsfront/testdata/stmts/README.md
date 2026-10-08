This is the statement-parser import closure from abaplint core at
577f875ebec44cfaf64841cfe71c8ab8dc32622e. The 779 TypeScript files under src/
are copied verbatim; LICENSE.abaplint contains the upstream MIT notice.
The harness/ directory belongs to abapiti.

Regenerate the corpus oracle with tools/statements-oracle.mjs against the
clean original upstream checkout at the pinned commit. The oracle checks the Git pin, tracked-source cleanliness and locked TypeScript version, then compiles into a disposable directory before loading any JavaScript. STMTS_EXPLORE=1 runs the diagnostic inventory;
STMTS_EXPLORE=1 STMTS_GATE=1 additionally requires complete lowering and
verified HIR. The lowering gate passes with zero blocking diagnostics and zero HIR
errors. ABAPITI_TEST_OUT exports the HIR and full diagnostic/verification
lists for investigation. Fingerprinted package overrides are inventoried in tsfront/overrides/abaplint.go.

Emit the full closure and its 64-case, one-method differential harness with:

    STMTS_EMIT=1 ABAPITI_TEST_OUT=/tmp/stmts-abap go test ./tsfront -run TestEmitStatementsClosure -v

Run tools/osgjs-unit.mjs /tmp/stmts-abap --json in the clean pinned runtime
checkout first, then tools/osgo-unit.mjs /tmp/stmts-abap --json. The harness
counts completed cases, exact statement trees, and per-case token/statement
counts before asserting equality, so failures report partial progress.
The independent oracle covers 5,229 tokens and 926 statements.

Run 3 result: all 64 cases and all 926 statement trees match the independent
upstream oracle on both OSG-JS and osgo at runtime pin
ad3d1e87cd3c3545b32dd4ba2ddeceb25708f05f. All 5,229 token counts match.

The emitted harness also runs the separately stored `stmtsregressions` oracle for
`INCLUDE zfoo.` without a registry, preserving the 64-case baseline counts.
Lifted-function parameter defaults are a blocking diagnostic until a faithful
capture-aware default prologue is available; they cannot pass the strict gate.

Run 4 integrates the parser onto main's native ABAP 7.50/binary64 emitter.
The v750 syntax gate passes for 798 files; all 784 non-final global classes
have every visibility section. The baseline remains 64/64 cases and 926/926
statements on OSG-JS and osgo; the separate no-registry INCLUDE test also passes.
The lexer remains 44/44 on both engines. Three-level erased dispatch (including
an interface receiver, a void interface value slot and super) executes correctly
on both engines. Skipping Alternative.run's second branch in emitted ABAP
compiles and fails at 27/64 cases and 574/926 statement trees.

After rebasing onto PR #45, both runtimes use the same default native ABAP 7.50
output. The CI runtime pin is 7e729432 (the full commit is recorded in
.github/ci/osgo.ref); no separate compatibility output is generated.
