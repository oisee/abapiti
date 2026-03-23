CLASS zcl_wasm_codegen DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    METHODS compile
      IMPORTING io_module  TYPE REF TO zcl_wasm_module
                iv_name    TYPE string DEFAULT 'ZWASM_OUT'
      RETURNING VALUE(rv)  TYPE string.
  PRIVATE SECTION.
    CONSTANTS: c_block TYPE i VALUE 1,
               c_loop  TYPE i VALUE 2,
               c_if    TYPE i VALUE 3.
    DATA mo_mod TYPE REF TO zcl_wasm_module.
    DATA mv_out TYPE string.
    DATA mv_indent TYPE i.
    DATA mv_stack_depth TYPE i.
    DATA mv_num_params TYPE i.
    DATA mt_block_kinds TYPE STANDARD TABLE OF i WITH DEFAULT KEY.
    METHODS line IMPORTING iv TYPE string.
    METHODS push RETURNING VALUE(rv) TYPE string.
    METHODS pop RETURNING VALUE(rv) TYPE string.
    METHODS peek RETURNING VALUE(rv) TYPE string.
    METHODS emit_function IMPORTING is_func TYPE zcl_wasm_module=>ty_function.
    METHODS emit_instructions IMPORTING it_code TYPE zcl_wasm_module=>ty_instructions.
    METHODS emit_call IMPORTING iv_func_idx TYPE i.
    METHODS func_name IMPORTING iv_idx TYPE i RETURNING VALUE(rv) TYPE string.
    METHODS local_name IMPORTING iv_idx TYPE i RETURNING VALUE(rv) TYPE string.
    METHODS valtype_abap IMPORTING iv_type TYPE i RETURNING VALUE(rv) TYPE string.
ENDCLASS.

