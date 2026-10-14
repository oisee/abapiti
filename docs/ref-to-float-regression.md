# Tail batch 4: receiver-call numeric storage regression

Runtime pin: `f83dc5c3`; upstream: `577f875e`.

## Reproduction and cause

At `1b61759`, the OSG keep-single-parameter selection passes 6/19 and traps
13 times with `conversion ref -> f`. Kept emission identifies the first bad
statement in `Z_SRC_RULES_KE_6B142691166794=>Z_MEMBER_RUNPA_CDE5660C74D3B2`
(source line 393):

```abap
DATA t71 TYPE f.
t71 = me->z_member_calcs_fc4bc73478475c( z_param_c_17db8d24cbae2e = t72 ).
```

The callee's result is a reference to the tagged optional-Number runtime.
`1fdec0f` correctly lowered the result signature, but its storage analysis
resolved calls using only `Expr.Owner`. Receiver-based virtual calls have no
Owner, so their result expressions and consumers retained the stale native
Number type.

A combined double-space/downport/line-only-punctuation/config run then exposes
16 traps in the union-interface `getCol` forwarder on
`Z_SRC_ABAP_1_L_84A98CE5F498E0`: its interface result is `f`, but its inherited
implementation returns the numeric box. Signature linking visited only methods
declared on the implementing subclass, missing inherited implementations.

## Repair

`hir/abap/nonfinite_storage.go` now resolves instance calls from the receiver
type, matching `body.call`. It also resolves super calls from the enclosing
method's class and strips super-method specialization suffixes, as the
emitter does. Interface signature linking resolves each implementation through
the class hierarchy rather than requiring a body on the implementing subclass.
The existing fixed-point analysis then propagates the tagged result and argument
storage through callers, inherited methods, and interfaces. Scalar-only consumers still use the existing checked unboxing;
finite operands entering tagged arithmetic are boxed normally.

The corrected statement declares `t71` as a tagged-number reference and
uses tagged comparison against the boxed finite configuration value. No
NaN/Infinity lane is discarded, no nonfinite value is replaced by zero, and
no OSG/frontend/Go-emitter source changes are needed.

`TestNonfiniteReceiverCallStorage` exercises direct, virtual, inherited,
interface, inherited-interface, and super dispatch through `emitABAPNamed`.
Its virtual case fails when receiver resolution is removed; its
inherited-interface case fails when signature linking is restricted to local
implementations. Both expose the native-float signature and reference MOVE.
Each case also executes an isNaN assertion on
the pinned runtime. The existing scalar-storage fixture checks NaN and
finite values across constructor, field, and getter storage.

## Verification

Final OSG ONE union (five selected files, one shared build):

| Selection | Passed / Node-active |
| --- | ---: |
| keep_single_parameter_on_one_line | 19 / 19 |
| double_space | 38 / 38 |
| line_only_punc | 18 / 18 |
| downport | 298 / 298 |
| config (including both old-config cases) | 7 / 7 |
| **Total** | **380 / 380** |

- OSG complete `abap/lexer` shard (`count.ts`, `lexer.ts`, `type.ts`):
  **200/200**. All OSG selections have **0 FAIL, 0 traps**.
- Go target, all eight corresponding test files: **580/580**, 0 FAIL/traps.
  Five upstream-pending downport tests are excluded on both targets.
- All six dispatch fixtures, the original scalar NaN fixture, and the four
  tail-4 fixtures (Error metadata, optional parseFloat including Infinity,
  native JSON extras, optional live identity) execute successfully on OSGO.
- Mandatory `hir-unit.sh` with `HIR_RUNTIMES=" "`: exit 0.
- `go vet ./...` and `go test -short -p 2 ./hir/... ./tsfront/ ./cmd/...
  -count=1`: exit 0.
- All 15 individual `tools/*-test.mjs` scripts: exit 0 with Go 1.26.0.
- Lint gate against `origin/main`: **0 new issues**, canary **5/5**.

Kept evidence includes the original 13-trap run, the intermediate 16-trap
interface-forwarder run, both red regression checks, and the final green
union, lexer, and Go runs. A canceled single-file lexer selection is not a
verification result; the final lexer result includes all three files.
All heavy runs used the shared lock and memory-limited scope. OSG source was
not modified; scratch and the requested OSG gogen Go cache were cleaned.
