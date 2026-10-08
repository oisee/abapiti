FUNCTION z_abapiti_corpus_list.
  CLEAR et_files.
  IF iv_setname IS INITIAL.
    RETURN.
  ENDIF.
  SELECT idx name size sha256 FROM zabapiti_corpus INTO CORRESPONDING FIELDS OF TABLE et_files WHERE setname = iv_setname ORDER BY PRIMARY KEY.
ENDFUNCTION.
