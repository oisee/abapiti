Closed-world progress settlement on `feat/fit-async`

This continues the pushed `ea3088f` settled-promise implementation, rebased onto
`origin/main` at `52dbd9c` (including the separately owned void ABI and LSP changes).
The baseline is the rebased async branch, rather than the older 635-blocker main.
Only TS to HIR and the settled-promise drain in Go/ABAP change here.

`IProgress.tick` is certified from the compilation's checked source declarations.
Each lowering starts with a fresh proof cache. The scan enumerates explicitly
implementing classes, inherited and structural implementations, static class
providers, and object literals. Bodies and immutable callback aliases must be in
the translated files. Async bodies must have certified awaits and promise
adoption; synchronous bodies must return certified promises without a fallthrough.
Pending executors, unknown providers, recursive settlement dependencies, unsafe
casts/erasure (including input wrappers), and tick mutation invalidate the proof.
The await-site diagnostic identifies the failing implementer and its source file.
Object-literal lowering and the ordinary void ABI remain with their owners.

The scan covers all non-declaration source files in the loaded Program. This
includes test files when the harness loads them. A provider loaded by the checker
but omitted from Lower's translated file list fails the proof. This is a closed
compilation; it does not authorize injecting arbitrary external implementations.
Await sites of the enumerated bodies also determine whether an unawaited call
could expose microtask interleaving. An empty provider inventory has no such
sites, so the existing optional calls in `RulesRunner` remain lowerable.

Optional progress lowering evaluates the receiver once and skips both the tick
call and argument effects on absence. Its HIR drain accepts an optional
`Promise<void>` only after the front end proves this IProgress call. A defined
promise is drained with its original rejection payload. Go executes the guarded
drain; ABAP guards the promise reference before checking and raising rejection.

Upstream test audit at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`:

- `packages/core/test` contains 301 `.parseAsync()` call sites in 132 files.
  Every call omits its argument; no test supplies a progress object.
- Core source and core tests contain zero IProgress provider classes or objects.
  The translated core closure consequently has an empty provider inventory.
- `git ls-remote --heads origin '*harness*'` returned no harness branch, including
  `feat/abaplint-tests-harness`. The compiler's multi-file regression probes load
  providers from `test.ts` together with a separately imported core interface.
- Tests cover safe async, synchronous, structural, inherited, static and object
  providers, optional receiver/argument effects, rejection propagation, and
  named refusal after adding an unsafe test implementation. They also cover
  loaded-but-untranslated providers, nested unproven inputs and tick mutation.

Measurement and gate commands, totals and every blocking message are recorded in
[fit-async-closed-world.json](fit-async-closed-world.json). The full measurement
uses `abapiti abaplint --full`, with the reachability manifest disabled by that
flag. `Registry.parseAsync` at `registry.ts:379` loses its settlement blocker;
there are no remaining blocking diagnostics in that method and no new HIR
verification findings (node IDs change as the method gains lowered expressions).

The default `all,go` outputs from main and the branch are compared at the same
output path, moving the main output aside first so generated executable paths
also agree. `diff -r` is empty. Vet and the complete requested short package suite
pass with Node on PATH and Go parallelism capped at two.

The full closure still has 491 blocking diagnostics (215 root, 276 cascade) and
386 HIR verification findings, all outside this async repair. The remaining
features include string/regexp operations, object and callback cases, types,
assignment and other separately owned roots. The 11,060 upstream Mocha tests
cannot yet run as a wholly compiled suite; this change removes the async root,
not those remaining compiler gaps. Future providers that require a scheduler or
whose source bodies cannot establish settlement are deliberately refused.
