#include "common.h"
/* FNV-1a 64-bit of the bytes 0..n-1, folded to 32 bits (hi ^ lo) */
EXPORT int fnv1a64_seq(int n) {
  u64 h = 1469598103934665603ULL;
  for (int i = 0; i < n; i++) { h ^= (u8)i; h *= 1099511628211ULL; }
  return (int)(u32)(h ^ (h >> 32));
}
