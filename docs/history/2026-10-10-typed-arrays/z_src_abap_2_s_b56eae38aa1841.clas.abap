CLASS z_src_abap_2_s_b56eae38aa1841 DEFINITION PUBLIC CREATE PUBLIC.
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
CLASS z_src_abap_2_s_b56eae38aa1841 IMPLEMENTATION.
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
DATA(t10) = CONV string( |Sequence, length error| ).
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
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:596| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:604| ).
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
DATA(t4) = CAST z_runtime_arra_c22708faf2e354( t1 ).
DATA t5 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t5.
DATA t6 TYPE REF TO object.
CLEAR t6.
LOOP AT t4->items INTO t6.
t5 ?= t6.
DATA t7 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t8.
DATA t9 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t9.
DATA t10 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t10.
CREATE OBJECT t10.
t9 = t10.
DATA(t11) = CAST z_runtime_arra_c22708faf2e354( t9 ).
DATA(t12) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
APPEND t12 TO t11->items.
DATA(t13) = CAST z_runtime_arra_c22708faf2e354( t9 ).
t8 = t13.
t7 = t8.
DATA(t14) = VALUE abap_bool( ).
DATA(t15) = CONV abap_bool( abap_true ).
t14 = t15.
DATA(t16) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t17 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t17.
DATA t18 TYPE REF TO object.
CLEAR t18.
LOOP AT t16->items INTO t18.
t17 ?= t18.
DATA t19 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t19.
DATA(t20) = CAST z_src_abap_2_s_e1e980746d06e1( t17 ).
DATA(t21) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t19 = t20->z_member_run_71be1bca84374a( z_param_r_f634cd02b53efc = t21 ).
t7 = t19.
DATA(t22) = VALUE abap_bool( ).
DATA(t23) = VALUE i( ).
DATA(t24) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t23 = lines( t24->items ).
DATA(t25) = CONV i( '0' ).
t22 = xsdbool( t23 = t25 ).
IF t22 = abap_true.
DATA(t26) = CONV abap_bool( abap_false ).
t14 = t26.
EXIT.
ENDIF.
ENDLOOP.
DATA(t27) = VALUE abap_bool( ).
DATA(t28) = CONV abap_bool( t14 ).
DATA(t29) = CONV abap_bool( abap_true ).
t27 = xsdbool( t28 = t29 ).
IF t27 = abap_true.
DATA(t30) = VALUE abap_bool( ).
DATA(t31) = VALUE i( ).
DATA(t32) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t31 = lines( t32->items ).
DATA(t33) = CONV i( '1000' ).
t30 = xsdbool( t31 > t33 ).
IF t30 = abap_true.
DATA t34 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t34.
DATA(t35) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t36) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t34 = t35->concat( p0 = t36 ).
t2 = t34.
ELSE.
DATA(t37) = VALUE abap_bool( ).
DATA(t38) = VALUE i( ).
DATA(t39) = CAST z_runtime_arra_c22708faf2e354( t7 ).
t38 = lines( t39->items ).
DATA(t40) = CONV i( '1' ).
t37 = xsdbool( t38 = t40 ).
IF t37 = abap_true.
DATA(t41) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t42 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t42.
DATA(t43) = CAST z_runtime_arra_c22708faf2e354( t7 ).
DATA t44 TYPE REF TO object.
CLEAR t44.
READ TABLE t43->items INDEX 1 INTO t44.
t42 ?= t44.
APPEND t42 TO t41->items.
ELSE.
DATA(t45) = VALUE i( ).
DATA(t46) = CAST z_runtime_arra_c22708faf2e354( t7 ).
DATA t47 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t47.
DATA t48 TYPE REF TO object.
CLEAR t48.
LOOP AT t46->items INTO t48.
t47 ?= t48.
DATA(t49) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t50) = CAST z_src_abap_2_s_b6c5f651b292e5( t47 ).
APPEND t50 TO t49->items.
ENDLOOP.
DATA(t51) = VALUE i( ).
DATA(t52) = CAST z_runtime_arra_c22708faf2e354( t2 ).
t51 = lines( t52->items ).
t45 = t51.
ENDIF.
ENDIF.
ENDIF.
ENDLOOP.
DATA(t53) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t53.
RETURN.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:637| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:642| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA t1 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t1.
DATA t2 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t2.
DATA(t3) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t4 TYPE REF TO object.
CLEAR t4.
READ TABLE t3->items INDEX 1 INTO t4.
t2 ?= t4.
t1 = t2->z_member_first_8d87896c21eb17(  ).
result = t1.
RETURN.
ENDMETHOD.
METHOD z_member_run_o_b86da49c574129.
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
DATA t6 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t6.
DATA t7 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t7.
DATA t8 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t8.
DATA t9 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t9.
CREATE OBJECT t9.
t8 = t9.
DATA(t10) = CAST z_runtime_arra_c22708faf2e354( t8 ).
DATA(t11) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
APPEND t11 TO t10->items.
DATA(t12) = CAST z_runtime_arra_c22708faf2e354( t8 ).
t7 = t12.
t6 = t7.
DATA(t13) = VALUE abap_bool( ).
DATA(t14) = CONV abap_bool( abap_true ).
t13 = t14.
DATA(t15) = CAST z_runtime_arra_c22708faf2e354( me->z_member_list_484fd43dd9f8d0 ).
DATA t16 TYPE REF TO z_src_abap_2_s_e1e980746d06e1.
CLEAR t16.
DATA t17 TYPE REF TO object.
CLEAR t17.
LOOP AT t15->items INTO t17.
t16 ?= t17.
DATA t18 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t18.
DATA(t19) = CAST z_src_abap_2_s_e1e980746d06e1( t16 ).
DATA(t20) = CAST z_runtime_arra_c22708faf2e354( t6 ).
t18 = t19->z_member_run_71be1bca84374a( z_param_r_f634cd02b53efc = t20 ).
t6 = t18.
DATA(t21) = VALUE abap_bool( ).
DATA(t22) = VALUE i( ).
DATA(t23) = CAST z_runtime_arra_c22708faf2e354( t6 ).
t22 = lines( t23->items ).
DATA(t24) = CONV i( '0' ).
t21 = xsdbool( t22 = t24 ).
IF t21 = abap_true.
DATA(t25) = CONV abap_bool( abap_false ).
t13 = t25.
EXIT.
ENDIF.
ENDLOOP.
DATA(t26) = VALUE abap_bool( ).
DATA(t27) = CONV abap_bool( t13 ).
DATA(t28) = CONV abap_bool( abap_true ).
t26 = xsdbool( t27 = t28 ).
IF t26 = abap_true.
DATA(t29) = VALUE abap_bool( ).
DATA(t30) = VALUE i( ).
DATA(t31) = CAST z_runtime_arra_c22708faf2e354( t6 ).
t30 = lines( t31->items ).
DATA(t32) = CONV i( '1000' ).
t29 = xsdbool( t30 > t32 ).
IF t29 = abap_true.
DATA t33 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t33.
DATA(t34) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t35) = CAST z_runtime_arra_c22708faf2e354( t6 ).
t33 = t34->concat( p0 = t35 ).
t2 = t33.
ELSE.
DATA(t36) = VALUE abap_bool( ).
DATA(t37) = VALUE i( ).
DATA(t38) = CAST z_runtime_arra_c22708faf2e354( t6 ).
t37 = lines( t38->items ).
DATA(t39) = CONV i( '1' ).
t36 = xsdbool( t37 = t39 ).
IF t36 = abap_true.
DATA(t40) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t41 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t41.
DATA(t42) = CAST z_runtime_arra_c22708faf2e354( t6 ).
DATA t43 TYPE REF TO object.
CLEAR t43.
READ TABLE t42->items INDEX 1 INTO t43.
t41 ?= t43.
APPEND t41 TO t40->items.
ELSE.
DATA(t44) = VALUE i( ).
DATA(t45) = CAST z_runtime_arra_c22708faf2e354( t6 ).
DATA t46 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t46.
DATA t47 TYPE REF TO object.
CLEAR t47.
LOOP AT t45->items INTO t47.
t46 ?= t47.
DATA(t48) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA(t49) = CAST z_src_abap_2_s_b6c5f651b292e5( t46 ).
APPEND t49 TO t48->items.
ENDLOOP.
DATA(t50) = VALUE i( ).
DATA(t51) = CAST z_runtime_arra_c22708faf2e354( t2 ).
t50 = lines( t51->items ).
t44 = t50.
ENDIF.
ENDIF.
ENDIF.
DATA(t52) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t52.
RETURN.
ENDMETHOD.
METHOD z_runtime_desc_e5fe811e0f4259~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_builtin_clas_3f891b097658ff.
result = z_runtime_desc_78196487750d0f=>z_member_d72_ee86dd67fdab30.
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
