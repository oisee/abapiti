# Grace

VS Code language support for `*.grace` rule files: syntax highlighting, semicolon
line comments, parentheses and quote pairing, and folding by matched parentheses.
Folding ignores parentheses inside strings and comments.

## Install

Download or build `grace-language-0.0.1.vsix`, then run:

```sh
code --install-extension grace-language-0.0.1.vsix
```

Open a `.grace` file, or select **Grace** from the language mode picker.

## Syntax

The grammar follows `grace/parse.go` and its rewrite parser in `grace/match.go`:

- Declarations: `rule NAME PRIORITY`, `grace NAME PRIORITY`, and `fact`.
- Sections: `head`, `base`, `tail`, `bound depth`, `match`, `where`, and `action`.
- Negation: `(not (predicate ...))`.
- Terms: `?variables`, wildcard `_`, signed integer numbers, double-quoted strings
  with escapes, and unquoted atoms.
- Predicate/relation names occupy the first position in an atom, including atoms
  in heads, conjunctions, negations, rewrite sections, and facts.

Keywords are structural; the same spelling used as a term remains an atom.
Predicates are open-ended, as in the parser. Numeric terms receive integer
highlighting; the parser stores term values as strings. This extension highlights
syntax and does not validate rules or evaluate them.

## Develop and package

From `editors/vscode-grace`:

```sh
npm ci
npm test
npx @vscode/vsce package
```

The test uses VS Code's TextMate engine and Oniguruma to tokenize both repository
samples in `hir/rewrite/rules`. To test another copy of the samples:

```sh
npm test -- /path/to/analysis.grace /path/to/inline.grace
```

The VSIX is a local build artifact and is not committed. Licensed under MIT.
