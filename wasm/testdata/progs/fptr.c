#include "common.h"
static int add(int a, int b) { return a + b; }
static int sub(int a, int b) { return a - b; }
static int mul(int a, int b) { return a * b; }
static int maxi(int a, int b) { return a > b ? a : b; }
static int (*const ops[4])(int, int) = { add, sub, mul, maxi };
/* fold 1..n with the operation chosen per step by (i % 4): call_indirect */
EXPORT int fold_ops(int n) {
  int acc = 1;
  for (int i = 1; i <= n; i++) acc = ops[i % 4](acc, i);
  return acc;
}
