export class Box {
  constructor(public value: number, private secret: string = "s") {}

  #hidden: string = "h";

  offset(this: Box, by = 1): number {
    return this.value + by + this.#hidden.length;
  }
}

type BoxTag = `box${string}`;

export function tag(b: Box): BoxTag {
  return `box${b.value}` as BoxTag;
}
