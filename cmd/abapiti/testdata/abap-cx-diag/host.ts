const node = (globalThis as any).process;
export function env(name: string): string | undefined { return node.env[name]; }
export function writeStdout(text: string): void { node.stdout.write(text); }
