// Controls for the four constructs; not a rewrite of gogen's IR.
export interface Cell { readonly name: string }
export interface Nullable { readonly cell: Cell | null; readonly label: string | null; readonly optional?: Cell }
export type Tag = {readonly k: 'i'} | {readonly k: 'c'; readonly len: number};
export function tagValue(x: Tag): number { return x.k === 'c' ? x.len : 0; }
export function maps(): Map<string, Cell> {
  const m = new Map<string, Cell>();
  m.set('x', {name: 'x'});
  const s = new Set<string>();
  s.add('x');
  if (s.has('x')) return m;
  return new Map<string, Cell>();
}
export function cast(x: unknown): string { return x as unknown as string; }
