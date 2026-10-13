// Only the test driver owns these boundaries. NOT_COMPILED is a host refusal,
// not an ABAP exception: source TRY/CATCH must never intercept it. Recover it
// after the real registration action/body has unwound, and let the driver
// record a trap. Assertions, runtime dumps and unknown panics still propagate.
export function protectTestBoundaries(go, program) {
 const runner = program.classes.find(c => c.name === 'ZCL_ABAPITI_TEST_RUNNER');
 if (!runner) throw new Error('missing OSG test runner');
 const methods = runner.methods.filter(m => /^HOST_(?:BODY|INIT_\d+)$/.test(m.name));
 if (!methods.some(m => m.name === 'HOST_BODY')) throw new Error('missing OSG body boundary');
 for (const method of methods) {
  const name = `ZCL_ABAPITI_TEST_RUNNER_${method.name}`;
  const signature = new RegExp(`^func ${name}\\([^\\n]*\\) (?:\\(result string\\) )?\\{`, 'gm');
  const matches = [...go.matchAll(signature)];
  if (matches.length !== 1) throw new Error(`missing or ambiguous OSG boundary: ${name}`);
  const header = matches[0][0];
  const body = method.name === 'HOST_BODY';
  if (body && !header.includes('message *string')) throw new Error('OSG body refusal output ABI changed');
  if (!body && !header.includes('(result string)')) throw new Error('OSG initialization refusal output ABI changed');
  go = go.replace(header, header + `
 defer func() {
  if x := recover(); x != nil {
   if e, ok := abap.AsError(x); ok && e.Class == "NOT_COMPILED" {
    ${body ? '*message' : 'result'} = e.Error()
    ${body ? '*settled = nil' : ''}
    return
   }
   panic(x)
  }
 }()
`);
 }
 // Only the final driver's CATCH INTO variables are native Exception unions.
 // Keep get_text's result and append its exact class identity for diagnostics;
 // neither source handlers nor assertion/coverage catches are changed.
 const run = /^func ZCL_ABAPITI_TEST_RUNNER_RUN\([^\n]*\n[\s\S]*?(?=^func |$(?![\s\S]))/m;
 const runs = [...go.matchAll(new RegExp(run.source, 'gm'))];
 if (runs.length !== 1) throw new Error('missing or ambiguous OSG report boundary');
 let driver = runs[0][0];
 if (!driver.includes('test_error.TextOf(s)') || !driver.includes('init_error.TextOf(s)')) throw new Error('OSG exception diagnostic ABI changed');
 driver = driver.replace(/\b(test_error|init_error)\.TextOf\(s\)/g, (text, receiver) => `(${text} + " [" + driverExceptionName(${receiver}.Class) + "]")`);
 go = go.replace(runs[0][0], driver);
 return go;
}
