# Trap batch K — train-8 long-tail audit

Base: `4b6a465c5663360a5f08cb249ef7cb936dfe36df`. Original upstream:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No upstream test, assertion,
registration or file selection is changed. No site override is added.

## Step 0: one complete base run

All 17 shards / 625 files: **10,934/11,060 active passes, FAIL 0, 123 traps,
35 not-run = 0 async/done + 28 skipped/focused + 7 unregistered**. Node has
11,060 passes, zero failures, 32 pending. Owned and whole-suite title multisets
reconcile; no exclusions, infrastructure errors, unmatched executions or borrowed
FAILs. [Base evidence](evidence/traps-k-base.json) includes **all 45 reached
message groups**, not just the top 20. Initialization refusals remain separate.
The summary tool now retains `allTraps`; a 22-group regression proves that the
long tail survives ranking and that its counts sum to the trapped total.

## Descending-order disposition of every base group

IDs below are the 1-based order in the base JSON's `allTraps`, including tie
ordering. Counts are reached bodies, not forecasts. Dell handoffs do not count
as repairs. Compound messages retain every diagnostic in the JSON.

| ID | Tests | Reached root | Disposition |
| --- | ---: | --- | --- |
| 1 | 29 | renamer `range` | Explicit exclusion: Dell's common edit-union field view. |
| 2 | 19 | objects/Domain conversion | Explicit exclusion. |
| 3 | 7 | pretty-printer `keyword_case` writes | Explicit exclusion. |
| 4 | 7 | NamingRuleConfig `patternKind` | Explicit exclusion. |
| 5 | 3 | ClassData conversion | Dell: erased statement/structure union and optional/heterogeneous `TypedIdentifier.getValue()` storage (see below). |
| 6 | 3 | LSP Rename factory assignment | Dell: mutable versioned/optional-nullable identifier layout; invariant field types differ. |
| 7 | 2 | ABAPObject text metadata | Dell: `parseInt(t.LENGTH,10)` may produce NaN; cannot fabricate maxLength 0 or omit its property. |
| 8 | 2 | AbstractObject XML subset | Repaired checked external-parser mode; `<foo></bar>` has Node's permissive result. Strict subset operation is unchanged. |
| 9 | 2 | Dynpros conversion | Dell: erased XML fields to required native string/number row storage, with absent fields retained. |
| 10 | 2 | MessageClass conversion | Dell: optional XML values / structured text flowing through native fields and Message arguments. |
| 11 | 2 | RenamerHelper conversion | Dell: TTYP/AUTH objects asserted as DataElement solely to call a common XML-edit method; sibling/common-method view, not a nominal instance. |
| 12 | 2 | View conversion | Dell: erased XML row values (including absent KEYFLAG/join fields) to native row layouts. |
| 13 | 2 | SpaceBeforeDot conversion | Dell: partial live rule-config reads and absent primitive fields; do not supply false/defaults on read. |
| 14 | 2 | WHEN TYPE recursive `fn_visit` | Repaired lexical recursive local-method publication and self binding. |
| 15 | 2 | Config `applyUnspecifiedRules` writes | Explicit exclusion. |
| 16 | 2 | pretty-printer compound config writes | Explicit exclusion (`keyword_case`, `sequential_blank`). |
| 17 | 2 | NumberRange global parseFloat | Repaired finite decimal-prefix parsing; nonfinite results still refuse. |
| 18 | 2 | BasicVariables padStart | Repaired typed UTF-16 padStart; subsequent missing-local cascade disappears. |
| 19 | 2 | CDS dynamic `get` / `getChildren` | Dell: erased node receiver needs checked original common-method dispatch, not a string operation. |
| 20 | 2 | Chai `.all.members` | Repaired dense-array unordered members assertion, not a no-op fluent chain. |
| 21 | 2 | CreateFile.is / DeleteFile.is | Predicates repaired independently; follow-up reaches `_apply_edit.ts:49` common edit-union `range`, handed to Dell. No body passes credited. |
| 22 | 2 | UnusedVariablesConf `skipAbstract` | Dell: inherited structural rule-config view/shape. |
| 23 | 1 | Chai enumerable keys | Dell: native ordered record storage versus Map shares physical representation; cannot invent enumerable keys or conflate Map with object. |
| 24 | 1 | Help constructor metadata | Dell: erased `statement.constructor.name` / class-descriptor reflection ABI. |
| 25 | 1 | Semantic classvalue narrowing | Dell: class descriptor is not a structural instance; keep invalid narrowing refusal. |
| 26 | 1 | Domain invalid AFF JSON | Dell handoff: native SyntaxError payload/subclass ABI plus JSON syntax diagnostics; strict parser's domain trap must not be swallowed as an ordinary exception. Not a Domain conversion repair. |
| 27 | 1 | EnhancementSpot conversion | Dell: erased/absent BADI_NAME and INTERFACE_NAME to required native IBadiDefinition fields. |
| 28 | 1 | FunctionGroup malformed XML | Repaired external parser's native Error payload for unterminated ordinary opening tags. |
| 29 | 1 | ProxyObject assignment | Dell: original `xmlToArray` erased rows assigned to native ParsedProxy.proxyData; requires live row/array view. |
| 30 | 1 | LineOnlyPunc conversion | Dell: partial live rule-config primitive reads, as SpaceBeforeDot. |
| 31 | 1 | Syntax recursive `fn_visit` | Same general recursive local-method repair as ID 14. |
| 32 | 1 | Types helper conversion | Dell: Program asserted as Class to invoke common object methods; preserve original runtime identity, not nominal downcast. |
| 33 | 1 | Domain test conversion | Explicit Domain exclusion, separate located test boundary. |
| 34 | 1 | CDS dynamic `getFirstToken` | Dell: erased original node common-method dispatch. |
| 35–38 | 1 each | Table zero-argument expect.fail + getComponents | Repaired zero-argument assertion failure and `never`-directed checker narrowing; no branch or assertion is deleted. |
| 39 | 1 | DumpScope ToPrimitive | Dell: native class string conversion requires original own/prototype `Symbol.toPrimitive`, toString/valueOf dispatch. No fabricated `[object Object]`. |
| 40 | 1 | Help delete + pretty JSON | Delete is Dell's original own-property/presence ABI. Literal pretty JSON is independently repaired, without crediting this blocked body. |
| 41 | 1 | BasicVariables compound `.all` | Same members repair as ID 20. |
| 42 | 1 | pretty-printer delete | Dell: live config own-property deletion/presence. |
| 43 | 1 | Registry delete | Dell: native structure deletion / original own keys and optional presence. |
| 44 | 1 | Config pretty JSON | Literal indentation repaired independently. Follow-up reaches Config's existing pinned strict-JSON package adapter on original JSON5 comments/trailing commas; Dell's config constructor/package-view handoff, not a credited pass or a new site override. |
| 45 | 1 | UnnecessaryPragmaConf `allowNoTextGlobal` | Dell: inherited config structural view/shape. |

