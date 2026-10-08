REPORT zabapiti_log_setup.
DATA obj TYPE balobj.
DATA objt TYPE balobjt.
DATA subs TYPE STANDARD TABLE OF balsub WITH DEFAULT KEY.
DATA subts TYPE STANDARD TABLE OF balsubt WITH DEFAULT KEY.
START-OF-SELECTION.
  obj-object = 'ZABAPITI'.
  objt-spras = 'E'.
  objt-object = 'ZABAPITI'.
  objt-objtxt = 'abapiti runs'.
  subs = VALUE #( ( object = 'ZABAPITI' subobject = 'BENCH' ) ( object = 'ZABAPITI' subobject = 'LOAD' ) ( object = 'ZABAPITI' subobject = 'CHECK' ) ).
  subts = VALUE #( ( spras = 'E' object = 'ZABAPITI' subobject = 'BENCH' subobjtxt = 'benchmarks' )
                   ( spras = 'E' object = 'ZABAPITI' subobject = 'LOAD' subobjtxt = 'corpus loads' )
                   ( spras = 'E' object = 'ZABAPITI' subobject = 'CHECK' subobjtxt = 'abaplint checks' ) ).
  MODIFY balobj FROM obj.
  MODIFY balobjt FROM objt.
  MODIFY balsub FROM TABLE subs.
  MODIFY balsubt FROM TABLE subts.
  COMMIT WORK.
  WRITE / 'ZABAPITI log object ready'.
