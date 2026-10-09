FUNCTION z_abapiti_corpus_delete_set.
  IF iv_setname IS INITIAL.
    ev_count = 0.
    RETURN.
  ENDIF.
  DELETE FROM zabapiti_corpus WHERE setname = iv_setname.
  ev_count = sy-dbcnt.
  COMMIT WORK.
ENDFUNCTION.