### Concrete conversion handoffs

Generated base Go was inspected, not inferred from the generic range-fault label.
ClassData has checked union reference conversions and a `dynString(getValue())`
at the `as string` assignment. TypeScript's assertion does not turn undefined
or a structured value into a string; the erased destination must retain those
values. Fixing this requires absence/union storage, not returning an empty string.
ABAPObject's explicit located refusal is the finite-number parseInt adapter at
151:82, not its repaired string map keys. RenamerHelper's real sibling assertions
are at 115 and 133. ProxyObject's exact assignment is line 81. These are ABI
handoffs rather than site-specific downcast relaxation, copying rows, replacing
missing fields or weakening the verifier. Partial config reads remain live over
original storage and must distinguish omitted from explicitly assigned undefined.

## Owned general repairs and Node oracles

- **Recursive locals:** publish the complete lifted method signature/identity
  before lowering its body, and exclude its own checked binding from captures.
  Captured values/cells continue through the existing ABI. Duplicate lexical
  names have distinct methods. Pruned default lowering retains its old method
  publication/order. `local_recursion_test.go` checks traversal effects, recursive
  numeric results, captures and repeated lexical names against Node.
- **External XML:** `xml.parseWithErrors` is separate from `xml.parseSubset`.
  It reproduces the pinned parser's mismatched closing-name/incomplete-node
  behavior in the reviewed ordinary-tag grammar and throws an actual synthesized
  Error for unterminated opening tags, with Node's exact UTF-16 context/message.
  Catches and `instanceof Error` work through the existing native payload ABI.
  The two legacy XML site adapters are **retired in full closures**, not replaced
  by new site overrides: the generic registry `FullSource` policy validates their
  original fingerprints but lowers original source/catches through the general
  library operation. Pruned workloads keep the existing strict adapters and bytes.
  The registry regression retains stale-pin refusal and verifies pruned selection;
  the XML Node oracle proves a retired fixture's builder is never invoked.
  CDATA, DTD, unknown entities and other unreviewed languages still raise typed
  refusals which catches cannot swallow. `xml_parser_call_test.go` compares
  original fast-xml-parser 5.10.1 output, error messages and refusal boundaries.
