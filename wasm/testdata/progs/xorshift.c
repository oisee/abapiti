#include "common.h"
/* xorshift32: state after n steps */
EXPORT int xorshift32(int seed, int n) {
  u32 x = (u32)seed;
  for (int i = 0; i < n; i++) { x ^= x << 13; x ^= x >> 17; x ^= x << 5; }
  return (int)x;
}
/* xorshift64*: low 32 bits after n steps */
EXPORT int xorshift64s(int seed, int n) {
  u64 x = (u64)(u32)seed | 0x100000000ULL;
  for (int i = 0; i < n; i++) { x ^= x >> 12; x ^= x << 25; x ^= x >> 27; }
  return (int)(u32)((x * 2685821657736338717ULL) >> 32);
}
