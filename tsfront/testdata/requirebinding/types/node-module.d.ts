declare module "node:module" {
  interface NodeRequire {
    (id: string): any;
    resolve(id: string): string;
  }
  export function createRequire(path: string): NodeRequire;
}
interface ImportMeta {
  url: string;
}
