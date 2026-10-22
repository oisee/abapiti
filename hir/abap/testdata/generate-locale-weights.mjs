// Development-time only: node hir/abap/testdata/generate-locale-weights.mjs
// Output is embedded in locale_compare.go; no Node dependency at runtime.
import assert from 'node:assert/strict';
const chars = Array.from({length: 95}, (_, i) => String.fromCharCode(i + 32));
const tertiary = new Intl.Collator('en');
const primary = new Intl.Collator('en', {sensitivity: 'base'});
const weights = chars.map(c => chars.filter(d => primary.compare(d, c) < 0).length);
const cases = chars.map(c => Number(c >= 'A' && c <= 'Z'));
function compare(a, b) {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const d = weights[a.charCodeAt(i) - 32] - weights[b.charCodeAt(i) - 32];
    if (d) return Math.sign(d);
  }
  if (a.length !== b.length) return Math.sign(a.length - b.length);
  for (let i = 0; i < n; i++) {
    const d = cases[a.charCodeAt(i) - 32] - cases[b.charCodeAt(i) - 32];
    if (d) return Math.sign(d);
  }
  return 0;
}
// All printable ASCII pairs determine the order and the case equivalences.
for (const a of chars) for (const b of chars) {
  assert.equal(compare(a, b), Math.sign(tertiary.compare(a, b)), JSON.stringify([a,b]));
  assert.equal(Math.sign(tertiary.compare(a,b)), Math.sign(a.localeCompare(b)));
}
// Primary differences and length must precede case differences.
for (const a of chars) for (const b of chars) for (const suffix of ['', 'a', 'A', '_', ' ']) {
  assert.equal(compare(a + suffix, b + 'A'), Math.sign(tertiary.compare(a + suffix, b + 'A')));
}
console.log('// Node ' + process.versions.node + ', ICU ' + process.versions.icu);
console.log('const localePrimaryWeights = "' + weights.map(w => String(w).padStart(2, '0')).join('') + '"');
console.log('const localeTertiaryWeights = "' + cases.join('') + '"');
