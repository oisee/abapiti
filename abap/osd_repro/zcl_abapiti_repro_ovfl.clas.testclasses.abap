* Item 3: TYPE i overflow must raise, as on the kernel. WASM i32 wraps
* instead (factorial(13) = 1932053504 in wazero, 6227020800 does not fit
* in i), so abapiti has to emit the wrap itself; it cannot rely on OSD.
CLASS ltcl_ovfl DEFINITION FINAL FOR TESTING
  DURATION SHORT RISK LEVEL HARMLESS.
  PRIVATE SECTION.
    METHODS add_overflow FOR TESTING.
    METHODS sub_overflow FOR TESTING.
    METHODS mul_overflow FOR TESTING.
    METHODS f_to_i_overflow FOR TESTING.
    METHODS p_to_i_overflow FOR TESTING.
    METHODS boundaries_fit FOR TESTING.
ENDCLASS.

CLASS ltcl_ovfl IMPLEMENTATION.

  METHOD add_overflow.
    DATA lv_i TYPE i.
    DATA lv_raised TYPE abap_bool.
    lv_i = 2147483647.
    TRY.
        lv_i = lv_i + 1.
      CATCH cx_sy_arithmetic_overflow.
        lv_raised = abap_true.
    ENDTRY.
    cl_abap_unit_assert=>assert_equals( act = lv_raised exp = abap_true ).
  ENDMETHOD.

  METHOD sub_overflow.
    DATA lv_i TYPE i.
    DATA lv_raised TYPE abap_bool.
    lv_i = -2147483648.
    TRY.
        lv_i = lv_i - 1.
      CATCH cx_sy_arithmetic_overflow.
        lv_raised = abap_true.
    ENDTRY.
    cl_abap_unit_assert=>assert_equals( act = lv_raised exp = abap_true ).
  ENDMETHOD.

  METHOD mul_overflow.
* factorial(13) step: 479001600 * 13.
    DATA lv_i TYPE i.
    DATA lv_raised TYPE abap_bool.
    lv_i = 479001600.
    TRY.
        lv_i = lv_i * 13.
      CATCH cx_sy_arithmetic_overflow.
        lv_raised = abap_true.
    ENDTRY.
    cl_abap_unit_assert=>assert_equals( act = lv_raised exp = abap_true ).
  ENDMETHOD.

  METHOD f_to_i_overflow.
    DATA lv_f TYPE f.
    DATA lv_i TYPE i.
    DATA lv_raised TYPE abap_bool.
    lv_f = '3000000000'.
    TRY.
        lv_i = lv_f.
      CATCH cx_sy_conversion_overflow.
        lv_raised = abap_true.
    ENDTRY.
    cl_abap_unit_assert=>assert_equals( act = lv_raised exp = abap_true ).
  ENDMETHOD.

  METHOD p_to_i_overflow.
    DATA lv_p TYPE p LENGTH 16 DECIMALS 0.
    DATA lv_i TYPE i.
    DATA lv_raised TYPE abap_bool.
    lv_p = '6227020800'.
    TRY.
        lv_i = lv_p.
      CATCH cx_sy_conversion_overflow.
        lv_raised = abap_true.
    ENDTRY.
    cl_abap_unit_assert=>assert_equals( act = lv_raised exp = abap_true ).
  ENDMETHOD.

  METHOD boundaries_fit.
* No false alarms at the edges. factorial(12) = 479001600 fits.
    DATA lv_i TYPE i.
    lv_i = 2147483646.
    lv_i = lv_i + 1.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 2147483647 ).
    lv_i = 39916800.
    lv_i = lv_i * 12.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 479001600 ).
  ENDMETHOD.

ENDCLASS.
