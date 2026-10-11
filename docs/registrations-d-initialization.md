# Batch D: complete baseline initialization inventory

This is the fresh all-17-shard baseline at `2169c5d`: **37 refused actions in
11 files**, all typed `trapped` records, no genuine initialization failures.
Messages below are copied verbatim from the normalized
[baseline JSON](abaplint-tests-registrations-base.json); repeated messages retain
their distinct action locations. Counts describe initialization actions, not
assertion results. See [implementation and boundaries](registrations-d-roots.md)
and the [final measurement](abaplint-tests.md).

## Immutable function aliases and their table/registration cascades

### `test/abap/combi.ts`

Action `test/abap/combi.ts:9:1`, status `trapped`:

```text
test/abap/combi.ts:9:1: lowering trap: unsupported-type test/abap/combi.ts:9:7: checker type (s: string) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:10:1`, status `trapped`:

```text
test/abap/combi.ts:10:1: lowering trap: unsupported-type test/abap/combi.ts:10:7: checker type (first: InputType, second: InputType, ...rest: InputType[]) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:11:1`, status `trapped`:

```text
test/abap/combi.ts:11:1: lowering trap: unsupported-type test/abap/combi.ts:11:7: checker type (first: InputType, second: InputType, ...rest: InputType[]) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:12:1`, status `trapped`:

```text
test/abap/combi.ts:12:1: lowering trap: unsupported-type test/abap/combi.ts:12:7: checker type (first: InputType, second: InputType, ...rest: InputType[]) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:13:1`, status `trapped`:

```text
test/abap/combi.ts:13:1: lowering trap: unsupported-type test/abap/combi.ts:13:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:14:1`, status `trapped`:

```text
test/abap/combi.ts:14:1: lowering trap: unsupported-type test/abap/combi.ts:14:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:15:1`, status `trapped`:

```text
test/abap/combi.ts:15:1: lowering trap: unsupported-type test/abap/combi.ts:15:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:16:1`, status `trapped`:

```text
test/abap/combi.ts:16:1: lowering trap: unsupported-type test/abap/combi.ts:16:7: checker type (r: RegExp) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:17:1`, status `trapped`:

```text
test/abap/combi.ts:17:1: lowering trap: unsupported-type test/abap/combi.ts:17:7: checker type (t: new (p: Position, s: string) => AbstractToken) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:18:1`, status `trapped`:

```text
test/abap/combi.ts:18:1: lowering trap: unsupported-type test/abap/combi.ts:18:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:19:1`, status `trapped`:

```text
test/abap/combi.ts:19:1: lowering trap: unsupported-type test/abap/combi.ts:19:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi.ts:21:1`, status `trapped`:

```text
test/abap/combi.ts:21:1: lowering trap: unsupported-call test/abap/combi.ts:22:18: call to KindIdentifier is not lowered
```

Action `test/abap/combi.ts:110:1`, status `trapped`:

```text
test/abap/combi.ts:110:30: lowering trap: unsupported-expr test/abap/combi.ts:111:3: identifier tests is not a lowered local or module value; unsupported-expr test/abap/combi.ts:111:3: identifier tests is not a lowered local or module value
```

### `test/abap/combi_size.ts`

Action `test/abap/combi_size.ts:10:1`, status `trapped`:

```text
test/abap/combi_size.ts:10:1: lowering trap: unsupported-type test/abap/combi_size.ts:10:7: checker type (s: string) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi_size.ts:11:1`, status `trapped`:

```text
test/abap/combi_size.ts:11:1: lowering trap: unsupported-type test/abap/combi_size.ts:11:7: checker type (first: InputType, second: InputType, ...rest: InputType[]) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi_size.ts:12:1`, status `trapped`:

```text
test/abap/combi_size.ts:12:1: lowering trap: unsupported-type test/abap/combi_size.ts:12:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi_size.ts:13:1`, status `trapped`:

```text
test/abap/combi_size.ts:13:1: lowering trap: unsupported-type test/abap/combi_size.ts:13:7: checker type (first: InputType) => IStatementRunnable is not a lowered declaration
```

Action `test/abap/combi_size.ts:30:1`, status `trapped`:

```text
test/abap/combi_size.ts:30:1: lowering trap: unsupported-call test/abap/combi_size.ts:31:7: call to KindIdentifier is not lowered
```

Action `test/abap/combi_size.ts:51:1`, status `trapped`:

```text
test/abap/combi_size.ts:51:31: lowering trap: unsupported-expr test/abap/combi_size.ts:52:3: identifier resultSize is not a lowered local or module value; unsupported-expr test/abap/combi_size.ts:52:3: identifier resultSize is not a lowered local or module value
```

