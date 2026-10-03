#include "common.h"
static u8 composite[10001];
/* number of primes <= n (n <= 10000) */
EXPORT int primes_upto(int n) {
  for (int i = 0; i <= n; i++) composite[i] = 0;
  int count = 0;
  for (int i = 2; i <= n; i++) {
    if (composite[i]) continue;
    count++;
    for (int j = i * i; j <= n; j += i) composite[j] = 1;
  }
  return count;
}
