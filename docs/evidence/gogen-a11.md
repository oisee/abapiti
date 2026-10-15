Gogen A11 evidence

Base: `feat/gogen-a9`, `6bd1fa7`. OSG: `2349afd4`.
Both measurements use the supplied `measure-step4.sh`, `nice`, and a
`systemd-run --user --scope` with `MemoryMax=16G`. Reference hash:
`61618e3b5618ce52`; coverage: 52 files, 47 loaded, 428 bodies,
216 executed, 3 unmatched.

```
base: full: 886 blocking (620 root, 266 cascade), 1991 HIR errors in 33 files
A11:  full: 882 blocking (616 root, 266 cascade), 1991 HIR errors in 33 files
```

| Refusal | Base | A11 |
| --- | ---: | ---: |
| `tagged record spread needs object storage` | 3 | 0 |
| `object literal needs a named lowered shape` | 3 | 0 |

Plain string-keyed records now copy present entries into the tagged property
snapshot. Data-interface views use the same runtime own-key enumeration as
shape values. Distributed spread results synthesize a common-property data
view while retaining variant-specific keys in the snapshot. Record-to-record
copies preserve optional stored values without a redundant narrowing.

All three previously refused shape sites now resolve a structural destination.
The storage diagnostics at remaining pilot sites were secondary to refused operands (`typed`, `fsType.k`,
`Object.fromEntries`, `options.kind`); their original located diagnostics remain,
without an additional storage refusal for an operand that produced no value.
The pilot still has unrelated lowering and HIR verification failures.

Initially red Go-vs-Node oracles cover record spreads, optional values,
multiple sources, named/contextual and inferred shapes, partial unions,
integer-key ordering, absent versus present-undefined, overwrites, evaluation
order, and snapshot isolation. ABAP emission uses fixture-export helpers;
unsupported JSON views retain located refusals. A negative regression keeps
the operand refusal without a spurious storage diagnostic.

Validation: gofmt; requested vet and full test packages; full Grace closure
(1,927 classes, 64 interfaces); lexer Grace facts unchanged. All seven default
zabapgit Go sources match the base byte-for-byte.
