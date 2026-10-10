CLASS z_src_abap_2_s_4228f5ccb84481 DEFINITION PUBLIC ABSTRACT CREATE PUBLIC.
PUBLIC SECTION.
INTERFACES z_src_abap_2_s_e1e980746d06e1.
INTERFACES z_union_b0cca5_ee926be4fb96e6.
INTERFACES z_union_a89e3b_daf022d4653557.
DATA z_member_runna_7456acee370aba TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
METHODS constructor.
METHODS z_member_run_71be1bca84374a IMPORTING z_param_r_f634cd02b53efc TYPE REF TO z_runtime_arra_c22708faf2e354 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_c22708faf2e354.
METHODS z_member_getru_b627c08d5eeec8 ABSTRACT RETURNING VALUE(result) TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
METHODS z_member_listk_af22c428e8602b RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_getus_a169a267c2678f RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_getna_2b4e529c00f5dc RETURNING VALUE(result) TYPE string.
METHODS z_member_railr_af4a583f62a6dc RETURNING VALUE(result) TYPE string.
METHODS z_member_tostr_36e91adbba659c RETURNING VALUE(result) TYPE string.
METHODS z_member_first_8d87896c21eb17 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
METHODS z_member_run_o_b86da49c574129 IMPORTING z_param_x_af55868a593612 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5 RETURNING VALUE(result) TYPE REF TO z_runtime_arra_c22708faf2e354.
INTERFACES z_runtime_desc_e5fe811e0f4259.
METHODS z_builtin_clas_3f891b097658ff RETURNING VALUE(result) TYPE REF TO z_runtime_clas_a39833db329515.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS z_src_abap_2_s_4228f5ccb84481 IMPLEMENTATION.
METHOD constructor.
DATA t1 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t1.
me->z_member_runna_7456acee370aba = t1.
ENDMETHOD.
METHOD z_member_run_71be1bca84374a.
DATA range_int8_bound_min TYPE int8.
DATA range_int8_bound_max TYPE int8.
range_int8_bound_min = -9007199.
range_int8_bound_min = range_int8_bound_min * 1000000000.
range_int8_bound_min = range_int8_bound_min - 254740991.
range_int8_bound_max = 9007199.
range_int8_bound_max = range_int8_bound_max * 1000000000.
range_int8_bound_max = range_int8_bound_max + 254740991.
DATA(t1) = CAST z_runtime_arra_c22708faf2e354( z_param_r_f634cd02b53efc ).
DATA t2 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t2.
DATA t3 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t3.
CREATE OBJECT t3.
t2 = t3.
DATA(t4) = VALUE abap_bool( ).
DATA t5 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t5.
DATA(t6) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_runna_7456acee370aba ).
t5 = t6.
DATA t7 TYPE REF TO z_runtime_dyna_a429568d5be89e.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_dyna_a429568d5be89e.
CLEAR t8.
t7 = t8.
DATA(t9) = VALUE abap_bool( ).
DATA(t10) = CAST z_src_abap_2_s_e1e980746d06e1( t5 ).
IF t10 IS INITIAL.
t9 = abap_true.
ENDIF.
t4 = t9.
IF t4 = abap_true.
DATA t11 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t11.
DATA(t12) = CONV abap_bool( abap_true ).
IF t12 = abap_true.
DATA(t13) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_getru_b627c08d5eeec8(  ) ).
t11 = t13.
ELSE.
DATA t14 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t14.
t11 = t14.
ENDIF.
me->z_member_runna_7456acee370aba = t11.
ENDIF.
DATA(t15) = CAST z_runtime_arra_c22708faf2e354( t1 ).
DATA t16 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t16.
DATA t17 TYPE REF TO object.
CLEAR t17.
LOOP AT t15->items INTO t17.
t16 ?= t17.
DATA t18 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t18.
DATA t19 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t19.
DATA t20 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t20.
DATA(t21) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_runna_7456acee370aba ).
t20 = t21.
DATA(t22) = CAST z_src_abap_2_s_b6c5f651b292e5( t16 ).
t19 = t20->z_member_run_o_b86da49c574129( z_param_x_af55868a593612 = t22 ).
t18 = t19.
DATA(t23) = CAST z_runtime_arra_c22708faf2e354( t18 ).
DATA t24 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t24.
DATA t25 TYPE REF TO object.
CLEAR t25.
LOOP AT t23->items INTO t25.
t24 ?= t25.
DATA(t26) = VALUE int8( ).
DATA(t27) = VALUE int8( ).
DATA(t28) = VALUE int8( ).
DATA t29 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t29.
DATA(t30) = CAST z_src_abap_2_s_b6c5f651b292e5( t16 ).
t29 = t30.
DATA(t31) = VALUE int8( ).
DATA(t32) = VALUE int8( ).
DATA(t33) = VALUE i( ).
DATA t34 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t34.
DATA(t35) = CAST z_src_abap_2_s_b6c5f651b292e5( t29 ).
t34 = t35->z_member_token_ed4bfabc8c4cec.
t33 = lines( t34->items ).
t32 = t33.
DATA(t36) = VALUE int8( ).
DATA(t37) = VALUE i( ).
DATA(t38) = CAST z_src_abap_2_s_b6c5f651b292e5( t29 ).
t37 = t38->z_member_token_62f41b3bcb0bad.
t36 = t37.
t31 = t32 - t36.
IF t31 < range_int8_bound_min OR t31 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t28 = t31.
DATA(t39) = VALUE int8( ).
DATA t40 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t40.
DATA(t41) = CAST z_src_abap_2_s_b6c5f651b292e5( t24 ).
t40 = t41.
DATA(t42) = VALUE int8( ).
DATA(t43) = VALUE int8( ).
DATA(t44) = VALUE i( ).
DATA t45 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t45.
DATA(t46) = CAST z_src_abap_2_s_b6c5f651b292e5( t40 ).
t45 = t46->z_member_token_ed4bfabc8c4cec.
t44 = lines( t45->items ).
t43 = t44.
DATA(t47) = VALUE int8( ).
DATA(t48) = VALUE i( ).
DATA(t49) = CAST z_src_abap_2_s_b6c5f651b292e5( t40 ).
t48 = t49->z_member_token_62f41b3bcb0bad.
t47 = t48.
t42 = t43 - t47.
IF t42 < range_int8_bound_min OR t42 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t39 = t42.
t27 = t28 - t39.
IF t27 < range_int8_bound_min OR t27 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t26 = t27.
DATA(t50) = VALUE abap_bool( ).
DATA(t51) = CONV int8( t26 ).
DATA(t52) = CONV int8( 0 ).
t50 = xsdbool( t51 > t52 ).
IF t50 = abap_true.
DATA t53 TYPE REF TO z_src_abap_nod_582169980ca52f.
CLEAR t53.
DATA t54 TYPE REF TO z_src_abap_nod_582169980ca52f.
CLEAR t54.
DATA(t55) = CAST z_src_abap_2_s_4228f5ccb84481( me ).
t54 = NEW z_src_abap_nod_582169980ca52f( z_param_expres_0c24dd0b5695e5 = t55 ).
t53 = t54.
DATA(t56) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t57 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t57.
DATA(t58) = CAST z_src_abap_2_s_b6c5f651b292e5( t24 ).
DATA(t59) = CONV int8( t26 ).
DATA(t60) = CAST z_src_abap_nod_582169980ca52f( t53 ).
t57 = t58->z_member_wrapc_402ce4e1f019a6( z_param_consum_74b8bf7d11c654 = t59 z_param_node_1324e81b8c7326 = t60 ).
APPEND t57 TO t56->items.
ELSE.
DATA(t61) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t62) = CAST z_src_abap_2_s_b6c5f651b292e5( t24 ).
APPEND t62 TO t61->items.
ENDIF.
ENDLOOP.
ENDLOOP.
DATA(t63) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t63.
RETURN.
ENDMETHOD.
METHOD z_member_listk_af22c428e8602b.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:729| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:734| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getna_2b4e529c00f5dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:738| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:742| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:746| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA t1 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t1.
DATA(t2) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_getru_b627c08d5eeec8(  ) ).
t1 = t2->z_member_first_8d87896c21eb17(  ).
result = t1.
RETURN.
ENDMETHOD.
METHOD z_member_run_o_b86da49c574129.
DATA range_int8_bound_min TYPE int8.
DATA range_int8_bound_max TYPE int8.
range_int8_bound_min = -9007199.
range_int8_bound_min = range_int8_bound_min * 1000000000.
range_int8_bound_min = range_int8_bound_min - 254740991.
range_int8_bound_max = 9007199.
range_int8_bound_max = range_int8_bound_max * 1000000000.
range_int8_bound_max = range_int8_bound_max + 254740991.
DATA(t1) = CAST z_src_abap_2_s_b6c5f651b292e5( z_param_x_af55868a593612 ).
DATA t2 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t2.
DATA t3 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t3.
CREATE OBJECT t3.
t2 = t3.
DATA(t4) = VALUE abap_bool( ).
DATA t5 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t5.
DATA(t6) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_runna_7456acee370aba ).
t5 = t6.
DATA t7 TYPE REF TO z_runtime_dyna_a429568d5be89e.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_dyna_a429568d5be89e.
CLEAR t8.
t7 = t8.
DATA(t9) = VALUE abap_bool( ).
DATA(t10) = CAST z_src_abap_2_s_e1e980746d06e1( t5 ).
IF t10 IS INITIAL.
t9 = abap_true.
ENDIF.
t4 = t9.
IF t4 = abap_true.
DATA t11 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t11.
DATA(t12) = CONV abap_bool( abap_true ).
IF t12 = abap_true.
DATA(t13) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_getru_b627c08d5eeec8(  ) ).
t11 = t13.
ELSE.
DATA t14 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t14.
t11 = t14.
ENDIF.
me->z_member_runna_7456acee370aba = t11.
ENDIF.
DATA t15 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t15.
DATA(t16) = CAST z_src_abap_2_s_b6c5f651b292e5( t1 ).
t15 = t16.
DATA t17 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t17.
DATA t18 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t18.
DATA t19 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t19.
DATA(t20) = CAST z_src_abap_2_s_e1e980746d06e1( me->z_member_runna_7456acee370aba ).
t19 = t20.
DATA(t21) = CAST z_src_abap_2_s_b6c5f651b292e5( t15 ).
t18 = t19->z_member_run_o_b86da49c574129( z_param_x_af55868a593612 = t21 ).
t17 = t18.
DATA(t22) = CAST z_runtime_arra_c22708faf2e354( t17 ).
DATA t23 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t23.
DATA t24 TYPE REF TO object.
CLEAR t24.
LOOP AT t22->items INTO t24.
t23 ?= t24.
DATA(t25) = VALUE int8( ).
DATA(t26) = VALUE int8( ).
DATA(t27) = VALUE int8( ).
DATA t28 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t28.
DATA(t29) = CAST z_src_abap_2_s_b6c5f651b292e5( t15 ).
t28 = t29.
DATA(t30) = VALUE int8( ).
DATA(t31) = VALUE int8( ).
DATA(t32) = VALUE i( ).
DATA t33 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t33.
DATA(t34) = CAST z_src_abap_2_s_b6c5f651b292e5( t28 ).
t33 = t34->z_member_token_ed4bfabc8c4cec.
t32 = lines( t33->items ).
t31 = t32.
DATA(t35) = VALUE int8( ).
DATA(t36) = VALUE i( ).
DATA(t37) = CAST z_src_abap_2_s_b6c5f651b292e5( t28 ).
t36 = t37->z_member_token_62f41b3bcb0bad.
t35 = t36.
t30 = t31 - t35.
IF t30 < range_int8_bound_min OR t30 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t27 = t30.
DATA(t38) = VALUE int8( ).
DATA t39 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t39.
DATA(t40) = CAST z_src_abap_2_s_b6c5f651b292e5( t23 ).
t39 = t40.
DATA(t41) = VALUE int8( ).
DATA(t42) = VALUE int8( ).
DATA(t43) = VALUE i( ).
DATA t44 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t44.
DATA(t45) = CAST z_src_abap_2_s_b6c5f651b292e5( t39 ).
t44 = t45->z_member_token_ed4bfabc8c4cec.
t43 = lines( t44->items ).
t42 = t43.
DATA(t46) = VALUE int8( ).
DATA(t47) = VALUE i( ).
DATA(t48) = CAST z_src_abap_2_s_b6c5f651b292e5( t39 ).
t47 = t48->z_member_token_62f41b3bcb0bad.
t46 = t47.
t41 = t42 - t46.
IF t41 < range_int8_bound_min OR t41 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t38 = t41.
t26 = t27 - t38.
IF t26 < range_int8_bound_min OR t26 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t25 = t26.
DATA(t49) = VALUE abap_bool( ).
DATA(t50) = CONV int8( t25 ).
DATA(t51) = CONV int8( 0 ).
t49 = xsdbool( t50 > t51 ).
IF t49 = abap_true.
DATA t52 TYPE REF TO z_src_abap_nod_582169980ca52f.
CLEAR t52.
DATA t53 TYPE REF TO z_src_abap_nod_582169980ca52f.
CLEAR t53.
DATA(t54) = CAST z_src_abap_2_s_4228f5ccb84481( me ).
t53 = NEW z_src_abap_nod_582169980ca52f( z_param_expres_0c24dd0b5695e5 = t54 ).
t52 = t53.
DATA(t55) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t56 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t56.
DATA(t57) = CAST z_src_abap_2_s_b6c5f651b292e5( t23 ).
DATA(t58) = CONV int8( t25 ).
DATA(t59) = CAST z_src_abap_nod_582169980ca52f( t52 ).
t56 = t57->z_member_wrapc_402ce4e1f019a6( z_param_consum_74b8bf7d11c654 = t58 z_param_node_1324e81b8c7326 = t59 ).
APPEND t56 TO t55->items.
ELSE.
DATA(t60) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t61) = CAST z_src_abap_2_s_b6c5f651b292e5( t23 ).
APPEND t61 TO t60->items.
ENDIF.
ENDLOOP.
DATA(t62) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t62.
RETURN.
ENDMETHOD.
METHOD z_runtime_desc_e5fe811e0f4259~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_builtin_clas_3f891b097658ff.
result = z_runtime_desc_78196487750d0f=>z_member_d74_8cc8f0f34eff40.
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
