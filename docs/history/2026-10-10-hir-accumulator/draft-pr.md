# Add opt-in HIR accumulator variants for whole-array appends

With `ABAPITI_ACCUMULATOR=1`, a consumer that appends an entire child result
can pass its private output array to a generated `m_into` variant. Qualifying
fresh append-only producers push directly into that array. Other receivers
retain virtual dispatch through default variants that call the original method
and append its result. Both Go and ABAP use the shared pre-emission hook;
flag-OFF remains byte-identical to main.

Grace freshness, identity, use/last-use and resolved transitive exception facts
combine with conservative shape/escape checks. Each rewritten site reports
whether all targets are nonthrowing or a fresh private output is discarded on
exception. Families are keyed by name and signature, added interface methods
are never Abstract, and existing calls are rewritten before defaults are added.
General iteration and Sequence child reassignment are outside this change.

The pinned abaplint closure has 16 transformed consumers (Alternative 1,
Optional 1, Permutation 2, rules 2, other 10), 21 direct variants and 146 dispatch
fallbacks. Clean and seeded zabapgit output matches the expected bytes in all
normal and profile runs. Array site executions fall by 432,920 and 432,963.
See the adjacent timing/GC CSVs for interleaved median-of-three results and
per-site allocation CSVs for exact reductions.

Validation: go vet; go test -short ./...; lint gate against origin/main
(zero new issues, canary 5/5); flag-ON HIR tests and lexer differential (44/44);
full Go/native ABAP generation and default recursive byte comparison.
Guard tests check independent Grace-reference agreement and execute mixed
receiver dispatch in both flag states. A4H/OSGO execution remains with abapiti.
