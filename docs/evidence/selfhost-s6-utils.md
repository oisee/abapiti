# selfhost-s6: boxed canonical array storage

Owner review: **abapiti-zone** (TypeScript frontend lowering).

Train: `70e4baf73e4303a66d9edb16aece1d5df233079c`.
Baseline: `9b7aef51cf7846249021cc502f558bed251253bb` (`origin/selfhost`).
Abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
OSG: `f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6`, clean checkout,
`npm ci` and `node tools/osd-libs.mjs`.

The failing titles are:

- `Name validator does not violate a rule if the pattern is ignored`
- `Name validator does not violate a rule if the name is ignored`

Both fail with `live_array.go: arrayView: incompatible native array reference`.
`git bisect run` on the two original test bodies found
`2f2fcaff5da2e5e43f15c7804184d53b3d878f4d` as the first bad commit;
`7d41594`, `8b625f1`, `2403393`, and `8e636df` pass. No revisions were skipped.
The scratch Mocha registration hook selected only these exact full titles;
test bodies and application sources were unchanged. Only the compiler changed
between revisions, with a fixed closure, runtime, and OSG bridge.

That commit forced every nonempty array literal with a dynamic destination to
`array<dynamic>`, including canonical `string[]` literals in boxed object
properties. ABAP represents that as native reference-array storage, whereas
`NamingRuleConfig.ignorePatterns` needs native string-array storage. The
live field getter recovers the boxed native reference with `arrayView` and
correctly refuses its incompatible storage class. Go's dynamic array view
can bridge those element representations, which explains the passing Go run.

Preserve the checker's canonical array element ABI when boxing a literal.
Keep the dynamic-element fallback for contextual tuples, including the
recursive tuple/capture cases supported by the original feature. This is a
frontend repair; the ABAP view's storage checks remain intact.

`TestBoxedCanonicalArrayLiteralStorage` fails on the untouched train with
`boxed canonical literal changed storage: array<dynamic>` and passes after
repair. It also checks Go execution against Node and ABAP emission. The
recursive tuple, mutual capture, and destructured capture Node oracles pass.
The original focused OSG tests change from 0 passed / 2 trapped to
2 passed / 0 trapped. The default generated Go lowering-trap count remains 2,
matching the baseline's 2.
