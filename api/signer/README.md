# Signer API

The signer API defines the gRPC service used by the Talos imager to delegate
SecureBoot and PCR signing to an out-of-process signer.

An example YubiKey-based implementation is available at:
https://github.com/jaakkonen/talos-secureboot
