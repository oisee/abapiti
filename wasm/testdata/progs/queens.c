#include "common.h"
static int cols[16];
static int ok(int r, int c) {
  for (int i = 0; i < r; i++) {
    int d = cols[i] - c;
    if (d == 0 || d == r - i || d == i - r) return 0;
  }
  return 1;
}
static int place(int r, int n) {
  if (r == n) return 1;
  int total = 0;
  for (int c = 0; c < n; c++) if (ok(r, c)) { cols[r] = c; total += place(r + 1, n); }
  return total;
}
/* number of solutions of the n-queens problem (8 -> 92) */
EXPORT int queens(int n) { return place(0, n); }
