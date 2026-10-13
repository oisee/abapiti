# OSG inbox: an exception caught INTO cannot be passed on, widened, or followed via ->previous

Found by: abapiti (ZCL_ABAPITI_TEST_RUNNER), 2026-10-13, open-steamgate 368baab28.

## What gogen does

A local that receives runtime exceptions (`CATCH cx_root INTO x`) becomes the
kind `exc` in tools/gogen/frontend.mjs (CATCH handling, `ctx.locals.set(nm, EXC)`).
On such a value only two forms are accepted (`excHolder`/`call`):

- `x->get_text( )`
- `cl_abap_classdescr=>get_class_name( x )` (and `cl_abap_typedescr=>`)

Everything else is refused:

- passing `x` to a parameter `TYPE REF TO cx_root` or assigning it to
  `REF TO object` / `REF TO cx_root`: `conversion exc -> ref` (frontend.mjs `convert`)
- `x->previous`: refused (attribute of an exception value)
- any other method: `exception method ...`

## Minimal repro

```abap
CLASS zcl_osg_exc_prev DEFINITION PUBLIC CREATE PUBLIC.
  PUBLIC SECTION.
    CLASS-METHODS run RETURNING VALUE(result) TYPE string.
    CLASS-METHODS describe IMPORTING error TYPE REF TO cx_root
                           RETURNING VALUE(result) TYPE string.
ENDCLASS.
CLASS zcl_osg_exc_prev IMPLEMENTATION.
  METHOD describe.
    DATA current TYPE REF TO cx_root.
    DATA name TYPE string.
    current = error.
    WHILE current IS BOUND.
      name = cl_abap_classdescr=>get_class_name( current ).
      REPLACE FIRST OCCURRENCE OF '\CLASS=' IN name WITH ''.
      IF result IS NOT INITIAL.
        result = result && | <- |.
      ENDIF.
      result = result && name.
      current = current->previous.
    ENDWHILE.
  ENDMETHOD.
  METHOD run.
    DATA error TYPE REF TO cx_root.
    DATA zero TYPE i.
    TRY.
        TRY.
            zero = 1 / zero.
          CATCH cx_sy_zerodivide INTO DATA(inner).
            RAISE EXCEPTION TYPE cx_sy_conversion_no_number EXPORTING previous = inner.
        ENDTRY.
      CATCH cx_root INTO error.
        result = describe( error ).
    ENDTRY.
  ENDMETHOD.
ENDCLASS.
```

## Expected (kernel)

`run( )` returns `CX_SY_CONVERSION_NO_NUMBER <- CX_SY_ZERODIVIDE`.

## Actual (gogen 368baab28)

`NOT_COMPILED in ZCL_OSG_EXC_PREV=>RUN: conversion exc -> ref` at
`result = describe( error )`; with `describe` inlined into RUN,
`current = error` and `error->previous` are refused instead.

## Ask

Either let an `exc` value widen to `REF TO cx_root` / `REF TO object`
(an object with class name, text and PREVIOUS), or support `x->previous`
on an `exc` value (an `exc` again, `IS BOUND` false at the end of the chain).

## abapiti fallback until then

The runner reports one level, in RUN itself:
`<CLASS>: <get_text( )>` via `get_class_name( x )` and `x->get_text( )`,
without the ->previous chain.
