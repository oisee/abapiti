#include <stdio.h>
#include "../sieve.c"
int main(void){
  printf("sieve primes_upto 10 = %d\n", primes_upto(10));
  printf("sieve primes_upto 100 = %d\n", primes_upto(100));
  printf("sieve primes_upto 1000 = %d\n", primes_upto(1000));
  printf("sieve primes_upto 10000 = %d\n", primes_upto(10000));
  return 0; }
