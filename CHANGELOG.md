<a name="v0.7.0-alpha.0"></a>

## [v0.7.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.6.0-beta.1...v0.7.0-alpha.0) (2020-08-17)

### Chore

- re-import talos-systems/pkg/crypto/tls
- extract pkg/crypto as external module
- integrate importvet
- update capi CI manifests to use control planes
- update node dependencies
- update packages

### Docs

- fix download link

### Feat

- upgrade etcd to 3.4.10
- add persist flag to gen config

### Fix

- run health check for etcd service with Get API
- ignore eth0 interface in docker provisioner
- update e2e scripts to work with python3
- retry non-HTTP errors from API server
- update qemu launcher on arm64 to boot Talos properly

### Refactor

- move external API packages into `machinery/`
- rework `pkg/grpc/tls` to break dependency on `pkg/grpc/gen`
- extract `pkg/net` as `github.com/talos-systems/net`
- expose `provision` as public package
- remove structs from config provider
