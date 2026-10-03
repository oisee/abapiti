#include "common.h"
static u32 table[256];
static u8 buf[256];
static void init(void) {
  for (u32 i = 0; i < 256; i++) {
    u32 c = i;
    for (int k = 0; k < 8; k++) c = (c & 1) ? 0xEDB88320u ^ (c >> 1) : c >> 1;
    table[i] = c;
  }
}
/* CRC-32 of the bytes 0,1,2,...,n-1 (mod 256); n <= 256 */
EXPORT int crc32_seq(int n) {
  init();
  for (int i = 0; i < n; i++) buf[i] = (u8)i;
  u32 c = 0xFFFFFFFFu;
  for (int i = 0; i < n; i++) c = table[(c ^ buf[i]) & 0xFF] ^ (c >> 8);
  return (int)(c ^ 0xFFFFFFFFu);
}
/* CRC-32 of the ASCII string "123456789" (check value 0xCBF43926) */
EXPORT int crc32_check(void) {
  init();
  const char *s = "123456789";
  u32 c = 0xFFFFFFFFu;
  for (int i = 0; s[i]; i++) c = table[(c ^ (u8)s[i]) & 0xFF] ^ (c >> 8);
  return (int)(c ^ 0xFFFFFFFFu);
}
