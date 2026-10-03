#include <stdio.h>
#include "../base64.c"
int main(void){
  printf("base64 base64_seq 1 = %d\n", base64_seq(1));
  printf("base64 base64_seq 2 = %d\n", base64_seq(2));
  printf("base64 base64_seq 3 = %d\n", base64_seq(3));
  printf("base64 base64_seq 64 = %d\n", base64_seq(64));
  return 0; }
