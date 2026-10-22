import * as io from "./host.ts";
import {greeting} from "./state.ts";

declare const process: {env: {[key: string]: string | undefined}};

function optional(value: string | undefined): string {
  return value ?? "unset";
}

function lines(): string[] {
  const out: string[] = [greeting];
  const text = io.readText("data/in.txt");
  out.push(text);
  for (const [name, directory] of io.listEntries("data")) {
    out.push(name + ":" + directory);
  }
  out.push(io.listDir("data/sub").join(","));
  try {
    io.listDir("data/missing");
    out.push("missing:listed");
  } catch (e) {
    out.push("missing:caught");
  }
  out.push(io.basename("/a/b/"), io.basename("x"), io.basename("/"), io.dirname("/a/b/c"), io.dirname("//a"), io.dirname("a"), io.dirname(""));
  out.push(io.join("a", "..", "b", "./c/"), io.join(), io.join("/", "..", "x"), io.join("a/b", "../../.."), io.join("", ""), io.join("./a", "b//"));
  // Exercise both host outcomes in each optional-string consumer shape.
  // Reuse the call site in a loop so an absent value cannot retain a prior one.
  for (const name of ["ABAPITI_SMOKE", "ABAPITI_SMOKE_UNSET"]) {
    const value: string | undefined = io.env(name);
    out.push(optional(value));
    out.push(io.env(name) ?? "unset");
    out.push(io.env(name) === undefined ? "absent" : "present");
    out.push(optional(io.env(name)));
    out.push(io.basename(io.env(name) ?? "unset"));
    // Ambient process.env uses the declared env host capability as well.
    const ambient: string | undefined = process.env[name];
    out.push(optional(ambient));
    out.push(process.env[name] ?? "unset");
    out.push(process.env[name] === undefined ? "absent" : "present");
    out.push(optional(process.env[name]));
    out.push(io.basename(process.env[name] ?? "unset"));
  }
  out.push(io.env("ABAPITI_SMOKE") === "on" ? "equal" : "different");
  out.push(io.gitHead("data") ?? "no-git", io.gitHead("/") ?? "no-git");
  io.writeText("out/copy.txt", text);
  out.push(io.readText("out/copy.txt") === text ? "copy:equal" : "copy:different");
  return out;
}

io.writeStdout(lines().join("\n") + "\n");
io.writeStdout("second write\n");

// The smoke harness replays a second snapshot to test uncaught Error payloads.
if (io.env("ABAPITI_SMOKE_THROW") === "on") {
  throw new Error("extern smoke payload message");
}
