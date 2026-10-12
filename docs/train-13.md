# Train 13: 11,060 of 11,060

Integration train 13 = train 12 (b7f9c60 content, 11,051) + batch O (reconciled with batch N and record deletion, d0ac0b9) + LineLength config (dell, 7835b36) + the view-cast nil-method guard and the transpiler generality probe (ba4f47d, d738cae).

| Run | Machine | Passed / Node-active | FAIL | Traps | Unregistered | Node-pending (outside) |
|---|---|---:|---:|---:|---:|---:|
| a6122d2 | i10 (dell, cross-check) | 11,060 / 11,060 | 0 | 0 | 0 | 32 |
| a6122d2 | u7 | 11,060 / 11,060 | 0 | 0 | 0 | 32 |

All 17 shards, 625 files, against a fresh original Node build of upstream 577f875. Short Go tests with Grace pass on both. `abapiti abaplint --full`: 19 blocking diagnostics, 54 HIR errors (core unpruned; these are paths no test reaches). Default ABAP output unchanged; five generated Go files differ from main, explained by batches N and O. zabapgit: Go = Node = kit (clean 0, seeded 5).

The 32 Node-pending tests are `it.skip` cases in upstream; they are not run on Node either and are outside the denominator. Earlier notes said 28 skipped/focused plus 4 pending unregistered StructureErrors rows; those 4 are pending too, so the total outside is 32.
