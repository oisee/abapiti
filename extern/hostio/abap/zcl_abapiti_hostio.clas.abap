CLASS zcl_abapiti_hostio DEFINITION PUBLIC FINAL CREATE PUBLIC.
  PUBLIC SECTION.
    TYPES: BEGIN OF ty_pair,
             f0 TYPE string,
             f1 TYPE abap_bool,
           END OF ty_pair.
    TYPES ty_pairs TYPE STANDARD TABLE OF ty_pair WITH DEFAULT KEY.
    CLASS-METHODS configure IMPORTING snapshot TYPE string OPTIONAL stdout TYPE string OPTIONAL.
    CLASS-METHODS read_text IMPORTING file TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS write_text IMPORTING file TYPE string text TYPE string.
    CLASS-METHODS write_stdout IMPORTING text TYPE string.
    CLASS-METHODS list_dir IMPORTING dir TYPE string RETURNING VALUE(result) TYPE string_table.
    CLASS-METHODS list_entries IMPORTING dir TYPE string RETURNING VALUE(result) TYPE ty_pairs.
    CLASS-METHODS basename IMPORTING file TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS dirname IMPORTING file TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS join IMPORTING parts TYPE string_table RETURNING VALUE(result) TYPE string.
    CLASS-METHODS normalize IMPORTING path TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS git_head IMPORTING cwd TYPE string RETURNING VALUE(result) TYPE REF TO string.
    CLASS-METHODS env IMPORTING name TYPE string RETURNING VALUE(result) TYPE REF TO string.
    CLASS-METHODS decode_utf8 IMPORTING bytes TYPE xstring RETURNING VALUE(result) TYPE string.
  PRIVATE SECTION.
    TYPES: BEGIN OF ty_dir,
             path    TYPE string,
             failed  TYPE abap_bool,
             entries TYPE ty_pairs,
           END OF ty_dir.
    TYPES ty_dirs TYPE HASHED TABLE OF ty_dir WITH UNIQUE KEY path.
    TYPES: BEGIN OF ty_value,
             name    TYPE string,
             present TYPE abap_bool,
             value   TYPE string,
           END OF ty_value.
    TYPES ty_values TYPE HASHED TABLE OF ty_value WITH UNIQUE KEY name.
    CLASS-DATA snapshot_path TYPE string VALUE 'fs-snapshot.json'.
    CLASS-DATA stdout_path TYPE string.
    CLASS-DATA stdout_open TYPE abap_bool.
    CLASS-DATA loaded TYPE abap_bool.
    CLASS-DATA cwd_base TYPE string.
    CLASS-DATA dirs TYPE ty_dirs.
    CLASS-DATA envs TYPE ty_values.
    CLASS-DATA heads TYPE ty_values.
    CLASS-DATA json TYPE string.
    CLASS-DATA json_pos TYPE i.
    CLASS-DATA json_len TYPE i.
    CLASS-METHODS load.
    CLASS-METHODS fail IMPORTING path TYPE string.
    CLASS-METHODS lookup IMPORTING dir TYPE string RETURNING VALUE(result) TYPE ty_pairs.
    CLASS-METHODS resolve IMPORTING path TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS normalize_string IMPORTING value TYPE string above TYPE abap_bool RETURNING VALUE(result) TYPE string.
    CLASS-METHODS last_slash IMPORTING value TYPE string RETURNING VALUE(result) TYPE i.
    CLASS-METHODS write_bytes IMPORTING file TYPE string text TYPE string append TYPE abap_bool.
    CLASS-METHODS decode_valid IMPORTING bytes TYPE xstring RETURNING VALUE(result) TYPE string.
    CLASS-METHODS byte_at IMPORTING bytes TYPE xstring index TYPE i RETURNING VALUE(result) TYPE i.
    CLASS-METHODS boxed IMPORTING value TYPE string RETURNING VALUE(result) TYPE REF TO string.
    CLASS-METHODS code_unit IMPORTING hex TYPE string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS json_peek RETURNING VALUE(result) TYPE string.
    CLASS-METHODS json_expect IMPORTING expected TYPE string.
    CLASS-METHODS json_comma.
    CLASS-METHODS json_word RETURNING VALUE(result) TYPE string.
    CLASS-METHODS json_string RETURNING VALUE(result) TYPE string.
    CLASS-METHODS json_values IMPORTING target TYPE string.
