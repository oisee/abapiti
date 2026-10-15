export function readText(file: string): string { throw new Error("must not lower"); }
export function writeText(file: string, text: string): void { throw new Error("must not lower"); }
export function listDir(dir: string): string[] { throw new Error("must not lower"); }
export function listEntries(dir: string): [string, boolean][] { throw new Error("must not lower"); }
export function basename(file: string): string { throw new Error("must not lower"); }
export function dirname(file: string): string { throw new Error("must not lower"); }
export function join(...parts: string[]): string { throw new Error("must not lower"); }
export function gitHead(cwd: string): string | undefined { throw new Error("must not lower"); }
export function env(name: string): string | undefined { throw new Error("must not lower"); }
