// The host-io.ts contract of gogen, plus the step-4 driver's writeStdout.
// Node modules come from process.getBuiltinModule: no @types/node needed.
const node = (globalThis as any).process;
const fs = node.getBuiltinModule("node:fs");
const path = node.getBuiltinModule("node:path");
const childProcess = node.getBuiltinModule("node:child_process");

export function readText(file: string): string {
  return fs.readFileSync(file, "utf8");
}
export function writeText(file: string, text: string): void {
  fs.writeFileSync(file, text, "utf8");
}
export function writeStdout(text: string): void {
  node.stdout.write(text);
}
export function listDir(dir: string): string[] {
  return fs.readdirSync(dir);
}
export function listEntries(dir: string): [string, boolean][] {
  return fs.readdirSync(dir, {withFileTypes: true}).map((entry: any) => [entry.name, entry.isDirectory()]);
}
export function basename(file: string): string {
  return path.basename(file);
}
export function dirname(file: string): string {
  return path.dirname(file);
}
export function join(...parts: string[]): string {
  return path.join(...parts);
}
export function gitHead(cwd: string): string | undefined {
  try {
    return childProcess.execFileSync("git", ["rev-parse", "HEAD"], {cwd, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"]}).trim();
  } catch {
    return undefined;
  }
}
export function env(name: string): string | undefined {
  return node.env[name];
}
