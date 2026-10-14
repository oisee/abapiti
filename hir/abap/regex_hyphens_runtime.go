package abap

// Runtime counterpart of regexClassHyphens. Keep source/toString untouched;
// normalize only the pattern that will have shorthand members expanded.
const regexHyphensRuntime = `METHOD class_hyphens.
TYPES: BEGIN OF atom,
text TYPE string,
is_set TYPE abap_bool,
END OF atom.
DATA atoms TYPE STANDARD TABLE OF atom WITH DEFAULT KEY.
DATA item TYPE atom.
DATA middle TYPE atom.
DATA last TYPE atom.
DATA i TYPE i.
DATA start TYPE i.
DATA count TYPE i.
DATA j TYPE i.
DATA next_atom TYPE i.
DATA c TYPE string.
DATA body TYPE string.
DATA negative TYPE abap_bool.
DATA hyphen TYPE abap_bool.
WHILE i < strlen( pattern ).
c = pattern+i(1).
IF c = '\' AND i + 1 < strlen( pattern ).
CONCATENATE result pattern+i(2) INTO result RESPECTING BLANKS.
i = i + 2.
CONTINUE.
ENDIF.
IF c <> '['.
CONCATENATE result c INTO result RESPECTING BLANKS.
i = i + 1.
CONTINUE.
ENDIF.
start = i.
i = i + 1.
CLEAR: negative, hyphen, body, atoms.
IF i < strlen( pattern ) AND pattern+i(1) = '^'.
negative = abap_true.
i = i + 1.
ENDIF.
WHILE i < strlen( pattern ) AND pattern+i(1) <> ']'.
CLEAR item.
count = 1.
IF pattern+i(1) = '\' AND i + 1 < strlen( pattern ).
j = i + 1.
c = pattern+j(1).
count = 2.
item-is_set = xsdbool( c CA 'dDsSwW' ).
IF c = 'x' AND i + 4 <= strlen( pattern ).
count = 4.
ELSEIF c = 'u' AND i + 6 <= strlen( pattern ).
count = 6.
ENDIF.
ENDIF.
item-text = pattern+i(count).
APPEND item TO atoms.
i = i + count.
ENDWHILE.
IF i >= strlen( pattern ).
body = substring( val = pattern off = start ).
CONCATENATE result body INTO result RESPECTING BLANKS.
RETURN.
ENDIF.
j = 1.
WHILE j <= lines( atoms ).
READ TABLE atoms INDEX j INTO item.
CLEAR: middle, last.
next_atom = j + 1.
READ TABLE atoms INDEX next_atom INTO middle.
next_atom = j + 2.
READ TABLE atoms INDEX next_atom INTO last.
IF middle-text = '-' AND last-text IS NOT INITIAL AND item-is_set = abap_false AND last-is_set = abap_false.
CONCATENATE body item-text '-' last-text INTO body RESPECTING BLANKS.
j = j + 3.
ELSE.
IF item-text = '-' OR item-text = '\-'.
hyphen = abap_true.
ELSE.
CONCATENATE body item-text INTO body RESPECTING BLANKS.
ENDIF.
j = j + 1.
ENDIF.
ENDWHILE.
CONCATENATE result '[' INTO result RESPECTING BLANKS.
IF negative = abap_true.
CONCATENATE result '^' INTO result RESPECTING BLANKS.
ENDIF.
IF hyphen = abap_true AND body NP '-*'.
CONCATENATE result '-' INTO result RESPECTING BLANKS.
ENDIF.
CONCATENATE result body INTO result RESPECTING BLANKS.
IF hyphen = abap_true AND body CP '-*'.
CONCATENATE result '-' INTO result RESPECTING BLANKS.
ENDIF.
CONCATENATE result ']' INTO result RESPECTING BLANKS.
i = i + 1.
ENDWHILE.
ENDMETHOD.
`
