CLASS ltcl_overflow DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS assignment FOR TESTING.
METHODS arithmetic FOR TESTING.
METHODS returned FOR TESTING.
METHODS compared FOR TESTING.
ENDCLASS.
CLASS ltcl_overflow IMPLEMENTATION.
METHOD assignment.
DATA obj TYPE REF TO zcl_overflow.
DATA n TYPE i.
DATA trapped TYPE abap_bool.
CREATE OBJECT obj.
TRY.
obj->value = 2147483648.
CATCH cx_root.
trapped = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = trapped exp = abap_true ).
ENDMETHOD.
METHOD arithmetic.
DATA obj TYPE REF TO zcl_overflow.
DATA n TYPE i.
DATA trapped TYPE abap_bool.
CREATE OBJECT obj.
TRY.
obj->value = 2147483647 + 1.
CATCH cx_root.
trapped = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = trapped exp = abap_true ).
ENDMETHOD.
METHOD returned.
DATA obj TYPE REF TO zcl_overflow.
DATA n TYPE i.
DATA trapped TYPE abap_bool.
CREATE OBJECT obj.
TRY.
n = obj->run( ).
CATCH cx_root.
trapped = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = trapped exp = abap_true ).
ENDMETHOD.
METHOD compared.
DATA obj TYPE REF TO zcl_overflow.
DATA n TYPE i.
DATA trapped TYPE abap_bool.
CREATE OBJECT obj.
TRY.
IF 2147483647 + 1 < 0.
 n = 1.
ENDIF.
CATCH cx_root.
trapped = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = trapped exp = abap_true ).
ENDMETHOD.
ENDCLASS.
