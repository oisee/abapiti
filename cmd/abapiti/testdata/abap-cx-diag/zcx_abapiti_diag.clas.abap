CLASS zcx_abapiti_diag DEFINITION PUBLIC INHERITING FROM cx_dynamic_check CREATE PUBLIC.
  PUBLIC SECTION.
    DATA reason TYPE string.
    METHODS constructor IMPORTING previous TYPE REF TO cx_root OPTIONAL reason TYPE string OPTIONAL.
    METHODS get_text REDEFINITION.
    METHODS get_source_position REDEFINITION.
ENDCLASS.
CLASS zcx_abapiti_diag IMPLEMENTATION.
  METHOD constructor.
    super->constructor( previous = previous ).
    me->reason = reason.
  ENDMETHOD.
  METHOD get_text.
    result = reason.
  ENDMETHOD.
  METHOD get_source_position.
    program_name = 'ZDIAG_PROGRAM'.
    include_name = 'ZDIAG_INCLUDE'.
    source_line = 42.
  ENDMETHOD.
ENDCLASS.
