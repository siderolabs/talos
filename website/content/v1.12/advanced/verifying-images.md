---
title: "Verifying Images"
description: "Verifying Talos container image signatures."
---

Sidero Labs signs the container images generated for the Talos release with [cosign](https://docs.sigstore.dev/cosign/overview/):

* `ghcr.io/siderolabs/installer` (Talos installer)
* `ghcr.io/siderolabs/talos` (Talos image for container runtime)
* `ghcr.io/siderolabs/talosctl` (`talosctl` client packaged as a container image)
* `ghcr.io/siderolabs/imager` (Talos install image generator)
* all [system extension images](https://github.com/siderolabs/extensions/)

## Verifying Container Image Signatures

The `cosign` tool can be used to verify the signatures of the Talos container images:

```bash
$ cosign verify --certificate-identity-regexp '@siderolabs\.com$' --certificate-oidc-issuer https://accounts.google.com ghcr.io/siderolabs/installer:v1.4.0

Verification for ghcr.io/siderolabs/installer:v1.4.0 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The code-signing certificate was verified using trusted certificate authority certificates

[{"critical":{"identity":{"docker-reference":"ghcr.io/siderolabs/installer"},"image":{"docker-manifest-digest":"sha256:f41795cc88f40eb1bc6b3c638c4a3123f6ef3c90627bfc35c04ebab82581e3ee"},"type":"cosign container image signature"},"optional":{"1.3.6.1.4.1.57264.1.1":"https://accounts.google.com","Bundle":{"SignedEntryTimestamp":"MEQCIERkQpgEnPWnfjUHIWO9QxC9Ute3/xJOc7TO5GUnu59xAiBKcFvrDWHoUYChT0/+gaazTrI+r0/GWSbi+Q+sEQ5AKA==","Payload":{"body":"eyJhcGlWZXJzaW9uIjoiMC4wLjEiLCJraW5kIjoiaGFzaGVkcmVrb3JkIiwic3BlYyI6eyJkYXRhIjp7Imhhc2giOnsiYWxnb3JpdGhtIjoic2hhMjU2IiwidmFsdWUiOiJkYjhjYWUyMDZmODE5MDlmZmI4NjE4ZjRkNjIzM2ZlYmM3NzY5MzliOGUxZmZkMTM1ODA4ZmZjNDgwNjYwNGExIn19LCJzaWduYXR1cmUiOnsiY29udGVudCI6Ik1FVUNJUURQWXhiVG5vSDhJTzBEakRGRE9rNU1HUjRjMXpWMys3YWFjczNHZ2J0TG1RSWdHczN4dVByWUgwQTAvM1BSZmZydDRYNS9nOUtzQVdwdG9JbE9wSDF0NllrPSIsInB1YmxpY0tleSI6eyJjb250ZW50IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVTXhha05EUVd4NVowRjNTVUpCWjBsVlNIbEhaRTFQVEhkV09WbFFSbkJYUVRKb01qSjRVM1ZIZVZGM2QwTm5XVWxMYjFwSmVtb3dSVUYzVFhjS1RucEZWazFDVFVkQk1WVkZRMmhOVFdNeWJHNWpNMUoyWTIxVmRWcEhWakpOVWpSM1NFRlpSRlpSVVVSRmVGWjZZVmRrZW1SSE9YbGFVekZ3WW01U2JBcGpiVEZzV2tkc2FHUkhWWGRJYUdOT1RXcE5kMDVFUlRSTlZHZDZUbXBWTlZkb1kwNU5hazEzVGtSRk5FMVVaekJPYWxVMVYycEJRVTFHYTNkRmQxbElDa3R2V2tsNmFqQkRRVkZaU1V0dldrbDZhakJFUVZGalJGRm5RVVZaUVdKaVkwbDZUVzR3ZERBdlVEZHVUa0pNU0VscU1rbHlORTFQZGpoVVRrVjZUemNLUkVadVRXSldVbGc0TVdWdmExQnVZblJHTVZGMmRWQndTVm95VkV3NFFUUkdSMWw0YldFeGJFTk1kMkk0VEZOVWMzRlBRMEZZYzNkblowWXpUVUUwUndwQk1WVmtSSGRGUWk5M1VVVkJkMGxJWjBSQlZFSm5UbFpJVTFWRlJFUkJTMEpuWjNKQ1owVkdRbEZqUkVGNlFXUkNaMDVXU0ZFMFJVWm5VVlZqYWsweUNrbGpVa1lyTkhOVmRuRk5ia3hsU0ZGMVJIRkdRakZqZDBoM1dVUldVakJxUWtKbmQwWnZRVlV6T1ZCd2VqRlphMFZhWWpWeFRtcHdTMFpYYVhocE5Ga0tXa1E0ZDB0M1dVUldVakJTUVZGSUwwSkRSWGRJTkVWa1dWYzFhMk50VmpWTWJrNTBZVmhLZFdJeldrRmpNbXhyV2xoS2RtSkhSbWxqZVRWcVlqSXdkd3BMVVZsTFMzZFpRa0pCUjBSMmVrRkNRVkZSWW1GSVVqQmpTRTAyVEhrNWFGa3lUblprVnpVd1kzazFibUl5T1c1aVIxVjFXVEk1ZEUxRGMwZERhWE5IQ2tGUlVVSm5OemgzUVZGblJVaFJkMkpoU0ZJd1kwaE5Oa3g1T1doWk1rNTJaRmMxTUdONU5XNWlNamx1WWtkVmRWa3lPWFJOU1VkTFFtZHZja0puUlVVS1FXUmFOVUZuVVVOQ1NIZEZaV2RDTkVGSVdVRXpWREIzWVhOaVNFVlVTbXBIVWpSamJWZGpNMEZ4U2t0WWNtcGxVRXN6TDJnMGNIbG5Remh3TjI4MFFRcEJRVWRJYkdGbVp6Um5RVUZDUVUxQlVucENSa0ZwUVdKSE5tcDZiVUkyUkZCV1dUVXlWR1JhUmtzeGVUSkhZVk5wVW14c1IydHlSRlpRVXpsSmJGTktDblJSU1doQlR6WlZkbnBFYVVOYVFXOXZSU3RLZVdwaFpFdG5hV2xLT1RGS00yb3ZZek5CUTA5clJIcFhOamxaVUUxQmIwZERRM0ZIVTAwME9VSkJUVVFLUVRKblFVMUhWVU5OUVZCSlRUVjJVbVpIY0VGVWNqQTJVR1JDTURjeFpFOXlLMHhFSzFWQ04zbExUVWRMWW10a1UxTnJaMUp5U3l0bGNuZHdVREp6ZGdvd1NGRkdiM2h0WlRkM1NYaEJUM2htWkcxTWRIQnpjazFJZGs5cWFFSmFTMVoxVG14WmRXTkJaMVF4V1VWM1ZuZHNjR2QzYTFWUFdrWjRUemRrUnpONkNtVnZOWFJ3YVdoV1kyTndWMlozUFQwS0xTMHRMUzFGVGtRZ1EwVlNWRWxHU1VOQlZFVXRMUzB0TFFvPSJ9fX19","integratedTime":1681843022,"logIndex":18304044,"logID":"c0d23d6ad406973f9559f3ba2d1ca01f84147d8ffc5b8445c224f98b9591801d"}},"Issuer":"https://accounts.google.com","Subject":"andrey.smirnov@siderolabs.com"}}]
```

The image should be signed using [cosign certificate authority flow](https://docs.sigstore.dev/certificate_authority/certificate-issuing-overview/) by a Sidero Labs employee with and email from `siderolabs.com` domain.

## Reproducible Builds

Talos builds for `kernel`, `initramfs`, `talosctl`, ISO image, and container images are reproducible.
So you can verify that the build is the same as the one as provided on [GitHub releases page](https://github.com/siderolabs/talos/releases).

See [building Talos images]({{< relref "building-images" >}}) for more details.
