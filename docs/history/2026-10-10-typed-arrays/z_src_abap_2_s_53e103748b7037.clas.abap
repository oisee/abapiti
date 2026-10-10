CLASS z_src_abap_2_s_53e103748b7037 DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
INTERFACES z_src_abap_2_s_e1e980746d06e1.
INTERFACES z_union_b0cca5_ee926be4fb96e6.
INTERFACES z_union_a89e3b_daf022d4653557.
DATA z_member_name_af98a037ec38c5 TYPE string.
DATA z_member_token_9f21cad060fb60 TYPE REF TO z_runtime_clas_a39833db329515.
METHODS constructor IMPORTING z_param_t_a7fc2749377e71 TYPE REF TO z_runtime_clas_a39833db329515.
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
CLASS z_src_abap_2_s_53e103748b7037 IMPLEMENTATION.
METHOD constructor.
DATA(t1) = CAST z_runtime_clas_a39833db329515( z_param_t_a7fc2749377e71 ).
DATA(t2) = CAST z_runtime_clas_a39833db329515( t1 ).
me->z_member_token_9f21cad060fb60 = t2.
DATA(t3) = VALUE string( ).
DATA(t4) = CAST z_runtime_clas_a39833db329515( t1 ).
t3 = t4->name.
me->z_member_name_af98a037ec38c5 = t3.
ENDMETHOD.
METHOD z_member_listk_af22c428e8602b.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:107| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:111| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
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
DATA(t4) = CAST z_runtime_arra_c22708faf2e354( t1 ).
DATA t5 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t5.
DATA t6 TYPE REF TO object.
CLEAR t6.
LOOP AT t4->items INTO t6.
t5 ?= t6.
DATA(t7) = VALUE abap_bool( ).
DATA(t8) = VALUE abap_bool( ).
DATA(t9) = VALUE int8( ).
DATA t10 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t10.
DATA(t11) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t10 = t11.
DATA(t12) = VALUE int8( ).
DATA(t13) = VALUE int8( ).
DATA(t14) = VALUE i( ).
DATA t15 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t15.
DATA(t16) = CAST z_src_abap_2_s_b6c5f651b292e5( t10 ).
t15 = t16->z_member_token_ed4bfabc8c4cec.
t14 = lines( t15->items ).
t13 = t14.
DATA(t17) = VALUE int8( ).
DATA(t18) = VALUE i( ).
DATA(t19) = CAST z_src_abap_2_s_b6c5f651b292e5( t10 ).
t18 = t19->z_member_token_62f41b3bcb0bad.
t17 = t18.
t12 = t13 - t17.
IF t12 < range_int8_bound_min OR t12 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t9 = t12.
DATA(t20) = CONV int8( 0 ).
t8 = xsdbool( t9 <> t20 ).
t7 = t8.
IF t8 = abap_true.
DATA(t21) = VALUE abap_bool( ).
DATA t22 TYPE REF TO z_runtime_clas_a39833db329515.
CLEAR t22.
DATA t23 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t23.
DATA t24 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t24.
DATA(t25) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t24 = t25.
DATA t26 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t26.
DATA t27 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t27.
DATA(t28) = CAST z_src_abap_2_s_b6c5f651b292e5( t24 ).
t27 = t28->z_member_token_ed4bfabc8c4cec.
DATA(t29) = VALUE i( ).
DATA(t30) = CAST z_src_abap_2_s_b6c5f651b292e5( t24 ).
t29 = t30->z_member_token_62f41b3bcb0bad.
t29 = t29 + 1.
DATA t31 TYPE REF TO object.
CLEAR t31.
READ TABLE t27->items INDEX t29 INTO t31.
t26 ?= t31.
t23 = t26.
t22 = t23->z_builtin_clas_3f891b097658ff( ).
DATA(t32) = CAST z_runtime_clas_a39833db329515( me->z_member_token_9f21cad060fb60 ).
t21 = xsdbool( t22 = t32 ).
t7 = t21.
ENDIF.
IF t7 = abap_true.
DATA(t33) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t34 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t34.
DATA t35 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t35.
DATA(t36) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t35 = t36.
DATA t37 TYPE REF TO z_union_667199_4cf1d0543504ab.
CLEAR t37.
DATA t38 TYPE REF TO z_src_abap_nod_935d26003cb490.
CLEAR t38.
DATA t39 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t39.
DATA t40 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t40.
DATA(t41) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t40 = t41.
DATA t42 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t42.
DATA t43 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t43.
DATA(t44) = CAST z_src_abap_2_s_b6c5f651b292e5( t40 ).
t43 = t44->z_member_token_ed4bfabc8c4cec.
DATA(t45) = VALUE i( ).
DATA(t46) = CAST z_src_abap_2_s_b6c5f651b292e5( t40 ).
t45 = t46->z_member_token_62f41b3bcb0bad.
t45 = t45 + 1.
DATA t47 TYPE REF TO object.
CLEAR t47.
READ TABLE t43->items INDEX t45 INTO t47.
t42 ?= t47.
t39 = t42.
t38 = NEW z_src_abap_nod_935d26003cb490( z_param_token_06f9e7a29ec34c = t39 ).
t37 = t38.
DATA t48 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t48.
DATA t49 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t49.
DATA(t50) = CAST z_src_abap_2_s_b6c5f651b292e5( t35 ).
t49 = t50->z_member_token_ed4bfabc8c4cec.
DATA(t51) = VALUE int8( ).
DATA(t52) = VALUE int8( ).
DATA(t53) = VALUE i( ).
DATA(t54) = CAST z_src_abap_2_s_b6c5f651b292e5( t35 ).
t53 = t54->z_member_token_62f41b3bcb0bad.
t52 = t53.
DATA(t55) = CONV int8( 1 ).
t51 = t52 + t55.
IF t51 < range_int8_bound_min OR t51 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
DATA t56 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t56.
DATA(t57) = CONV abap_bool( abap_true ).
IF t57 = abap_true.
DATA t58 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t58.
DATA(t59) = CAST z_union_667199_4cf1d0543504ab( t37 ).
DATA t60 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t60.
DATA(t61) = CAST z_src_abap_2_s_b6c5f651b292e5( t35 ).
t60 = t61->z_member_nodes_022617c0b510ec.
t58 = NEW z_src_abap_2_s_c435ad477cf33f( z_param_node_1324e81b8c7326 = t59 z_param_previo_4981fbd290610f = t60 ).
t56 = t58.
ELSE.
DATA t62 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t62.
t56 = t62.
ENDIF.
DATA(t63) = VALUE int8( ).
DATA(t64) = VALUE int8( ).
DATA(t65) = CAST z_src_abap_2_s_b6c5f651b292e5( t35 ).
t64 = t65->z_member_nodec_32889faa6e522a.
DATA(t66) = CONV int8( 1 ).
t63 = t64 + t66.
IF t63 < range_int8_bound_min OR t63 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t48 = z_src_abap_2_s_b6c5f651b292e5=>z_member_fromc_9d7b43515b1343( z_param_tokens_3a66ced5b5b26a = t49 z_param_tokeni_36bd2fa45d0f58 = t51 z_param_nodes_51988edc19892d = t56 z_param_nodeco_6f0a19763cdb2a = t63 ).
t34 = t48.
APPEND t34 TO t33->items.
ENDIF.
ENDLOOP.
DATA(t67) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t67.
RETURN.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:127| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:140| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:144| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
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
DATA t4 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t4.
DATA(t5) = CAST z_src_abap_2_s_b6c5f651b292e5( t1 ).
t4 = t5.
DATA(t6) = VALUE abap_bool( ).
DATA(t7) = VALUE abap_bool( ).
DATA(t8) = VALUE int8( ).
DATA t9 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t9.
DATA(t10) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t9 = t10.
DATA(t11) = VALUE int8( ).
DATA(t12) = VALUE int8( ).
DATA(t13) = VALUE i( ).
DATA t14 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t14.
DATA(t15) = CAST z_src_abap_2_s_b6c5f651b292e5( t9 ).
t14 = t15->z_member_token_ed4bfabc8c4cec.
t13 = lines( t14->items ).
t12 = t13.
DATA(t16) = VALUE int8( ).
DATA(t17) = VALUE i( ).
DATA(t18) = CAST z_src_abap_2_s_b6c5f651b292e5( t9 ).
t17 = t18->z_member_token_62f41b3bcb0bad.
t16 = t17.
t11 = t12 - t16.
IF t11 < range_int8_bound_min OR t11 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t8 = t11.
DATA(t19) = CONV int8( 0 ).
t7 = xsdbool( t8 <> t19 ).
t6 = t7.
IF t7 = abap_true.
DATA(t20) = VALUE abap_bool( ).
DATA t21 TYPE REF TO z_runtime_clas_a39833db329515.
CLEAR t21.
DATA t22 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t22.
DATA t23 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t23.
DATA(t24) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t23 = t24.
DATA t25 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t25.
DATA t26 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t26.
DATA(t27) = CAST z_src_abap_2_s_b6c5f651b292e5( t23 ).
t26 = t27->z_member_token_ed4bfabc8c4cec.
DATA(t28) = VALUE i( ).
DATA(t29) = CAST z_src_abap_2_s_b6c5f651b292e5( t23 ).
t28 = t29->z_member_token_62f41b3bcb0bad.
t28 = t28 + 1.
DATA t30 TYPE REF TO object.
CLEAR t30.
READ TABLE t26->items INDEX t28 INTO t30.
t25 ?= t30.
t22 = t25.
t21 = t22->z_builtin_clas_3f891b097658ff( ).
DATA(t31) = CAST z_runtime_clas_a39833db329515( me->z_member_token_9f21cad060fb60 ).
t20 = xsdbool( t21 = t31 ).
t6 = t20.
ENDIF.
IF t6 = abap_true.
DATA(t32) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t33 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t33.
DATA t34 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t34.
DATA(t35) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t34 = t35.
DATA t36 TYPE REF TO z_union_667199_4cf1d0543504ab.
CLEAR t36.
DATA t37 TYPE REF TO z_src_abap_nod_935d26003cb490.
CLEAR t37.
DATA t38 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t38.
DATA t39 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t39.
DATA(t40) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t39 = t40.
DATA t41 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t41.
DATA t42 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t42.
DATA(t43) = CAST z_src_abap_2_s_b6c5f651b292e5( t39 ).
t42 = t43->z_member_token_ed4bfabc8c4cec.
DATA(t44) = VALUE i( ).
DATA(t45) = CAST z_src_abap_2_s_b6c5f651b292e5( t39 ).
t44 = t45->z_member_token_62f41b3bcb0bad.
t44 = t44 + 1.
DATA t46 TYPE REF TO object.
CLEAR t46.
READ TABLE t42->items INDEX t44 INTO t46.
t41 ?= t46.
t38 = t41.
t37 = NEW z_src_abap_nod_935d26003cb490( z_param_token_06f9e7a29ec34c = t38 ).
t36 = t37.
DATA t47 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t47.
DATA t48 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t48.
DATA(t49) = CAST z_src_abap_2_s_b6c5f651b292e5( t34 ).
t48 = t49->z_member_token_ed4bfabc8c4cec.
DATA(t50) = VALUE int8( ).
DATA(t51) = VALUE int8( ).
DATA(t52) = VALUE i( ).
DATA(t53) = CAST z_src_abap_2_s_b6c5f651b292e5( t34 ).
t52 = t53->z_member_token_62f41b3bcb0bad.
t51 = t52.
DATA(t54) = CONV int8( 1 ).
t50 = t51 + t54.
IF t50 < range_int8_bound_min OR t50 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
DATA t55 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t55.
DATA(t56) = CONV abap_bool( abap_true ).
IF t56 = abap_true.
DATA t57 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t57.
DATA(t58) = CAST z_union_667199_4cf1d0543504ab( t36 ).
DATA t59 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t59.
DATA(t60) = CAST z_src_abap_2_s_b6c5f651b292e5( t34 ).
t59 = t60->z_member_nodes_022617c0b510ec.
t57 = NEW z_src_abap_2_s_c435ad477cf33f( z_param_node_1324e81b8c7326 = t58 z_param_previo_4981fbd290610f = t59 ).
t55 = t57.
ELSE.
DATA t61 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t61.
t55 = t61.
ENDIF.
DATA(t62) = VALUE int8( ).
DATA(t63) = VALUE int8( ).
DATA(t64) = CAST z_src_abap_2_s_b6c5f651b292e5( t34 ).
t63 = t64->z_member_nodec_32889faa6e522a.
DATA(t65) = CONV int8( 1 ).
t62 = t63 + t65.
IF t62 < range_int8_bound_min OR t62 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t47 = z_src_abap_2_s_b6c5f651b292e5=>z_member_fromc_9d7b43515b1343( z_param_tokens_3a66ced5b5b26a = t48 z_param_tokeni_36bd2fa45d0f58 = t50 z_param_nodes_51988edc19892d = t55 z_param_nodeco_6f0a19763cdb2a = t62 ).
t33 = t47.
APPEND t33 TO t32->items.
ENDIF.
DATA(t66) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t66.
RETURN.
ENDMETHOD.
METHOD z_runtime_desc_e5fe811e0f4259~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_builtin_clas_3f891b097658ff.
result = z_runtime_desc_78196487750d0f=>z_member_d61_558ec3353ed9dc.
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
