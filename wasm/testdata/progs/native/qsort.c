#include <stdio.h>
#include "../qsort.c"
int main(void){
  printf("qsort sort_checksum 10 1 = %d\n", sort_checksum(10, 1));
  printf("qsort sort_checksum 100 42 = %d\n", sort_checksum(100, 42));
  printf("qsort sort_checksum 500 7 = %d\n", sort_checksum(500, 7));
  return 0; }
