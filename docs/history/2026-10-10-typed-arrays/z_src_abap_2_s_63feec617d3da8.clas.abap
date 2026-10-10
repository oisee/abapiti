CLASS z_src_abap_2_s_63feec617d3da8 DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
INTERFACES z_src_abap_2_s_e1e980746d06e1.
INTERFACES z_union_b0cca5_ee926be4fb96e6.
INTERFACES z_union_a89e3b_daf022d4653557.
DATA z_member_s_f3835518d3428a TYPE string.
METHODS constructor IMPORTING z_param_s_5c4d15ea64c11b TYPE string.
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
CLASS z_src_abap_2_s_63feec617d3da8 IMPLEMENTATION.
METHOD constructor.
DATA(t1) = CONV string( z_param_s_5c4d15ea64c11b ).
DATA(t2) = VALUE string( ).
DATA(t3) = CONV string( t1 ).
DATA(t4) = CONV i( strlen( t3 ) ).
t2 = t3.
REPLACE ALL OCCURRENCES OF `ß` IN t2 WITH `SS`.
REPLACE ALL OCCURRENCES OF `ŉ` IN t2 WITH `ʼN`.
REPLACE ALL OCCURRENCES OF `ǰ` IN t2 WITH `J̌`.
REPLACE ALL OCCURRENCES OF `ΐ` IN t2 WITH `Ϊ́`.
REPLACE ALL OCCURRENCES OF `ΰ` IN t2 WITH `Ϋ́`.
REPLACE ALL OCCURRENCES OF `և` IN t2 WITH `ԵՒ`.
REPLACE ALL OCCURRENCES OF `ẖ` IN t2 WITH `H̱`.
REPLACE ALL OCCURRENCES OF `ẗ` IN t2 WITH `T̈`.
REPLACE ALL OCCURRENCES OF `ẘ` IN t2 WITH `W̊`.
REPLACE ALL OCCURRENCES OF `ẙ` IN t2 WITH `Y̊`.
REPLACE ALL OCCURRENCES OF `ẚ` IN t2 WITH `Aʾ`.
REPLACE ALL OCCURRENCES OF `ὐ` IN t2 WITH `Υ̓`.
REPLACE ALL OCCURRENCES OF `ὒ` IN t2 WITH `Υ̓̀`.
REPLACE ALL OCCURRENCES OF `ὔ` IN t2 WITH `Υ̓́`.
REPLACE ALL OCCURRENCES OF `ὖ` IN t2 WITH `Υ̓͂`.
REPLACE ALL OCCURRENCES OF `ᾀ` IN t2 WITH `ἈΙ`.
REPLACE ALL OCCURRENCES OF `ᾁ` IN t2 WITH `ἉΙ`.
REPLACE ALL OCCURRENCES OF `ᾂ` IN t2 WITH `ἊΙ`.
REPLACE ALL OCCURRENCES OF `ᾃ` IN t2 WITH `ἋΙ`.
REPLACE ALL OCCURRENCES OF `ᾄ` IN t2 WITH `ἌΙ`.
REPLACE ALL OCCURRENCES OF `ᾅ` IN t2 WITH `ἍΙ`.
REPLACE ALL OCCURRENCES OF `ᾆ` IN t2 WITH `ἎΙ`.
REPLACE ALL OCCURRENCES OF `ᾇ` IN t2 WITH `ἏΙ`.
REPLACE ALL OCCURRENCES OF `ᾈ` IN t2 WITH `ἈΙ`.
REPLACE ALL OCCURRENCES OF `ᾉ` IN t2 WITH `ἉΙ`.
REPLACE ALL OCCURRENCES OF `ᾊ` IN t2 WITH `ἊΙ`.
REPLACE ALL OCCURRENCES OF `ᾋ` IN t2 WITH `ἋΙ`.
REPLACE ALL OCCURRENCES OF `ᾌ` IN t2 WITH `ἌΙ`.
REPLACE ALL OCCURRENCES OF `ᾍ` IN t2 WITH `ἍΙ`.
REPLACE ALL OCCURRENCES OF `ᾎ` IN t2 WITH `ἎΙ`.
REPLACE ALL OCCURRENCES OF `ᾏ` IN t2 WITH `ἏΙ`.
REPLACE ALL OCCURRENCES OF `ᾐ` IN t2 WITH `ἨΙ`.
REPLACE ALL OCCURRENCES OF `ᾑ` IN t2 WITH `ἩΙ`.
REPLACE ALL OCCURRENCES OF `ᾒ` IN t2 WITH `ἪΙ`.
REPLACE ALL OCCURRENCES OF `ᾓ` IN t2 WITH `ἫΙ`.
REPLACE ALL OCCURRENCES OF `ᾔ` IN t2 WITH `ἬΙ`.
REPLACE ALL OCCURRENCES OF `ᾕ` IN t2 WITH `ἭΙ`.
REPLACE ALL OCCURRENCES OF `ᾖ` IN t2 WITH `ἮΙ`.
REPLACE ALL OCCURRENCES OF `ᾗ` IN t2 WITH `ἯΙ`.
REPLACE ALL OCCURRENCES OF `ᾘ` IN t2 WITH `ἨΙ`.
REPLACE ALL OCCURRENCES OF `ᾙ` IN t2 WITH `ἩΙ`.
REPLACE ALL OCCURRENCES OF `ᾚ` IN t2 WITH `ἪΙ`.
REPLACE ALL OCCURRENCES OF `ᾛ` IN t2 WITH `ἫΙ`.
REPLACE ALL OCCURRENCES OF `ᾜ` IN t2 WITH `ἬΙ`.
REPLACE ALL OCCURRENCES OF `ᾝ` IN t2 WITH `ἭΙ`.
REPLACE ALL OCCURRENCES OF `ᾞ` IN t2 WITH `ἮΙ`.
REPLACE ALL OCCURRENCES OF `ᾟ` IN t2 WITH `ἯΙ`.
REPLACE ALL OCCURRENCES OF `ᾠ` IN t2 WITH `ὨΙ`.
REPLACE ALL OCCURRENCES OF `ᾡ` IN t2 WITH `ὩΙ`.
REPLACE ALL OCCURRENCES OF `ᾢ` IN t2 WITH `ὪΙ`.
REPLACE ALL OCCURRENCES OF `ᾣ` IN t2 WITH `ὫΙ`.
REPLACE ALL OCCURRENCES OF `ᾤ` IN t2 WITH `ὬΙ`.
REPLACE ALL OCCURRENCES OF `ᾥ` IN t2 WITH `ὭΙ`.
REPLACE ALL OCCURRENCES OF `ᾦ` IN t2 WITH `ὮΙ`.
REPLACE ALL OCCURRENCES OF `ᾧ` IN t2 WITH `ὯΙ`.
REPLACE ALL OCCURRENCES OF `ᾨ` IN t2 WITH `ὨΙ`.
REPLACE ALL OCCURRENCES OF `ᾩ` IN t2 WITH `ὩΙ`.
REPLACE ALL OCCURRENCES OF `ᾪ` IN t2 WITH `ὪΙ`.
REPLACE ALL OCCURRENCES OF `ᾫ` IN t2 WITH `ὫΙ`.
REPLACE ALL OCCURRENCES OF `ᾬ` IN t2 WITH `ὬΙ`.
REPLACE ALL OCCURRENCES OF `ᾭ` IN t2 WITH `ὭΙ`.
REPLACE ALL OCCURRENCES OF `ᾮ` IN t2 WITH `ὮΙ`.
REPLACE ALL OCCURRENCES OF `ᾯ` IN t2 WITH `ὯΙ`.
REPLACE ALL OCCURRENCES OF `ᾲ` IN t2 WITH `ᾺΙ`.
REPLACE ALL OCCURRENCES OF `ᾳ` IN t2 WITH `ΑΙ`.
REPLACE ALL OCCURRENCES OF `ᾴ` IN t2 WITH `ΆΙ`.
REPLACE ALL OCCURRENCES OF `ᾶ` IN t2 WITH `Α͂`.
REPLACE ALL OCCURRENCES OF `ᾷ` IN t2 WITH `Α͂Ι`.
REPLACE ALL OCCURRENCES OF `ᾼ` IN t2 WITH `ΑΙ`.
REPLACE ALL OCCURRENCES OF `ῂ` IN t2 WITH `ῊΙ`.
REPLACE ALL OCCURRENCES OF `ῃ` IN t2 WITH `ΗΙ`.
REPLACE ALL OCCURRENCES OF `ῄ` IN t2 WITH `ΉΙ`.
REPLACE ALL OCCURRENCES OF `ῆ` IN t2 WITH `Η͂`.
REPLACE ALL OCCURRENCES OF `ῇ` IN t2 WITH `Η͂Ι`.
REPLACE ALL OCCURRENCES OF `ῌ` IN t2 WITH `ΗΙ`.
REPLACE ALL OCCURRENCES OF `ῒ` IN t2 WITH `Ϊ̀`.
REPLACE ALL OCCURRENCES OF `ΐ` IN t2 WITH `Ϊ́`.
REPLACE ALL OCCURRENCES OF `ῖ` IN t2 WITH `Ι͂`.
REPLACE ALL OCCURRENCES OF `ῗ` IN t2 WITH `Ϊ͂`.
REPLACE ALL OCCURRENCES OF `ῢ` IN t2 WITH `Ϋ̀`.
REPLACE ALL OCCURRENCES OF `ΰ` IN t2 WITH `Ϋ́`.
REPLACE ALL OCCURRENCES OF `ῤ` IN t2 WITH `Ρ̓`.
REPLACE ALL OCCURRENCES OF `ῦ` IN t2 WITH `Υ͂`.
REPLACE ALL OCCURRENCES OF `ῧ` IN t2 WITH `Ϋ͂`.
REPLACE ALL OCCURRENCES OF `ῲ` IN t2 WITH `ῺΙ`.
REPLACE ALL OCCURRENCES OF `ῳ` IN t2 WITH `ΩΙ`.
REPLACE ALL OCCURRENCES OF `ῴ` IN t2 WITH `ΏΙ`.
REPLACE ALL OCCURRENCES OF `ῶ` IN t2 WITH `Ω͂`.
REPLACE ALL OCCURRENCES OF `ῷ` IN t2 WITH `Ω͂Ι`.
REPLACE ALL OCCURRENCES OF `ῼ` IN t2 WITH `ΩΙ`.
REPLACE ALL OCCURRENCES OF `ﬀ` IN t2 WITH `FF`.
REPLACE ALL OCCURRENCES OF `ﬁ` IN t2 WITH `FI`.
REPLACE ALL OCCURRENCES OF `ﬂ` IN t2 WITH `FL`.
REPLACE ALL OCCURRENCES OF `ﬃ` IN t2 WITH `FFI`.
REPLACE ALL OCCURRENCES OF `ﬄ` IN t2 WITH `FFL`.
REPLACE ALL OCCURRENCES OF `ﬅ` IN t2 WITH `ST`.
REPLACE ALL OCCURRENCES OF `ﬆ` IN t2 WITH `ST`.
REPLACE ALL OCCURRENCES OF `ﬓ` IN t2 WITH `ՄՆ`.
REPLACE ALL OCCURRENCES OF `ﬔ` IN t2 WITH `ՄԵ`.
REPLACE ALL OCCURRENCES OF `ﬕ` IN t2 WITH `ՄԻ`.
REPLACE ALL OCCURRENCES OF `ﬖ` IN t2 WITH `ՎՆ`.
REPLACE ALL OCCURRENCES OF `ﬗ` IN t2 WITH `ՄԽ`.
TRANSLATE t2 TO UPPER CASE.
me->z_member_s_f3835518d3428a = t2.
ENDMETHOD.
METHOD z_member_listk_af22c428e8602b.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:63| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_getus_a169a267c2678f.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:67| ).
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
DATA(t22) = VALUE string( ).
DATA t23 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t23.
DATA t24 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t24.
DATA t25 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t25.
DATA(t26) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t25 = t26.
DATA t27 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t27.
DATA t28 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t28.
DATA(t29) = CAST z_src_abap_2_s_b6c5f651b292e5( t25 ).
t28 = t29->z_member_token_ed4bfabc8c4cec.
DATA(t30) = VALUE i( ).
DATA(t31) = CAST z_src_abap_2_s_b6c5f651b292e5( t25 ).
t30 = t31->z_member_token_62f41b3bcb0bad.
t30 = t30 + 1.
DATA t32 TYPE REF TO object.
CLEAR t32.
READ TABLE t28->items INDEX t30 INTO t32.
t27 ?= t32.
t24 = t27.
t23 = t24.
DATA(t33) = VALUE string( ).
DATA(t34) = CAST z_src_abap_1_l_506101b34e5cc0( t23 ).
t33 = t34->z_member_strup_d4c097dcba87bc.
t22 = t33.
DATA(t35) = CONV string( me->z_member_s_f3835518d3428a ).
t21 = xsdbool( t22 = t35 ).
t7 = t21.
ENDIF.
IF t7 = abap_true.
DATA(t36) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t37 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t37.
DATA t38 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t38.
DATA(t39) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t38 = t39.
DATA t40 TYPE REF TO z_union_667199_4cf1d0543504ab.
CLEAR t40.
DATA t41 TYPE REF TO z_src_abap_nod_935d26003cb490.
CLEAR t41.
DATA t42 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t42.
DATA t43 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t43.
DATA(t44) = CAST z_src_abap_2_s_b6c5f651b292e5( t5 ).
t43 = t44.
DATA t45 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t45.
DATA t46 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t46.
DATA(t47) = CAST z_src_abap_2_s_b6c5f651b292e5( t43 ).
t46 = t47->z_member_token_ed4bfabc8c4cec.
DATA(t48) = VALUE i( ).
DATA(t49) = CAST z_src_abap_2_s_b6c5f651b292e5( t43 ).
t48 = t49->z_member_token_62f41b3bcb0bad.
t48 = t48 + 1.
DATA t50 TYPE REF TO object.
CLEAR t50.
READ TABLE t46->items INDEX t48 INTO t50.
t45 ?= t50.
t42 = t45.
t41 = NEW z_src_abap_nod_935d26003cb490( z_param_token_06f9e7a29ec34c = t42 ).
t40 = t41.
DATA t51 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t51.
DATA t52 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t52.
DATA(t53) = CAST z_src_abap_2_s_b6c5f651b292e5( t38 ).
t52 = t53->z_member_token_ed4bfabc8c4cec.
DATA(t54) = VALUE int8( ).
DATA(t55) = VALUE int8( ).
DATA(t56) = VALUE i( ).
DATA(t57) = CAST z_src_abap_2_s_b6c5f651b292e5( t38 ).
t56 = t57->z_member_token_62f41b3bcb0bad.
t55 = t56.
DATA(t58) = CONV int8( 1 ).
t54 = t55 + t58.
IF t54 < range_int8_bound_min OR t54 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
DATA t59 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t59.
DATA(t60) = CONV abap_bool( abap_true ).
IF t60 = abap_true.
DATA t61 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t61.
DATA(t62) = CAST z_union_667199_4cf1d0543504ab( t40 ).
DATA t63 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t63.
DATA(t64) = CAST z_src_abap_2_s_b6c5f651b292e5( t38 ).
t63 = t64->z_member_nodes_022617c0b510ec.
t61 = NEW z_src_abap_2_s_c435ad477cf33f( z_param_node_1324e81b8c7326 = t62 z_param_previo_4981fbd290610f = t63 ).
t59 = t61.
ELSE.
DATA t65 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t65.
t59 = t65.
ENDIF.
DATA(t66) = VALUE int8( ).
DATA(t67) = VALUE int8( ).
DATA(t68) = CAST z_src_abap_2_s_b6c5f651b292e5( t38 ).
t67 = t68->z_member_nodec_32889faa6e522a.
DATA(t69) = CONV int8( 1 ).
t66 = t67 + t69.
IF t66 < range_int8_bound_min OR t66 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t51 = z_src_abap_2_s_b6c5f651b292e5=>z_member_fromc_9d7b43515b1343( z_param_tokens_3a66ced5b5b26a = t52 z_param_tokeni_36bd2fa45d0f58 = t54 z_param_nodes_51988edc19892d = t59 z_param_nodeco_6f0a19763cdb2a = t66 ).
t37 = t51.
APPEND t37 TO t36->items.
ENDIF.
ENDLOOP.
DATA(t70) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t70.
RETURN.
ENDMETHOD.
METHOD z_member_railr_af4a583f62a6dc.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:84| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_tostr_36e91adbba659c.
DATA(t1) = NEW z_exception_un_5f2d02892fe8b8( ).
DATA(t2) = CONV string( |src/abap/2_statements/combi.ts:88| ).
t1->source_location = t2.
RAISE EXCEPTION t1.
ENDMETHOD.
METHOD z_member_first_8d87896c21eb17.
DATA t1 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t1.
DATA t2 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t2.
DATA t3 TYPE REF TO z_runtime_arra_8ecd6afe19e0a5.
CLEAR t3.
CREATE OBJECT t3.
t2 = t3.
DATA(t4) = CAST z_runtime_arra_8ecd6afe19e0a5( t2 ).
DATA(t5) = CONV string( me->z_member_s_f3835518d3428a ).
APPEND t5 TO t4->items.
DATA(t6) = CAST z_runtime_arra_8ecd6afe19e0a5( t2 ).
t1 = t6.
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
DATA(t21) = VALUE string( ).
DATA t22 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t22.
DATA t23 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t23.
DATA t24 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t24.
DATA(t25) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
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
t22 = t23.
DATA(t32) = VALUE string( ).
DATA(t33) = CAST z_src_abap_1_l_506101b34e5cc0( t22 ).
t32 = t33->z_member_strup_d4c097dcba87bc.
t21 = t32.
DATA(t34) = CONV string( me->z_member_s_f3835518d3428a ).
t20 = xsdbool( t21 = t34 ).
t6 = t20.
ENDIF.
IF t6 = abap_true.
DATA(t35) = CAST z_runtime_arra_c22708faf2e354( t2 ).
DATA t36 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t36.
DATA t37 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t37.
DATA(t38) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t37 = t38.
DATA t39 TYPE REF TO z_union_667199_4cf1d0543504ab.
CLEAR t39.
DATA t40 TYPE REF TO z_src_abap_nod_935d26003cb490.
CLEAR t40.
DATA t41 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t41.
DATA t42 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t42.
DATA(t43) = CAST z_src_abap_2_s_b6c5f651b292e5( t4 ).
t42 = t43.
DATA t44 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
CLEAR t44.
DATA t45 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t45.
DATA(t46) = CAST z_src_abap_2_s_b6c5f651b292e5( t42 ).
t45 = t46->z_member_token_ed4bfabc8c4cec.
DATA(t47) = VALUE i( ).
DATA(t48) = CAST z_src_abap_2_s_b6c5f651b292e5( t42 ).
t47 = t48->z_member_token_62f41b3bcb0bad.
t47 = t47 + 1.
DATA t49 TYPE REF TO object.
CLEAR t49.
READ TABLE t45->items INDEX t47 INTO t49.
t44 ?= t49.
t41 = t44.
t40 = NEW z_src_abap_nod_935d26003cb490( z_param_token_06f9e7a29ec34c = t41 ).
t39 = t40.
DATA t50 TYPE REF TO z_src_abap_2_s_b6c5f651b292e5.
CLEAR t50.
DATA t51 TYPE REF TO z_runtime_arra_c22708faf2e354.
CLEAR t51.
DATA(t52) = CAST z_src_abap_2_s_b6c5f651b292e5( t37 ).
t51 = t52->z_member_token_ed4bfabc8c4cec.
DATA(t53) = VALUE int8( ).
DATA(t54) = VALUE int8( ).
DATA(t55) = VALUE i( ).
DATA(t56) = CAST z_src_abap_2_s_b6c5f651b292e5( t37 ).
t55 = t56->z_member_token_62f41b3bcb0bad.
t54 = t55.
DATA(t57) = CONV int8( 1 ).
t53 = t54 + t57.
IF t53 < range_int8_bound_min OR t53 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
DATA t58 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t58.
DATA(t59) = CONV abap_bool( abap_true ).
IF t59 = abap_true.
DATA t60 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t60.
DATA(t61) = CAST z_union_667199_4cf1d0543504ab( t39 ).
DATA t62 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t62.
DATA(t63) = CAST z_src_abap_2_s_b6c5f651b292e5( t37 ).
t62 = t63->z_member_nodes_022617c0b510ec.
t60 = NEW z_src_abap_2_s_c435ad477cf33f( z_param_node_1324e81b8c7326 = t61 z_param_previo_4981fbd290610f = t62 ).
t58 = t60.
ELSE.
DATA t64 TYPE REF TO z_src_abap_2_s_c435ad477cf33f.
CLEAR t64.
t58 = t64.
ENDIF.
DATA(t65) = VALUE int8( ).
DATA(t66) = VALUE int8( ).
DATA(t67) = CAST z_src_abap_2_s_b6c5f651b292e5( t37 ).
t66 = t67->z_member_nodec_32889faa6e522a.
DATA(t68) = CONV int8( 1 ).
t65 = t66 + t68.
IF t65 < range_int8_bound_min OR t65 > range_int8_bound_max.
RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.
ENDIF.
t50 = z_src_abap_2_s_b6c5f651b292e5=>z_member_fromc_9d7b43515b1343( z_param_tokens_3a66ced5b5b26a = t51 z_param_tokeni_36bd2fa45d0f58 = t53 z_param_nodes_51988edc19892d = t58 z_param_nodeco_6f0a19763cdb2a = t65 ).
t36 = t50.
APPEND t36 TO t35->items.
ENDIF.
DATA(t69) = CAST z_runtime_arra_c22708faf2e354( t2 ).
result = t69.
RETURN.
ENDMETHOD.
METHOD z_runtime_desc_e5fe811e0f4259~z_builtin_clas_3f891b097658ff.
result = me->z_builtin_clas_3f891b097658ff( ).
ENDMETHOD.
METHOD z_builtin_clas_3f891b097658ff.
result = z_runtime_desc_78196487750d0f=>z_member_d60_29c3968778af84.
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
