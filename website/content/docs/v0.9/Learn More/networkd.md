# Networkd

Networkd is an internal Talos service which implements the core network
functionality.  Its scope is generally intended to not extend beyond what is
necessary to run Talos, get its configuration, and join the cluster.  Once
Kubernetes is running, all further network-related functionality should be
implemented inside kubernetes.

Obviously, this still leaves a large amount of necessary functionality to be
handled internally.

## Structure

Networkd is constructed around three core controllers:

  - Interface providers
  - Address providers
  - Route providers

These controllers take static configurations, API requests, and system events
in, and continually try to make the system's network reality match the requests.

Significantly, the testing for each provider is up to the provider.  In this
way, non-local network settings (such as cloud-provided, non-local public IP
address assignments) can also be implemented and tested.



