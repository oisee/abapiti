// TASK3 Addendum 3: one reviewed list of observable subset divergences.
// These are expectations, not omissions: a translated run must raise the
// specified exception with a nonempty explanation. All other fields compare.
export const documentedDivergences = Object.freeze([
 Object.freeze({name:'json5', exception:'RegistryJSONSubsetError', original:'return', reason:'strict JSON adapter rejects comments, unquoted keys, single quotes and trailing commas'}),
 Object.freeze({name:'malformed_config', exception:'RegistryJSONSubsetError', original:'throw', reason:'invalid JSON raises the adapter exception rather than JSON5-specific syntax diagnostics'}),
 Object.freeze({name:'malformed_xml', exception:'RegistryXMLSubsetError', original:'return', reason:'mismatched XML end tags are outside the supported fast-xml-parser subset'}),
]);
export const supportedObjectTypes = Object.freeze(['CLAS','INTF','PROG','TYPE','XSLT']);
export const excludedAPIs = Object.freeze(['Promise/async APIs']);

export function compareDocumentedDivergences(expected, actual) {
 const checked = [];
 for (const d of documentedDivergences) {
  const original = expected[d.name], translated = actual[d.name];
  if (!original || original.outcome !== d.original) throw new Error('stale documented divergence: '+d.name);
  const pass = translated?.outcome === 'throw' && translated.error === d.exception &&
   typeof translated.message === 'string' && translated.message.trim().length > 0;
  checked.push({...d, label:'documented divergence', pass, original, translated});
 }
 return checked;
}
