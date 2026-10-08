REPORT zabapiti_corpus_load.
PARAMETERS p_path TYPE char255 LOWER CASE DEFAULT '/tmp/abapiti/abapiti-drop.zip'.
PARAMETERS p_repl TYPE abap_bool AS CHECKBOX DEFAULT 'X'.
START-OF-SELECTION.
  IF zcl_abapiti_corpus=>load_zip( iv_path = p_path iv_replace = p_repl ) = abap_true.
    WRITE / 'Load finished'.
  ELSE.
    WRITE / 'Load failed'.
  ENDIF.
