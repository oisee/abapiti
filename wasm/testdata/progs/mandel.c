#include "common.h"
/* Mandelbrot set on a w x h grid in 16.16 fixed point, max 64 iterations;
   returns the sum of iteration counts */
EXPORT int mandel(int w, int h) {
  int total = 0;
  for (int py = 0; py < h; py++) for (int px = 0; px < w; px++) {
    int cx = -2 * 65536 + (int)(((long long)px * 3 * 65536) / w);
    int cy = -1 * 65536 + (int)(((long long)py * 2 * 65536) / h);
    int x = 0, y = 0, i = 0;
    while (i < 64) {
      long long xx = ((long long)x * x) >> 16, yy = ((long long)y * y) >> 16;
      if (xx + yy > 4 * 65536) break;
      int xy = (int)(((long long)x * y) >> 16);
      x = (int)(xx - yy) + cx;
      y = 2 * xy + cy;
      i++;
    }
    total += i;
  }
  return total;
}
