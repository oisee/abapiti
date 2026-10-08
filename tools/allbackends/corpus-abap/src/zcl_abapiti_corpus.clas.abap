CLASS zcl_abapiti_corpus DEFINITION PUBLIC FINAL CREATE PUBLIC.
PUBLIC SECTION.
  INTERFACES zif_abapiti_corpus.
  TYPES BEGIN OF ty_file.
    TYPES idx TYPE i.
    TYPES name TYPE c LENGTH 255.
    TYPES v TYPE xstring.
    TYPES size TYPE int8.
    TYPES sha256 TYPE c LENGTH 64.
  TYPES END OF ty_file.
  TYPES ty_files TYPE STANDARD TABLE OF ty_file WITH DEFAULT KEY.
  TYPES ty_hash TYPE c LENGTH 64.
  METHODS constructor IMPORTING iv_set TYPE clike.
  CLASS-METHODS load_zip IMPORTING iv_path TYPE clike iv_replace TYPE abap_bool RETURNING VALUE(result) TYPE abap_bool.
  CLASS-METHODS get_by_name IMPORTING setname TYPE clike name TYPE clike RETURNING VALUE(result) TYPE xstring.
  CLASS-METHODS get_500 RETURNING VALUE(result) TYPE ty_files.
  CLASS-METHODS get_text IMPORTING setname TYPE clike idx TYPE i OPTIONAL name TYPE string OPTIONAL RETURNING VALUE(result) TYPE string.
  CLASS-METHODS get_raw IMPORTING setname TYPE clike idx TYPE i OPTIONAL name TYPE string OPTIONAL RETURNING VALUE(result) TYPE xstring.
PROTECTED SECTION.
PRIVATE SECTION.
  TYPES BEGIN OF ty_summary.
    TYPES set_id TYPE c LENGTH 30.
    TYPES files TYPE i.
    TYPES bytes TYPE int8.
    TYPES verified TYPE i.
    TYPES mismatches TYPE i.
    TYPES seconds TYPE i.
  TYPES END OF ty_summary.
  TYPES ty_summaries TYPE SORTED TABLE OF ty_summary WITH UNIQUE KEY set_id.
  TYPES BEGIN OF ty_meta.
    TYPES path TYPE string.
    TYPES set_id TYPE c LENGTH 30.
    TYPES name TYPE string.
    TYPES size TYPE int8.
    TYPES sha256 TYPE c LENGTH 64.
    TYPES idx TYPE i.
  TYPES END OF ty_meta.
  TYPES ty_meta_hash TYPE HASHED TABLE OF ty_meta WITH UNIQUE KEY path.
  TYPES ty_paths TYPE STANDARD TABLE OF string WITH DEFAULT KEY.
  TYPES BEGIN OF ty_expected.
    TYPES path TYPE string.
    TYPES size TYPE int8.
    TYPES sha256 TYPE c LENGTH 64.
  TYPES END OF ty_expected.
  TYPES ty_expected_hash TYPE HASHED TABLE OF ty_expected WITH UNIQUE KEY path.
  TYPES ty_zip_entity TYPE cl_abap_zip=>entity.
  TYPES BEGIN OF ty_zip.
    TYPES expected TYPE ty_expected_hash.
    TYPES plain TYPE ty_paths.
    TYPES corpus TYPE ty_paths.
    TYPES meta TYPE ty_meta_hash.
    TYPES zabapgit_idx TYPE i.
    TYPES deps_idx TYPE i.
    TYPES sets_idx TYPE i.
    TYPES mismatches TYPE i.
  TYPES END OF ty_zip.
  CLASS-DATA summaries TYPE ty_summaries.
  CLASS-DATA summary TYPE ty_summary.
  CLASS-DATA zip_data TYPE ty_zip.
  CLASS-DATA zip_archive TYPE REF TO cl_abap_zip.
  CLASS-METHODS find_raw IMPORTING setname TYPE clike idx TYPE i name TYPE string RETURNING VALUE(result) TYPE xstring RAISING cx_sy_itab_line_not_found.
  CLASS-METHODS read_zip IMPORTING iv_path TYPE clike RAISING cx_static_check.
  CLASS-METHODS parse_manifests RAISING cx_static_check.
  CLASS-METHODS validate_members RAISING cx_static_check.
  CLASS-METHODS add_member IMPORTING entity TYPE cl_abap_zip=>entity set_id TYPE clike member_name TYPE string idx TYPE i.
  CLASS-METHODS member_hash IMPORTING content TYPE xstring RETURNING VALUE(result) TYPE ty_hash.
  CLASS-METHODS expected_hash IMPORTING path TYPE string RETURNING VALUE(result) TYPE ty_hash.
  CLASS-METHODS bump_mismatch IMPORTING set_id TYPE clike.
  CLASS-METHODS summarize_valid.
  CLASS-METHODS validate_inventory.
  CLASS-METHODS insert_members IMPORTING set_id TYPE clike RAISING cx_static_check.
  CLASS-METHODS add_summary IMPORTING set_id TYPE clike files TYPE i bytes TYPE int8 verified TYPE i mismatches TYPE i seconds TYPE i.
  CLASS-METHODS write_summaries.
  TYPES BEGIN OF ty_row.
    TYPES idx TYPE i.
    TYPES name TYPE c LENGTH 255.
    TYPES v TYPE xstring.
  TYPES END OF ty_row.
  TYPES ty_rows TYPE STANDARD TABLE OF ty_row WITH DEFAULT KEY.
  DATA set_id TYPE c LENGTH 30.
  DATA rows TYPE ty_rows.
  METHODS load.
  CLASS-METHODS is_raw RETURNING VALUE(result) TYPE abap_bool.
  CLASS-METHODS quote IMPORTING value TYPE string RETURNING VALUE(result) TYPE string.
