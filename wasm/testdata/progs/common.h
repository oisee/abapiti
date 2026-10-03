/* Freestanding helpers: no libc. Every program exports int functions that
   return a checksum, so a test compares one number per case. */
typedef unsigned int u32;
typedef unsigned long long u64;
typedef unsigned char u8;
#define EXPORT __attribute__((visibility("default")))
