import * as io from "./host.ts";
import * as cx from "./cx.ts";

function rethrow(value: unknown): void {
  throw value;
}

function run(): void {
  // This message must stay exactly the same as on Node, even when ABAP
  // diagnostic support is present in the same emitted Error class.
  try {
    throw new Error("JS message unchanged");
  } catch (e) {
    io.writeStdout(e.message + "\n");
  }

  const mode = io.env("ABAPITI_CX_MODE");
  if (mode === "rethrow") {
    try {
      cx.runtimeFailure(0);
    } catch (e) {
      io.writeStdout("caught: " + e.message + "\n");
      // Diagnostics must survive user mutation as well as catch/rethrow.
      e.message = "edited JS message";
      throw e;
    }
  }
  if (mode === "dynamic") {
    try {
      cx.runtimeFailure(0);
    } catch (e) {
      rethrow(e);
    }
  }
  if (mode === "uncaught") {
    cx.runtimeFailure(0);
  }
  if (mode === "chain") {
    cx.chainFailure(0);
  }
  if (mode === "raw") {
    cx.rawFailure(0);
  }
  if (mode === "js") {
    throw new Error("uncaught JS message unchanged");
  }
}
run();
