#include <stdio.h>

//__attribute__((export_name("entry"))) int entry(char *str) {

int main(int argc, char *argv[]) {
  if (argc <= 1) {
    printf("missing arguments\n");
    return 1;
  }

  printf("this is the string: %s\n", argv[1]);
  return 0;
}