FUNCTION z_abapiti_corpus_put.
  DATA row TYPE zabapiti_corpus.
  DATA hash TYPE string.
  CLEAR ev_ok.
  ev_size = 0.
  ev_message = space.
  IF iv_setname IS INITIAL OR iv_idx <= 0 OR iv_name IS INITIAL OR strlen( iv_name ) > 255.
    ev_message = 'Invalid set name, index, or file name'.
    RETURN.
  ENDIF.
  TRY.
      DATA decoded TYPE xstring.
      decoded = cl_http_utility=>decode_base64( iv_data ).
      IF iv_part = 0.
        CLEAR row.
        row-setname = iv_setname.
        row-idx = iv_idx.
        row-name = iv_name.
        row-v = decoded.
        row-size = xstrlen( row-v ).
        DELETE FROM zabapiti_corpus WHERE setname = iv_setname AND idx = iv_idx.
        INSERT zabapiti_corpus FROM row.
      ELSE.
        SELECT SINGLE * FROM zabapiti_corpus INTO row WHERE setname = iv_setname AND idx = iv_idx.
        IF sy-subrc <> 0.
          ev_message = 'Part 0 is missing'.
          RETURN.
        ENDIF.
        IF row-size <> 393216 * iv_part.
          ev_message = 'Part number is not consecutive'.
          RETURN.
        ENDIF.
        CONCATENATE row-v decoded INTO row-v IN BYTE MODE.
        row-size = xstrlen( row-v ).
        UPDATE zabapiti_corpus FROM row.
      ENDIF.
      IF sy-subrc <> 0.
        ev_message = 'Database update failed'.
        RETURN.
      ENDIF.
      ev_size = row-size.
      IF iv_last = abap_true.
        CLEAR row-sha256.
        cl_abap_message_digest=>calculate_hash_for_raw( EXPORTING if_algorithm = 'SHA256' if_data = row-v IMPORTING ef_hashstring = hash ).
        row-sha256 = to_upper( hash ).
        IF to_upper( iv_sha256 ) <> row-sha256.
          ev_message = 'SHA-256 mismatch'.
          RETURN.
        ENDIF.
        UPDATE zabapiti_corpus FROM row.
        IF sy-subrc <> 0.
          ev_message = 'Hash update failed'.
          RETURN.
        ENDIF.
      ENDIF.
      COMMIT WORK.
      IF sy-subrc <> 0.
        ev_message = 'Commit failed'.
        RETURN.
      ENDIF.
      ev_ok = abap_true.
    CATCH cx_root.
      ROLLBACK WORK.
      ev_message = 'Upload failed'.
  ENDTRY.
ENDFUNCTION.
