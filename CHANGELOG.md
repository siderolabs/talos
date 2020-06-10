<a name="v0.6.0-alpha.2"></a>

## [v0.6.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.1...v0.6.0-alpha.2) (2020-06-10)

### Chore

- update provision test versions

### Docs

- add v0.6 docs
- add kernel options to firecracker reqs
- remove repeated component in the Arges architecture image
- add talosctl docs document
- fix a few minor styling issues

### Feat

- update kubernetes to 1.19.0-beta.1
- update k8s and sonobuoy versions
- add rollback API
- allow reset API at all times
- adjust time properly in timed via adjtime()

### Fix

- allow node names
- make services depend on timed
- correctly handle IPv6 address in apid

### Refactor

- implement LoggingManager as central log flow processor

### Test

- fix race in some tests caused by `SetT`

<a name="v0.6.0-alpha.1"></a>

## [v0.6.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.0...v0.6.0-alpha.1) (2020-05-27)

### Chore

- fix markdown lint
- upgrade Go to 1.14.3 and use toolchain for race detector
- replace underlying event implementation with single slice

### Docs

- make v0.5 docs the default
- fix markdown
- add metal overview diagram
- fix broken links in components pages (fixes [#2117](https://github.com/talos-systems/talos/issues/2117))
- add some information about Arges and expand the bare metal section a bit
- overview of talos components

### Feat

- add LVM2
- implement simplified client method to consume events
- upgrade Linux to v5.6.13

### Fix

- prevent panic on nil pointer in ServiceInfo method
- bump service wait to ten minutes
- allow all seccomp profile names
- wrap etcd address URLs with formatting

### Release

- **v0.6.0-alpha.1:** prepare release

### Test

- improve reboot/reset test resiliency against request timeouts
- update Talos versions for upgrade tests
