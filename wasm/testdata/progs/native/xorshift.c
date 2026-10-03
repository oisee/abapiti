#include <stdio.h>
#include "../xorshift.c"
int main(void){
  printf("xorshift xorshift32 1 1 = %d\n", xorshift32(1, 1));
  printf("xorshift xorshift32 12345 10 = %d\n", xorshift32(12345, 10));
  printf("xorshift xorshift64s 1 1 = %d\n", xorshift64s(1, 1));
  printf("xorshift xorshift64s 42 100 = %d\n", xorshift64s(42, 100));
  return 0; }
