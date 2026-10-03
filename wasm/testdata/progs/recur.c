#include "common.h"
static int hanoi_moves(int n, int a, int b, int c) {
  if (n == 0) return 0;
  return hanoi_moves(n - 1, a, c, b) + 1 + hanoi_moves(n - 1, c, b, a);
}
/* moves for n discs: 2^n - 1 */
EXPORT int hanoi(int n) { return hanoi_moves(n, 1, 2, 3); }
static int ack(int m, int n) {
  if (m == 0) return n + 1;
  if (n == 0) return ack(m - 1, 1);
  return ack(m - 1, ack(m, n - 1));
}
/* Ackermann A(m, n), small arguments only */
EXPORT int ackermann(int m, int n) { return ack(m, n); }
