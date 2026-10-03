#include "common.h"
static const char tbl[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static u8 in[96], outb[132];
/* base64 of the bytes 0..n-1 (n <= 96); returns sum of output chars * position */
EXPORT int base64_seq(int n) {
  for (int i = 0; i < n; i++) in[i] = (u8)(i * 7 + 3);
  int o = 0;
  for (int i = 0; i < n; i += 3) {
    u32 v = (u32)in[i] << 16;
    if (i + 1 < n) v |= (u32)in[i + 1] << 8;
    if (i + 2 < n) v |= in[i + 2];
    outb[o++] = tbl[(v >> 18) & 63];
    outb[o++] = tbl[(v >> 12) & 63];
    outb[o++] = i + 1 < n ? tbl[(v >> 6) & 63] : '=';
    outb[o++] = i + 2 < n ? tbl[v & 63] : '=';
  }
  int sum = 0;
  for (int i = 0; i < o; i++) sum += outb[i] * (i + 1);
  return sum;
}
