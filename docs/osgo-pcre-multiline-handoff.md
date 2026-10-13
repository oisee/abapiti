# O3+O4 multiline PCRE handoff

Owner: **stoker / OSG kernel PCRE**. Host:
`98fa0675f47104ded037083c453ac0bb33b5ff57`. Compiler executable tree:
`cd5b643`. Upstream abaplint:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The final [train measurements](train-o3.md#final-o3o4-integration-2026-10-13)
compile all seven requested shards, but `abap/types` and `abap` abort before
returning trustworthy reports. Their 29 and 216 Node-active titles remain
no-report. Abap additionally has four Node-pending titles outside that count.
The other five reports total 5,307 passes, eight traps and zero FAIL.

## Exact abap refusal

Owned title: `abap_parser macro in CASE, no unknown expected`.
Upstream test: `packages/core/test/abap/abap_parser.ts:48`.
Raw PCRE pattern: `&1`. Pattern after OSG extended-mode normalization: `&1`.
Input, as an escaped string: `"WHEN &1.\n"`. Global replacement: `'a'`.

Upstream `packages/core/src/abap/2_statements/expand_macros.ts:182–203`,
`ExpandMacros.expandContents`, appends a newline to each macro statement,
constructs `new RegExp("&" + i, "g")`, and replaces the argument. The emitted
ABAP preserves the pattern and `g` flag; its replacement method calls
`FIND ALL OCCURRENCES OF PCRE posix IN text RESULTS matches`.
Node's independent control returns `"WHEN 'a'.\n"`.

The captured call chain is:

```text
ABAPParser.parse
StatementParser.run
ExpandMacros.handleMacros
ExpandMacros.expandContents
regexp.replace
abap.FindResults
abap.compilePCRE
```

## Exact types refusal

Owned title: `Types, class_definition method, FOR LOCK, derived signature, entity found`.
Upstream test: `packages/core/test/abap/types/class_definition.ts:91`.
Raw and normalized PCRE pattern: the single literal **U+00A0**, represented as
`"\u00a0"` in the diagnostic log. It is not the six characters of a backslash
escape. Replacement: one ordinary space. Exact input:

```text
"\ndefine root view entity zi_lockentity as select from zfoo {\n  key field1\n}"
```

Upstream `packages/core/src/cds/cds_lexer.ts:68`, `CDSLexer.run`, normalizes
NBSP using `.replace(/\u00a0/g, " ")`. This input contains no NBSP, so Node
returns it unchanged. The captured call chain is
`Registry.parse → DataDefinition.parse → CDSParser.parse → CDSLexer.run →
regexp.replace → abap.FindResults → compilePCRE`.

## Refusal site and ownership evidence

At the pinned OSG commit, `tools/gogen/go/abap/find.go:338–339`, in
`compilePCRE`, contains this admission guard:

```go
if strings.ContainsAny(s, "\n\r") && ((!strings.Contains(plain, `\A`) && !strings.Contains(plain, `\z`)) || strings.ContainsAny(outsideClasses(p), "^$.")) {
    panic(NotCompiled("FIND PCRE", "a text with line ends: PCRE's ^ $ and . around them are not measured"))
}
```

Both patterns contain no anchors or dot operators. The first disjunct refuses
unanchored literal matching on newline-containing text. Both raw/normalized
captures preserve the intended literal, and both calls are reproduced directly
in the original kernel without an abapiti emitter or regex normalizer.
The refusal escapes the ABAP exception boundary; the host bridge exits 2 and
the accounting commands exit 1. This is an OSG domain-admission boundary.
Neither reproducer contains a hexadecimal escape, so #721's escape support
does not remove it.

A minimal reproducer in a scratch command package of the pinned gogen Go module:

```go
package main

import (
    "fmt"
    "osg/gogen/abap"
)

func probe(input, pattern string) {
    defer func() { fmt.Printf("pattern=%q input=%q refusal=%v\n", pattern, input, recover()) }()
    fmt.Println(abap.FindResults(input, pattern, 'P', false, true))
}

func main() {
    probe("WHEN &1.\n", "&1")
    probe("\ndefine root view entity zi_lockentity as select from zfoo {\n  key field1\n}", "\u00a0")
}
```

Both calls were executed against an unmodified copy of the pinned Go kernel;
both return the exact `NOT_COMPILED in FIND PCRE` refusal above. The direct Node
controls produce the expected replacement and unchanged CDS input respectively.
Full-shard diagnostic replays only add raw-pattern/input/stack logging to a
separate generated runtime copy and preserve the existing panic. Those replays
are attribution evidence; they do not replace the original shard measurements.

Requested host follow-up: measure PCRE literal/global matching and replacement
on line-ended input, including a literal absent from the input, and establish
an admitted Go/JS kernel domain for those cases. Then rerun `abap/types` and
`abap`, retain the active/pending distinction, and check for any later refusal.
No claim is made that removing this first boundary will complete either shard.
No OSG fix or refusal bypass was made in this train; its checkout remains clean
at the pin. All diagnostics remain local under `.tmp/o3/final/`.
