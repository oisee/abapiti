CLASS z_src_abap_2_s_97adc9efefb8d1 DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
INTERFACES z_src_abap_2_s_e1e980746d06e1.
INTERFACES z_union_b0cca5_ee926be4fb96e6.
INTERFACES z_union_a89e3b_daf022d4653557.
DATA z_member_list_484fd43dd9f8d0 TYPE REF TO z_runtime_arra_c22708faf2e354.
METHODS constructor IMPORTING z_param_list_750b7e98c594f7 TYPE REF TO z_runtime_arra_c22708faf2e354.
METHODS z_member_listk_af22c428e8602b RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_getus_a169a267c2678f RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_run_71be1bca84374a IMPORTING z_param_r_f634cd02b53efc TYPE REF TO z_runtime_arra_c22708faf2e354 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_c22708faf2e354.
METHODS z_member_railr_af4a583f62a6dc RETURNING VALUE(result) TYPE string.
METHODS z_member_tostr_36e91adbba659c RETURNING VALUE(result) TYPE string.
METHODS z_member_first_8d87896c21eb17 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_run_o_b86da49c574129 IMPORTING z_param_x_af55868a593612 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_c22708faf2e354.
INTERFACES z_runtime_desc_e5fe811e0f4259.
METHODS z_builtin_clas_3f891b097658ff RETURNING VALUE(result) TYPE REF TO z_runtime_clas_a39833db329515.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS z_src_abap_2_s_97adc9efefb8d1 IMPLEMENTATION.
METHOD constructor.
DATA(t1) = CAST z_runtime_arra_c22708faf2e354( z_param_list_750b7e98c594f7 ).
DATA(t2) = VALUE abap_bool( ).
DATA(t3) = VALUE i( ).
DATA(t4) = CAST z_runtime_arra_c22708faf2e354( t1 ).
t3 = lines( t4->items ).
DATA(t5) = CONV i( '2' ).
t2 = xsdbool( t3 < t5 ).
IF t2 = abap_true.
DATA t6 TYPE REF TO z_exception_cl_30eec8e8805d60.
CLEAR t6.
DATA t7 TYPE REF TO z_builtin_erro_e8b3bd7e8a547c.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_opti_4ba3bdccd169bb.
CLEAR t8.
DATA(t9) = CONV abap_bool( abap_true ).
IF t9 = abap_true.
DATA(t10) = CONV string( |Alternative, length error| ).
DATA(t11) = NEW z_runtime_opti_4ba3bdccd169bb( ).
t11->has = abap_true.
t11->value = t10.
t8 = t11.
ELSE.
DATA t12 TYPE REF TO z_runtime_opti_4ba3bdccd169bb.
CLEAR t12.
t8 = t12.
ENDIF.
t7 = NEW z_builtin_erro_e8b3bd7e8a547c( z_param_messag_26cb69beb0eed4 = t8 ).
t6 = NEW #( ).
t6->payload = t7.
RAISE EXCEPTION t6.
ENDIF.
DATA(t13) = CAST z_runtime_arra_c22708faf2e354( t1 ).
me->z_member_list_484fd43dd9f8d0 = t13.
ENDMETHOD.
METHOD z_member_listk_af22c428e8602b.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:958| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:966| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_run_71be1bca84374a.
DATA(t1) = CAST z_runtime_arra_c22708faf2e354( z_param_r_f634cd02b53efc ).
DATA t2 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t2.
DATA t3 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t3.
CREATE OBJECT t3.
t2 = t3.
DATA(t4) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t5 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t5.
DATA t6 TYPE REF TO object.
CLEAR t6.
LOOP AT t4->items INTO t6.
t5 ?= t6.
DATA t7 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t8.
DATA(t9) = CAST z_src_abap_2_s_e1e980746d06e1( t5 ).
DATA(t10) = CAST z_runtime_arra_c22708faf2e354( t1 ).
t8 = t9->z_member_run_71be1bca84374a( z_param_r_f634cd02b53efc = t10 ).
t7 = t8.
DATA(t11) = VALUE abap_bool( ).
DATA(t12) = VALUE i( ).
DATA(t13) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t12 = lines( t13->items ).
DATA(t14) = CONV i( '0' ).
t11 = xsdbool( t12 > t14 ).
IF t11 = abap_true.
DATA(t15) = VALUE abap_bool( ).
DATA(t16) = VALUE i( ).
DATA(t17) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t16 = lines( t17->items ).
DATA(t18) = CONV i( '1' ).
t15 = xsdbool( t16 = t18 ).
IF t15 = abap_true.
DATA(t19) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t20 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t20.
DATA(t21) = CAST z_runtime_arra_c22708faf2e354( t7 ).
DATA t22 TYPE REF TO object.
CLEAR t22.
READ TABLE t21->items INDEX 1 INTO t22.
t20 ?= t22.
APPEND t20 TO t19->items.
ELSE.
DATA(t23) = VALUE i( ).
DATA(t24) = CAST z_runtime_arra_c22708faf2e354( t7 ).
DATA t25 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t25.
DATA t26 TYPE REF TO object.
CLEAR t26.
LOOP AT t24->items INTO t26.
t25 ?= t26.
DATA(t27) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t28) = CAST z_src_abap_2_s_b6c5f651b292e5( t25 ).
APPEND t28 TO t27->items.
ENDLOOP.
DATA(t29) = VALUE i( ).
DATA(t30) = CAST z_runtime_arra_c22708faf2e354( t2 ).
t29 = lines( t30->items ).
t23 = t29.
ENDIF.
EXIT.
ENDIF.
ENDLOOP.
DATA(t31) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t31.
RETURN.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:990| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:995| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA(t1) = VALUE abap_bool( ).
DATA(t2) = VALUE i( ).
DATA(t3) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
t2 = lines( t3->items ).
DATA(t4) = CONV i( '2' ).
t1 = xsdbool( t2 <> t4 ).
IF t1 = abap_true.
DATA t5 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t5.
DATA t6 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t6.
DATA t7 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t7.
CREATE OBJECT t7.
t6 = t7.
DATA(t8) = CAST z_runtime_arra_8ecd6afe19e0a5( t6 ).
DATA(t9) = CONV string( || ).
APPEND t9 TO t8->items.
DATA(t10) = CAST z_runtime_arra_8ecd6afe19e0a5( t6 ).
t5 = t10.
result = t5.
RETURN.
ENDIF.
DATA t11 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t11.
DATA t12 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t12.
DATA t13 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t13.
DATA(t14) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t15 TYPE REF TO object.
CLEAR t15.
READ TABLE t14->items INDEX 1 INTO t15.
t13 ?= t15.
t12 = t13->z_member_first_8d87896c21eb17(  ).
t11 = t12.
DATA t16 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t16.
DATA t17 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t17.
DATA t18 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t18.
DATA(t19) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t20 TYPE REF TO object.
CLEAR t20.
READ TABLE t19->items INDEX 2 INTO t20.
t18 ?= t20.
t17 = t18->z_member_first_8d87896c21eb17(  ).
t16 = t17.
DATA(t21) = VALUE abap_bool( ).
DATA(t22) = VALUE abap_bool( ).
DATA(t23) = VALUE i( ).
DATA(t24) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
t23 = lines( t24->items ).
DATA(t25) = CONV i( '1' ).
t22 = xsdbool( t23 = t25 ).
t21 = t22.
IF t22 = abap_true.
DATA(t26) = VALUE abap_bool( ).
DATA(t27) = VALUE string( ).
DATA(t28) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
READ TABLE t28->items INDEX 1 INTO t27.
DATA(t29) = CONV string( || ).
t26 = xsdbool( t27 = t29 ).
t21 = t26.
ENDIF.
IF t21 = abap_true.
DATA(t30) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
result = t30.
RETURN.
ENDIF.
DATA(t31) = VALUE abap_bool( ).
DATA(t32) = VALUE abap_bool( ).
DATA(t33) = VALUE i( ).
DATA(t34) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
t33 = lines( t34->items ).
DATA(t35) = CONV i( '1' ).
t32 = xsdbool( t33 = t35 ).
t31 = t32.
IF t32 = abap_true.
DATA(t36) = VALUE abap_bool( ).
DATA(t37) = VALUE string( ).
DATA(t38) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
READ TABLE t38->items INDEX 1 INTO t37.
DATA(t39) = CONV string( || ).
t36 = xsdbool( t37 = t39 ).
t31 = t36.
ENDIF.
IF t31 = abap_true.
DATA(t40) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
result = t40.
RETURN.
ENDIF.
DATA(t41) = VALUE abap_bool( ).
DATA(t42) = VALUE abap_bool( ).
DATA(t43) = VALUE abap_bool( ).
DATA(t44) = VALUE i( ).
DATA(t45) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
t44 = lines( t45->items ).
DATA(t46) = CONV i( '1' ).
t43 = xsdbool( t44 = t46 ).
t42 = t43.
IF t43 = abap_true.
DATA(t47) = VALUE abap_bool( ).
DATA(t48) = VALUE i( ).
DATA(t49) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
t48 = lines( t49->items ).
DATA(t50) = CONV i( '1' ).
t47 = xsdbool( t48 = t50 ).
t42 = t47.
ENDIF.
t41 = t42.
IF t42 = abap_true.
DATA(t51) = VALUE abap_bool( ).
DATA(t52) = VALUE string( ).
DATA(t53) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
READ TABLE t53->items INDEX 1 INTO t52.
DATA(t54) = VALUE string( ).
DATA(t55) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
READ TABLE t55->items INDEX 1 INTO t54.
t51 = xsdbool( t52 = t54 ).
t41 = t51.
ENDIF.
IF t41 = abap_true.
DATA(t56) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
result = t56.
RETURN.
ENDIF.
DATA(t57) = VALUE i( ).
DATA(t58) = CAST z_runtime_arra_8ecd6afe19e0a5( t16 ).
DATA(t59) = VALUE string( ).
LOOP AT t58->items INTO t59.
DATA(t60) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
DATA(t61) = CONV string( t59 ).
APPEND t61 TO t60->items.
ENDLOOP.
DATA(t62) = VALUE i( ).
DATA(t63) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
t62 = lines( t63->items ).
t57 = t62.
DATA(t64) = CAST z_runtime_arra_8ecd6afe19e0a5( t11 ).
result = t64.
RETURN.
ENDMETHOD.
METHOD z_member_run_o_b86da49c574129.
DATA(t1) = CAST z_src_abap_2_s_b6c5f651b292e5( z_param_x_af55868a593612 ).
DATA t2 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t2.
DATA t3 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t3.
DATA t4 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t4.
DATA t5 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t5.
CREATE OBJECT t5.
t4 = t5.
DATA(t6) = CAST z_runtime_arra_c22708faf2e354( t4 ).
DATA(t7) = CAST z_src_abap_2_s_b6c5f651b292e5( t1 ).
APPEND t7 TO t6->items.
DATA(t8) = CAST z_runtime_arra_c22708faf2e354( t4 ).
t3 = t8.
t2 = me->z_member_run_71be1bca84374a( z_param_r_f634cd02b53efc = t3 ).
result = t2.
RETURN.
ENDMETHOD.
METHOD z_runtime_desc_e5fe811e0f4259~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_builtin_clas_3f891b097658ff.
result = z_runtime_desc_78196487750d0f=>z_member_d78_b753e51a71819e.
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_run_71be1bca84374a.
result = me->z_member_run_71be1bca84374a(  z_param_r_f634cd02b53efc = z_param_r_f634cd02b53efc ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_railr_af4a583f62a6dc.
result = me->z_member_railr_af4a583f62a6dc(  ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_tostr_36e91adbba659c.
result = me->z_member_tostr_36e91adbba659c(  ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_getus_a169a267c2678f.
result = me->z_member_getus_a169a267c2678f(  ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_listk_af22c428e8602b.
result = me->z_member_listk_af22c428e8602b(  ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_first_8d87896c21eb17.
result = me->z_member_first_8d87896c21eb17(  ).
ENDMETHOD.
METHOD z_src_abap_2_s_e1e980746d06e1~z_member_run_o_b86da49c574129.
result = me->z_member_run_o_b86da49c574129(  z_param_x_af55868a593612 = z_param_x_af55868a593612 ).
ENDMETHOD.
METHOD z_union_b0cca5_ee926be4fb96e6~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_union_a89e3b_daf022d4653557~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
ENDCLASS.
