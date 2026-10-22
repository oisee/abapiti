# Library outline differential

`files.json` contains small real ABAP inputs: class includes, visibility sections,
methods, attributes, structured DATA, program events, pragmas (including isolated
pragmas), Unicode, invalid syntax, an empty file, and a non-ABAP object.
`api_test.go.txt` runs against a freshly emitted full-core library via
`TestRegistryGoLibraryTree`, enabled by `ABAPITI_LIBRARY_TREE_DIR`.

`dump.go.txt` is a consumer of the public library API. It prints one JSON object
per registry object, sorted by type/name, including trees, concatTokens results
for **every file statement**, and CheckObject issues in their original order.
No issue sorting or deduplication is performed. The matching independent Node
oracle is `tools/library-tree-reference.mjs` and uses the published original
`@abaplint/core@2.120.73`. Non-ABAP objects are included as well.

Both drivers read a manifest with this shape:

```json
{"Files":[{"Filename":"ztest.prog.abap","Raw":"REPORT ztest."}],"Config":"{\"syntax\":{\"version\":\"open-abap\",\"errorNamespace\":\".\"},\"rules\":{\"parser_error\":true}}"}
```

Create the manifest without changing source bytes or logical names:

```sh
node tools/library-tree-manifest.mjs /absolute/path/to/frozen-m1-kit > m1.json
node tools/library-tree-manifest.mjs --fixtures > small.json
```

The M1 generator checks the lists, config, file count and ordered input hashes
against `../selfhost-73/m1/reference.json` before writing the manifest. The task
report records the frozen kit location and hashes. `deps.txt` is empty.

Emit `abapiti abaplint --target go --go-package lintlib -o OUTPUT`. Create a
consumer module requiring `zabaplint v0.0.0` with `replace zabaplint =>
/absolute/path/to/OUTPUT/go`, copy `dump.go.txt` to its `main.go`, and build it.
Run the compiled driver with the manifest as its sole argument. Independently
run:

```sh
node tools/library-tree-reference.mjs /absolute/path/to/node_modules/@abaplint/core manifest.json > node.jsonl
./tree-dump manifest.json > go.jsonl
cmp node.jsonl go.jsonl
sha256sum node.jsonl go.jsonl
```

Use the task's required 16 GiB systemd scopes and scratch TMPDIR/GOCACHE for
all Go builds/tests. The Node oracle reports counts to stderr; `FileStatements` and
`FileTokens` include every file statement; `StructureStatements` and
`StructureTokens` count only structure descendants.

The only API extension beyond the agreed initial fields is `Token.Pragma`.
Empty structures use zero positions because the original core's token accessors
throw when there are no children. All other coordinates come from the original
accessors. JSON uses Go's default string escaping on both sides and preserves
nil versus empty arrays.
