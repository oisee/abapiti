# Certified facts pilot, 2026-10-10

Four conditional structures-cache certificates are accepted for **TS-HG@Go warm-up/freeze only**. Both independent auditors returned YES on the exact same claims and source/dependency bindings. No runtime parallelism was added. **A parallel structures map is still not provable.** Default ABAP emission is unchanged and its complete output directory is byte-identical to `origin/main`.

The abapGit source differential uses an explicitly representative ten-file kit. The unfiltered source tree does **not** pass Node parity: both vanilla Go and the pilot refuse outside the existing translator scope. This limitation is retained below, rather than hidden behind the smaller green kit.

The implementation follows `/home/alice/dev/dell-work/certpilot-design.md`. Source pins are abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e` and abapGit `3b6485b5d0b09ef3861006b966e68542446721ce`. The embedded upstream archive SHA-256 is `5ea3cd76e2058563ca5311dcc50707e78df5fd04bcb0ce84255ccbd3d0c8be4b`.

| Certificate | Bound declaration | GPT-6.1-sol | ZAI / GLM-5.3 | Discharged obligation |
|---|---|---|---|---|
| `structure-parser-singletons` | `StructureParser.runFile` | YES | YES | No late root-cache stores after successful guarded warm-up |
| `alternative-setup-map` | `Alternative.setupMap` | YES | YES | No late map initialization or bucket appends after successful guarded warm-up |
| `substructure-setup-matcher` | `SubStructure.setupMatcher` | YES | YES | No late matcher-field initialization after successful guarded warm-up |
| `module-sub-singletons` | module `sub()` | YES | YES | No late constructor-name singleton insertion after successful guarded warm-up |

These are cache-stability predicates, not purity claims. Every claim retains all five audited preconditions: the pinned closed matcher graph; a successful cycle-aware identity walk from Any, ClassGlobal, InterfaceGlobal and DynproLogic; completed reachable setup methods and populated singleton caches; no external cache aliases, invalidation, monkey patching or new receiver implementations; and the pre-store guard with fresh-process quarantine and sequential rerun from retained inputs. The four target fingerprints are respectively `01880132c915dd980afca090db0d8a2776722cc86a03116bde366d3c89592505`, `3d80a767b020fdb054a71943605cb342b7dda9d2f7dda7be38a8ad96a1ba5d5f`, `4abf2ccf0605226e9fa03167a3157f6f3a28de5f046ed9b487126cd001d22eb5`, and `56d70da4f75c37870830279c6ba910d1c79004cf6c517fdc19281c3dc4d33034`.

The versioned records and separate registry are in [`tsfront/certificates`](../../tsfront/certificates/registry-certificates.json). The source index uses the same TS file/symbol/AST-kind convention, `scanner.GetTokenPosOfNode` start and AST end, and `overrides.Fingerprint` as the override loader. No whitespace normalization occurs. Each record pins 477 declaration spans covering the structures constructors/methods/factories and statement combinators, plus the closed structures receiver-set digest. The whole original archive digest binds the rest of the transitive source closure. A missing, ambiguous or changed declaration, changed receiver set, changed archive, changed monitor, different upstream commit pin, incomplete audit, changed evidence, disagreement, stale/proposed status or revocation refuses consumption. Registry tombstones persist revoked IDs; facts are constructed afresh, so revoked conclusions cannot remain as derived inputs. A tombstone intentionally refuses the complete four-cache feature until the contract is rebuilt.

`cert_cache_stable`, `cert_requires`, `cert_monitor` and `cert_source_closure` are analysis-only Grace axioms. `cmd/grace-cert-pilot` derives four conditional `cert_discharge` conclusions carrying their certificate IDs and source declaration hashes. Go `names.json` records each declaring file, member, AST kind, exact original byte span/hash and emitted Go function byte span. It does not use the merged `member.*` identity as provenance. No certificate is imported into the ABAP emitter or default inliner rule inputs; no certificate is used as `p_complete`, `p_pure`, ownership or `mark_parallel` evidence. The existing 86 overrides and existing derived/trusted initializer inventory were not converted or expanded by this pilot.

The build flag is `abapiti abaplint --target go --cert-pilot -o OUTPUT`. It refuses mixed Go/ABAP targets. The emitted executable uses `--cert-pilot` to select the guarded attempt; without it the same retained source bodies run sequentially without warm-up/freeze. The default `--target go` build stays outside certificate consumption. Original constructor/first/getMatcher bodies needed by the walk are retained only for the pilot. The representative interface kit additionally needs the original `Interface.getSequencedFiles` and `getAllowedNaming` bodies, which the older workload coverage had trapped. This is explicit Go-only source retention, not an audited purity assumption or an ABAP change. The pre-pilot Go binary still refuses this newly exercised interface kit; the source-retained unguarded reference and the Node oracle both complete.

The identity walk marks nodes before following edges, visits all six concrete matcher kinds, calls setupMap/setupMatcher at their original lazy initialization boundaries, and freezes only after the complete walk succeeds. It warms **587 matcher identities**, rather than parsing a sample. Map indexes are also initialized before freeze. Miss-site guards fire before the original initialization RHS and stores; tagged map and array mutation methods guard aliases before their mutations. A warm-up exception or unexpected graph node becomes a quarantine refusal. These guards exist only in the emitted guarded Go build, without modifying shared HIR or `hir/abap`.

The supervisor reads inputs once. Raw file, dependency and configuration contents are byte slices encoded losslessly through JSON/base64; a regression test includes invalid UTF-8, NUL, CR and LF. The guarded child owns its entire attempt, including parse state and mutable intermediate inputs. Its stdout is buffered. A pre-store violation exits with a distinct status, its results are discarded, and a **fresh vanilla child** receives the same serialized input snapshot. It never rereads mutable input files between attempts or resumes an already-mutated statement stream. Only completed sequential output is published. This isolation is stronger than the eventual per-iteration quarantine required for workers; this pilot creates no workers.

Every real miss path has a negative test. The tests check the exact violation and unchanged root/sub cache counts, nil Alternative.map and SubStructure.matcher fields, a direct map alias and an actual warmed bucket-array alias. All four end-to-end forced misses also demonstrate quarantine and byte-identical sequential output. Example stderr is:

```text
certificate quarantine: guarded results discarded; sequential rerun from unchanged input snapshot
frozen cache write blocked before store: StructureParser.singletons
```

The [reproducible differential tool](../../tools/cert-pilot-gate.mjs) builds a fresh original Node oracle via `tools/statements-upstream.mjs`, never importing an existing build. Upstream lockfiles were installed at both repository and core package level; the verified compiler was TypeScript 6.0.3. The rebuilt kit, input hashes, dependency/config hashes, oracle outputs, Go outputs, guard traces and results are in `$HOME/.cache/kits/abapgit-cert-pilot`. The source clones are `$HOME/.cache/kits/abapgit-src` and `$HOME/.cache/kits/abaplint-577f875e`.

| Differential | Result | Coverage / limitation |
|---|---|---|
| zabapgit clean | PASS, byte-identical | Node = source-retained vanilla Go = guarded Go; 0 issues; supplied golden also matched |
| zabapgit seeded | PASS, byte-identical | Same three outputs; 5 issues in the same order; supplied golden also matched |
| abapGit representative source kit | PASS, byte-identical | Five smallest `.intf.abap` files by byte size/path plus original XML metadata; 10 unmodified files, 0 issues |
| Four forced late-cache misses | PASS, byte-identical | Pre-store refusal, discarded guarded attempt, fresh sequential rerun for every certificate |
| All abapGit source ABAP/XML files | **FAIL Node parity** | 1,388 files: Node 0 issues; vanilla Go and pilot refuse `src/artifacts_objects.ts:8` on unsupported object types |
| All source files of documented supported object types | **FAIL Node parity** | 1,322 files: Node 0 issues; vanilla Go and pilot refuse `CurrentScope.isBadiDef`, `src/abap/5_syntax/_current_scope.ts:303` |
| OSG's own ABAP | Not run | Optional third corpus remains open |

The representative selection is mechanical and recorded, not changed according to test outcomes: `zif_abapgit_ecatt_download`, `zif_abapgit_ecatt_upload`, `zif_abapgit_gui_modal`, `zif_abapgit_gui_page_title`, and `zif_abapgit_version`, with their XML. It exercises multiple real upstream files and the global-interface root. It is **not** evidence for checking the full abapGit repository. The larger matching vanilla/pilot refusals demonstrate preserved refusal boundaries, not equivalence with Node.

The ABAP hard condition was checked with actual binaries built from `origin/main` (`d2d49dd87e7630bc57e464679c363ae4b59b941e`) and this branch. Both used the same output path; the first result was preserved before the second generation. `diff -r` of the entire directories returned **0 with no differences**, including ABAP sources, names maps, README files and zip artifacts. An earlier comparison generated into different directories and showed only absolute paths in README files; those outputs were not normalized or accepted as the proof. The same-path regeneration is the byte-identity proof. Acceptance was followed by a fresh production CLI generation and another recursive diff against that main snapshot.

`go vet ./...`, `go test -short ./...` and `./.github/ci/lint.sh gate origin/main` pass. The lint canary reports 5/5 and zero new issues with golangci-lint 2.13.2. The opt-in full pilot compilation and runtime pre-store/snapshot tests also pass. Heavy builds, tests and oracle/differential runs use `flock /tmp/abapiti-heavy.lock`; Go uses `GOCACHE=$HOME/.cache/abapiti-go-cache` and `GOFLAGS=-buildvcs=false`.

Audit processes were `codex exec -s read-only -m gpt-6.1-sol` and `codex exec -s read-only -c model_provider=zai -m glm-5.3`, launched via `bash -ic`. They worked independently and never received each other's answers. Initial audits were UNKNOWN because the inline prompt/source environment did not supply inspectable transitive and generated runtime coverage; those attestations are retained and are not acceptance evidence. Fresh blind audits used a complete immutable snapshot through a small read-only MCP view confined to that snapshot, avoiding unavailable Linux user namespaces without granting writes or shell execution. Final follow-ups resumed only each auditor's own session to inspect lossless snapshot transport and the Go interface sequencing change. All final verdicts are YES on identical claims/binding hashes. GLM rounds 2 and 3 emitted four extra outer JSON braces; only those braces were removed for parsing. Raw outputs remain alongside normalized attestations. No verdict, claim, evidence or fingerprint was changed.

| Audit round | GPT time / CLI tokens | GLM time / CLI tokens | Outcome |
|---|---:|---:|---|
| Initial insufficient inspection | 179 s / 179,581 | 269 s / 537,917 | UNKNOWN / UNKNOWN |
| Full blind source/runtime snapshot | 223 s / 128,154 | 278 s / 195,802 | YES / YES, all four |
| Final own-session follow-up | 131 s / 178,661 | 266 s / 257,492 | YES / YES, all four |

The six CLI counters sum to **1,477,607 reported tokens**. They include cached prompt tokens, and resumed-session totals can overlap; this is not a billing total or dollar estimate. The sum of process durations is 1,346 seconds; parallel round maxima sum to about 813 seconds. The expensive initial failed inspection is included. Machine-readable costs and raw/normalized attestations are committed under `tsfront/certificates/attestations`.

On this shared machine, the first uncached required-gate set took about 174 seconds; the later cached host gate set took about 69 seconds, including lock scheduling. The final full pilot/vanilla compilation and negative tests took 115.27 seconds. The reproducible fresh-Node differential plus all four fallback runs took **212.306 seconds**. The final acceptance run, including required gates, accepted production Go generation, fresh ABAP generation and recursive diff, took about **220 seconds**. These are wall times for this run, not CI performance guarantees. Warm-up/guards add work; no speedup or parallel throughput is claimed.

What remains open for a structures map:

- Complete virtual receiver and transitive effect summaries for matcher `run`, not merely the graph's construction/cache writers. The four axioms cannot discharge unexpected-dispatch or all receiver/effect obligations.
- Ownership and aliases of statement arrays, splice/view tails, tokens, parent nodes, result arrays and exception-visible state. Fresh-process isolation does not prove iteration ownership for future workers.
- File-information parser effects and the ABAPFile/issue publication path, including joins and source order.
- Initialization timing, hidden effects/counters, identity and exception order. The cache claim does not certify that eager initialization is unobservable on every input/configuration.
- Ordered result/issue slots, lowest-index exception selection, guards before any additional shared effect, and per-iteration quarantine from unchanged inputs if workers are ever introduced.
- Full abapGit Node parity and OSG coverage. The representative green kit is explicitly narrower.

The answer to “is a parallel structures map now provable?” is **NO**. The pilot supplies four guarded, source-bound conditional cache facts. It supplies neither the roughly six additional matcher-run summaries discussed in the design nor the ownership, receiver, file-information and exception/join proofs.

To reproduce the larger Go validation and the green representative gate:

```sh
export GOCACHE="$HOME/.cache/abapiti-go-cache" GOFLAGS=-buildvcs=false
export TMPDIR="$HOME/.cache/certpilot-tmp"
ABAPITI_CERT_PILOT_TEST=1 \
ABAPITI_CERT_PILOT_OUT="$PWD/.local/cert-pilot/go" \
ABAPITI_CERT_PILOT_VANILLA_OUT="$PWD/.local/cert-vanilla/go" \
flock /tmp/abapiti-heavy.lock go test ./cmd/abapiti -run '^TestGoCertificatePilot$' -v
flock /tmp/abapiti-heavy.lock node tools/cert-pilot-gate.mjs \
  --pilot="$PWD/.local/cert-pilot/go/zabaplint" \
  --vanilla="$PWD/.local/cert-vanilla/go/zabaplint" \
  --upstream="$HOME/.cache/kits/abaplint-577f875e" \
  --abapgit="$HOME/.cache/kits/abapgit-src" \
  --zabapgit=/home/alice/dev/dell-work/kits/zabapgit-check-kit \
  --out="$HOME/.cache/kits/abapgit-cert-pilot"
go run ./cmd/grace-cert-pilot
```

An upstream pin bump expires the validation even when local declaration/archive SHAs match. Re-audit changed and transitively affected bindings, including the closed receiver set, and rerun all gates before restoring accepted status. Monitor changes also expire validation. Revocation is represented by retained tombstones and fresh fact derivation; it must not silently fall through to an optimization.
