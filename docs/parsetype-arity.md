# TS→HIR call arguments and parseType

The `FindGlobalDefinitions.run` call at
`src/abap/5_syntax/global_definitions/find_global_definitions.ts:39` supplies
one argument, `o.parseType(this.reg)`. Its narrowed implementations declare
one parameter. The receiver's synthesized union ABI also includes
`Domain.parseType(reg, dataElement?, description?)`, so its virtual slot has
three parameters. A wider union discovered later during body lowering changes shared
implementation slots and therefore earlier union views. The already-built
call still has one argument when the final slot has three. Selecting a
constituent signature also bypasses the receiver slot. HIR correctly refused
that physical call with `call arity mismatch`.

Union/interface completion and bridge generation now run to a fixed point
after all bodies, before call arguments are completed against the final ABI.
The compatible views grow optional tails from a finite set of source
signatures. A bridge widened more than once gets a distinct specialized target
name, retaining earlier targets. The regression creates two successively wider
unions after the caller body and executes every constituent implementation.
Calls also use the receiver's slot, including erased generic slots. Existing
optional argument lowering supplies typed absent values for omitted optional
parameters; bridges drop parameters an implementation does not declare.
There is no parseType-specific override, backend repair, or verifier exception.

Fixed calls evaluate every supplied argument in source order. Accepted values
are saved before evaluating extra arguments for effects and discarding their
values. Member receivers run before argument-construction preludes. Finite
tuple spreads distribute their values against the implementation ABI; rest
arguments are collected into a fresh array. Defaults run inside the callee's
absence checks, in declaration order, after caller argument evaluation.
Lifted functions use that same prologue after their leading capture parameters.
Overload declarations contribute no duplicate runtime methods: the body-bearing
implementation defines the HIR signature. Skipped signature declarations retain
their source IDs so later generated closure names stay stable.

Named array callbacks now pass the operation's actual arguments and adapt them
to the callee's parameter list. Callbacks with defaults, rest, or omitted tails
use a lifted callee and the same adapter. Map-like operations supply element,
index, array; reduce supplies accumulator, element, index, array; sort supplies
two elements. Short callbacks ignore the additional supplied values. Existing
parameterless thunk calls also evaluate and discard extra arguments without
changing the thunk ABI. Simple existing callbacks retain their original inline
lowering. HIR is shared by ABAP and Go; focused tests execute emitted Go and
verify ABAP emission for every supported arity case.

Accepted arguments are saved as they are lowered, before later argument
construction can emit a prelude. This also applies to the fixed prefix of a
rest call. Focused regressions nest a rest call in the second argument and
check the order of three observable effects.

Required parameters outside the supplied typed domain still refuse instead of
inventing a primitive zero value. Arbitrary fixed-parameter array spreads,
sparse-array/getter semantics, general escaping functions with parameters,
and impure sort comparators remain outside the established lowering domain.
The existing sort implementation assumes a pure comparator; this change does
not change its comparison algorithm. Dense-callback proofs are unchanged.

## Reproduce

The task base is `origin/feat/fit-config-json` at
`0ca327c96f530d1bfbca3b97c721db243234498c`. Harness measurements combine it locally
with `origin/feat/abaplint-tests-harness` at
`f51056bd2139b3e2dc779079773fbd6d1538cd7c`, then apply only this feature patch.
The combination is never pushed. Merge conflicts retain both the config writer
and conditional harness repeat runtime, both call hooks, the base's unique
lifted names with the harness trap boundary, and the base's facts golden.
The feature thunk conflict retains the harness's field receiver and Mocha hook,
adding only argument evaluation. Both local measurement copies also reuse the
verifier's declaration maps during body repair. The original recovery repeatedly
rebuilt those maps for every method after each backend refusal; a diagnostic
stack after 16 minutes identified that work. The cache is built after signature
repair, retains the same class/interface pointers, and observes field removal.
It leaves body checks, error order, trap replacement, and registrations intact.
A local equivalence test compares fresh and cached verification across field
removal, traps, bad return types, missing bodies, and abstract-body changes.
This optimization and recovery logging exist only in the disposable harness
copies, not in this branch. The final rules binary was built just before
removing the unused inline zero-parameter array-callback rejection. A local
AST audit of every file in its materialized program finds zero such callbacks
for map/filter/some/reduce/find/sort/forEach/every, so that final removal does
not affect the shard output. Its new focused regression passes; full-closure
and default-output gates use the final compiler. The rules closure uses the current harness
shim files and every sorted `test/rules/*.ts` path, including helpers: 192 files.
The original pinned Node oracle supplies the 3,159 test titles for reconciliation.
The historical 196-test cohort is identified by title occurrence from the
config-json task's raw rules results, independently of new registrations.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOFLAGS=-p=2 GOMAXPROCS=1
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./tsfront -run TestCallArity -count=1

