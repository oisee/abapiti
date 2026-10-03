#include "common.h"
static int a[512];
static void qs(int lo, int hi) {
  if (lo >= hi) return;
  int p = a[(lo + hi) / 2], i = lo, j = hi;
  while (i <= j) {
    while (a[i] < p) i++;
    while (a[j] > p) j--;
    if (i <= j) { int t = a[i]; a[i] = a[j]; a[j] = t; i++; j--; }
  }
  qs(lo, j); qs(i, hi);
}
/* fill n pseudo-random values from seed, sort, return a position-weighted checksum
   (0 if not sorted) */
EXPORT int sort_checksum(int n, int seed) {
  u32 x = (u32)seed;
  for (int i = 0; i < n; i++) { x = x * 1103515245u + 12345u; a[i] = (int)((x >> 8) % 100000u) - 50000; }
  qs(0, n - 1);
  u32 sum = 0;
  for (int i = 0; i < n; i++) {
    if (i > 0 && a[i - 1] > a[i]) return 0;
    sum = sum * 31u + (u32)a[i];
  }
  return (int)sum;
}
