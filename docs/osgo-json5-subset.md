# JSON5 comments and trailing commas on TS-HA@OSGO

The ABAP `json5.parse` operation normalizes comments and trailing commas,
then calls the existing strict JSON parser. It does not claim full JSON5
support: unquoted keys, single quotes, hexadecimal numbers, nonfinite values,
JSON5-specific escapes and other extensions receive a typed coverage refusal.
Invalid input also receives a coverage refusal, rather than a fabricated JS
SyntaxError. The Go JSON5 implementation remains unchanged.

The normalizer uses a seven-state scanner with windows of 128 UTF-16 units.
CO/CN skip runs of ordinary characters and whitespace; CA locates string
escapes, comment terminators and line endings. Each operator's `sy-fdpos`
is captured before subsequent comparisons. State persists across windows.
There are no regular expressions in normalization. The existing strict JSON
number validator is reused unchanged.

Edits reference original source slices. Comments become one space so tokens
cannot accidentally join; a comma is removed only before a closing bracket
or brace and after a possible value terminator. The strict parser validates
the resulting grammar. String contents, escapes and numeric spelling,
including `-0`, are preserved. Fragments are joined once, and strict JSON
without edits is returned directly.

## Host prerequisite

The previous OSG pin did not update `sy-fdpos` for CO/CA comparisons.
The isolated `fix/gogen-character-set-fdpos` branch fixes both Go and JS
emitters; CN/NA retain the same position while negating the result.
This branch is pushed and tested in [draft PR #719](https://github.com/oisee/open-steamgate/pull/719), but has not been merged. The abapiti pin
in this branch points at its immutable commit `fd07c7928`.

Focused helper tests pass; the existing emitter/runtime suites give 50 passes.
An ABAP Unit fixture with all four operators passes on both Go and JS hosts.
The shared original OSG checkout is untouched.

## Differential verification

[Unit evidence](evidence/osgo-json5-subset-unit.json) records a successful
ABAP Unit execution against real `json5@2.2.3` answers:

- 50 valid inputs, including strings that contain comment-like text,
  CR/LF and Unicode comment line endings, duplicate keys, nesting and
  boundaries around 128 and 256 units.
- 22 invalid inputs that the real JSON5 parser rejects.
- 11 valid JSON5 inputs outside the supported subset, explicitly refused.
- The normalizer preserves the text `-0`.

This also found an existing strict-parser delimiter bug: its string token
scanner compared a space against the fixed-width `space` operand. It now
uses a string literal containing a space. Comments immediately after a
number exercise the regression, such as `{"a":1,/*c*/}` becoming `{"a":1 }`.

Local short tests, vet, lint (0 new issues, canary 5/5), and uncached Grace
lexer facts pass. Default Go generation at the same path is byte-identical
to baseline `5822a07`, including the executable (raw diff exit 0, 0 bytes).

Full lexer, statements and root shard measurements are in progress.
No full-suite pass claim or merge has been made. The four sparse-array
regressions documented in [osgo-chai.md](osgo-chai.md) still require a
presence-aware storage ABI; these guards have not been weakened.

The overall goal remains 11,060 active passes, zero wrong answers, traps or
unregistered tests on TS-HA@OSGO, followed by the same suite on TS-HA@A4H.
