CLASS zcl_abapiti_log DEFINITION PUBLIC FINAL CREATE PRIVATE.
  PUBLIC SECTION.
    DATA runid TYPE char32 READ-ONLY.
    CLASS-METHODS start IMPORTING subobject TYPE csequence extnumber TYPE csequence RETURNING VALUE(result) TYPE REF TO zcl_abapiti_log.
    METHODS info IMPORTING text TYPE csequence.
    METHODS error IMPORTING text TYPE csequence.
    METHODS stage IMPORTING name TYPE csequence microseconds TYPE int8.
  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS connection TYPE dbcon_name VALUE 'R/3*ZABAPITI'.
    DATA handle TYPE balloghndl.
    DATA sub TYPE balsubobj.
    DATA seq TYPE i.
    METHODS add IMPORTING type TYPE symsgty text TYPE csequence.
ENDCLASS.
CLASS zcl_abapiti_log IMPLEMENTATION.
  METHOD start.
    DATA header TYPE bal_s_log.
    CREATE OBJECT result.
    result->sub = subobject.
    TRY.
        result->runid = cl_system_uuid=>create_uuid_c32_static( ).
      CATCH cx_uuid_error.
        result->runid = |{ sy-datum }{ sy-uzeit }|.
    ENDTRY.
    header-object = 'ZABAPITI'.
    header-subobject = subobject.
    header-extnumber = extnumber.
    CALL FUNCTION 'BAL_LOG_CREATE'
      EXPORTING
        i_s_log      = header
      IMPORTING
        e_log_handle = result->handle
      EXCEPTIONS
        OTHERS       = 1.
    IF sy-subrc <> 0.
      CLEAR result->handle.
    ENDIF.
    result->info( |run { result->runid } started: { extnumber }| ).
  ENDMETHOD.
  METHOD info.
    add( type = 'I' text = text ).
  ENDMETHOD.
  METHOD error.
    add( type = 'E' text = text ).
  ENDMETHOD.
  METHOD stage.
    add( type = 'I' text = |{ name }: { microseconds } us| ).
  ENDMETHOD.
  METHOD add.
    DATA handles TYPE bal_t_logh.
    DATA free TYPE c LENGTH 200.
    DATA row TYPE zabapiti_runlog.
    IF handle IS NOT INITIAL.
      free = text.
      CALL FUNCTION 'BAL_LOG_MSG_ADD_FREE_TEXT'
        EXPORTING
          i_log_handle = handle
          i_msgty      = type
          i_text       = free
        EXCEPTIONS
          OTHERS       = 1.
      INSERT handle INTO TABLE handles.
      CALL FUNCTION 'BAL_DB_SAVE'
        EXPORTING
          i_t_log_handle       = handles
          i_2th_connection     = abap_true
          i_2th_connect_commit = abap_true
        EXCEPTIONS
          OTHERS               = 1.
    ENDIF.
    seq = seq + 1.
    row-runid = runid.
    row-seqno = seq.
    GET TIME STAMP FIELD row-tstamp.
    row-subobj = sub.
    row-msgty = type.
    row-msgtext = text.
    TRY.
        INSERT zabapiti_runlog CONNECTION (connection) FROM row.
        COMMIT CONNECTION (connection).
      CATCH cx_sy_open_sql_db.
        RETURN.
    ENDTRY.
  ENDMETHOD.
ENDCLASS.
