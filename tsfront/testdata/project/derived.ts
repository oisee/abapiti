import { Shape } from "./base";

export class Circle extends Shape {
  private readonly radius: number;
  static count = 0;

  constructor(radius: number) {
    super("circle");
    this.radius = radius;
  }

  public area(): number {
    return Math.PI * this.radius * this.radius;
  }

  public describeMore(): string {
    return this.describe() + " " + this.name;
  }
}