## Heterogeneous registration-row context

### `test/abap/nesting.ts`

Action `test/abap/nesting.ts:251:1`, status `trapped`:

```text
test/abap/nesting.ts:251:31: lowering trap: unsupported-expr test/abap/nesting.ts:253:15: property n is not lowered; unsupported-expr test/abap/nesting.ts:253:42: property top is not lowered
```

## Nominal/closure type or ABI verification (Dell ownership)

### `test/abap/structure_errors.ts`

Action `test/abap/structure_errors.ts:15:1`, status `trapped`:

```text
test/abap/structure_errors.ts:15:54: lowering trap: test/abap/structure_errors.ts:15:54: HIR verification: node 33034 (test/abap/structure_errors.ts:17:13): invalid narrowing classref<shape.c4b328c5aa10399a63c13fd4296a6a7727d5bb372e83f88119bf41b22f8b727c> to classref<shape.a075496bed66948e99eac227f642b108931b76f9160ec3ce3d32fd5c1860f414>
```

### `test/abap/expressions/reuse.ts`

Action `test/abap/expressions/reuse.ts:10:1`, status `trapped`:

```text
test/abap/expressions/reuse.ts:10:1: lowering trap: test/abap/expressions/reuse.ts:10:1: HIR verification: node 30081 (test/abap/expressions/reuse.ts:17:3): call argument type mismatch: optional<classref<src/abap/2_statements/expressions/field_chain.ts.FieldChain>> <- classref<src/abap/2_statements/expressions/field_length.ts.FieldLength> in new .
```

Action `test/abap/expressions/reuse.ts:70:1`, status `trapped`:

```text
test/abap/expressions/reuse.ts:70:54: lowering trap: test/abap/expressions/reuse.ts:70:54: HIR verification: node 30296 (test/abap/expressions/reuse.ts:71:3): unresolved field test.ready.tests
```

### `test/abap/syntax/syntax.ts`

Action `test/abap/syntax/syntax.ts:16428:1`, status `trapped`:

```text
test/abap/syntax/syntax.ts:16428:61: lowering trap: test/abap/syntax/syntax.ts:16428:61: HIR verification: node 72276 (test/abap/syntax/syntax.ts:16428:61): expression type mismatch: classref<test/abap/syntax/syntax.ts.module> / classref<closure.72232>
```

## Local class declaration

### `test/abap/structures/_combi.ts`

Action `test/abap/structures/_combi.ts:276:1`, status `trapped`:

```text
test/abap/structures/_combi.ts:276:39: lowering trap: unsupported-statement test/abap/structures/_combi.ts:277:3: KindClassDeclaration is not lowered; unsupported-expr test/abap/structures/_combi.ts:282:25: identifier Normal is not a lowered local or module value
```

## Mutable module binding (no registration loss)

### `test/abap/types/form_definition.ts`

Action `test/abap/types/form_definition.ts:8:1`, status `trapped`:

```text
test/abap/types/form_definition.ts:8:1: lowering trap: unsupported-top-level test/abap/types/form_definition.ts:8:1: mutable module variables are not lowered
```

## Module title arrow and heterogeneous local registration rows

### `test/pretty_printer/pretty_printer.ts`

Action `test/pretty_printer/pretty_printer.ts:11:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:11:1: lowering trap: unsupported-type test/pretty_printer/pretty_printer.ts:11:7: checker type (text: string) => string is not a lowered declaration
```

Action `test/pretty_printer/pretty_printer.ts:19:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:19:49: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:38:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:48:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:48:41: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:57:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:67:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:67:50: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:76:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:86:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:86:47: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:100:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:110:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:110:30: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:157:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:170:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:170:49: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:179:8: call to KindIdentifier is not lowered
```

Action `test/pretty_printer/pretty_printer.ts:192:1`, status `trapped`:

```text
test/pretty_printer/pretty_printer.ts:192:54: lowering trap: unsupported-call test/pretty_printer/pretty_printer.ts:231:8: call to KindIdentifier is not lowered
```

## Runtime conversion domain (batch C ownership)

### `test/rules/keyword_case.ts`

Action `test/rules/keyword_case.ts:289:1`, status `trapped`:

```text
src/rules/keyword_case.ts:129:3: value outside supported numeric, ordering or conversion domain
```

Action `test/rules/keyword_case.ts:304:1`, status `trapped`:

```text
src/rules/keyword_case.ts:129:3: value outside supported numeric, ordering or conversion domain
```

### `test/rules/line_length.ts`

Action `test/rules/line_length.ts:17:1`, status `trapped`:

```text
src/rules/line_length.ts:35:3: value outside supported numeric, ordering or conversion domain
```