- **Finite parseFloat:** typed string input, JS leading whitespace and longest
  decimal prefix, optional exponent, signed zero, rounding and underflow. Numeric
  assignment hints do not leak into the string argument; a contextual optional-
  number/erased-string regression preserves the actual NumberRange pattern. NaN
  and Infinity refuse before escaping as a HIR Number. No whole-string Go numeric
  conversion replaces prefix parsing. **padStart** evaluates receiver/length/fill
  once in order, truncates numeric length, defaults to space, pads UTF-16 code
  units, and refuses allocations above 1,048,576 units; empty fill and already-long
  strings do not allocate/refuse. `traps_k_strings_test.go` certifies supported
  results/effects and independently measures the nonfinite/refused Node cases.
- **Chai:** `.all.members` consumes one matching occurrence per element, with
  strict reference identity or existing deep-eql when `.deep` is requested.
  Length, multiplicity, empty arrays, negation and deep/shallow distinction are
  asserted. Sparse arrays explicitly refuse; the negative fixture still compares
  original Node. `expect.fail()` really throws `expect.fail()`; one-message fail
  and the genuine-failure fixture remain. Its `never` annotation gives the checker
  real terminating-flow information for the original Table guards, not a compiler
  override. Only explicit non-returning **function result annotations** erase to
  the existing void call ABI; checker `never` values/array elements do not become
  storable void. The full short gate caught that overly broad prototype mapping
  via the unchanged `TestGoArrayReceiverContextNodeOracle`; the mapping was
  narrowed and that Node regression passed before final certification.
  `chai_small_test.go` and `chai_members_test.go` use original Chai.
- **LSP predicates:** pinned CreateFile/DeleteFile predicates read original kind,
  uri, option fields and annotationId. Falsy nonboolean values are consumed only
  in conditions (including ternary conditions); value contexts refuse rather than
  turn null/undefined into false. Tagged JSON arrays have only indexed storage;
  custom property writes to this representation already refuse. No edit-union
  field/constructor layout is widened. `lsp_resource_guards_test.go` executes
  pinned vscode-languageserver-types 3.18.0 on Node.
- **Pretty JSON:** literal numeric/string gap, null/undefined replacer only.
  Format the existing compact writer's output without reparsing/reordering keys,
  renormalizing numbers, or changing native graph storage/hook domains. String gaps
  truncate to ten UTF-16 units; numeric gaps truncate/clamp to ten spaces.
  Replacers and dynamic gap/coercion remain refused. `json_indent_test.go` checks
  nested empty containers, punctuation/escapes, ordering, effects, numeric leaves,
  lone surrogates and gap truncation against Node. Help's native delete still blocks.

All seven new runtime operations have Node oracles and conservative effect facts.
ABAP explicitly refuses them; none widens the default reachability-pruned workload.

## Genuine prototype mismatch — stopped and reported first

The initial external XML extension returned `{"root":"tail"}` for `<root>tail`;
Node returns `{"root":""}`. Work stopped and the mismatch was reported before
any changed-source shard run. Root cause: committing unfinished EOF text. The
repair discards that final text segment, retaining already committed preceding
segments, and the exact case remains in the Node oracle. The expanded XML
regression passed before work resumed. No mismatch is accepted or relabeled.

Earlier exploratory attempts also hit ordinary harness/compiler refusals, not
accepted value mismatches: a recursive self binding became an unknown capture;
Node's CLI interpreted `-Infinity` as an option; a generated XML helper name
collided with the existing validator; an unavailable helper spelling caused a
Go build error; the predicate's ternary context was not recognized; and tagged
JSON arrays reached an unsupported property read. These were corrected with
regressions retained before certification.

The first pre-certificate shard attempt on `51f96c0` was stopped after that
short-gate refusal, before completing all shards. It is not counted as a final
suite measurement. The initial default diff had only a synthetic closure identity
shift (106100 to 106104); an independently archived compiler with the broad
checker-never change reverted recovered 106100. Corrected-source gates and the
same-path default comparison recovered raw `diff -r` exit 0; no identity diff is
accepted. A development full run on `1dc8749` measured **10,946 passes, FAIL 0,
111 traps, 35 not-run** and exposed two unrepaired follow-ups: the strict legacy
XML adapters still bypassed the new general call, and parseFloat inherited a
numeric assignment hint. Adapter retirement and argument-context regressions
were added before the final frozen-source rerun. This development result is not
the final certificate.

## Certification

Final counts, all remaining messages, gates, default-output diff and clean/seeded
workload comparisons will be recorded after the frozen-source final run. Only
compact Markdown/JSON is retained; generated builds, closures, binaries, archive,
kit, facts and logs are deleted. No push or transpiler clone.
