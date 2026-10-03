#include <stdio.h>
#include "../crc32.c"
int main(void){
  printf("crc32 crc32_check = %d\n", crc32_check());
  printf("crc32 crc32_seq 0 = %d\n", crc32_seq(0));
  printf("crc32 crc32_seq 1 = %d\n", crc32_seq(1));
  printf("crc32 crc32_seq 256 = %d\n", crc32_seq(256));
  return 0; }
