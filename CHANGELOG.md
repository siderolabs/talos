<a name="v0.5.0-beta.1"></a>

## [v0.5.0-beta.1](https://github.com/talos-systems/talos/compare/v0.5.0-beta.0...v0.5.0-beta.1) (2020-05-18)

### Chore

- fix nits in the events code

### Fix

- wrap etcd address URLs with formatting
- run machined API as a service
- respect nameservers when using docker cluster
- update Events API response type to match proxying conventions
- register event service with router

### Release

- **v0.5.0-beta.1:** prepare release

<a name="v0.5.0-beta.0"></a>

## [v0.5.0-beta.0](https://github.com/talos-systems/talos/compare/v0.5.0-alpha.2...v0.5.0-beta.0) (2020-05-13)

### Chore

- serialize firecracker e2e tests
- pin markdown linting libraries
- use clusterctl and v1alpha3 providers for tests
- fix prototool lint

### Docs

- add a sitemap and Netlify redirects
- adjust docs layouts and add tables of contents
- update copyright date
- backport intro text to 0.3 and 0.4 docs
- fix netlify deep linking for 0.5 docs by generating fallback routes
- add 0.5 pre-release docs, add linkable anchors, other fixes

### Feat

- add events API
- add support for file scheme
- enable rpfilter
- add bootstrap API
- add recovery API
- allow dual-stack support with bootkube wrapper

### Fix

- refactor client creation API
- update kernel package
- write machined RPC logs to file
- clean up docs page scripts in preparation for 0.5 docs
- ipv6 static default gateway not set if gateway is a LL unicast address

### Refactor

- remove warning about missing boot partition

### Release

- **v0.5.0-beta.0:** prepare release

### Test

- add node name to error messages in RebootAllNodes
- stabilize tests by bumping timeouts

<a name="v0.5.0-alpha.2"></a>

## [v0.5.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.5.0-alpha.1...v0.5.0-alpha.2) (2020-04-28)

### Chore

- fix markdown linting issues

### Docs

- add install and troubleshooting section in firecracker getting started

### Feat

- add commands talosctl health/crashdump

### Fix

- ensure disk is not busy
- pass dev path to mkfs

### Refactor

- improve machined

### Release

- **v0.5.0-alpha.2:** prepare release

<a name="v0.5.0-alpha.1"></a>

## [v0.5.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.5.0-alpha.0...v0.5.0-alpha.1) (2020-04-21)

### Chore

- add bug report issue template
- use a single CHANGELOG
- remove random.trust_cpu references
- update pkgs tag to v0.2.0
- address random CI nits

### Docs

- improve CLI menu and metal docs
- default to v0.4
- add firecracker documentation
- sidebar improvements and content organization

### Feat

- disable kubelet ro port
- make machine config persist by default
- add extra headers to fetch of extraManifests
- upgrade Go to 1.14.2

### Fix

- prevent formatting the ephemeral partition twice
- set ephemeral partition to max size
- ensure ordering of interfaces when deciding hostname
- resolve race condition in createNodes
- add hpsa drivers

### Refactor

- use upstream bootkube
- rename ntpd to timed
- rename system-containerd and containerd services
- don't log installer verification

### Release

- **v0.5.0-alpha.1:** prepare release
