#include <stdint.h>
#include <stddef.h>
#include "monocypher.h"
#ifdef __wasm__
#define EXPORT(n) __attribute__((export_name(n)))
#else
#define EXPORT(n)
#endif
static uint8_t buf[1024];
static uint8_t out[64];
static int fold(const uint8_t *p, int n) { uint32_t h = 2166136261u; for (int i = 0; i < n; i++) { h ^= p[i]; h *= 16777619u; } return (int)h; }
static void fill(int n, int seed) { for (int i = 0; i < n; i++) buf[i] = (uint8_t)(i * 7 + seed); }
EXPORT("blake2b") int blake2b(int n, int seed) { if (n < 0 || n > 1024) return -1; fill(n, seed); crypto_blake2b(out, 64, buf, (size_t)n); return fold(out, 64); }
EXPORT("x25519") int x25519(int seed) { uint8_t sk[32], pk[32]; for (int i = 0; i < 32; i++) sk[i] = (uint8_t)(seed + i * 13); crypto_x25519_public_key(pk, sk); return fold(pk, 32); }
EXPORT("chacha") int chacha(int n, int seed) { uint8_t key[32], nonce[8]; if (n < 0 || n > 1024) return -1; for (int i = 0; i < 32; i++) key[i] = (uint8_t)(seed ^ i); for (int i = 0; i < 8; i++) nonce[i] = (uint8_t)i; fill(n, seed); crypto_chacha20_djb(buf, buf, (size_t)n, key, nonce, 0); return fold(buf, n); }
EXPORT("poly") int poly(int n, int seed) { uint8_t key[32], mac[16]; if (n < 0 || n > 1024) return -1; for (int i = 0; i < 32; i++) key[i] = (uint8_t)(seed + 3 * i); fill(n, seed); crypto_poly1305(mac, buf, (size_t)n, key); return fold(mac, 16); }
EXPORT("eddsa") int eddsa(int seed) { uint8_t s[32], sk[64], pk[32], sig[64]; for (int i = 0; i < 32; i++) s[i] = (uint8_t)(seed * 5 + i); crypto_eddsa_key_pair(sk, pk, s); fill(40, seed); crypto_eddsa_sign(sig, sk, buf, 40); int ok = crypto_eddsa_check(sig, pk, buf, 40); return fold(sig, 64) ^ (ok == 0 ? 0 : 0x55555555); }
#ifndef __wasm__
#include <stdio.h>

#endif
