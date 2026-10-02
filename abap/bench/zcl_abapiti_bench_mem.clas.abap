* Memory-model benchmark for ABAPiti's WASM linear memory (little-endian).
* Three models behind one dispatch, so the dispatch cost is equal:
*   A  xstring; writes through REPLACE SECTION ... IN BYTE MODE (today's
*      generated code), reads by offset
*   B  STANDARD TABLE OF x LENGTH 4096 (16 rows per WASM page); writes in
*      place through a field symbol: <row>+off(n) = ...
*   C  as B, but i32 goes through ASSIGN ... CASTING TYPE i, which uses the
*      platform's byte order (little-endian on x86, as in WASM)
* Workloads: W1 byte fill + byte sum, W2 aligned i32 store + load sum,
* W3 unaligned i32 across row boundaries, W4 recursion (fib).
* Correctness: the ABAP Unit test class. Timings: run as a console app
* (if_oo_adt_classrun) or call run( ) and read the result table.
* ABAP 7.02 syntax, ASCII.
CLASS zcl_abapiti_bench_mem DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    INTERFACES if_oo_adt_classrun.

    TYPES: BEGIN OF ty_result,
             model    TYPE c LENGTH 1,
             workload TYPE c LENGTH 2,
             checksum TYPE p LENGTH 16 DECIMALS 0,
             micros   TYPE i,
             error    TYPE string,
           END OF ty_result,
           ty_results TYPE STANDARD TABLE OF ty_result WITH DEFAULT KEY.

    CONSTANTS: gc_page TYPE i VALUE 65536,
               gc_row  TYPE i VALUE 4096.

    METHODS constructor IMPORTING iv_model TYPE c DEFAULT 'B' iv_pages TYPE i DEFAULT 1.
    METHODS st_i32 IMPORTING iv_addr TYPE i iv_val TYPE i.
    METHODS ld_i32 IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.
    METHODS st_8 IMPORTING iv_addr TYPE i iv_val TYPE i.
    METHODS ld_8u IMPORTING iv_addr TYPE i RETURNING VALUE(rv) TYPE i.
    METHODS fib IMPORTING iv_n TYPE i RETURNING VALUE(rv) TYPE i.

    METHODS w1_bytes IMPORTING iv_n TYPE i RETURNING VALUE(rv) TYPE ty_result-checksum.
    METHODS w2_i32 IMPORTING iv_n TYPE i RETURNING VALUE(rv) TYPE ty_result-checksum.
    METHODS w3_unaligned IMPORTING iv_n TYPE i RETURNING VALUE(rv) TYPE ty_result-checksum.

    CLASS-METHODS run IMPORTING iv_models TYPE string DEFAULT 'ABC'
      RETURNING VALUE(rt) TYPE ty_results.

  PRIVATE SECTION.
    TYPES ty_row TYPE x LENGTH 4096.
    TYPES ty_x4 TYPE x LENGTH 4.
    DATA mv_model TYPE c LENGTH 1.
    DATA mv_mem TYPE xstring.
    DATA mt_mem TYPE STANDARD TABLE OF ty_row WITH DEFAULT KEY.

    METHODS swap4 IMPORTING iv_in TYPE ty_x4 RETURNING VALUE(rv) TYPE ty_x4.
    CLASS-METHODS measure IMPORTING iv_model TYPE c iv_workload TYPE c
      RETURNING VALUE(rs) TYPE ty_result.
ENDCLASS.

