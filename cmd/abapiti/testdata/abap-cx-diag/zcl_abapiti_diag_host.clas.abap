CLASS zcl_abapiti_diag_host DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    CLASS-METHODS runtime_failure IMPORTING zero TYPE f.
    CLASS-METHODS chain_failure IMPORTING zero TYPE f.
    CLASS-METHODS raw_failure IMPORTING zero TYPE f.
ENDCLASS.
CLASS zcl_abapiti_diag_host IMPLEMENTATION.
  METHOD runtime_failure.
    DATA result TYPE i.
    result = 1 / zero.
  ENDMETHOD.
  METHOD chain_failure.
    TRY.
      runtime_failure( zero ).
    CATCH cx_root INTO DATA(original).
      DATA(inner) = NEW zcx_abapiti_diag( previous = original reason = 'inner host text' ).
      RAISE EXCEPTION TYPE zcx_abapiti_diag EXPORTING previous = inner reason = 'outer host text'.
    ENDTRY.
  ENDMETHOD.
  METHOD raw_failure.
    TRY.
      chain_failure( zero ).
    CATCH cx_root INTO DATA(original).
      " CX_NO_CHECK deliberately bypasses the extern Error conversion.
      RAISE EXCEPTION TYPE cx_no_check EXPORTING previous = original.
    ENDTRY.
  ENDMETHOD.
ENDCLASS.
