#include <stdio.h>
#include "../fnv.c"
int main(void){
  printf("fnv fnv1a64_seq 0 = %d\n", fnv1a64_seq(0));
  printf("fnv fnv1a64_seq 1 = %d\n", fnv1a64_seq(1));
  printf("fnv fnv1a64_seq 100 = %d\n", fnv1a64_seq(100));
  return 0; }
