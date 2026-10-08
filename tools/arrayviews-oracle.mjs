// Original JS observations for tsfront/testdata/arrayviews/probe.ts (Node type stripping).
const {Probe} = await import(new URL("../tsfront/testdata/arrayviews/probe.ts", import.meta.url));
const out = {};
for (const m of ["pushOnRoot", "loops", "write", "chain", "single", "negative"]) out[m] = Probe[m]();
console.log(JSON.stringify(out, null, 1));
