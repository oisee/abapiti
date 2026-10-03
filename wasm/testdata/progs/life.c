#include "common.h"
#define W 16
#define H 16
static u8 g[H][W], h[H][W];
/* Conway's Game of Life on a 16x16 torus, glider + blinker, n generations;
   returns the number of live cells times 1000 plus a position checksum mod 1000 */
EXPORT int life(int n) {
  for (int y = 0; y < H; y++) for (int x = 0; x < W; x++) g[y][x] = 0;
  g[0][1] = g[1][2] = g[2][0] = g[2][1] = g[2][2] = 1;
  g[8][8] = g[8][9] = g[8][10] = 1;
  for (int t = 0; t < n; t++) {
    for (int y = 0; y < H; y++) for (int x = 0; x < W; x++) {
      int c = 0;
      for (int dy = -1; dy <= 1; dy++) for (int dx = -1; dx <= 1; dx++)
        if (dy || dx) c += g[(y + dy + H) % H][(x + dx + W) % W];
      h[y][x] = (c == 3) || (g[y][x] && c == 2);
    }
    for (int y = 0; y < H; y++) for (int x = 0; x < W; x++) g[y][x] = h[y][x];
  }
  int live = 0, pos = 0;
  for (int y = 0; y < H; y++) for (int x = 0; x < W; x++) if (g[y][x]) { live++; pos += y * W + x; }
  return live * 1000 + pos % 1000;
}