ENDCLASS.

CLASS zcl_abapiti_hostio IMPLEMENTATION.
  METHOD configure.
    IF snapshot IS SUPPLIED.
      snapshot_path = snapshot.
      loaded = abap_false.
      CLEAR: dirs, envs, heads, cwd_base.
    ENDIF.
    IF stdout IS SUPPLIED.
      stdout_path = stdout.
      stdout_open = abap_false.
    ENDIF.
  ENDMETHOD.

  METHOD read_text.
    DATA name TYPE string.
    DATA bytes TYPE xstring.
    name = file.
    OPEN DATASET name FOR INPUT IN BINARY MODE.
    IF sy-subrc <> 0.
      fail( name ).
    ENDIF.
    READ DATASET name INTO bytes.
    CLOSE DATASET name.
    result = decode_utf8( bytes ).
  ENDMETHOD.

  METHOD write_text.
    write_bytes( file = file text = text append = abap_false ).
  ENDMETHOD.

  METHOD write_stdout.
    IF stdout_path IS INITIAL.
      TRY.
          load( ).
        CATCH cx_sy_file_open.
      ENDTRY.
    ENDIF.
    IF stdout_path IS INITIAL.
      stdout_path = 'gogen.go'.
    ENDIF.
    write_bytes( file = stdout_path text = text append = stdout_open ).
    stdout_open = abap_true.
  ENDMETHOD.

  METHOD write_bytes.
    DATA name TYPE string.
    DATA bytes TYPE xstring.
    DATA conv TYPE REF TO cl_abap_conv_out_ce.
    name = file.
    conv = cl_abap_conv_out_ce=>create( encoding = 'UTF-8' ).
    conv->convert( EXPORTING data = text IMPORTING buffer = bytes ).
    IF append = abap_true.
      OPEN DATASET name FOR APPENDING IN BINARY MODE.
    ELSE.
      OPEN DATASET name FOR OUTPUT IN BINARY MODE.
    ENDIF.
    IF sy-subrc <> 0.
      fail( name ).
    ENDIF.
    TRANSFER bytes TO name.
    CLOSE DATASET name.
  ENDMETHOD.

  METHOD list_dir.
    DATA entries TYPE ty_pairs.
    DATA entry TYPE ty_pair.
    entries = lookup( dir ).
    LOOP AT entries INTO entry.
      APPEND entry-f0 TO result.
    ENDLOOP.
  ENDMETHOD.

  METHOD list_entries.
    result = lookup( dir ).
  ENDMETHOD.

  METHOD lookup.
    DATA key TYPE string.
    DATA found TYPE ty_dir.
    load( ).
    key = resolve( dir ).
    READ TABLE dirs WITH TABLE KEY path = key INTO found.
    IF sy-subrc <> 0.
      fail( key ).
    ENDIF.
    IF found-failed = abap_true.
      fail( key ).
    ENDIF.
    result = found-entries.
  ENDMETHOD.

  METHOD git_head.
    DATA key TYPE string.
    DATA found TYPE ty_value.
    load( ).
    key = resolve( cwd ).
    READ TABLE heads WITH TABLE KEY name = key INTO found.
    IF sy-subrc = 0 AND found-present = abap_true.
      result = boxed( found-value ).
    ENDIF.
  ENDMETHOD.

  METHOD env.
    DATA found TYPE ty_value.
    load( ).
    READ TABLE envs WITH TABLE KEY name = name INTO found.
    IF sy-subrc = 0 AND found-present = abap_true.
      result = boxed( found-value ).
    ENDIF.
  ENDMETHOD.

  METHOD fail.
    RAISE EXCEPTION TYPE cx_sy_file_open EXPORTING filename = path.
  ENDMETHOD.

  METHOD basename.
    DATA i TYPE i.
    DATA start TYPE i.
    DATA stop TYPE i VALUE -1.
    DATA matched TYPE abap_bool VALUE abap_true.
    DATA c TYPE string.
    i = strlen( file ) - 1.
    WHILE i >= 0.
      c = substring( val = file off = i len = 1 ).
      IF c = '/'.
        IF matched = abap_false.
          start = i + 1.
          EXIT.
        ENDIF.
      ELSEIF stop = -1.
        matched = abap_false.
        stop = i + 1.
      ENDIF.
      i = i - 1.
    ENDWHILE.
    IF stop = -1.
      RETURN.
    ENDIF.
    result = substring( val = file off = start len = stop - start ).
  ENDMETHOD.

  METHOD dirname.
    DATA n TYPE i.
    DATA i TYPE i.
    DATA stop TYPE i VALUE -1.
    DATA matched TYPE abap_bool VALUE abap_true.
    DATA root TYPE abap_bool.
    DATA c TYPE string.
    n = strlen( file ).
    IF n = 0.
      result = '.'.
      RETURN.
    ENDIF.
    IF substring( val = file len = 1 ) = '/'.
      root = abap_true.
    ENDIF.
    i = n - 1.
    WHILE i >= 1.
      c = substring( val = file off = i len = 1 ).
      IF c = '/'.
        IF matched = abap_false.
          stop = i.
          EXIT.
        ENDIF.
      ELSE.
        matched = abap_false.
      ENDIF.
      i = i - 1.
    ENDWHILE.
    IF stop = -1.
      IF root = abap_true.
        result = '/'.
      ELSE.
        result = '.'.
      ENDIF.
      RETURN.
    ENDIF.
    IF root = abap_true AND stop = 1.
      result = '//'.
      RETURN.
    ENDIF.
    result = substring( val = file len = stop ).
  ENDMETHOD.

  METHOD join.
    DATA part TYPE string.
    DATA joined TYPE string.
    DATA has TYPE abap_bool.
    LOOP AT parts INTO part.
      IF part IS INITIAL.
        CONTINUE.
      ENDIF.
      IF has = abap_true.
        joined = joined && '/' && part.
      ELSE.
        joined = part.
        has = abap_true.
      ENDIF.
    ENDLOOP.
    IF has = abap_false.
      result = '.'.
      RETURN.
    ENDIF.
    result = normalize( joined ).
  ENDMETHOD.

  METHOD normalize.
    DATA n TYPE i.
    DATA absolute TYPE abap_bool.
    DATA trailing TYPE abap_bool.
    DATA body TYPE string.
    n = strlen( path ).
    IF n = 0.
      result = '.'.
      RETURN.
    ENDIF.
    IF substring( val = path len = 1 ) = '/'.
      absolute = abap_true.
    ENDIF.
    IF substring( val = path off = n - 1 len = 1 ) = '/'.
      trailing = abap_true.
    ENDIF.
    body = normalize_string( value = path above = xsdbool( absolute = abap_false ) ).
    IF body IS INITIAL.
      IF absolute = abap_true.
        result = '/'.
      ELSEIF trailing = abap_true.
        result = './'.
      ELSE.
        result = '.'.
      ENDIF.
      RETURN.
    ENDIF.
    IF trailing = abap_true.
      body = body && '/'.
    ENDIF.
    IF absolute = abap_true.
      result = '/' && body.
    ELSE.
      result = body.
    ENDIF.
  ENDMETHOD.

  METHOD normalize_string.
    DATA n TYPE i.
    DATA i TYPE i.
    DATA c TYPE string.
    DATA seg TYPE i.
    DATA prev TYPE i VALUE -1.
    DATA dots TYPE i.
    DATA size TYPE i.
    DATA idx TYPE i.
    DATA tail TYPE abap_bool.
    n = strlen( value ).
    WHILE i <= n.
      IF i < n.
        c = substring( val = value off = i len = 1 ).
      ELSEIF c = '/'.
        EXIT.
      ELSE.
        c = '/'.
      ENDIF.
      IF c = '/'.
        IF prev = i - 1 OR dots = 1.
        ELSEIF dots = 2.
          size = strlen( result ).
          tail = abap_false.
          IF size >= 2 AND seg = 2.
            IF substring( val = result off = size - 2 len = 2 ) = '..'.
              tail = abap_true.
            ENDIF.
          ENDIF.
          IF tail = abap_false AND size > 2.
            idx = last_slash( result ).
            IF idx = -1.
              CLEAR result.
              seg = 0.
            ELSE.
              result = substring( val = result len = idx ).
              seg = strlen( result ) - 1 - last_slash( result ).
            ENDIF.
            prev = i.
            dots = 0.
            i = i + 1.
            CONTINUE.
          ELSEIF tail = abap_false AND size <> 0.
            CLEAR result.
            seg = 0.
            prev = i.
            dots = 0.
            i = i + 1.
            CONTINUE.
          ENDIF.
          IF above = abap_true.
            IF size > 0.
              result = result && '/..'.
            ELSE.
              result = '..'.
            ENDIF.
            seg = 2.
          ENDIF.
        ELSE.
          IF strlen( result ) > 0.
            result = result && '/' && substring( val = value off = prev + 1 len = i - prev - 1 ).
          ELSE.
            result = substring( val = value off = prev + 1 len = i - prev - 1 ).
          ENDIF.
          seg = i - prev - 1.
        ENDIF.
        prev = i.
        dots = 0.
      ELSEIF c = '.' AND dots <> -1.
        dots = dots + 1.
      ELSE.
        dots = -1.
      ENDIF.
      i = i + 1.
    ENDWHILE.
  ENDMETHOD.

  METHOD last_slash.
    result = strlen( value ) - 1.
    WHILE result >= 0.
      IF substring( val = value off = result len = 1 ) = '/'.
        RETURN.
      ENDIF.
      result = result - 1.
    ENDWHILE.
  ENDMETHOD.

  METHOD resolve.
    DATA absolute TYPE abap_bool.
    DATA n TYPE i.
    IF strlen( path ) > 0.
      IF substring( val = path len = 1 ) = '/'.
        absolute = abap_true.
      ENDIF.
    ENDIF.
    IF absolute = abap_true.
      result = normalize( path ).
    ELSE.
      result = normalize( cwd_base && '/' && path ).
    ENDIF.
    n = strlen( result ).
    WHILE n > 1.
      IF substring( val = result off = n - 1 len = 1 ) <> '/'.
        EXIT.
      ENDIF.
      n = n - 1.
      result = substring( val = result len = n ).
    ENDWHILE.
  ENDMETHOD.

  METHOD decode_utf8.
    DATA n TYPE i.
    DATA i TYPE i.
    DATA run TYPE i.
    DATA size TYPE i.
    DATA lead TYPE i.
    DATA want TYPE i.
    DATA k TYPE i.
    DATA v TYPE i.
    DATA part TYPE xstring.
    DATA replacement TYPE string.
    TRY.
        result = decode_valid( bytes ).
        RETURN.
      CATCH cx_sy_conversion_codepage.
    ENDTRY.
    replacement = code_unit( 'FFFD' ).
    n = xstrlen( bytes ).
    WHILE i < n.
      lead = byte_at( bytes = bytes index = i ).
      IF lead < 128.
        i = i + 1.
        CONTINUE.
      ENDIF.
      want = 1.
      IF lead >= 194 AND lead <= 223.
        want = 2.
      ELSEIF lead >= 224 AND lead <= 239.
        want = 3.
      ELSEIF lead >= 240 AND lead <= 244.
        want = 4.
      ENDIF.
      k = 1.
      WHILE k < want AND i + k < n.
        v = byte_at( bytes = bytes index = i + k ).
        IF v < 128 OR v > 191.
          EXIT.
        ENDIF.
        IF k = 1 AND ( lead = 224 AND v < 160 OR lead = 237 AND v >= 160 OR lead = 240 AND v < 144 OR lead = 244 AND v >= 144 ).
          EXIT.
        ENDIF.
        k = k + 1.
      ENDWHILE.
      IF want > 1 AND k = want.
        i = i + k.
        CONTINUE.
      ENDIF.
      IF i > run.
        size = i - run.
        part = bytes+run(size).
        result = result && decode_valid( part ).
      ENDIF.
      result = result && replacement.
      i = i + k.
      run = i.
    ENDWHILE.
    IF n > run.
      size = n - run.
      part = bytes+run(size).
      result = result && decode_valid( part ).
    ENDIF.
  ENDMETHOD.

  METHOD decode_valid.
    DATA conv TYPE REF TO cl_abap_conv_in_ce.
    IF xstrlen( bytes ) = 0.
      RETURN.
    ENDIF.
    conv = cl_abap_conv_in_ce=>create( encoding = 'UTF-8' ).
    conv->convert( EXPORTING input = bytes IMPORTING data = result ).
  ENDMETHOD.

  METHOD byte_at.
    DATA one TYPE x LENGTH 1.
    one = bytes+index(1).
    result = one.
  ENDMETHOD.

  METHOD boxed.
    FIELD-SYMBOLS <value> TYPE string.
    CREATE DATA result.
    ASSIGN result->* TO <value>.
    <value> = value.
  ENDMETHOD.

  METHOD code_unit.
    DATA unit TYPE x LENGTH 2.
    unit = hex.
    result = cl_abap_conv_in_ce=>uccp( unit ).
  ENDMETHOD.

  METHOD load.
    DATA key TYPE string.
    DATA dir TYPE ty_dir.
    DATA pair TYPE ty_pair.
    IF loaded = abap_true.
      RETURN.
    ENDIF.
    CLEAR: dirs, envs, heads, cwd_base.
    json = read_text( snapshot_path ).
    json_len = strlen( json ).
    json_pos = 0.
    json_expect( '{' ).
    WHILE json_peek( ) <> '}'.
      key = json_string( ).
      json_expect( ':' ).
      CASE key.
        WHEN 'version'.
          IF json_word( ) <> '1'.
            fail( snapshot_path ).
          ENDIF.
        WHEN 'cwd'.
          cwd_base = json_string( ).
        WHEN 'stdout'.
          IF stdout_path IS INITIAL.
            stdout_path = json_string( ).
          ELSE.
            json_string( ).
          ENDIF.
        WHEN 'dirs'.
          json_expect( '{' ).
          WHILE json_peek( ) <> '}'.
            CLEAR dir.
            dir-path = json_string( ).
            json_expect( ':' ).
            IF json_peek( ) = '{'.
              json_expect( '{' ).
              json_string( ).
              json_expect( ':' ).
              json_string( ).
              json_expect( '}' ).
              dir-failed = abap_true.
            ELSE.
              json_expect( '[' ).
              WHILE json_peek( ) <> ']'.
                json_expect( '[' ).
                pair-f0 = json_string( ).
                json_expect( ',' ).
                pair-f1 = xsdbool( json_word( ) = 'true' ).
                json_expect( ']' ).
                APPEND pair TO dir-entries.
                json_comma( ).
              ENDWHILE.
              json_expect( ']' ).
            ENDIF.
            INSERT dir INTO TABLE dirs.
            json_comma( ).
          ENDWHILE.
          json_expect( '}' ).
        WHEN 'env' OR 'gitHead'.
          json_values( key ).
        WHEN OTHERS.
          fail( snapshot_path ).
      ENDCASE.
      json_comma( ).
    ENDWHILE.
    CLEAR json.
    loaded = abap_true.
  ENDMETHOD.

  METHOD json_values.
    DATA item TYPE ty_value.
    json_expect( '{' ).
    WHILE json_peek( ) <> '}'.
      CLEAR item.
      item-name = json_string( ).
      json_expect( ':' ).
      IF json_peek( ) = 'n'.
        json_word( ).
      ELSE.
        item-value = json_string( ).
        item-present = abap_true.
      ENDIF.
      IF target = 'env'.
        INSERT item INTO TABLE envs.
      ELSE.
        INSERT item INTO TABLE heads.
      ENDIF.
      json_comma( ).
    ENDWHILE.
    json_expect( '}' ).
  ENDMETHOD.

  METHOD json_peek.
    DATA c TYPE string.
    WHILE json_pos < json_len.
      c = substring( val = json off = json_pos len = 1 ).
      IF c <> ` ` AND c <> cl_abap_char_utilities=>newline AND c <> cl_abap_char_utilities=>horizontal_tab.
        result = c.
        RETURN.
      ENDIF.
      json_pos = json_pos + 1.
    ENDWHILE.
    fail( snapshot_path ).
  ENDMETHOD.

  METHOD json_expect.
    IF json_peek( ) <> expected.
      fail( snapshot_path ).
    ENDIF.
    json_pos = json_pos + 1.
  ENDMETHOD.

  METHOD json_comma.
    IF json_pos >= json_len.
      RETURN.
    ENDIF.
    IF json_peek( ) = ','.
      json_pos = json_pos + 1.
    ENDIF.
  ENDMETHOD.

  METHOD json_word.
    DATA start TYPE i.
    DATA c TYPE string.
    json_peek( ).
    start = json_pos.
    WHILE json_pos < json_len.
      c = substring( val = json off = json_pos len = 1 ).
      IF c = ',' OR c = '}' OR c = ']' OR c = ` ` OR c = cl_abap_char_utilities=>newline.
        EXIT.
      ENDIF.
      json_pos = json_pos + 1.
    ENDWHILE.
    result = substring( val = json off = start len = json_pos - start ).
  ENDMETHOD.

  METHOD json_string.
    DATA start TYPE i.
    DATA c TYPE string.
    DATA hex TYPE string.
    json_expect( '"' ).
    DO.
      start = json_pos.
      WHILE json_pos < json_len.
        c = substring( val = json off = json_pos len = 1 ).
        IF c = '"' OR c = '\'.
          EXIT.
        ENDIF.
        json_pos = json_pos + 1.
      ENDWHILE.
      IF json_pos >= json_len.
        fail( snapshot_path ).
      ENDIF.
      result = result && substring( val = json off = start len = json_pos - start ).
      json_pos = json_pos + 1.
      IF c = '"'.
        RETURN.
      ENDIF.
      IF json_pos >= json_len.
        fail( snapshot_path ).
      ENDIF.
      c = substring( val = json off = json_pos len = 1 ).
      json_pos = json_pos + 1.
      CASE c.
        WHEN 'u'.
          IF json_pos + 4 > json_len.
            fail( snapshot_path ).
          ENDIF.
          hex = to_upper( substring( val = json off = json_pos len = 4 ) ).
          json_pos = json_pos + 4.
        WHEN 'n'.
          hex = '000A'.
        WHEN 't'.
          hex = '0009'.
        WHEN 'r'.
          hex = '000D'.
        WHEN 'b'.
          hex = '0008'.
        WHEN 'f'.
          hex = '000C'.
        WHEN OTHERS.
          result = result && c.
          CONTINUE.
      ENDCASE.
      result = result && code_unit( hex ).
    ENDDO.
  ENDMETHOD.
ENDCLASS.
