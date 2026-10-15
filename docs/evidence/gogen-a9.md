Gogen A9 evidence

Baseline: `feat/gogen-a8`, `0f24e71ac69020bbf5164085f5aa0bc8e5fac882`.
OSG: `2349afd4`. Both supplied-kit runs used `MemoryMax=16G` and `nice`,
with reference hash prefix `61618e3b5618ce52` and identical coverage
(52 files, 47 loaded, 428 bodies, 216 executed, 3 unmatched).

```
base: full: 939 blocking (649 root, 290 cascade), 1916 HIR errors in 33 files
A9:   full: 886 blocking (620 root, 266 cascade), 1991 HIR errors in 33 files
```

| Refusal | Base | A9 |
| --- | ---: | ---: |
| `type reference Value is not a lowered declaration` | 5 | 0 |
| Anonymous `readonly name` / `readonly ctorParams` checker type | 5 | 0 |
| `object literal needs a named lowered shape` | 7 | 3 |

Remaining spreads: `frontend-provenance.mjs:399:47`, `frontend.mjs:1745:47`,
`frontend.mjs:2819:19`. Counts come from `evidence/registry-blocking.json`.

Validation passed: gofmt; requested vet and full test packages;
`ABAPITI_GRACE_FULL=1` and `ABAPITI_GRACECHECK=1` checks. Grace remains
1,927 classes / 64 interfaces; exported lexer facts match the golden exactly.
All seven default zabapgit Go sources match the base byte-for-byte.

Initially red Go-vs-Node oracles cover aliases, generic instantiations,
anonymous positions, and presence; additional oracles cover mapped graphs,
ordering, identity, null, undefined, and callable JSON. ABAP uses fixture-export
helpers and located refusals.
