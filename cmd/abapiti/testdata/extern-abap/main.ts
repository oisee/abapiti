import * as io from "./host.ts";
import {greeting} from "./state.ts";

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
  out.push(io.env("ABAPITI_SMOKE_UNSET") ?? "unset");
  out.push(io.gitHead("data") ?? "no-git", io.gitHead("/") ?? "no-git");
  io.writeText("out/copy.txt", text);
  out.push(io.readText("out/copy.txt") === text ? "copy:equal" : "copy:different");
  return out;
}

io.writeStdout(lines().join("\n") + "\n");
io.writeStdout("second write\n");
