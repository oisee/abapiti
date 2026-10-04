#include <stdio.h>
int blake2b(int,int); int x25519(int); int chacha(int,int); int poly(int,int); int eddsa(int);
int main(void){
 printf("mono blake2b 0 0 = %d\n",blake2b(0,0)); printf("mono blake2b 3 1 = %d\n",blake2b(3,1)); printf("mono blake2b 200 7 = %d\n",blake2b(200,7));
 printf("mono x25519 1 = %d\n",x25519(1)); printf("mono x25519 77 = %d\n",x25519(77));
 printf("mono chacha 64 5 = %d\n",chacha(64,5)); printf("mono chacha 300 9 = %d\n",chacha(300,9));
 printf("mono poly 0 2 = %d\n",poly(0,2)); printf("mono poly 100 4 = %d\n",poly(100,4)); printf("mono eddsa 3 = %d\n",eddsa(3)); return 0; }
