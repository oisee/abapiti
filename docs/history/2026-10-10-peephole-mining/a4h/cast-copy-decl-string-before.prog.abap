REPORT zpeephole_bench.
CLASS lcl_probe DEFINITION FINAL.
  PUBLIC SECTION.
    DATA payload TYPE i VALUE 7.
    CLASS-METHODS consume IMPORTING obj TYPE REF TO lcl_probe RETURNING VALUE(result) TYPE i.
    METHODS read RETURNING VALUE(result) TYPE i.
ENDCLASS.
CLASS lcl_box DEFINITION FINAL.
  PUBLIC SECTION.
    DATA oval TYPE REF TO object.
ENDCLASS.
CLASS lcl_box IMPLEMENTATION.
ENDCLASS.
CLASS lcl_probe IMPLEMENTATION.
  METHOD consume.
    result = obj->payload.
  ENDMETHOD.
  METHOD read.
    result = payload.
  ENDMETHOD.
ENDCLASS.
TYPES: BEGIN OF ty_row,
         low TYPE i,
         sign TYPE c LENGTH 1,
         option TYPE c LENGTH 2,
       END OF ty_row.
DATA source TYPE REF TO object.
DATA box TYPE REF TO lcl_box.
DATA input TYPE c LENGTH 12 VALUE 'abcdef'.
DATA sink TYPE i.
DATA ref_sink TYPE REF TO lcl_probe.
DATA bool_sink TYPE abap_bool.
DATA text_sink TYPE string.
DATA row_sink TYPE ty_row.
DATA begin_us TYPE i.
DATA end_us TYPE i.
DATA elapsed_us TYPE int8.
source = NEW lcl_probe( ).
box = NEW lcl_box( ).
DO 2 TIMES.
  GET RUN TIME FIELD begin_us.
  DO 5000000 TIMES.
  DATA t1 TYPE REF TO object.
  t1 = source.
  DATA unused TYPE string.
  DATA(t3) = CAST lcl_probe( t1 ).
  ENDDO.
  GET RUN TIME FIELD end_us.
  elapsed_us = CONV int8( end_us ) - CONV int8( begin_us ).
ENDDO.
ASSERT t3 = source.
WRITE: / elapsed_us, / sink, / text_sink, / row_sink-low.