CLASS zcl_abapiti_bench_mem IMPLEMENTATION.

  METHOD constructor.
    DATA lv_chunk TYPE x LENGTH 256.
    DATA lv_rows TYPE i.
    mv_model = iv_model.
    IF mv_model = 'A'.
      DO iv_pages * 256 TIMES.
        CONCATENATE mv_mem lv_chunk INTO mv_mem IN BYTE MODE.
      ENDDO.
    ELSE.
      lv_rows = iv_pages * 16.
      DO lv_rows TIMES.
        APPEND INITIAL LINE TO mt_mem.
      ENDDO.
    ENDIF.
  ENDMETHOD.

  METHOD swap4.
    rv+0(1) = iv_in+3(1).
    rv+1(1) = iv_in+2(1).
    rv+2(1) = iv_in+1(1).
    rv+3(1) = iv_in+0(1).
  ENDMETHOD.

  METHOD st_i32.
    DATA lv_b TYPE x LENGTH 1.
    DATA lv_v TYPE i.
    DATA lv_be TYPE x LENGTH 4.
    DATA lv_le TYPE x LENGTH 4.
    DATA lv_row TYPE i.
    DATA lv_off TYPE i.
    FIELD-SYMBOLS <ls_row> TYPE ty_row.
    FIELD-SYMBOLS <lv_i> TYPE i.
    CASE mv_model.
      WHEN 'A'.
        lv_be = iv_val.
        lv_le = swap4( lv_be ).
        REPLACE SECTION OFFSET iv_addr LENGTH 4 OF mv_mem WITH lv_le IN BYTE MODE.
      WHEN OTHERS.
        lv_row = iv_addr DIV gc_row + 1.
        lv_off = iv_addr MOD gc_row.
        IF lv_off > gc_row - 4.
* Crosses a row boundary: byte by byte.
          lv_be = iv_val.
          lv_le = swap4( lv_be ).
          DO 4 TIMES.
            lv_off = sy-index - 1.
            lv_b = lv_le+lv_off(1).
            lv_v = lv_b.
            st_8( iv_addr = iv_addr + lv_off iv_val = lv_v ).
          ENDDO.
          RETURN.
        ENDIF.
        READ TABLE mt_mem INDEX lv_row ASSIGNING <ls_row>.
        IF mv_model = 'C'.
          ASSIGN <ls_row>+lv_off(4) TO <lv_i> CASTING.
          <lv_i> = iv_val.
        ELSE.
          lv_be = iv_val.
          <ls_row>+lv_off(4) = swap4( lv_be ).
        ENDIF.
    ENDCASE.
  ENDMETHOD.

  METHOD ld_i32.
    DATA lv_le TYPE x LENGTH 4.
    DATA lv_be TYPE x LENGTH 4.
    DATA lv_row TYPE i.
    DATA lv_off TYPE i.
    FIELD-SYMBOLS <ls_row> TYPE ty_row.
    FIELD-SYMBOLS <lv_i> TYPE i.
    CASE mv_model.
      WHEN 'A'.
        lv_le = mv_mem+iv_addr(4).
      WHEN OTHERS.
        lv_row = iv_addr DIV gc_row + 1.
        lv_off = iv_addr MOD gc_row.
        IF lv_off > gc_row - 4.
          lv_le+0(1) = ld_8u( iv_addr ).
          lv_le+1(1) = ld_8u( iv_addr + 1 ).
          lv_le+2(1) = ld_8u( iv_addr + 2 ).
          lv_le+3(1) = ld_8u( iv_addr + 3 ).
        ELSE.
          READ TABLE mt_mem INDEX lv_row ASSIGNING <ls_row>.
          IF mv_model = 'C'.
            ASSIGN <ls_row>+lv_off(4) TO <lv_i> CASTING.
            rv = <lv_i>.
            RETURN.
          ENDIF.
          lv_le = <ls_row>+lv_off(4).
        ENDIF.
    ENDCASE.
    lv_be = swap4( lv_le ).
    rv = lv_be.
  ENDMETHOD.

  METHOD st_8.
    DATA lv_b TYPE x LENGTH 1.
    DATA lv_row TYPE i.
    DATA lv_off TYPE i.
    FIELD-SYMBOLS <ls_row> TYPE ty_row.
    lv_b = iv_val.
    IF mv_model = 'A'.
      REPLACE SECTION OFFSET iv_addr LENGTH 1 OF mv_mem WITH lv_b IN BYTE MODE.
    ELSE.
      lv_row = iv_addr DIV gc_row + 1.
      lv_off = iv_addr MOD gc_row.
      READ TABLE mt_mem INDEX lv_row ASSIGNING <ls_row>.
      <ls_row>+lv_off(1) = lv_b.
    ENDIF.
  ENDMETHOD.

  METHOD ld_8u.
    DATA lv_b TYPE x LENGTH 1.
    DATA lv_row TYPE i.
    DATA lv_off TYPE i.
    FIELD-SYMBOLS <ls_row> TYPE ty_row.
    IF mv_model = 'A'.
      lv_b = mv_mem+iv_addr(1).
    ELSE.
      lv_row = iv_addr DIV gc_row + 1.
      lv_off = iv_addr MOD gc_row.
      READ TABLE mt_mem INDEX lv_row ASSIGNING <ls_row>.
      lv_b = <ls_row>+lv_off(1).
    ENDIF.
    rv = lv_b.
  ENDMETHOD.

  METHOD fib.
    IF iv_n < 2.
      rv = iv_n.
    ELSE.
      rv = fib( iv_n - 1 ) + fib( iv_n - 2 ).
    ENDIF.
  ENDMETHOD.

  METHOD w1_bytes.
