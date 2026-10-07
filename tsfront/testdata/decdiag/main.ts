// `p`'s inferred type is the un-exported `Priv` from ./priv: main.d.ts
// could not name it, which declaration emit reports (TS4023) only when
// `declaration` is set.
import { make } from "./priv";

export const p = make();
