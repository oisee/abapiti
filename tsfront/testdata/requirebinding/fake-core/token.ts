export class Token {
  private readonly value: string;
  constructor(value: string) {
    this.value = value;
  }
  text(): string {
    return "'" + this.value + "'";
  }
}
