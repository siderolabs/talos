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

### Test

- improve reboot/reset test resiliency against request timeouts
- update Talos versions for upgrade tests
