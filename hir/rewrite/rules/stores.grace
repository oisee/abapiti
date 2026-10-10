; The adapter proves exact types, one definition/use, and safe evaluation order.
; Liveness includes exceptional/finally continuations and definition kills.
(grace copy-local 20
 (match (node ?use local))
 (where (copy_proven ?use ?def ?v) (def ?v ?def) (use ?v ?use)
        (local_ref ?use ?v))
 (action (substitute-use ?use ?def)))
; store_next contracts the CFG to the first read or definition of this binding
; on every path. Other bindings are transparent; a definition kills the value.
(rule store-read-after 0
 (head (store_read_after ?v ?site))
 (base (store_next ?v ?site ?next) (use ?v ?next)))
(rule dead-definition 0
 (head (not_read_after ?v ?site))
 (base (store_inert ?site ?v) (def ?v ?site)
       (not (store_read_after ?v ?site))))
(grace dead-local-store 10
 (match (node ?site ?kind))
 (where (store_inert ?site ?v) (def ?v ?site)
        (not_read_after ?v ?site))
 (action (remove-statement ?site)))
