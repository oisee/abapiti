CLASS z_src_abap_2_s_da316f761b216a DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
INTERFACES z_src_abap_2_s_e1e980746d06e1.
INTERFACES z_union_b0cca5_ee926be4fb96e6.
INTERFACES z_union_a89e3b_daf022d4653557.
DATA z_member_regex_5ebdfeea2030b9 TYPE REF TO z_runtime_rege_5a210f82384ebf.
METHODS constructor IMPORTING z_param_r_f634cd02b53efc TYPE REF TO z_runtime_rege_5a210f82384ebf.
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
CLASS z_src_abap_2_s_da316f761b216a IMPLEMENTATION.
METHOD constructor.
DATA(t1) = CAST z_runtime_rege_5a210f82384ebf( z_param_r_f634cd02b53efc ).
DATA(t2) = CAST z_runtime_rege_5a210f82384ebf( t1 ).
me->z_member_regex_5ebdfeea2030b9 = t2.
ENDMETHOD.
METHOD z_member_listk_af22c428e8602b.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:18| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:22| ).
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
DATA(t8) = VALUE int8( ).
DATA t9 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t9.
DATA(t10) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
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
t7 = xsdbool( t8 = t19 ).
IF t7 = abap_true.
CONTINUE.
ENDIF.
DATA t20 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t20.
DATA t21 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t21.
DATA t22 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t22.
DATA(t23) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t22 = t23.
DATA t24 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t24.
DATA t25 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t25.
DATA(t26) = CAST z_src_abap_2_s_b6c5f651b292e5( t22 ).
t25 = t26->z_member_token_ed4bfabc8c4cec.
DATA(t27) = VALUE i( ).
DATA(t28) = CAST z_src_abap_2_s_b6c5f651b292e5( t22 ).
t27 = t28->z_member_token_62f41b3bcb0bad.
t27 = t27 + 1.
DATA t29 TYPE REF TO object.
CLEAR t29.
READ TABLE t25->items INDEX t27 INTO t29.
t24 ?= t29.
t21 = t24.
t20 = t21.
DATA(t30) = VALUE abap_bool( ).
DATA(t31) = VALUE abap_bool( ).
DATA(t32) = CAST z_runtime_rege_5a210f82384ebf( me->z_member_regex_5ebdfeea2030b9 ).
DATA(t33) = VALUE string( ).
DATA t34 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t34.
DATA(t35) = CAST z_src_abap_1_l_506101b34e5cc0( t20 ).
t34 = t35.
DATA(t36) = VALUE string( ).
DATA(t37) = CAST z_src_abap_1_l_506101b34e5cc0( t34 ).
t36 = t37->z_member_str_36e44b82e4af28.
t33 = t36.
t31 = t32->test( p0 = t33 ).
DATA(t38) = CONV abap_bool( abap_true ).
t30 = xsdbool( t31 = t38 ).
IF t30 = abap_true.
DATA(t39) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t40 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t40.
DATA t41 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t41.
DATA(t42) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t41 = t42.
DATA t43 TYPE REF TO z_union_667199_4cf1d0543504ab.
CLEAR t43.
DATA t44 TYPE REF TO z_src_abap_nod_bf2dba3016e34c.
CLEAR t44.
DATA(t45) = CAST z_src_abap_1_l_506101b34e5cc0( t20 ).
t44 = NEW z_src_abap_nod_bf2dba3016e34c( z_param_token_06f9e7a29ec34c = t45 ).
t43 = t44.
DATA t46 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t46.
DATA t47 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t47.
DATA(t48) = CAST z_src_abap_2_s_b6c5f651b292e5( t41 ).
t47 = t48->z_member_token_ed4bfabc8c4cec.
DATA(t49) = VALUE int8( ).
DATA(t50) = VALUE int8( ).
DATA(t51) = VALUE i( ).
DATA(t52) = CAST z_src_abap_2_s_b6c5f651b292e5( t41 ).
t51 = t52->z_member_token_62f41b3bcb0bad.
t50 = t51.
DATA(t53) = CONV int8( 1 ).
t49 = t50 + t53.
IF t49 < range_int8_bound_min OR t49 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
DATA t54 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t54.
DATA(t55) = CONV abap_bool( abap_true ).
IF t55 = abap_true.
DATA t56 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t56.
DATA(t57) = CAST z_union_667199_4cf1d0543504ab( t43 ).
DATA t58 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t58.
DATA(t59) = CAST z_src_abap_2_s_b6c5f651b292e5( t41 ).
t58 = t59->z_member_nodes_022617c0b510ec.
t56 = NEW z_src_abap_2_s_c435ad477cf33f( z_param_node_1324e81b8c7326 = t57 z_param_previo_4981fbd290610f = t58 ).
t54 = t56.
ELSE.
DATA t60 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t60.
t54 = t60.
ENDIF.
DATA(t61) = VALUE int8( ).
DATA(t62) = VALUE int8( ).
DATA(t63) = CAST z_src_abap_2_s_b6c5f651b292e5( t41 ).
t62 = t63->z_member_nodec_32889faa6e522a.
DATA(t64) = CONV int8( 1 ).
t61 = t62 + t64.
IF t61 < range_int8_bound_min OR t61 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t46 = z_src_abap_2_s_b6c5f651b292e5=>z_member_fromc_9d7b43515b1343( z_param_tokens_3a66ced5b5b26a = t47 z_param_tokeni_36bd2fa45d0f58 = t49 z_param_nodes_51988edc19892d = t54 z_param_nodeco_6f0a19763cdb2a = t61 ).
t40 = t46.
APPEND t40 TO t39->items.
ENDIF.
ENDLOOP.
DATA(t65) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t65.
RETURN.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:42| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:46| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:50| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
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
result = z_runtime_desc_78196487750d0f=>z_member_d59_2e4f9aa68d37c4.
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
