# Writing TypeScript that abapiti compiles well

This is a style for TypeScript meant to be compiled by abapiti to Go (TS-HG) and ABAP (TS-HA), and still run unchanged on Node and Bun. It is what abaplint core already mostly is, written down as rules for new code. The first user is open-steamgate's gogen, which is moving from JavaScript to TypeScript so that the toolchain can become self-hosted.

The rule behind all the others: **the static type must say what the value is at runtime.** abapiti builds a typed object IR (HIR) from the TypeScript checker's types. Where the types are precise, it emits plain Go structs and ABAP classes. Where they are vague (`any`, object bags, unions without a discriminant), it has to fall back to a dynamic representation: slower, and some operations are refused.

## Compiler settings

```jsonc
{
  "compilerOptions": {
    "strict": true,
    "noEmit": true,
    "erasableSyntaxOnly": true,          // runs as is with Node type stripping and Bun
    "allowImportingTsExtensions": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true
  }
}
```

`erasableSyntaxOnly` rules out `enum`, `namespace`, parameter properties (`constructor(private x)`) and decorators. That is also the subset abapiti handles best.

## Data

- **Use classes and interfaces with declared fields.** Do not add properties after construction, and do not use `delete`. A class becomes a Go struct and an ABAP class; a field added later has nowhere to live.
- **Use discriminated unions for variant data**, with a literal tag: `type Expr = {e: "lit", value: string} | {e: "call", name: string, args: Expr[]}`. Switch on the tag. A union without a tag forces a dynamic representation.
- **Use `Map` and `Set` for dictionaries**, not objects used as maps. `for…in`, `Object.keys` and computed property access work only on records whose shape the checker knows.
- **Use arrays of one element type** (`Expr[]`, not `(Expr | string | number)[]`). Arrays are fine, including `push`, `pop`, `shift`, `splice`, `slice`, `map`, `filter`, `find`, `some`, `every`, `forEach`, `join` and spread.
- **Mark `readonly` where it is true.** It helps the alias and ownership analyses that decide when a value can be copied instead of shared.
- **Avoid `any` and `unknown` in data structures.** At an API boundary (parsing JSON, for example), narrow once into typed data and keep the typed form.
- **Prefer optional fields (`x?: T`) and `undefined` to `null`.** Use one absence value per field, not both.

## Numbers

- TypeScript's `number` is one type, but abapiti infers integers where the code only does integer arithmetic. Then it emits `int8` or `i` in ABAP and `int64` in Go instead of a double. Keep counters, indexes and lengths integral (`Math.trunc`, `|0` are fine) and do not mix them with fractions.
- `NaN`, `Infinity` and `-0` exist in Go but not in ABAP `f`. Code that relies on them (for example `Math.min()` with no arguments) traps on the ABAP side instead of returning a wrong value.

## Control flow

- `if`, `while`, `do…while`, `for`, `for…of`, `break`, `continue`, labelled loops, `try/catch/finally` and `throw` are all supported.
- **`switch`:** grouped empty labels (`case "a": case "b": body`) and `default` anywhere are supported. A non-empty case that falls through into the next one is refused. Write the shared part as a function or duplicate it.
- **Closures:** arrow functions and callbacks are supported, including those that capture locals. A loop variable declared with `const` in `for…of` is a fresh binding per iteration, as in JavaScript.
- **Generators** (`function*`, `yield`, `for…of` over them) are supported as resumable iterators.
- **`async`/`await`** are supported only for promises the compiler can prove already settled (synchronous work wrapped in `async`). Pending promises, timers and microtask ordering are refused. Do not use `async` for code that does not actually wait.

## Strings and regular expressions

- Strings are UTF-16, as in JavaScript. Supported: indexing, `charCodeAt`, `slice`, `substring`, `indexOf`, `startsWith`, `endsWith`, `includes`, `split`, `trim*`, `toUpperCase`/`toLowerCase`, `padStart`/`padEnd`, `repeat`, `replace`, `match`, template literals.
- **Regular expressions:** literals and checked `new RegExp(string)` in the subset described in [tsfront/regexp_exec.md](../tsfront/regexp_exec.md). Lookbehind, backreferences and the `m`/`s` flags have limits there. `exec` and `match` need ABAP 7.55+ (PCRE) on the ABAP side.

## Not supported

- `eval`, `new Function`, `with`, `arguments`, prototype manipulation (`Object.setPrototypeOf`, assigning to `.prototype`), `Proxy` and `Reflect`.
- Getters and setters with side effects that the code relies on implicitly.
- Monkey-patching globals or built-ins.
- Module-level mutable state that is initialised in an order other modules depend on. Use explicit initialisation functions.

## Checking it

For abaplint, `abapiti abaplint --full` lowers everything without pruning and lists each construct it cannot lower, with its TypeScript location (see [docs/abaplint-cli.md](abaplint-cli.md)). A general command for other TypeScript projects such as gogen does not exist yet; until it does, ask on the abapiti side for a `--full`-style run of a branch.