CLASS zcl_wasm_codegen IMPLEMENTATION.

  METHOD compile.
    mo_mod = io_module.
    CLEAR: mv_out, mv_indent.

    " Program header
    line( |PROGRAM { iv_name }.| ).
    line( || ).

    " Global data for memory and break propagation
    line( |DATA: gv_mem TYPE xstring, gv_mem_pages TYPE i, gv_br TYPE i.| ).

    " Globals
    LOOP AT mo_mod->mt_globals INTO DATA(ls_g).
      DATA(lv_gi) = sy-tabix - 1.
      line( |DATA: gv_g{ lv_gi } TYPE { valtype_abap( ls_g-type ) }.| ).
    ENDLOOP.

    line( || ).

    " Init memory
    IF mo_mod->ms_memory-min_pages > 0.
      DATA(lv_pages) = mo_mod->ms_memory-min_pages.
      DATA(lv_bytes) = lv_pages * 65536.
      line( |gv_mem_pages = { lv_pages }.| ).
      line( |DATA(lv_z) = CONV xstring( '00' ).| ).
      line( |DO { lv_bytes - 1 } TIMES. CONCATENATE gv_mem lv_z INTO gv_mem IN BYTE MODE. ENDDO.| ).
    ENDIF.

    " Init globals
    LOOP AT mo_mod->mt_globals INTO ls_g.
      DATA(lv_gi2) = sy-tabix - 1.
      IF ls_g-init_i32 <> 0.
        line( |gv_g{ lv_gi2 } = { ls_g-init_i32 }.| ).
      ENDIF.
    ENDLOOP.

    " Init data segments
    LOOP AT mo_mod->mt_data INTO DATA(ls_d).
      DATA(lv_dlen) = xstrlen( ls_d-data ).
      IF lv_dlen > 0.
        line( |gv_mem+{ ls_d-offset }({ lv_dlen }) = '{ ls_d-data }'.| ).
      ENDIF.
    ENDLOOP.

    line( || ).

    " Memory helper FORMs
    line( |FORM mem_ld_i32 USING iv_addr TYPE i CHANGING rv TYPE i.| ).
    line( |  DATA lv_b TYPE x LENGTH 4. lv_b = gv_mem+iv_addr(4).| ).
    line( |  DATA(lv_r) = lv_b+3(1) && lv_b+2(1) && lv_b+1(1) && lv_b+0(1). rv = lv_r.| ).
    line( |ENDFORM.| ).
    line( || ).
    line( |FORM mem_st_i32 USING iv_addr TYPE i iv_val TYPE i.| ).
    line( |  DATA lv_b TYPE x LENGTH 4. lv_b = iv_val.| ).
    line( |  DATA(lv_r) = lv_b+3(1) && lv_b+2(1) && lv_b+1(1) && lv_b+0(1). gv_mem+iv_addr(4) = lv_r.| ).
    line( |ENDFORM.| ).
    line( || ).
    line( |FORM mem_ld_i32_8u USING iv_addr TYPE i CHANGING rv TYPE i.| ).
    line( |  DATA lv_b TYPE x LENGTH 1. lv_b = gv_mem+iv_addr(1). rv = lv_b.| ).
    line( |ENDFORM.| ).
    line( || ).
    line( |FORM mem_st_i32_8 USING iv_addr TYPE i iv_val TYPE i.| ).
    line( |  DATA lv_b TYPE x LENGTH 1. lv_b = iv_val. gv_mem+iv_addr(1) = lv_b.| ).
    line( |ENDFORM.| ).
    line( || ).

    " Emit functions
    LOOP AT mo_mod->mt_functions INTO DATA(ls_func).
      emit_function( ls_func ).
    ENDLOOP.

    rv = mv_out.
  ENDMETHOD.


  METHOD line.
    DO mv_indent TIMES.
      mv_out = mv_out && `  `.
    ENDDO.
    mv_out = mv_out && iv && cl_abap_char_utilities=>newline.
  ENDMETHOD.


  METHOD push.
    rv = |lv_s{ mv_stack_depth }|.
    mv_stack_depth = mv_stack_depth + 1.
  ENDMETHOD.


  METHOD pop.
    mv_stack_depth = mv_stack_depth - 1.
    rv = |lv_s{ mv_stack_depth }|.
  ENDMETHOD.


  METHOD peek.
    rv = |lv_s{ mv_stack_depth - 1 }|.
  ENDMETHOD.


  METHOD func_name.
    DATA(lv_fi) = iv_idx - mo_mod->mv_num_imported_funcs.
    IF lv_fi >= 0 AND lv_fi < lines( mo_mod->mt_functions ).
      READ TABLE mo_mod->mt_functions INDEX lv_fi + 1 INTO DATA(ls_f).
      IF sy-subrc = 0 AND ls_f-export_name IS NOT INITIAL.
        rv = ls_f-export_name.
        RETURN.
      ENDIF.
    ENDIF.
    rv = |f{ iv_idx }|.
  ENDMETHOD.


  METHOD local_name.
    IF iv_idx < mv_num_params.
      rv = |p{ iv_idx }|.
    ELSE.
      rv = |lv_l{ iv_idx }|.
    ENDIF.
  ENDMETHOD.


  METHOD valtype_abap.
    CASE iv_type.
      WHEN 127. rv = 'i'.    " 0x7F = i32
      WHEN 126. rv = 'int8'. " 0x7E = i64
      WHEN OTHERS. rv = 'i'.
    ENDCASE.
  ENDMETHOD.


  METHOD emit_function.
    " Determine function name
    DATA(lv_name) = is_func-export_name.
    IF lv_name IS INITIAL. lv_name = |f{ is_func-index }|. ENDIF.

    " Get function type
    READ TABLE mo_mod->mt_types INDEX is_func-type_index + 1 INTO DATA(ls_type).

    " Build FORM signature
    DATA(lv_sig) = |FORM { lv_name }|.

    mv_num_params = lines( ls_type-params ).
    LOOP AT ls_type-params INTO DATA(ls_p).
      DATA(lv_pi) = sy-tabix - 1.
      lv_sig = lv_sig && | USING p{ lv_pi } TYPE { valtype_abap( ls_p-type ) }|.
    ENDLOOP.

    DATA(lv_has_result) = xsdbool( lines( ls_type-results ) > 0 ).
    IF lv_has_result = abap_true.
      lv_sig = lv_sig && | CHANGING rv TYPE { valtype_abap( ls_type-results[ 1 ]-type ) }|.
    ENDIF.

    line( |{ lv_sig }.| ).
    mv_indent = mv_indent + 1.

    " Declare local variables (beyond params)
    LOOP AT is_func-locals INTO DATA(ls_l).
      DATA(lv_li) = sy-tabix - 1 + mv_num_params.
      line( |DATA: lv_l{ lv_li } TYPE { valtype_abap( ls_l-type ) }.| ).
    ENDLOOP.

    " Declare stack variables (generous: 32 slots)
    DATA(lv_decl) = |DATA: gv_br TYPE i|.
    DO 32 TIMES.
      DATA(lv_si) = sy-index - 1.
      lv_decl = lv_decl && |, lv_s{ lv_si } TYPE i|.
    ENDDO.
    line( |{ lv_decl }.| ).

    " Reset compiler state
    mv_stack_depth = 0.
    CLEAR mt_block_kinds.

    " Emit instructions
    emit_instructions( is_func-code ).

    " Return value from stack top
    IF lv_has_result = abap_true AND mv_stack_depth > 0.
      line( |rv = { peek( ) }.| ).
    ENDIF.

    mv_indent = mv_indent - 1.
    line( |ENDFORM.| ).
    line( || ).
  ENDMETHOD.


  METHOD emit_instructions.
    DATA: lv_a TYPE string, lv_b TYPE string, lv_r TYPE string, lv_c TYPE string.

    LOOP AT it_code INTO DATA(ls_i).
      CASE ls_i-op.

        " --- Constants ---
        WHEN 65. " i32.const
          line( |{ push( ) } = { ls_i-i32_value }.| ).
        WHEN 66. " i64.const
          line( |{ push( ) } = { ls_i-i64_value }.| ).

        " --- Local/Global access ---
        WHEN 32. " local.get
          line( |{ push( ) } = { local_name( ls_i-local_idx ) }.| ).
        WHEN 33. " local.set
          line( |{ local_name( ls_i-local_idx ) } = { pop( ) }.| ).
        WHEN 34. " local.tee
          line( |{ local_name( ls_i-local_idx ) } = { peek( ) }.| ).
        WHEN 35. " global.get
          line( |{ push( ) } = gv_g{ ls_i-global_idx }.| ).
        WHEN 36. " global.set
          line( |gv_g{ ls_i-global_idx } = { pop( ) }.| ).

        " --- i32 Arithmetic ---
        WHEN 106. " i32.add
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } + { lv_b }.| ).
        WHEN 107. " i32.sub
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } - { lv_b }.| ).
        WHEN 108. " i32.mul
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } * { lv_b }.| ).
        WHEN 109. " i32.div_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } / { lv_b }.| ).
        WHEN 111. " i32.rem_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } MOD { lv_b }.| ).

        " --- i32 Comparisons ---
        WHEN 69. " i32.eqz
          lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } = 0. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 70. " i32.eq
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } = { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 71. " i32.ne
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } <> { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 72. " i32.lt_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } < { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 74. " i32.gt_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } > { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 76. " i32.le_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } <= { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).
        WHEN 78. " i32.ge_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_a } >= { lv_b }. { lv_r } = 1. ELSE. { lv_r } = 0. ENDIF.| ).

        " --- Bitwise (via ipow) ---
        WHEN 113. " i32.and — simplified via BIT-AND DATA statement trick
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |DATA(lv_xa) = CONV x4( { lv_a } ). DATA(lv_xb) = CONV x4( { lv_b } ).| ).
          line( |DATA(lv_xr) = lv_xa BIT-AND lv_xb. { lv_r } = lv_xr.| ).
        WHEN 114. " i32.or
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |DATA(lv_xa) = CONV x4( { lv_a } ). DATA(lv_xb) = CONV x4( { lv_b } ).| ).
          line( |DATA(lv_xr) = lv_xa BIT-OR lv_xb. { lv_r } = lv_xr.| ).
        WHEN 115. " i32.xor
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |DATA(lv_xa) = CONV x4( { lv_a } ). DATA(lv_xb) = CONV x4( { lv_b } ).| ).
          line( |DATA(lv_xr) = lv_xa BIT-XOR lv_xb. { lv_r } = lv_xr.| ).
        WHEN 116. " i32.shl
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } * ipow( base = 2 exp = { lv_b } MOD 32 ).| ).
        WHEN 117. " i32.shr_s
          lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |{ lv_r } = { lv_a } / ipow( base = 2 exp = { lv_b } MOD 32 ).| ).

        " --- Memory ---
        WHEN 40. " i32.load
          lv_a = pop( ). lv_r = push( ).
          IF ls_i-offset > 0.
            line( |PERFORM mem_ld_i32 USING { lv_a } + { ls_i-offset } CHANGING { lv_r }.| ).
          ELSE.
            line( |PERFORM mem_ld_i32 USING { lv_a } CHANGING { lv_r }.| ).
          ENDIF.
        WHEN 54. " i32.store
          lv_b = pop( ). lv_a = pop( ).
          IF ls_i-offset > 0.
            line( |PERFORM mem_st_i32 USING { lv_a } + { ls_i-offset } { lv_b }.| ).
          ELSE.
            line( |PERFORM mem_st_i32 USING { lv_a } { lv_b }.| ).
          ENDIF.
        WHEN 44. " i32.load8_u
          lv_a = pop( ). lv_r = push( ).
          IF ls_i-offset > 0.
            line( |PERFORM mem_ld_i32_8u USING { lv_a } + { ls_i-offset } CHANGING { lv_r }.| ).
          ELSE.
            line( |PERFORM mem_ld_i32_8u USING { lv_a } CHANGING { lv_r }.| ).
          ENDIF.
        WHEN 58. " i32.store8
          lv_b = pop( ). lv_a = pop( ).
          IF ls_i-offset > 0.
            line( |PERFORM mem_st_i32_8 USING { lv_a } + { ls_i-offset } { lv_b }.| ).
          ELSE.
            line( |PERFORM mem_st_i32_8 USING { lv_a } { lv_b }.| ).
          ENDIF.

        " --- Control flow ---
        WHEN 2. " block
          line( |DO 1 TIMES. " block| ).
          mv_indent = mv_indent + 1.
          APPEND c_block TO mt_block_kinds.
        WHEN 3. " loop
          line( |DO. " loop| ).
          mv_indent = mv_indent + 1.
          APPEND c_loop TO mt_block_kinds.
        WHEN 4. " if
          lv_c = pop( ).
          line( |IF { lv_c } <> 0.| ).
          mv_indent = mv_indent + 1.
          APPEND c_if TO mt_block_kinds.
        WHEN 5. " else
          mv_indent = mv_indent - 1.
          line( |ELSE.| ).
          mv_indent = mv_indent + 1.
        WHEN 11. " end
          IF lines( mt_block_kinds ) > 0.
            DATA(lv_kind) = mt_block_kinds[ lines( mt_block_kinds ) ].
            DELETE mt_block_kinds INDEX lines( mt_block_kinds ).
            mv_indent = mv_indent - 1.
            CASE lv_kind.
              WHEN c_block.
                line( |ENDDO.| ).
                line( |IF gv_br > 0. gv_br = gv_br - 1. EXIT. ENDIF.| ).
              WHEN c_loop.
                line( |ENDDO.| ).
                line( |IF gv_br > 0. gv_br = gv_br - 1. EXIT. ENDIF.| ).
              WHEN c_if.
                line( |ENDIF.| ).
            ENDCASE.
          ENDIF.

        WHEN 12. " br
          IF ls_i-label_idx = 0.
            " Check if targeting a loop → CONTINUE, else → EXIT
            IF lines( mt_block_kinds ) > 0 AND
               mt_block_kinds[ lines( mt_block_kinds ) ] = c_loop.
              line( |CONTINUE. " br 0 (loop)| ).
            ELSE.
              line( |EXIT. " br 0| ).
            ENDIF.
          ELSE.
            line( |gv_br = { ls_i-label_idx }. EXIT. " br { ls_i-label_idx }| ).
          ENDIF.

        WHEN 13. " br_if
          lv_c = pop( ).
          IF ls_i-label_idx = 0.
            IF lines( mt_block_kinds ) > 0 AND
               mt_block_kinds[ lines( mt_block_kinds ) ] = c_loop.
              line( |IF { lv_c } <> 0. CONTINUE. ENDIF. " br_if 0 (loop)| ).
            ELSE.
              line( |IF { lv_c } <> 0. EXIT. ENDIF. " br_if 0| ).
            ENDIF.
          ELSE.
            line( |IF { lv_c } <> 0. gv_br = { ls_i-label_idx }. EXIT. ENDIF. " br_if { ls_i-label_idx }| ).
          ENDIF.

        WHEN 15. " return
          IF mv_stack_depth > 0.
            line( |rv = { pop( ) }. RETURN.| ).
          ELSE.
            line( |RETURN.| ).
          ENDIF.

        " --- Call ---
        WHEN 16. " call
          emit_call( ls_i-func_idx ).

        " --- Stack ---
        WHEN 26. " drop
          pop( ).
        WHEN 27. " select
          lv_c = pop( ). lv_b = pop( ). lv_a = pop( ). lv_r = push( ).
          line( |IF { lv_c } <> 0. { lv_r } = { lv_a }. ELSE. { lv_r } = { lv_b }. ENDIF.| ).

        " --- Nop / Unreachable ---
        WHEN 0. " unreachable
          line( |RAISE EXCEPTION TYPE cx_sy_program_error. " unreachable| ).
        WHEN 1. " nop
          " nothing

      ENDCASE.
    ENDLOOP.
  ENDMETHOD.


  METHOD emit_call.
    DATA(lv_fname) = func_name( iv_func_idx ).

    " Get function type
    DATA(lv_fi) = iv_func_idx - mo_mod->mv_num_imported_funcs.
    READ TABLE mo_mod->mt_functions INDEX lv_fi + 1 INTO DATA(ls_f).
    READ TABLE mo_mod->mt_types INDEX ls_f-type_index + 1 INTO DATA(ls_type).

    " Pop arguments in reverse
    DATA(lv_np) = lines( ls_type-params ).
    DATA: lt_args TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
    DO lv_np TIMES.
      INSERT pop( ) INTO lt_args INDEX 1.
    ENDDO.

    " Build PERFORM call
    DATA(lv_has_result) = xsdbool( lines( ls_type-results ) > 0 ).
    DATA(lv_result) = ||.
    IF lv_has_result = abap_true.
      lv_result = push( ).
    ENDIF.

    DATA(lv_call) = |PERFORM { lv_fname }|.
    LOOP AT lt_args INTO DATA(lv_arg).
      lv_call = lv_call && | USING { lv_arg }|.
    ENDLOOP.
    IF lv_has_result = abap_true.
      lv_call = lv_call && | CHANGING { lv_result }|.
    ENDIF.
    line( |{ lv_call }.| ).
  ENDMETHOD.

ENDCLASS.
