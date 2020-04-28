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
