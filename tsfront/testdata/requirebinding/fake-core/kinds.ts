export abstract class Kind {
  abstract name(): string;
}

export class CharKind extends Kind {
  length: number;
  constructor(length: number) {
    super();
    this.length = length;
  }
  name(): string {
    return "C" + this.length;
  }
  static of(length: number): CharKind {
    return new CharKind(length);
  }
}

export class IntKind extends Kind {
  name(): string {
    return "I";
  }
}
