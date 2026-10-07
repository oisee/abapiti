// With noLib the global type Array does not exist; tsc reports that as a
// checker *global* diagnostic, not as a per-file semantic diagnostic.
export const xs: number[] = [];
