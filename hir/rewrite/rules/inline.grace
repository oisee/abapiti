; Selection uses initial-round syntax, as hir.Inline does. Purity is deliberately
; not a guard: inline binds effects once, in source order. CHA narrowing does not
; relax the reference's static-class, closed-world no-override restriction.
(rule inline-dispatch 30
 (head (inline_dispatch ?caller ?s ?c ?name ?m))
 (base (inline_call ?caller ?s ?c ?name) (dispatch ?c ?name ?m)))
(rule inline-arity 30
 (head (inline_arity ?s))
 (base (inline_dispatch _ ?s _ _ ?m)
       (inline_arguments ?s ?n) (inline_parameters ?m ?n)))

(rule inline-forbidden 30
 (head (inline_forbidden ?m))
 (base (static _ ?m))
 (base (inline_abstract ?m))
 (base (inline_variadic ?m))
 (base (inline_name ?m class_constructor))
 (base (inline_name ?m constructor))
 (base (inline_name ?m ?name) (contains ?name "_instantiated_"))
 (base (inline_stmt ?m trap))
 (base (inline_stmt ?m throw))
 (base (inline_stmt ?m try))
 (base (inline_stmt ?m finally))
 (base (inline_stmt ?m while))
 (base (inline_stmt ?m foreach))
 (base (inline_stmt ?m break))
 (base (inline_stmt ?m continue))
 (base (inline_expr ?m super))
 (base (inline_seq_return ?m)))

(rule inline-candidate 20
 (head (inline_candidate ?m))
 (base (defined ?m) (inline_virtual ?m) (inline_block ?m) (inline_size ?m ?n)
       (le ?n 12) (not (inline_forbidden ?m))))

(rule inline-overridden 20
 (head (inline_overridden ?c ?name ?m))
 (base (inline_dispatch _ _ ?c ?name ?m) (inline_ancestor ?d ?c)
       (inline_decl ?d ?name ?other) (neq ?other ?m))
 (base (inline_dispatch _ _ ?c ?name ?m) (inline_ancestor ?d ?c)
       (inline_variant ?d ?name))
 (base (inline_dispatch _ _ ?c ?name ?m) (inline_owner ?m ?owner)
       (neq ?c ?owner) (inline_ancestor ?c ?a) (neq ?c ?a)
       (inline_ancestor ?a ?owner) (inline_variant ?a ?name)))

(rule inline-edge 10
 (head (inline_edge ?caller ?callee))
 (base (inline_candidate ?caller) (inline_candidate ?callee)
       (inline_dispatch ?caller ?s ?c ?name ?callee) (inline_arity ?s)
       (not (inline_overridden ?c ?name ?callee))))
(rule inline-path 10
 (head (inline_path ?a ?b))
 (base (inline_edge ?a ?b))
 (tail (inline_path ?a ?x) (inline_edge ?x ?b)))
(rule inline-allowed 0
 (head (inline_allowed ?s ?callee))
 (base (inline_dispatch _ ?s ?c ?name ?callee) (inline_arity ?s)
       (inline_candidate ?callee) (inline_template ?callee)
       (not (inline_overridden ?c ?name ?callee))
       (not (inline_path ?callee ?callee))))

(grace small-instance-method 10
 (match (node ?site virtual))
 (where (inline_allowed ?site ?callee))
 (action (inline ?site))
 (bound depth 64))
