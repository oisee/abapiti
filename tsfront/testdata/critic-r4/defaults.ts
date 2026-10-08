export class Probe {
  static run(): number {
    const first = (n: number = 7): number => n;
    const second = (): number => first();
    return second();
  }
}