* store8 (k MOD 251) at k, then sum load8_u.
    DATA lv_k TYPE i.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      st_8( iv_addr = lv_k iv_val = lv_k MOD 251 ).
    ENDDO.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      rv = rv + ld_8u( lv_k ).
    ENDDO.
  ENDMETHOD.

  METHOD w2_i32.
* i32.store (7k+1) at 4k, then sum i32.load.
    DATA lv_k TYPE i.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      st_i32( iv_addr = lv_k * 4 iv_val = lv_k * 7 + 1 ).
    ENDDO.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      rv = rv + ld_i32( lv_k * 4 ).
    ENDDO.
  ENDMETHOD.

  METHOD w3_unaligned.
* i32.store (k+1000) at 4k+1, crossing a row boundary every 1024 stores
* in models B and C; then sum i32.load at the same addresses.
    DATA lv_k TYPE i.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      st_i32( iv_addr = lv_k * 4 + 1 iv_val = lv_k + 1000 ).
    ENDDO.
    DO iv_n TIMES.
      lv_k = sy-index - 1.
      rv = rv + ld_i32( lv_k * 4 + 1 ).
    ENDDO.
  ENDMETHOD.

  METHOD measure.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    DATA lv_t0 TYPE i.
    DATA lv_t1 TYPE i.
    DATA lx TYPE REF TO cx_root.
    rs-model = iv_model.
    rs-workload = iv_workload.
    TRY.
        CREATE OBJECT lo EXPORTING iv_model = iv_model.
        GET RUN TIME FIELD lv_t0.
        CASE iv_workload.
          WHEN 'W1'. rs-checksum = lo->w1_bytes( 16384 ).
          WHEN 'W2'. rs-checksum = lo->w2_i32( 4096 ).
          WHEN 'W3'. rs-checksum = lo->w3_unaligned( 2048 ).
          WHEN 'W4'. rs-checksum = lo->fib( 20 ).
        ENDCASE.
        GET RUN TIME FIELD lv_t1.
        rs-micros = lv_t1 - lv_t0.
      CATCH cx_root INTO lx.
        rs-error = lx->get_text( ).
    ENDTRY.
  ENDMETHOD.

  METHOD run.
    DATA lv_i TYPE i.
    DATA lv_model TYPE c LENGTH 1.
    DATA ls TYPE ty_result.
    DO strlen( iv_models ) TIMES.
      lv_i = sy-index - 1.
      lv_model = iv_models+lv_i(1).
      ls = measure( iv_model = lv_model iv_workload = 'W1' ). APPEND ls TO rt.
      ls = measure( iv_model = lv_model iv_workload = 'W2' ). APPEND ls TO rt.
      ls = measure( iv_model = lv_model iv_workload = 'W3' ). APPEND ls TO rt.
    ENDDO.
    ls = measure( iv_model = 'B' iv_workload = 'W4' ). APPEND ls TO rt.
  ENDMETHOD.

  METHOD if_oo_adt_classrun~main.
    DATA lt TYPE ty_results.
    DATA ls TYPE ty_result.
    DATA lv_line TYPE string.
    DATA lv_sum TYPE string.
    DATA lv_us TYPE string.
    lt = run( ).
    out->write( 'model workload micros checksum error' ).
    LOOP AT lt INTO ls.
      lv_sum = ls-checksum.
      lv_us = ls-micros.
      CONCATENATE ls-model ls-workload lv_us lv_sum ls-error INTO lv_line SEPARATED BY space.
      out->write( lv_line ).
    ENDLOOP.
  ENDMETHOD.

ENDCLASS.
