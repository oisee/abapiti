The step-4 harness compares gogen compiled to Go by abapiti with the original
Node gogen driver on OSG's captured reference inputs:

```sh
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
export GOCACHE=/mnt/ssd-510/osg/go-build
export TMPDIR=/mnt/ssd-510/osg/abapiti/tmp/step4-e2e
python3 tools/selfhost-step4.py <abapiti-bin> <osg-checkout> <abaplint> <transpiler> <out-dir>
```

Use an empty output directory and a disposable OSG checkout with `npm ci`,
`osd-libs`, and `.local/step4/step4-driver.mjs` / `step4-inputs.json` prepared.
The pinned reference SHA256 is
`a13702d55b1eecdb200078205da5909961b4b49ce0f6c3fe3bdbf08e802d6227`.
Both executions run in the supplied OSG checkout. Input folders must point to
its prepared libraries. Abaplint core sources and the transpiler rearranger
are resolved through tsconfig paths, as in the pilot measurement kit.

Every Node, abapiti, generated program and Go build subprocess runs under
`systemd-run --user --scope -q -p MemoryMax=16G nice` (24G for the generated
module build). The harness checks for at least 40 GiB free before each step.
It executes heavy steps sequentially. A working user systemd manager, `node`,
Go 1.26.0, and `cmp` must be available.

The original Node driver produces `ref.go` and V8 coverage in one execution.
For Go emission, three exact host expressions in the driver are adapted:
its fs import becomes a typed extern import, its module-relative input URL
becomes the same checkout-relative path, and stdout uses a typed extern
function. Line numbers are preserved. The original driver is restored in
`finally`; existing temporary host/config files cause an early refusal.
No generator source or remaining blocking site is changed. The generator's
`host-io.ts` uses the existing `extern/hostio` package. Raw V8 coverage is
deleted after deriving the manifest.

`emit-go --trap-on-failure --overrides registry --reachability reach.json`
with `--initialize initialization.json`
uses the existing fail-closed lowering policy: refused functions retain
located traps; missing interface members retain their valid ABI as located
refusal stubs. Existing and inherited implementations are preserved.
Incompatible interface claims are removed so dependent conversions become
located verification traps; existing method signatures are preserved.
The shared Go emission helper also turns source-attributable backend
refusals (such as unreviewed regexps) into located traps, using the same
policy as the test pipeline. Unattributable failures still stop emission.
Unexecuted covered bodies also trap. Missing coverage entries
remain translated. Source hashes are validated before lowering. Host-only
bodies are omitted from the coverage manifest. `go/evidence/` contains
refusals and verification results.
The initialization list records dependency postorder of runtime imports,
after TypeScript erases type-only imports. Explicit abaplint and rearranger
package roots precede the driver graph. Module initializers run before main
entry code; cycles and duplicates initialize once.
The adapted driver, closure lists, config, manifest and step logs are retained
for inspection. The Go module is in `go/`, executable in `gogen`, and its
stdout in `gogen.go`.

`cmp` checks the exact bytes. On a difference the summary gives the first
1-based byte, line and byte column, including EOF differences. A runtime
trap gives its first source location and message from stderr. A build or
emission failure leaves `run=not-run cmp=not-run out=none`; its step log
contains the error. The final line is also saved in `summary.txt`. Exit 0
requires a successful run and equal output; every other outcome exits 1.

Boundary tests: `PYTHONDONTWRITEBYTECODE=1 python3 tools/selfhost-step4-test.py`.

Measured on base `396217c2` (OSG `1638d1cb`), the reference matched the pinned
SHA256 and emission completed with 440 refusal bodies, including nine Go
backend refusals. The 74 MiB `hir.go` reached `go build` under the 24G cap.
The run stopped there:

```text
step4: build=fail run=not-run cmp=not-run ref=a13702d55b1eecdb out=none
# generated
./hir.go:1048574: declared and not used: d
./hir.go:1048574: too many arguments in call to z_body_tools_g_a86aa652d13344
```

The live-field getter's unsupported conversion returns a trap without using
its local `d` (example generated lines 1823536–1823537). The emitted module
allocator passes `self` to the static module function named `constructor`
(generated line 2209027), whose body accepts only the three actual function
parameters (line 2212397). Its TS source is
`tools/gogen/frontend.mjs:3523:216`. These build defects remain as the measured
stop; no generated executable or `gogen.go` was produced, so no runtime trap
or output comparison was reached. Logs and small source excerpts are retained
under the scratch run directory; large generated outputs are removed after
validation.

Validation passed: gofmt; `go vet ./...`;
`go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2 -timeout 40m`;
the full abaplint Go traps gate (exactly two assignment-mismatch lines);
and M1's multifile build, run and byte comparison with `expected-clean.txt`.
Python boundary tests also passed, covering success and each stopping stage,
source restoration, memory scopes, driver drift and byte/EOF differences.
