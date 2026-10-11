const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { Registry, INITIAL } = require('vscode-textmate');
const onig = require('vscode-oniguruma');
const { foldRanges } = require('../folding');

(async () => {
  const wasm = fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'));
  await onig.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
  const registry = new Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (patterns) => new onig.OnigScanner(patterns),
      createOnigString: (s) => new onig.OnigString(s)
    }),
    loadGrammar: async () => JSON.parse(fs.readFileSync(
      path.join(__dirname, '../syntaxes/grace.tmLanguage.json'), 'utf8'))
  });
  const grammar = await registry.loadGrammar('source.grace');
  let assertions = 0;
  function tokenize(text) {
    let state = INITIAL;
    return text.split(/\r?\n/).map((line) => {
      const result = grammar.tokenizeLine(line, state);
      state = result.ruleStack;
      return { line, tokens: result.tokens };
    });
  }
  function scope(lines, fragment, token, expected) {
    const entry = lines.find(({ line }) => line.includes(fragment));
    assert.ok(entry, `Missing line: ${fragment}`);
    const column = entry.line.indexOf(token, entry.line.indexOf(fragment));
    assert.ok(column >= 0, `Missing token: ${token}`);
    const actual = entry.tokens.find((t) => t.startIndex <= column && column < t.endIndex);
    assert.ok(actual?.scopes.includes(expected), `${token}: wanted ${expected}, got ${actual?.scopes}`);
    assertions++;
  }
  const samples = process.argv.slice(2);
  assert.ok(samples.length === 0 || samples.length === 2, 'Supply both analysis.grace and inline.grace');
  const files = samples.length ? samples : ['analysis', 'inline'].map((name) =>
    path.join(__dirname, '../../../hir/rewrite/rules', `${name}.grace`));
  const [analysis, inline] = files.map((file) => tokenize(fs.readFileSync(file, 'utf8')));
  scope(analysis, '; Closed-world', ';', 'comment.line.semicolon.grace');
  scope(analysis, '(rule subtype', 'rule', 'keyword.control.declaration.grace');
  scope(analysis, '(rule subtype', 'subtype', 'entity.name.function.rule.grace');
  scope(analysis, '(rule subtype', '0', 'constant.numeric.grace');
  scope(analysis, '(head (subtype', 'head', 'keyword.control.section.grace');
  scope(analysis, '(subtype ?c ?b)', 'subtype', 'entity.name.function.predicate.grace');
  scope(analysis, '(subtype ?c ?b)', '?c', 'variable.other.grace');
  scope(analysis, '_ _ ?s)', '_', 'constant.language.wildcard.grace');
  scope(analysis, '(not (narrowed', 'not', 'keyword.operator.logical.grace');
  scope(analysis, '(not (narrowed', 'narrowed', 'entity.name.function.predicate.grace');
  scope(analysis, '(tail (extends', 'tail', 'keyword.control.section.grace');
  scope(analysis, '(neq ?kind None)', 'None', 'constant.other.atom.grace');
  scope(inline, '(grace small-instance-method', 'grace', 'keyword.control.declaration.grace');
  scope(inline, '(grace small-instance-method', 'small-instance-method', 'entity.name.function.rule.grace');
  for (const section of ['match', 'where', 'action', 'bound'])
    scope(inline, `(${section} `, section, 'keyword.control.section.grace');
  scope(inline, '(match (node', 'node', 'entity.name.function.predicate.grace');
  scope(inline, '(bound depth', 'depth', 'keyword.other.depth.grace');
  scope(inline, '(bound depth', '64', 'constant.numeric.grace');
  scope(inline, '(contains ?name', '"', 'string.quoted.double.grace');
  scope(inline, '(inline_name ?m constructor)', 'constructor', 'constant.other.atom.grace');

  // Extra parser cases absent from the two samples: facts, escapes, keyword
  // spellings used as predicates/terms, and comments between list and symbol.
  const extra = tokenize(String.raw`(fact relation head -12 "a\";()")
(rule multiline -1
 (head (base ?x))
 (base (relation ?x head _extra 12abc "x\\y")
       (; predicate on the next line
         other ?x))
 (bound depth 8))`);
  scope(extra, '(fact relation', 'fact', 'keyword.control.declaration.grace');
  scope(extra, '(fact relation', 'relation', 'entity.name.function.predicate.grace');
  scope(extra, '(fact relation', 'head', 'constant.other.atom.grace');
  scope(extra, '(fact relation', '-12', 'constant.numeric.grace');
  scope(extra, '(fact relation', '\\"', 'constant.character.escape.grace');
  scope(extra, '(head (base', 'base', 'entity.name.function.predicate.grace');
  scope(extra, '(relation ?x head', 'head', 'constant.other.atom.grace');
  scope(extra, '(relation ?x head', '_extra', 'constant.other.atom.grace');
  scope(extra, '(relation ?x head', '12abc', 'constant.other.atom.grace');
  scope(extra, 'other ?x', 'other', 'entity.name.function.predicate.grace');
  assert.deepEqual(foldRanges('(rule x 0\n (head (p "(;\\\"")) ; )\n (base (p x)))'), [[0, 2]]);
  for (const file of files) assert.ok(foldRanges(fs.readFileSync(file, 'utf8')).length > 0);
  console.log(`PASS: ${assertions} scope assertions across both samples and edge cases; parenthesis folding passed.`);
  registry.dispose();
})().catch((error) => { console.error(error); process.exitCode = 1; });
