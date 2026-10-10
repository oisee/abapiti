# A4H micro-benchmark specifications

Loop count: **5,000,000** per timed arm. Run each report before/after separately on the same A4H kernel, warm both arms once, then alternate order for seven samples; record median elapsed_us * 1000 / 5000000 in ns/execution. Compare scalar and reference sinks between arms (ASSERT in each report); retain observable output. No numbers have been measured. The reports use representative local types and exercise the store-elimination mechanism; instantiate each candidate's actual type and receiver/argument position before deciding it is profitable. Same result type and conversion are mandatory. Shared fixtures are explicitly mapped below; no workload call counts or OSGO equality are inferred.

Local parser/type-check evidence: [a4h-syntax.json](a4h-syntax.json). Recheck with `npm ci --ignore-scripts --cache "$HOME/.cache/abapiti-npm-cache"` then `node tools/peephole-check-benches.cjs`. The repository lock selects @abaplint/core 2.118.0 and the check targets v758 (matching the existing A4H benchmark context). Kernel activation and measurements remain pending.

Syntax-check/import each report as a temporary local program; the name is zpeephole_bench in each independent file. Reports run the same loop twice, with the first loop warming that arm; only the second is timed. Pure DATA declarations are outside timed loops except inline declarations under test. The class, allocation, inputs and timer overhead are identical in both arms. Check bound and incompatible-reference exceptions separately outside timing for CAST/?= guards; handlers observing removed locals invalidate the rule.

## PM0001 — cast-assign

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
ref_sink = t1.
```

After:
```abap
ref_sink = CAST lcl_probe( source ).
```

Ready reports: [before](a4h/cast-assign-before.prog.abap), [after](a4h/cast-assign-after.prog.abap).

## PM0002, PM0011 — cast-field

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
sink = t1->payload.
```

After:
```abap
sink = CAST lcl_probe( source )->payload.
```

Ready reports: [before](a4h/cast-field-before.prog.abap), [after](a4h/cast-field-after.prog.abap).

## PM0003 — conv-bool

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CONV abap_bool( abap_true ).
IF t1 = abap_true.
  sink = sy-index.
ENDIF.
```

After:
```abap
IF CONV abap_bool( abap_true ) = abap_true.
  sink = sy-index.
ENDIF.
```

Ready reports: [before](a4h/conv-bool-before.prog.abap), [after](a4h/conv-bool-after.prog.abap).

## PM0005 — box-copy

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA t1 TYPE REF TO object.
t1 = source.
box->oval = t1.
```

After:
```abap
box->oval = source.
```

Ready reports: [before](a4h/box-copy-before.prog.abap), [after](a4h/box-copy-after.prog.abap).

## PM0007 — cast

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
sink = lcl_probe=>consume( t1 ).
```

After:
```abap
sink = lcl_probe=>consume( CAST lcl_probe( source ) ).
```

Ready reports: [before](a4h/cast-before.prog.abap), [after](a4h/cast-after.prog.abap).

## PM0009 — cast-initial

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
IF t1 IS INITIAL.
  sink = 0.
ELSE.
  sink = sy-index.
ENDIF.
```

After:
```abap
IF CAST lcl_probe( source ) IS INITIAL.
  sink = 0.
ELSE.
  sink = sy-index.
ENDIF.
```

Ready reports: [before](a4h/cast-initial-before.prog.abap), [after](a4h/cast-initial-after.prog.abap).

## PM0010 — cast-call

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
sink = t1->read( ).
```

After:
```abap
sink = CAST lcl_probe( source )->read( ).
```

Ready reports: [before](a4h/cast-call-before.prog.abap), [after](a4h/cast-call-after.prog.abap).

## PM0012 — cast-copy

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA t1 TYPE REF TO object.
t1 = source.
DATA t2 TYPE REF TO lcl_probe.
t2 ?= t1.
```

After:
```abap
DATA t2 TYPE REF TO lcl_probe.
t2 ?= source.
```

Ready reports: [before](a4h/cast-copy-before.prog.abap), [after](a4h/cast-copy-after.prog.abap).

## PM0013 — cast-bound

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CAST lcl_probe( source ).
bool_sink = xsdbool( t1 IS BOUND ).
```

After:
```abap
bool_sink = xsdbool( CAST lcl_probe( source ) IS BOUND ).
```

Ready reports: [before](a4h/cast-bound-before.prog.abap), [after](a4h/cast-bound-after.prog.abap).

## PM0015 — bool

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = xsdbool( source IS BOUND ).
IF t1 = abap_true.
  sink = sy-index.
ENDIF.
```

After:
```abap
IF source IS BOUND.
  sink = sy-index.
ENDIF.
```

Ready reports: [before](a4h/bool-before.prog.abap), [after](a4h/bool-after.prog.abap).

## PM0016 — downcast

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA t1 TYPE REF TO lcl_probe.
t1 ?= source.
ref_sink = t1.
```

After:
```abap
ref_sink = CAST lcl_probe( source ).
```

Ready reports: [before](a4h/downcast-before.prog.abap), [after](a4h/downcast-after.prog.abap).

## PM0017 — conv-compare

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA(t1) = CONV string( |abcdef| ).
bool_sink = xsdbool( t1 = input ).
```

After:
```abap
bool_sink = xsdbool( CONV string( |abcdef| ) = input ).
```

Ready reports: [before](a4h/conv-compare-before.prog.abap), [after](a4h/conv-compare-after.prog.abap).

## PM0018 — cast-copy-decl-ref

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA t1 TYPE REF TO object.
t1 = source.
DATA unused TYPE REF TO lcl_probe.
DATA(t3) = CAST lcl_probe( t1 ).
```

After:
```abap
DATA unused TYPE REF TO lcl_probe.
DATA(t3) = CAST lcl_probe( source ).
```

Ready reports: [before](a4h/cast-copy-decl-ref-before.prog.abap), [after](a4h/cast-copy-decl-ref-after.prog.abap).

## PM0020 — cast-copy-decl-string

Loop count: 5,000,000. Savings if guard holds: one executed store per iteration.

Before:
```abap
DATA t1 TYPE REF TO object.
t1 = source.
DATA unused TYPE string.
DATA(t3) = CAST lcl_probe( t1 ).
```

After:
```abap
DATA unused TYPE string.
DATA(t3) = CAST lcl_probe( source ).
```

Ready reports: [before](a4h/cast-copy-decl-string-before.prog.abap), [after](a4h/cast-copy-decl-string-after.prog.abap).
