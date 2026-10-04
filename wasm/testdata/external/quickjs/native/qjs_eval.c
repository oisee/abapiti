#include <stdio.h>

int qjs_eval(int);

int main(void) {
  for (int i = 0; i < 16; i++) {
    printf("quickjs qjs_eval %d = %d\n", i, qjs_eval(i));
  }
  return 0;
}
