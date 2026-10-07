export abstract class Shape {
  protected name: string;

  constructor(name: string) {
    this.name = name;
  }

  public abstract area(): number;

  public describe(): string {
    return this.name + " " + this.area().toFixed(2);
  }
}

export function label(shape: Shape): string | undefined {
  const a = shape.area();
  return a > 1 ? shape.describe() : undefined;
}
