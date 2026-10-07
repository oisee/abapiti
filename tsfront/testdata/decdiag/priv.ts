// Not exported: consumers cannot name this type in their declarations.
interface Priv {
  x: number;
}

export function make(): Priv {
  return { x: 1 };
}