/tmp/parsetype-before-bin abaplint --full -o /tmp/parsetype-full-before
/tmp/parsetype-after-bin abaplint --full -o /tmp/parsetype-full-final
/tmp/parsetype-harness-before-bin abaplint-tests /tmp/parsetype-rules \
  -o /tmp/parsetype-rules-before
/tmp/parsetype-harness-after-bin abaplint-tests /tmp/parsetype-rules \
  -o /tmp/parsetype-rules-after-final
```

Build the baseline executable from `git archive` of the recorded task base and
the feature executable from this branch. Generate default outputs with identical
output spelling because generated READMEs embed it:

```sh
/tmp/parsetype-before-bin abaplint -o /tmp/parsetype-default --target all,go
cp -a /tmp/parsetype-default /tmp/parsetype-default-before
rm -rf /tmp/parsetype-default
/tmp/parsetype-after-bin abaplint -o /tmp/parsetype-default --target all,go
diff -r /tmp/parsetype-default-before /tmp/parsetype-default
```

Measured results are recorded in [evidence/parsetype-arity.json](evidence/parsetype-arity.json).

## Measured outcome

The current rules shard registers 3,074 of the Node oracle's 3,159 titles.
Before: 10 pass, 0 fail, 1,596 trap, 1,553 do not run. After: 15 pass,
104 fail, 1,487 trap, 1,553 do not run. The parseType trap goes from 245 to 0.
The 104 newly reached failures share the numeric/conversion guard in
`src/rules/7bit_ascii.ts:38`; they were hidden behind the arity trap before.
All previously passing titles still pass.

The original 196-test cohort goes from 196 arity traps to 4 passing,
104 failing and 88 trapped tests. The remaining cohort traps are 83
`edit_helper.ts` foreach array-type failures, 4 dense/stable receiver proof
refusals in `contains_tab.ts`, and 1 regex refusal in `omit_preceding_zeros.ts`.
The current 245-test cohort additionally reaches optional edit-map typing and
other regex refusals. Those blockers are independent of call arity and outside
this task's assigned scope. The dominant overall shard trap remains the
1,321 `_utils.ts` dense/stable foreach receiver refusals. The synchronous
harness still excludes 1,458 async/done tests; 10 are skipped and 85 titles
are unregistered because other module initializations fail.

The full closure retains 347 blocking lowering diagnostics (136 roots and
211 cascades), unchanged by exact message. HIR verification errors fall from
284 to 262: all 22 call-arity mismatches are removed. The eight affected source
files are `_check_database_fields.ts`, `select.ts`, `sql_compare.ts`,
`find_global_definitions.ts`, `ddic.ts`, `lsp/help.ts`, `lock_object.ts`, and
`maintenance_and_transport_object.ts`. Other typing and unsupported
feature diagnostics remain; this work does not claim the whole closure runs.

The default `abaplint --target all,go` output is byte-identical to the task
base (`diff -r` returns 0 with empty output), including ABAP distributions,
Go sources, metadata, archives, and the Go executable. The branch does not
contain the harness combination, body-verifier cache, or measurement logging.

Validation: `go vet`, the required short suite with `-p 2`, and the focused
arity suite pass. No WASM or backend ABI changes are included.
