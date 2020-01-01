# [v0.4.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.3.0-beta.0...v0.4.0-alpha.0) (2020-01-01)

### Bug Fixes

- don't log `token` metadata field in grpc request log ([f1a7f86](https://github.com/talos-systems/talos/commit/f1a7f8670370bbbe604591bbf58508f69455f4e4))
- extend list of kmsg facilities ([a490e3c](https://github.com/talos-systems/talos/commit/a490e3c7ea27fc67d64f66181346e2dad1fa9dc2))
- fail on muliple nodes for commands which don't support it ([f3dff87](https://github.com/talos-systems/talos/commit/f3dff87957fa8e0a47c4cd05dd99e0fad3dd8287)), closes [#1663](https://github.com/talos-systems/talos/issues/1663)
- fix error format ([2a449ae](https://github.com/talos-systems/talos/commit/2a449aea2ffb7234a28536d4a3105e8b22f93d38))
- fix output formats ([0fae1bc](https://github.com/talos-systems/talos/commit/0fae1bc92d0511bb93e08bb0aa0d3d49fad4f1ff))
- issues discovered by lgtm tool ([de35b4d](https://github.com/talos-systems/talos/commit/de35b4d5af8c610749a0b04c768a064b844d6ab4))
- Reset default http client to work around proxyEnv ([48b5da4](https://github.com/talos-systems/talos/commit/48b5da4e87349b153fc5b42669696576c7f50409))
- set the correct kernel args for VMware ([815aa99](https://github.com/talos-systems/talos/commit/815aa99cc4ff319afb8a3633a0b17b67475a1210))
- use specified kubelet and etcd images ([dce12c2](https://github.com/talos-systems/talos/commit/dce12c2c3cbfaf5b7fc21ffca70222bc4042cdb2))
- use the correct mf file name ([3f6a2cb](https://github.com/talos-systems/talos/commit/3f6a2cb7f7f8ee85ad153b4d5c396263d564a327))
- **machined:** Add additional defaults for http transport ([f722adb](https://github.com/talos-systems/talos/commit/f722adb865c8c62a6e510d4db9db785a5d815ac6)), closes [#1680](https://github.com/talos-systems/talos/issues/1680)
- update `osctl list` to report node name ([53f1cda](https://github.com/talos-systems/talos/commit/53f1cda715d774dc52d270d7b9f6445dfbf719db))
- use dash for default talos cluster name in docker ([47ae014](https://github.com/talos-systems/talos/commit/47ae0148a2632d9002ee71dd81225ba0d22719ca))
- use the correct TLD for the container version label ([93ba252](https://github.com/talos-systems/talos/commit/93ba252e428661d11d678e6c78fe581884b32111))
- **networkd:** Check for IFF_RUNNING on link up ([64a7eeb](https://github.com/talos-systems/talos/commit/64a7eeb0e1965bcacded86ffa8ab78aafa874e8e))
- **networkd:** Make better route scoping decisions ([da88d7b](https://github.com/talos-systems/talos/commit/da88d7bcb37c29e00b31cf76a9a69da073e8c337))

### Features

- add installer command to installer container ([5a7eb63](https://github.com/talos-systems/talos/commit/5a7eb631b20940a0590f192e7c73c34f27cb9f86))
- add support for tailing logs ([6e05dd7](https://github.com/talos-systems/talos/commit/6e05dd70c4051e3837ac4b9c7aa583260b2125f0)), closes [#1564](https://github.com/talos-systems/talos/issues/1564)
- add support for tftp download ([31baa14](https://github.com/talos-systems/talos/commit/31baa14e36177072d8d6eff2d68469f31147f78c))
- humanize timestamp and size in `osctl list` output ([c24ce2f](https://github.com/talos-systems/talos/commit/c24ce2fd5f6f9bf25f209ea21e9997dc85b285d4)), closes [#1565](https://github.com/talos-systems/talos/issues/1565)
- implement streaming mode of dmesg, parse messages ([1fbf407](https://github.com/talos-systems/talos/commit/1fbf40796f5c40704c2b9aa6e8499a26916fae68)), closes [#1563](https://github.com/talos-systems/talos/issues/1563)
- osctl bash/zsh completion support ([4c18f21](https://github.com/talos-systems/talos/commit/4c18f21088139a22197ab87123d027050764cc79)), closes [#1500](https://github.com/talos-systems/talos/issues/1500)
- support specifying CIDR for docker network ([dc8aab6](https://github.com/talos-systems/talos/commit/dc8aab632d042ebe86480d5558c44f05f56d8a6e))
- upgrade Linux to v5.4.5 ([907f87d](https://github.com/talos-systems/talos/commit/907f87d8e0f814b822efeaddfab907b5692f275b))
