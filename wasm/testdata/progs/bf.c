#include "common.h"
static u8 tape[1024];
static u8 out[256];
/* Runs a Brainfuck program that prints "Hello World!\n" and returns a checksum
   of the output (sum of byte*position) or -1 on a bad program. A switch over
   the opcode becomes a br_table. */
EXPORT int bf_hello(void) {
  const char *p =
    "++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++.";
  for (int i = 0; i < 1024; i++) tape[i] = 0;
  int ptr = 0, pc = 0, o = 0;
  while (p[pc]) {
    switch (p[pc]) {
      case '>': ptr++; break;
      case '<': ptr--; break;
      case '+': tape[ptr]++; break;
      case '-': tape[ptr]--; break;
      case '.': out[o++] = tape[ptr]; break;
      case '[':
        if (!tape[ptr]) { int d = 1; while (d) { pc++; if (p[pc] == '[') d++; else if (p[pc] == ']') d--; } }
        break;
      case ']':
        if (tape[ptr]) { int d = 1; while (d) { pc--; if (p[pc] == ']') d++; else if (p[pc] == '[') d--; } }
        break;
    }
    pc++;
  }
  int sum = 0;
  for (int i = 0; i < o; i++) sum += out[i] * (i + 1);
  return sum;
}
/* length of the output ("Hello World!\n" = 13) */
EXPORT int bf_hello_len(void) {
  bf_hello();
  int n = 0;
  while (n < 256 && out[n]) n++;
  return n;
}
