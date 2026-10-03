#include <stdio.h>
#include "../recur.c"
int main(void){
  printf("recur hanoi 10 = %d\n", hanoi(10));
  printf("recur hanoi 15 = %d\n", hanoi(15));
  printf("recur ackermann 2 3 = %d\n", ackermann(2, 3));
  printf("recur ackermann 3 3 = %d\n", ackermann(3, 3));
  return 0; }
