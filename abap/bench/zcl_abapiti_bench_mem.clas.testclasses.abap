* Correctness of the three memory models. Expected checksums are closed
* forms of the workloads (W1 sum k MOD 251, W2 sum 7k+1, W3 sum k+1000);
* fib(20) = 6765. Model C is in its own test class: on a kernel it
* dumps, and a runtime abortion ends the whole test class.
CLASS ltcl_bench DEFINITION FINAL FOR TESTING
  DURATION MEDIUM RISK LEVEL HARMLESS.
  PRIVATE SECTION.
    METHODS check IMPORTING iv_model TYPE c.
    METHODS model_a FOR TESTING.
    METHODS model_b FOR TESTING.
    METHODS b_cross_row_bytes FOR TESTING.
    METHODS fib20 FOR TESTING.
ENDCLASS.

CLASS ltcl_bench IMPLEMENTATION.

  METHOD check.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    DATA lv_sum TYPE zcl_abapiti_bench_mem=>ty_result-checksum.
    CREATE OBJECT lo EXPORTING iv_model = iv_model.
    lv_sum = lo->w1_bytes( 16384 ).
    cl_abap_unit_assert=>assert_equals( act = lv_sum exp = 2041721 msg = 'W1' ).
    CREATE OBJECT lo EXPORTING iv_model = iv_model.
    lv_sum = lo->w2_i32( 4096 ).
    cl_abap_unit_assert=>assert_equals( act = lv_sum exp = 58710016 msg = 'W2' ).
    CREATE OBJECT lo EXPORTING iv_model = iv_model.
    lv_sum = lo->w3_unaligned( 2048 ).
    cl_abap_unit_assert=>assert_equals( act = lv_sum exp = 4144128 msg = 'W3' ).
  ENDMETHOD.

  METHOD model_a.
    check( 'A' ).
  ENDMETHOD.

  METHOD model_b.
    check( 'B' ).
  ENDMETHOD.

  METHOD b_cross_row_bytes.
* An i32 at 4094 spans rows 1 and 2; little-endian bytes.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    CREATE OBJECT lo EXPORTING iv_model = 'B'.
    lo->st_i32( iv_addr = 4094 iv_val = -2 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 4094 ) exp = 254 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 4095 ) exp = 255 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 4096 ) exp = 255 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 4097 ) exp = 255 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_i32( 4094 ) exp = -2 ).
  ENDMETHOD.

  METHOD fib20.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    CREATE OBJECT lo EXPORTING iv_model = 'B'.
    cl_abap_unit_assert=>assert_equals( act = lo->fib( 20 ) exp = 6765 ).
  ENDMETHOD.

ENDCLASS.

* Model C only. EXPECT = A4H: dumps with ASSIGN_BASE_WRONG_ALIGNMENT
* (2026-10-02), so this class is expected to abort on a kernel; it is kept
* separate so the dump does not hide the results of ltcl_bench.
CLASS ltcl_casting DEFINITION FINAL FOR TESTING
  DURATION MEDIUM RISK LEVEL HARMLESS.
  PRIVATE SECTION.
    METHODS model_c FOR TESTING.
    METHODS c_byte_order FOR TESTING.
ENDCLASS.

CLASS ltcl_casting IMPLEMENTATION.

  METHOD model_c.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    DATA lv_sum TYPE zcl_abapiti_bench_mem=>ty_result-checksum.
    CREATE OBJECT lo EXPORTING iv_model = 'C'.
    lv_sum = lo->w2_i32( 4096 ).
    cl_abap_unit_assert=>assert_equals( act = lv_sum exp = 58710016 msg = 'W2' ).
  ENDMETHOD.

  METHOD c_byte_order.
* UNMEASURED: CASTING TYPE i uses the platform byte order.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    CREATE OBJECT lo EXPORTING iv_model = 'C'.
    lo->st_i32( iv_addr = 8 iv_val = 16909060 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 8 ) exp = 4 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 9 ) exp = 3 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 10 ) exp = 2 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( 11 ) exp = 1 ).
  ENDMETHOD.

ENDCLASS.
