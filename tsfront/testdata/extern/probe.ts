import {readText as read, writeText, listDir, listEntries, basename, dirname, join, gitHead, env} from "./host";
import * as io from "./host";
class Probe {
 static run(): string {
  writeText("z.txt", "æ中😀");
  let out = read("z.txt") + "|" + io.readText("z.txt") + "|" + basename("/a//") + "|" + dirname("//foo") + "|" + join("a", "..", "b", "c/");
  out += "|" + listDir(".").join(",");
  for (const e of listEntries(".")) { out += "|" + e[0] + ":" + e[1]; }
  out += "|" + (gitHead(".") === undefined) + "|" + (env("ABAPITI_EXTERN_MISSING_932A") === undefined) + "|" + env("ABAPITI_EXTERN_VALUE");
  try { read("missing"); } catch(e) { out += "|caught:" + (e instanceof Error); }
  return out;
 }
}