ENDCLASS.
CLASS zcl_abapiti_corpus IMPLEMENTATION.
  METHOD load_zip.
    DATA started TYPE i.
    DATA finished TYPE i.
    DATA table_name TYPE tabname.
    DATA set_names TYPE TABLE OF string.
    table_name = 'ZABAPITI_CORPUS'.
    CLEAR summaries.
    result = abap_false.
    GET RUN TIME FIELD started.
    TRY.
        read_zip( iv_path ).
        parse_manifests( ).
        validate_members( ).
        validate_inventory( ).
        IF zip_data-mismatches > 0.
          summarize_valid( ).
          write_summaries( ).
          RETURN.
        ENDIF.
        IF iv_replace = abap_true.
          DELETE FROM (table_name) WHERE setname = 'ZABAPGIT' OR setname = 'DEPS' OR setname = 'CORPUS' OR setname = 'SETS'.
          COMMIT WORK.
        ELSE.
          SELECT COUNT(*) FROM (table_name) INTO @DATA(old_count) WHERE setname = 'ZABAPGIT' OR setname = 'DEPS' OR setname = 'CORPUS' OR setname = 'SETS'.
          IF old_count > 0.
            RETURN.
          ENDIF.
        ENDIF.
        set_names = VALUE #( ( `ZABAPGIT` ) ( `DEPS` ) ( `CORPUS` ) ( `SETS` ) ).
        LOOP AT set_names INTO DATA(set_name).
          insert_members( set_name ).
        ENDLOOP.
        result = abap_true.
      CATCH cx_root.
        ROLLBACK WORK.
        CLEAR result.
    ENDTRY.
    write_summaries( ).
    GET RUN TIME FIELD finished.
    DATA(elapsed) = ( finished - started ) / 1000000.
    WRITE / |Elapsed { elapsed NUMBER = USER } seconds|.
  ENDMETHOD.
  METHOD read_zip.
    DATA archive TYPE xstring.
    DATA message TYPE string.
    CLEAR zip_data.
    CREATE OBJECT zip_archive.
    OPEN DATASET iv_path FOR INPUT IN BINARY MODE MESSAGE message.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
    READ DATASET iv_path INTO archive.
    IF sy-subrc <> 0.
      CLOSE DATASET iv_path.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
    CLOSE DATASET iv_path.
    zip_archive->load( zip = archive ).
  ENDMETHOD.
  METHOD parse_manifests.
    DATA manifest TYPE xstring.
    DATA text TYPE string.
    DATA hash TYPE c LENGTH 64.
    DATA path TYPE string.
    DATA size_text TYPE string.
    DATA stored TYPE string.
    DATA origin TYPE string.
    DATA expected TYPE ty_expected.
    zip_archive->get( EXPORTING name = 'abapiti/MANIFEST.sha256' IMPORTING content = manifest ).
    text = cl_abap_codepage=>convert_from( source = manifest codepage = `UTF-8` ).
    SPLIT text AT cl_abap_char_utilities=>newline INTO TABLE DATA(lines).
    LOOP AT lines INTO DATA(line).
      IF strlen( line ) < 66 OR line(1) CA ' #'.
        CONTINUE.
      ENDIF.
      hash = to_upper( line(64) ).
      path = substring( val = line off = 66 ).
      CONDENSE path.
      expected-path = |abapiti/{ path }|.
      expected-sha256 = hash.
      INSERT expected INTO TABLE zip_data-expected.
      APPEND expected-path TO zip_data-plain.
    ENDLOOP.
    zip_archive->get( EXPORTING name = 'abapiti/corpus/MANIFEST.tsv' IMPORTING content = manifest ).
    text = cl_abap_codepage=>convert_from( source = manifest codepage = `UTF-8` ).
    SPLIT text AT cl_abap_char_utilities=>newline INTO TABLE lines.
    LOOP AT lines INTO line.
      IF line IS INITIAL OR line CP 'sha256*'.
        CONTINUE.
      ENDIF.
      SPLIT line AT cl_abap_char_utilities=>horizontal_tab INTO hash size_text stored origin.
      CONDENSE size_text.
      expected-path = |abapiti/corpus/{ stored }|.
      expected-size = size_text.
      expected-sha256 = to_upper( hash ).
      INSERT expected INTO TABLE zip_data-expected.
      APPEND expected-path TO zip_data-corpus.
    ENDLOOP.
  ENDMETHOD.
  METHOD validate_members.
    DATA entity TYPE ty_zip_entity.
    SORT zip_data-plain.
    LOOP AT zip_data-plain INTO DATA(path).
      CLEAR entity.
      entity-name = path.
      zip_archive->get( EXPORTING name = entity-name IMPORTING content = entity-content ).
      DATA(rest) = substring( val = entity-name off = 8 ).
      SPLIT rest AT '/' INTO DATA(set_id) DATA(member_name).
      TRANSLATE set_id TO UPPER CASE.
      IF set_id = 'ZABAPGIT'.
        zip_data-zabapgit_idx = zip_data-zabapgit_idx + 1.
      ELSE.
        zip_data-deps_idx = zip_data-deps_idx + 1.
      ENDIF.
      DATA(member_idx) = COND i( WHEN set_id = 'ZABAPGIT' THEN zip_data-zabapgit_idx ELSE zip_data-deps_idx ).
      add_member( EXPORTING entity = entity set_id = set_id member_name = member_name idx = member_idx ).
    ENDLOOP.
    LOOP AT zip_data-corpus INTO DATA(corpus_path).
      IF NOT line_exists( zip_data-meta[ path = corpus_path ] ).
        bump_mismatch( 'CORPUS' ).
      ENDIF.
    ENDLOOP.
    LOOP AT zip_data-expected INTO DATA(expected).
      IF NOT line_exists( zip_data-meta[ path = expected-path ] ).
        IF expected-path CP 'abapiti/zabapgit/*'.
          bump_mismatch( 'ZABAPGIT' ).
        ELSE.
          bump_mismatch( 'DEPS' ).
        ENDIF.
      ENDIF.
    ENDLOOP.
    CLEAR entity.
    entity-name = 'abapiti/corpus/MANIFEST.tsv'.
    zip_archive->get( EXPORTING name = entity-name IMPORTING content = entity-content ).
    add_member( EXPORTING entity = entity set_id = 'CORPUS' member_name = 'MANIFEST.tsv' idx = lines( zip_data-corpus ) + 1 ).
    CLEAR entity.
    entity-name = 'abapiti/sets/corpus500.txt'.
    zip_archive->get( EXPORTING name = entity-name IMPORTING content = entity-content ).
    add_member( EXPORTING entity = entity set_id = 'SETS' member_name = 'corpus500.txt' idx = 1 ).
  ENDMETHOD.
  METHOD add_member.
    DATA meta TYPE ty_meta.
    IF member_name IS INITIAL OR strlen( member_name ) > 255 OR member_name CA '\'.
      bump_mismatch( set_id ).
      RETURN.
    ENDIF.
    meta-path = entity-name.
    TRANSLATE meta-path TO LOWER CASE.
    meta-set_id = set_id.
    meta-name = member_name.
    meta-size = xstrlen( entity-content ).
    meta-sha256 = member_hash( entity-content ).
    IF set_id = 'CORPUS'.
      IF meta-path = 'abapiti/corpus/manifest.tsv'.
        meta-idx = idx.
      ELSE.
        READ TABLE zip_data-corpus TRANSPORTING NO FIELDS WITH KEY table_line = meta-path.
        IF sy-subrc <> 0.
          bump_mismatch( set_id ).
          RETURN.
        ENDIF.
        meta-idx = sy-tabix.
      ENDIF.
    ELSE.
      meta-idx = idx.
    ENDIF.
    IF expected_hash( meta-path ) <> meta-sha256 OR ( line_exists( zip_data-expected[ path = meta-path ] ) AND zip_data-expected[ path = meta-path ]-size <> meta-size ).
      bump_mismatch( set_id ).
      RETURN.
    ENDIF.
    INSERT meta INTO TABLE zip_data-meta.
    IF sy-subrc <> 0.
      bump_mismatch( set_id ).
    ENDIF.
  ENDMETHOD.
  METHOD member_hash.
    DATA hash TYPE string.
    CALL METHOD cl_abap_message_digest=>('CALCULATE_HASH_FOR_RAW') EXPORTING if_algorithm = 'SHA256' if_data = content IMPORTING ef_hashstring = hash.
    result = to_upper( hash ).
  ENDMETHOD.
  METHOD expected_hash.
    IF line_exists( zip_data-expected[ path = path ] ).
      result = zip_data-expected[ path = path ]-sha256.
    ENDIF.
  ENDMETHOD.
  METHOD bump_mismatch.
    DATA item TYPE ty_summary.
    zip_data-mismatches = zip_data-mismatches + 1.
    READ TABLE summaries INTO item WITH TABLE KEY set_id = set_id.
    IF sy-subrc <> 0.
      CLEAR item.
      item-set_id = set_id.
    ENDIF.
    item-mismatches = item-mismatches + 1.
    MODIFY TABLE summaries FROM item.
  ENDMETHOD.
  METHOD summarize_valid.
    LOOP AT zip_data-meta INTO DATA(meta).
      IF expected_hash( meta-path ) IS NOT INITIAL.
        add_summary( set_id = meta-set_id files = 1 bytes = meta-size verified = 1 mismatches = 0 seconds = 0 ).
      ELSE.
        add_summary( set_id = meta-set_id files = 1 bytes = meta-size verified = 0 mismatches = 0 seconds = 0 ).
      ENDIF.
    ENDLOOP.
  ENDMETHOD.
  METHOD validate_inventory.
    FIELD-SYMBOLS <entities> TYPE ANY TABLE.
    FIELD-SYMBOLS <entity> TYPE any.
    FIELD-SYMBOLS <name> TYPE any.
    ASSIGN zip_archive->('ENTITIES') TO <entities>.
    IF sy-subrc <> 0.
      bump_mismatch( 'UNKNOWN' ).
      RETURN.
    ENDIF.
    LOOP AT <entities> ASSIGNING <entity>.
      ASSIGN COMPONENT 'NAME' OF STRUCTURE <entity> TO <name>.
      IF sy-subrc <> 0.
        bump_mismatch( 'UNKNOWN' ).
        CONTINUE.
      ENDIF.
      DATA(member) = to_lower( CONV string( <name> ) ).
      IF member CP '*/' OR member CP 'abapiti/*/' OR member = 'abapiti/manifest.sha256'.
        CONTINUE.
      ENDIF.
      IF member CP 'abapiti/zabapgit/*'.
        IF NOT line_exists( zip_data-expected[ path = member ] ).
          bump_mismatch( 'ZABAPGIT' ).
        ENDIF.
      ELSEIF member CP 'abapiti/deps/*'.
        IF NOT line_exists( zip_data-expected[ path = member ] ).
          bump_mismatch( 'DEPS' ).
        ENDIF.
      ELSEIF member CP 'abapiti/corpus/*'.
        IF member <> 'abapiti/corpus/manifest.tsv' AND NOT line_exists( zip_data-corpus[ table_line = member ] ).
          bump_mismatch( 'CORPUS' ).
        ENDIF.
      ELSEIF member = 'abapiti/sets/corpus500.txt'.
        CONTINUE.
      ELSE.
        bump_mismatch( 'UNKNOWN' ).
      ENDIF.
    ENDLOOP.
  ENDMETHOD.
  METHOD insert_members.
    DATA rows TYPE STANDARD TABLE OF zabapiti_corpus WITH DEFAULT KEY.
    DATA row TYPE zabapiti_corpus.
    DATA next_idx TYPE i.
    DATA bytes TYPE int8.
    DATA verified TYPE i.
    DATA set_started TYPE i.
    DATA set_finished TYPE i.
    DATA table_name TYPE tabname.
    table_name = 'ZABAPITI_CORPUS'.
    GET RUN TIME FIELD set_started.
    LOOP AT zip_data-meta INTO DATA(meta) WHERE set_id = set_id.
      next_idx = next_idx + 1.
      DATA content TYPE xstring.
      zip_archive->get( EXPORTING name = meta-path IMPORTING content = content ).
      CLEAR row.
      row-setname = set_id.
      row-idx = COND i( WHEN meta-idx > 0 THEN meta-idx ELSE next_idx ).
      row-name = meta-name.
      row-v = content.
      row-size = meta-size.
      row-sha256 = meta-sha256.
      APPEND row TO rows.
      bytes = bytes + row-size.
      IF expected_hash( meta-path ) IS NOT INITIAL.
        verified = verified + 1.
      ENDIF.
      IF lines( rows ) = 500.
        INSERT (table_name) FROM TABLE rows.
        IF sy-subrc <> 0.
          ROLLBACK WORK.
          RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
        ENDIF.
        COMMIT WORK.
        CLEAR rows.
      ENDIF.
    ENDLOOP.
    IF rows IS NOT INITIAL.
      INSERT (table_name) FROM TABLE rows.
      IF sy-subrc <> 0.
        ROLLBACK WORK.
        RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
      ENDIF.
      COMMIT WORK.
    ENDIF.
    GET RUN TIME FIELD set_finished.
    add_summary( set_id = set_id files = next_idx bytes = bytes verified = verified mismatches = 0 seconds = ( set_finished - set_started ) / 1000000 ).
  ENDMETHOD.
  METHOD add_summary.
    DATA item TYPE ty_summary.
    READ TABLE summaries INTO item WITH TABLE KEY set_id = set_id.
    IF sy-subrc = 0.
      item-files = item-files + files.
      item-bytes = item-bytes + bytes.
      item-verified = item-verified + verified.
      item-mismatches = item-mismatches + mismatches.
      item-seconds = item-seconds + seconds.
      MODIFY TABLE summaries FROM item.
    ELSE.
      item-set_id = set_id.
      item-files = files.
      item-bytes = bytes.
      item-verified = verified.
      item-mismatches = mismatches.
      item-seconds = seconds.
      INSERT item INTO TABLE summaries.
    ENDIF.
  ENDMETHOD.
  METHOD write_summaries.
    LOOP AT summaries INTO summary.
      WRITE / |{ summary-set_id }: files { summary-files } bytes { summary-bytes } verified { summary-verified } mismatches { summary-mismatches } seconds { summary-seconds }|.
    ENDLOOP.
    IF zip_data-mismatches > 0.
      WRITE / |Validation failed with { zip_data-mismatches } mismatches|.
    ENDIF.
  ENDMETHOD.
  METHOD get_text.
    TRY.
      result = cl_abap_codepage=>convert_from( source = find_raw( setname = setname idx = idx name = name ) codepage = `UTF-8` ).
    CATCH cx_sy_codepage_converter_init cx_sy_conversion_codepage cx_sy_itab_line_not_found.
      CLEAR result.
    ENDTRY.
  ENDMETHOD.
  METHOD get_by_name.
    result = get_raw( setname = setname name = name ).
  ENDMETHOD.
  METHOD get_500.
    DATA list TYPE xstring.
    DATA text TYPE string.
    DATA name TYPE string.
    DATA file TYPE ty_file.
    DATA table_name TYPE tabname.
    DATA condition TYPE string.
    table_name = 'ZABAPITI_CORPUS'.
    list = get_raw( setname = 'SETS' name = 'corpus500.txt' ).
    text = cl_abap_codepage=>convert_from( source = list codepage = `UTF-8` ).
    SPLIT text AT cl_abap_char_utilities=>newline INTO TABLE DATA(lines).
    LOOP AT lines INTO DATA(line).
      IF line IS INITIAL.
        CONTINUE.
      ENDIF.
      name = line.
      REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf IN name WITH ``.
      CLEAR file.
      file-name = name.
      file-v = get_by_name( setname = 'CORPUS' name = name ).
      condition = |SETNAME = 'CORPUS' AND NAME = { quote( name ) }|.
      SELECT SINGLE idx size sha256 FROM (table_name) INTO CORRESPONDING FIELDS OF file WHERE (condition).
      IF sy-subrc = 0.
        APPEND file TO result.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.
  METHOD get_raw.
    TRY.
      result = find_raw( setname = setname idx = idx name = name ).
    CATCH cx_sy_itab_line_not_found.
      CLEAR result.
    ENDTRY.
  ENDMETHOD.
  METHOD find_raw.
    DATA table_name TYPE tabname.
    DATA condition TYPE string.
    IF idx > 0 AND name IS INITIAL.
      condition = |SETNAME = { quote( |{ setname }| ) } AND IDX = { idx }|.
    ELSEIF idx <= 0 AND name IS NOT INITIAL.
      condition = |SETNAME = { quote( |{ setname }| ) } AND NAME = { quote( name ) }|.
    ELSE.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
    table_name = 'ZABAPITI_CORPUS'.
    SELECT SINGLE v FROM (table_name) INTO result WHERE (condition).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_sy_itab_line_not_found.
    ENDIF.
  ENDMETHOD.
  METHOD constructor.
    set_id = iv_set.
    load( ).
  ENDMETHOD.
  METHOD load.
    DATA table_name TYPE tabname.
    DATA condition TYPE string.
    table_name = 'ZABAPITI_CORPUS'.
    condition = |SETNAME = { quote( |{ set_id }| ) }|.
    SELECT idx name v FROM (table_name) INTO CORRESPONDING FIELDS OF TABLE rows WHERE (condition) ORDER BY PRIMARY KEY.
    IF sy-subrc <> 0.
      CLEAR rows.
    ENDIF.
  ENDMETHOD.
  METHOD zif_abapiti_corpus~count.
    result = lines( rows ).
  ENDMETHOD.
  METHOD zif_abapiti_corpus~get.
    READ TABLE rows INTO DATA(row) INDEX idx.
    IF sy-subrc = 0.
      result = cl_abap_codepage=>convert_from( source = row-v ).
    ENDIF.
  ENDMETHOD.
  METHOD zif_abapiti_corpus~name.
    READ TABLE rows INTO DATA(name_row) WITH KEY idx = idx.
    IF sy-subrc = 0.
      result = name_row-name.
    ELSE.
      CLEAR result.
    ENDIF.
  ENDMETHOD.
  METHOD is_raw.
    DATA(raw) = NEW cx_sy_itab_line_not_found( ).
    result = boolc( raw IS INSTANCE OF cx_sy_itab_line_not_found ).
  ENDMETHOD.
  METHOD quote.
    result = value.
    REPLACE ALL OCCURRENCES OF `'` IN result WITH `''`.
    result = |'{ result }'|.
  ENDMETHOD.
ENDCLASS.
