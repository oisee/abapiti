CLASS zcl_abapiti_corpus DEFINITION PUBLIC FINAL CREATE PUBLIC.
PUBLIC SECTION.
  INTERFACES zif_abapiti_corpus.
  METHODS constructor IMPORTING iv_set TYPE clike.
  CLASS-METHODS get_text IMPORTING setname TYPE clike idx TYPE i OPTIONAL name TYPE string OPTIONAL RETURNING VALUE(result) TYPE string.
  CLASS-METHODS get_raw IMPORTING setname TYPE clike idx TYPE i OPTIONAL name TYPE string OPTIONAL RETURNING VALUE(result) TYPE xstring.
PROTECTED SECTION.
PRIVATE SECTION.
  CLASS-METHODS find_raw IMPORTING setname TYPE clike idx TYPE i name TYPE string RETURNING VALUE(result) TYPE xstring RAISING cx_sy_itab_line_not_found.
  TYPES BEGIN OF ty_row.
    TYPES idx TYPE i.
    TYPES name TYPE c LENGTH 60.
    TYPES v TYPE xstring.
  TYPES END OF ty_row.
  TYPES ty_rows TYPE STANDARD TABLE OF ty_row WITH DEFAULT KEY.
  DATA set_id TYPE c LENGTH 30.
  DATA rows TYPE ty_rows.
  METHODS load.
  CLASS-METHODS is_raw RETURNING VALUE(result) TYPE abap_bool.
  CLASS-METHODS quote IMPORTING value TYPE string RETURNING VALUE(result) TYPE string.
ENDCLASS.
CLASS zcl_abapiti_corpus IMPLEMENTATION.
  METHOD get_text.
    TRY.
      result = cl_abap_codepage=>convert_from( source = find_raw( setname = setname idx = idx name = name ) codepage = `UTF-8` ).
    CATCH cx_sy_codepage_converter_init cx_sy_conversion_codepage cx_sy_itab_line_not_found.
      CLEAR result.
    ENDTRY.
  ENDMETHOD.
  METHOD get_raw.
    TRY.
      result = find_raw( setname = setname idx = idx name = name ).
    CATCH cx_sy_itab_line_not_found.
      CLEAR result.
    ENDTRY.
  ENDMETHOD.
  METHOD find_raw.
    DATA table_name TYPE tabname.
    DATA condition TYPE string.
    IF idx > 0 AND name IS INITIAL.
      condition = |SETNAME = { quote( |{ setname }| ) } AND IDX = { idx }|.
    ELSEIF idx <= 0 AND name IS NOT INITIAL.
      condition = |SETNAME = { quote( |{ setname }| ) } AND NAME = { quote( name ) }|.
    ELSE.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
    table_name = 'ZABAPITI_CORPUS'.
    SELECT SINGLE v FROM (table_name) INTO result WHERE (condition).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
  ENDMETHOD.
  METHOD constructor.
    set_id = iv_set.
    load( ).
  ENDMETHOD.
  METHOD load.
    DATA table_name TYPE tabname.
    DATA condition TYPE string.
    table_name = 'ZABAPITI_CORPUS'.
    condition = |SETNAME = { quote( |{ set_id }| ) }|.
    SELECT idx name v FROM (table_name) INTO CORRESPONDING FIELDS OF TABLE rows WHERE (condition) ORDER BY PRIMARY KEY.
    IF sy-subrc <> 0.
      CLEAR rows.
    ENDIF.
  ENDMETHOD.
  METHOD zif_abapiti_corpus~count.
    result = lines( rows ).
  ENDMETHOD.
  METHOD zif_abapiti_corpus~get.
    READ TABLE rows INTO DATA(row) INDEX idx.
    IF sy-subrc = 0.
      result = cl_abap_codepage=>convert_from( source = row-v ).
    ENDIF.
  ENDMETHOD.
  METHOD zif_abapiti_corpus~name.
    READ TABLE rows INTO DATA(name_row) WITH KEY idx = idx.
    IF sy-subrc = 0.
      result = name_row-name.
    ELSE.
      CLEAR result.
    ENDIF.
  ENDMETHOD.
  METHOD is_raw.
    DATA(raw) = NEW cx_sy_itab_line_not_found( ).
    result = boolc( raw IS INSTANCE OF cx_sy_itab_line_not_found ).
  ENDMETHOD.
  METHOD quote.
    result = value.
    REPLACE ALL OCCURRENCES OF `'` IN result WITH `''`.
    result = |'{ result }'|.
  ENDMETHOD.
ENDCLASS.
