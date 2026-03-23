*&---------------------------------------------------------------------*
*& Report ZWASM_COMPILER
*& Interactive WASM-to-ABAP compiler
*& Load .wasm binary → parse → compile → execute
*&---------------------------------------------------------------------*
REPORT zwasm_compiler.

*----------------------------------------------------------------------*
* Selection Screen
*----------------------------------------------------------------------*
SELECTION-SCREEN BEGIN OF BLOCK b1 WITH FRAME TITLE sc_src.
  PARAMETERS: p_file  RADIOBUTTON GROUP src DEFAULT 'X',
              p_smw0  RADIOBUTTON GROUP src,
              p_hex   RADIOBUTTON GROUP src.
  PARAMETERS: p_fname TYPE string LOWER CASE VISIBLE LENGTH 60.
  PARAMETERS: p_smw0n TYPE c LENGTH 40 LOWER CASE.
  PARAMETERS: p_hexin TYPE string LOWER CASE VISIBLE LENGTH 60.
SELECTION-SCREEN END OF BLOCK b1.

SELECTION-SCREEN BEGIN OF BLOCK b2 WITH FRAME TITLE sc_exe.
  PARAMETERS: p_exec AS CHECKBOX DEFAULT 'X'.
  PARAMETERS: p_func TYPE c LENGTH 30 LOWER CASE DEFAULT 'add'.
  PARAMETERS: p_arg0 TYPE i DEFAULT 2.
  PARAMETERS: p_arg1 TYPE i DEFAULT 3.
SELECTION-SCREEN END OF BLOCK b2.

SELECTION-SCREEN BEGIN OF BLOCK b3 WITH FRAME TITLE sc_out.
  PARAMETERS: p_show AS CHECKBOX DEFAULT 'X'.
SELECTION-SCREEN END OF BLOCK b3.

*----------------------------------------------------------------------*
* Texts
*----------------------------------------------------------------------*
INITIALIZATION.
  sc_src = 'WASM Source'.
  sc_exe = 'Execution'.
  sc_out = 'Output'.

*----------------------------------------------------------------------*
* File dialog for p_fname
*----------------------------------------------------------------------*
AT SELECTION-SCREEN ON VALUE-REQUEST FOR p_fname.
  DATA: lt_filetable TYPE filetable,
        lv_rc        TYPE i.

  cl_gui_frontend_services=>file_open_dialog(
    EXPORTING
      window_title  = 'Select WASM binary'
      file_filter   = 'WASM files (*.wasm)|*.wasm|All files (*.*)|*.*'
    CHANGING
      file_table    = lt_filetable
      rc            = lv_rc
    EXCEPTIONS OTHERS = 1 ).

  IF lv_rc > 0.
    READ TABLE lt_filetable INDEX 1 INTO p_fname.
  ENDIF.

