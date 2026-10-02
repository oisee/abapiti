* Item 4: x <-> i conversion. Kernel: x LENGTH 4 -> i is big-endian
* two's complement (signed); shorter x is padded with 00 on the left
* (unsigned); i -> x keeps the rightmost bytes. UNMEASURED marks lines that
* still need the A4H oracle.
CLASS ltcl_x4i DEFINITION FINAL FOR TESTING
  DURATION SHORT RISK LEVEL HARMLESS.
  PRIVATE SECTION.
    METHODS x4_to_i FOR TESTING.
    METHODS i_to_x4 FOR TESTING.
    METHODS x2_x1_to_i_unsigned FOR TESTING.
    METHODS i_to_x2_x1_truncates FOR TESTING.
    METHODS xstring_offset_read FOR TESTING.
ENDCLASS.

CLASS ltcl_x4i IMPLEMENTATION.

  METHOD x4_to_i.
    DATA lv_x TYPE x LENGTH 4.
    DATA lv_i TYPE i.
    lv_x = '00000001'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 1 ).
    lv_x = '01020304'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 16909060 ).
    lv_x = '7FFFFFFF'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 2147483647 ).
    lv_x = '80000000'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = -2147483648 ).
    lv_x = 'FFFFFFFF'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = -1 ).
    lv_x = 'FFFFFF85'. lv_i = lv_x.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = -123 ).
  ENDMETHOD.

  METHOD i_to_x4.
    DATA lv_x TYPE x LENGTH 4.
    DATA lv_i TYPE i.
    lv_i = 1. lv_x = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = '00000001' ).
    lv_i = 256. lv_x = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = '00000100' ).
    lv_i = -1. lv_x = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = 'FFFFFFFF' ).
    lv_i = -2147483648. lv_x = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = '80000000' ).
    lv_i = 2147483647. lv_x = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = '7FFFFFFF' ).
  ENDMETHOD.

  METHOD x2_x1_to_i_unsigned.
    DATA lv_x2 TYPE x LENGTH 2.
    DATA lv_x1 TYPE x LENGTH 1.
    DATA lv_i TYPE i.
    lv_x2 = 'FFFF'. lv_i = lv_x2.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 65535 ).
    lv_x2 = '8000'. lv_i = lv_x2.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 32768 ).
    lv_x1 = 'FF'. lv_i = lv_x1.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 255 ).
    lv_x1 = '80'. lv_i = lv_x1.
    cl_abap_unit_assert=>assert_equals( act = lv_i exp = 128 ).
  ENDMETHOD.

  METHOD i_to_x2_x1_truncates.
* UNMEASURED: i -> shorter x keeps the rightmost bytes, no exception.
* The generated store8/store16 helpers rely on this (WASM wraps).
    DATA lv_x2 TYPE x LENGTH 2.
    DATA lv_x1 TYPE x LENGTH 1.
    DATA lv_i TYPE i.
    lv_i = 74565. lv_x2 = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x2 exp = '2345' ).
    lv_i = -1. lv_x2 = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x2 exp = 'FFFF' ).
    lv_i = 511. lv_x1 = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x1 exp = 'FF' ).
    lv_i = -128. lv_x1 = lv_i.
    cl_abap_unit_assert=>assert_equals( act = lv_x1 exp = '80' ).
  ENDMETHOD.

  METHOD xstring_offset_read.
* Offset/length READ on an xstring is allowed (only writes are not).
    DATA lv_mem TYPE xstring.
    DATA lv_x TYPE x LENGTH 4.
    DATA lv_addr TYPE i.
    lv_mem = '00112233445566778899'.
    lv_addr = 3.
    lv_x = lv_mem+lv_addr(4).
    cl_abap_unit_assert=>assert_equals( act = lv_x exp = '33445566' ).
  ENDMETHOD.

ENDCLASS.
