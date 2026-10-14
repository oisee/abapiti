# ABAP constructor compatibility

Baseline: `origin/main` at `91e301d`. The unpruned embedded abaplint core
contains **195** live-view subclass constructors that called their superclass
conditionally. They now call it **once, unconditionally, at method scope**,
before binding the source and returning. The optional `view_source` argument
passes through all known ancestors; roots have no superclass call.

Native parameter defaults, statements before `super()`, argument evaluation
and lazy static initialization retain their order and run only on the native
path. Evaluated arguments are stored before the unconditional call. Parameter
locals retain default assignments after the call. Constructor bodies are copied
when removing the original HIR call; Go emission keeps the original HIR.

The execution fixture covers abstract roots, required constructor arguments,
parameter setup before `super()`, argument side effects, field initialization
after `super()`, constructors without parameters, synthesized constructors,
and native subclasses outside the view family. Live activation includes all
known ancestors. Unknown or unsafe ancestry still refuses a structural view;
no unsupported superclass gains a fabricated view ABI.

Conditional or duplicate calls, an early return before the call, and access to
`this` before the call remain explicit emitter refusals. Unknown argument
signatures and missing required superclass arguments also refuse.

The statement guard scans the production default zabapgit closure, the
unpruned core and an optionally supplied full test closure. It rejects a
superclass constructor call inside a control block, a preceding `RETURN`, or
multiple calls. Guard regressions include wrapped calls, quoted keywords and
nested control blocks.

## Core measurement

The core scan uses the suite's binary64 and fail-closed lowering policy,
`ABAPITI_INLINE=classic`, and the unchanged `MeasureABAPTestGap` counting tool
from [osgo-suite-gap.md](osgo-suite-gap.md). Its pre-pass runtime census and
post-pass deduplicated emitter-refusal census both contain **zero sites** before
and after this fix. Constructor kernel errors were accepted emission, rather
than recorded emitter refusals; the separate source guard measures them.

| Core measurement | Before | After constructor fix |
| --- | ---: | ---: |
| Conditional live-view subclass constructors | 195 | 0 |
| Live-view classes, including roots | 223 | 223 |
| Attributable runtime-operation refusal sites | 0 | 0 |
| Attributable emitter-refusal sites | 0 | 0 |

The corrected core scan covers **2,283 emitted files, 1,370 constructors and
936 superclass calls**. The default scan covers **2,108 files, 1,257
constructors and 853 superclass calls**, with no constructor violations.
No runtime catalogue entry is enabled by this change. `string.parseFloat`
remains unsupported and has zero sites in the measured core.

## Verification

`TestLiveConstructorFixtures` compiles through gogen and executes on OSGO at
CI pin `f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6`. It checks native field values,
view reads, constructor and argument counters, preservation of parameter
assignments after the call, and skipped lazy initialization for views. Its
native/view values and counters match a Node oracle. Negative constructor
shapes retain explicit refusals. The dedicated fixture also passes the CI ABAP
lint tool, including global class section checks.

The CI `hir-unit.sh` lint suite passes with `HIR_RUNTIMES=" "` and the pinned
transpiler. `go vet -p 2 ./...`, the Grace lexer facts report and the default
runtime-operation coverage gate pass. The lexer golden is unchanged.
`lint.sh gate origin/main` reports **0 new issues**, with **5/5** canaries.
Default zabapgit Go output, including its executable, is byte-for-byte equal
to the baseline at the same output path: raw `diff -r` exits 0.

No A4H execution was available here. The kernel-specific constructor constraint
is checked by the source guard; compilation and execution use gogen/OSGO.

## Reproduction

Supply a clean OSG checkout at the CI pin and the full materialized closure
created by the existing suite closure tool. All output paths below are chosen
by the caller.

```sh
ABAPITI_OSG_HOME="$OSG" go test -p 2 ./hir/abap -run '^TestLiveConstructor' -count=1
HIR_RUNTIMES=" " ./.github/ci/hir-unit.sh "$WORK"
ABAPITI_INLINE=classic ABAPITI_KERNEL_CORE=1 \
  ABAPITI_KERNEL_GAP_OUT="$EVIDENCE" \
  go test -p 2 -timeout 60m ./cmd/abapiti \
  -run '^TestAbaplintCoreKernelConstructors$' -count=1
ABAPITI_INLINE=classic ABAPITI_KERNEL_CLOSURE="$CLOSURE" \
  ABAPITI_KERNEL_GAP_OUT="$EVIDENCE" \
  go test -p 2 -timeout 60m ./cmd/abapiti \
  -run '^TestAbaplintSuiteKernelConstructors$' -count=1
```
