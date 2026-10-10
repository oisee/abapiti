; Analysis only. Every blocker carries a structural site and source provenance.
; Fail means a witnessed violation OR a conservative, explicitly labelled
; inability to prove the condition. Optional uses require an absent flag.
(rule vo-blocked 0 (head (vo_blocked ?c ?condition))
 (base (vo_violation ?c ?condition _ _ _)))
(rule vo-condition 0 (head (vo_pass ?c ?condition))
 (base (vo_class ?c _) (vo_condition ?condition) (not (vo_blocked ?c ?condition))))
(rule vo-immutable 0 (head (vo_immutable ?c)) (base (vo_pass ?c 1)))
(rule vo-identity-free 0 (head (vo_identity_free ?c)) (base (vo_pass ?c 2)))
(rule vo-closed 0 (head (vo_closed ?c)) (base (vo_pass ?c 3)))
(rule vo-absent 0 (head (vo_absent_flag ?c)) (base (vo_optional ?c _ _ _)))
(rule vo-value-object 0 (head (value_object ?c))
 (base (vo_immutable ?c) (vo_identity_free ?c) (vo_closed ?c) (vo_pass ?c 5)))
