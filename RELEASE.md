# Release Policy

Talos Linux provides minor releases every 4 months, aligned with the Kubernetes release cycle.
A new minor release is made available at the end of April, August, and December.
A release candidate is made available two weeks before the GA release, with two beta versions available before the release candidate.

A detailed schedule for the upcoming release can be found in the [GitHub repository issues](https://github.com/siderolabs/talos/issues), where the relevant issue is pinned.

Security updates are provided for the two latest minor releases of Talos Linux.
For example, if the latest release is `vX.Y.Z`, the supported releases are `vX.Y-1.x` and `vX.Y.x`.
See [Security](SECURITY.md) for more information on how to report security issues.

Patch releases are done every two weeks for the last minor release, and every month for the previous minor release. In case of a critical issue, a patch release may be made available for the last two minor releases.
