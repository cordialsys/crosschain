// C header for Monero BP+ wrapper - for use with CGO
#ifndef BP_PLUS_WRAPPER_H
#define BP_PLUS_WRAPPER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Generate a BP+ range proof for `count` amounts.
//
// amounts:    array of `count` uint64_t values
// masks:     array of `count` * 32 bytes (each mask/gamma is a 32-byte ed25519 scalar)
// count:     number of amounts (1 to 16)
// proof_out: output buffer (caller allocates, recommended 8192 bytes)
// proof_len: [in] capacity of proof_out; [out] actual bytes written
//
// Returns 0 on success, negative on error:
//   -1: invalid arguments
//   -2: output buffer too small
//   -3: internal exception
//   -4: unknown error
int monero_bp_plus_prove(
    const uint64_t *amounts,
    const unsigned char *masks,
    int count,
    unsigned char *proof_out,
    int *proof_len);

// Verify a BP+ range proof.
//
// proof_data: serialized proof bytes (from monero_bp_plus_prove)
// proof_len:  length of proof_data
//
// Returns:
//    1: proof is valid
//    0: proof is invalid
//   <0: error
int monero_bp_plus_verify(
    const unsigned char *proof_data,
    int proof_len);

#ifdef __cplusplus
}
#endif

#endif // BP_PLUS_WRAPPER_H
