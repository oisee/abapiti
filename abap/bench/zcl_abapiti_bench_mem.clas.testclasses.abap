* Correctness of the three memory models. Expected checksums are closed
* forms of the workloads (W1 sum k MOD 251, W2 sum 7k+1, W3 sum k+1000);
* W5 at 1 page n(n+1)/2 = 136; fib(20) = 6765. Byte order is checked
* directly for A and B (a round trip alone would pass with no byte swap).
* Model C has no tests here: on a kernel ASSIGN ... CASTING TYPE i from
* an x row dumps (ASSIGN_BASE_WRONG_ALIGNMENT), which aborts the class.
CLASS ltcl_bench DEFINITION FINAL FOR TESTING
  DURATION MEDIUM RISK LEVEL HARMLESS.
  PRIVATE SECTION.
    METHODS check IMPORTING iv_model TYPE c.
    METHODS model_a FOR TESTING.
    METHODS model_b FOR TESTING.
    METHODS byte_order IMPORTING iv_model TYPE c iv_addr TYPE i.
    METHODS a_byte_order FOR TESTING.
    METHODS b_byte_order FOR TESTING.
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
    CREATE OBJECT lo EXPORTING iv_model = iv_model.
    lv_sum = lo->w5_sweep( ).
    cl_abap_unit_assert=>assert_equals( act = lv_sum exp = 136 msg = 'W5' ).
  ENDMETHOD.

  METHOD byte_order.
* i32 0x01020304 is stored little-endian: bytes 04 03 02 01.
    DATA lo TYPE REF TO zcl_abapiti_bench_mem.
    CREATE OBJECT lo EXPORTING iv_model = iv_model.
    lo->st_i32( iv_addr = iv_addr iv_val = 16909060 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( iv_addr ) exp = 4 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( iv_addr + 1 ) exp = 3 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( iv_addr + 2 ) exp = 2 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_8u( iv_addr + 3 ) exp = 1 ).
    cl_abap_unit_assert=>assert_equals( act = lo->ld_i32( iv_addr ) exp = 16909060 ).
  ENDMETHOD.

  METHOD a_byte_order.
    byte_order( iv_model = 'A' iv_addr = 8 ).
  ENDMETHOD.

  METHOD b_byte_order.
    byte_order( iv_model = 'B' iv_addr = 8 ).
    byte_order( iv_model = 'B' iv_addr = 4093 ).
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