*----------------------------------------------------------------------*
* Main Processing
*----------------------------------------------------------------------*
START-OF-SELECTION.

  DATA: lv_wasm TYPE xstring.

  " ── Step 1: Load WASM binary ──
  IF p_file = abap_true.
    " Upload from local file
    IF p_fname IS INITIAL.
      WRITE: / 'ERROR: No file selected.'.
      RETURN.
    ENDIF.
    DATA: lt_data TYPE STANDARD TABLE OF x255,
          lv_flen TYPE i.
    cl_gui_frontend_services=>gui_upload(
      EXPORTING filename   = p_fname
                filetype   = 'BIN'
      IMPORTING filelength = lv_flen
      CHANGING  data_tab   = lt_data
      EXCEPTIONS OTHERS    = 1 ).
    IF sy-subrc <> 0.
      WRITE: / 'ERROR: Upload failed:', sy-subrc.
      RETURN.
    ENDIF.
    CALL FUNCTION 'SCMS_BINARY_TO_XSTRING'
      EXPORTING input_length = lv_flen
      IMPORTING buffer       = lv_wasm
      TABLES    binary_tab   = lt_data.

  ELSEIF p_smw0 = abap_true.
    " Load from SMW0
    IF p_smw0n IS INITIAL.
      WRITE: / 'ERROR: No SMW0 object name.'.
      RETURN.
    ENDIF.
    DATA: lt_mime   TYPE STANDARD TABLE OF w3mime,
          lv_size  TYPE i.
    CALL FUNCTION 'SAP_READ_BINARY_RESOURCE'
      EXPORTING key  = p_smw0n
      IMPORTING size = lv_size
      TABLES    data = lt_mime
      EXCEPTIONS OTHERS = 1.
    IF sy-subrc <> 0.
      WRITE: / 'ERROR: SMW0 read failed. Object:', p_smw0n.
      RETURN.
    ENDIF.
    CALL FUNCTION 'SCMS_BINARY_TO_XSTRING'
      EXPORTING input_length = lv_size
      IMPORTING buffer       = lv_wasm
      TABLES    binary_tab   = lt_mime.

  ELSEIF p_hex = abap_true.
    " Hex string input
    IF p_hexin IS INITIAL.
      WRITE: / 'ERROR: No hex string provided.'.
      RETURN.
    ENDIF.
    lv_wasm = p_hexin.
  ENDIF.

  IF xstrlen( lv_wasm ) < 8.
    WRITE: / 'ERROR: WASM binary too small:', xstrlen( lv_wasm ), 'bytes'.
    RETURN.
  ENDIF.

  WRITE: / '── WASM Binary ──'.
  WRITE: / 'Size:', xstrlen( lv_wasm ), 'bytes'.
  ULINE.

  " ── Step 2: Parse ──
  DATA(lo_mod) = NEW zcl_wasm_module( ).
  TRY.
      lo_mod->parse( lv_wasm ).
    CATCH cx_root INTO DATA(lx_parse).
      WRITE: / 'PARSE ERROR:', lx_parse->get_text( ).
      RETURN.
  ENDTRY.

  DATA(lv_summary) = lo_mod->summary( ).
  WRITE: / '── Module Summary ──'.
  WRITE: / lv_summary.
  ULINE.

  " Show exports
  WRITE: / '── Exports ──'.
  LOOP AT lo_mod->mt_exports INTO DATA(ls_exp).
    DATA(lv_ekind) = ||.
    CASE ls_exp-kind.
      WHEN 0. lv_ekind = 'func'.
      WHEN 1. lv_ekind = 'table'.
      WHEN 2. lv_ekind = 'memory'.
      WHEN 3. lv_ekind = 'global'.
    ENDCASE.
    WRITE: / '  ', lv_ekind, ls_exp-name, 'index:', ls_exp-index.
  ENDLOOP.
  ULINE.

  " ── Step 3: Compile ──
  DATA(lv_abap) = NEW zcl_wasm_codegen( )->compile(
    io_module = lo_mod
    iv_name   = 'ZWASM_GEN' ).

  DATA: lt_src_lines TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
  SPLIT lv_abap AT cl_abap_char_utilities=>newline INTO TABLE lt_src_lines.

  WRITE: / '── Compiled ABAP ──'.
  WRITE: / 'Lines:', lines( lt_src_lines ).

  " Show generated source
  IF p_show = abap_true.
    ULINE.
    WRITE: / '── Generated Source ──'.
    DATA(lv_lineno) = 0.
    LOOP AT lt_src_lines INTO DATA(lv_line).
      lv_lineno = lv_lineno + 1.
      WRITE: / lv_lineno, lv_line.
    ENDLOOP.
    ULINE.
  ENDIF.

  " ── Step 4: Execute ──
  IF p_exec = abap_true.
    WRITE: / '── Execution ──'.

    " Generate subroutine pool
    DATA lv_prog TYPE string.
    DATA lv_msg TYPE string.
    DATA lt_abap_code TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
    SPLIT lv_abap AT cl_abap_char_utilities=>newline INTO TABLE lt_abap_code.
    GENERATE SUBROUTINE POOL lt_abap_code NAME lv_prog MESSAGE lv_msg.
    IF sy-subrc <> 0.
      WRITE: / 'GENERATE failed:', sy-subrc.
      WRITE: / 'Message:', lv_msg.
      RETURN.
    ENDIF.
    WRITE: / 'Generated program:', lv_prog.

    " Call the function
    DATA: lv_result TYPE i.
    WRITE: / 'Calling:', p_func, 'with args:', p_arg0, p_arg1.

    TRY.
        PERFORM (p_func) IN PROGRAM (lv_prog)
          USING p_arg0 p_arg1
          CHANGING lv_result.
      CATCH cx_root INTO DATA(lx_exec).
        WRITE: / 'EXECUTION ERROR:', lx_exec->get_text( ).
        RETURN.
    ENDTRY.

    WRITE: / '────────────────────'.
    WRITE: / 'RESULT:', lv_result.
    WRITE: / '────────────────────'.
  ENDIF.
