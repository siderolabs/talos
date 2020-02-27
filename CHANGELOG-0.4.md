# [v0.4.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.5...v0.4.0-alpha.6) (2020-02-27)

### Bug Fixes

- add reboot flag to reset command ([8a3a76f](https://github.com/talos-systems/talos/commit/8a3a76f73e1b31e464299415ffdb0672ab45083c))
- allow kublet to handle multiple service CIDRs ([16594a8](https://github.com/talos-systems/talos/commit/16594a83a8bd6f92f622c155387a193f3551cc4f)), closes [#1888](https://github.com/talos-systems/talos/issues/1888)
- default reboot flag to false ([9cf217d](https://github.com/talos-systems/talos/commit/9cf217d2c125a5cc5a2ad670a97c3302fb325feb))
- ensre proxy is used when fetching additional manifests for bootkube ([f0f5cca](https://github.com/talos-systems/talos/commit/f0f5cca30b73d2676a88c2c83647c38709387bb4))
- fix reset command ([8092362](https://github.com/talos-systems/talos/commit/8092362098a6f30b5ff8b8b7d0301b85af918867))
- PodCIDR, ServiceCIDR should be comma sets ([1a71753](https://github.com/talos-systems/talos/commit/1a7175353e473dc07ccfd2b7679cdc26ee6d0ffd)), closes [/kubernetes.io/docs/concepts/services-networking/dual-stack/#enable-ipv4-ipv6](https://github.com//kubernetes.io/docs/concepts/services-networking/dual-stack//issues/enable-ipv4-ipv6) [#1883](https://github.com/talos-systems/talos/issues/1883)
- refresh proxy settings from environment in image resolver ([cafd33a](https://github.com/talos-systems/talos/commit/cafd33acd84ae0b90b2a086c45cbbe599327cc1e)), closes [#1901](https://github.com/talos-systems/talos/issues/1901) [#1680](https://github.com/talos-systems/talos/issues/1680) [#1690](https://github.com/talos-systems/talos/issues/1690)
- stop firecracker launcher on signal ([afea21b](https://github.com/talos-systems/talos/commit/afea21bc5aacc2f01339361403a2633981c755c8))
- unmount bind mounts for system (fixes upgrade stuck on disk busy) ([8913d9d](https://github.com/talos-systems/talos/commit/8913d9df7afde1468030f206277581baff031551))
- validate install disk ([5b50456](https://github.com/talos-systems/talos/commit/5b50456c051f692c10f7bf687f494c451965a13a))

### Features

- add reboot flag to reset API ([fe7847e](https://github.com/talos-systems/talos/commit/fe7847e0b8982c725299ee892dbe745c7fc9ed6d))
- support proxy in docker buildx ([08b1a78](https://github.com/talos-systems/talos/commit/08b1a782cd40a470606a00842ab6091b601c6c91))
- support sending machine info ([63ca83a](https://github.com/talos-systems/talos/commit/63ca83a02ca037d6bb6eb117c4187757552332ba))

# [v0.4.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.4...v0.4.0-alpha.5) (2020-02-15)

### Bug Fixes

- do not add empty netconf ([5f34859](https://github.com/talos-systems/talos/commit/5f3485979ad206813d41d708cbfe628ffc696020)), closes [#1869](https://github.com/talos-systems/talos/issues/1869)
- don't proxy gRPC unix connections ([fcaed8b](https://github.com/talos-systems/talos/commit/fcaed8b0dd27f582a7f81b516f938a6eb2701349))

### Features

- implement registry mirror & config for image pull ([e1779ac](https://github.com/talos-systems/talos/commit/e1779ac77cd942d23fde1374ddebd04242de05db))

# [v0.4.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.3...v0.4.0-alpha.4) (2020-02-04)

### Bug Fixes

- bind etcd to IPv6 if available ([dbf408e](https://github.com/talos-systems/talos/commit/dbf408ea58bec8b6f10bbbd9d47dc1c0e42d320e)), closes [#1842](https://github.com/talos-systems/talos/issues/1842) [#1843](https://github.com/talos-systems/talos/issues/1843)
- **networkd:** fix ticker leak ([4593c4f](https://github.com/talos-systems/talos/commit/4593c4f7270ef62186c7b1b5593eee244ca43bda))
- follow symlinks ([f567f8c](https://github.com/talos-systems/talos/commit/f567f8c84d4248328d0c972102e37d9d810be6f7))
- implement kubelet extra mounts ([6d1a2f7](https://github.com/talos-systems/talos/commit/6d1a2f7b6d415bf5e017c733dc5025a7adb096f2))

### Features

- **networkd:** Add health api ([88df1b5](https://github.com/talos-systems/talos/commit/88df1b50b81d1b27428971f345ee9d72b7e23a93))
- **networkd:** Make healthcheck perform a check ([e911353](https://github.com/talos-systems/talos/commit/e9113537f909cee7d96d49fc7d96934d69841dce))

# [v0.4.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.2...v0.4.0-alpha.3) (2020-01-28)

### Bug Fixes

- correctly split lines with /dev/kmsg output ([1edc08a](https://github.com/talos-systems/talos/commit/1edc08aa245225161d85ee6d9e536bd840558769))
- install sequence stuck on event bus ([565c747](https://github.com/talos-systems/talos/commit/565c7475826c0ce651202c551e8b4d64451eb3a4))
- leave etcd after draining node ([e7749d2](https://github.com/talos-systems/talos/commit/e7749d2e8fce4cd435efcb36b06f228e907af268))
- parse correctly kernel command line missing DNS config ([cebd88f](https://github.com/talos-systems/talos/commit/cebd88f77c312c3886a023881f2aa6d89e0228b9))
- re-enable control plane flags ([aabd46e](https://github.com/talos-systems/talos/commit/aabd46e65103bc26870c67217ffbfbe135925c1c)), closes [#1523](https://github.com/talos-systems/talos/issues/1523)
- retry system disk busy check ([e495e29](https://github.com/talos-systems/talos/commit/e495e293080ccd7093cf15cbcf97cd19fce166a7))

### Features

- allow ability to customize containerd ([e0181c8](https://github.com/talos-systems/talos/commit/e0181c85eb32c64f3acd07340cb09d46b669820b)), closes [#1718](https://github.com/talos-systems/talos/issues/1718)
- allow for bootkube images to be customized ([67e50f6](https://github.com/talos-systems/talos/commit/67e50f6f50bd3d1b7a67cefe5688eb31c7befce5))
- update kernel ([4f39907](https://github.com/talos-systems/talos/commit/4f39907b6e6cdda3d3309b7e882f1275f74dcfb9))

# [v0.4.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.1...v0.4.0-alpha.2) (2020-01-21)

### Bug Fixes

- block when handling bus event ([5b5d171](https://github.com/talos-systems/talos/commit/5b5d171c07eecc9d85076eea609dd7ef1f277d6b))
- stop race condition between kubelet and networkd ([28782c2](https://github.com/talos-systems/talos/commit/28782c2d46d7cd98a79072e8a4987495b3e62ae6))
- update networkd permissions ([aac899f](https://github.com/talos-systems/talos/commit/aac899f23d5135a079f2cf5119a5b2ffe4945ae4))
- **networkd:** Fix incorrect resolver settings ([9321868](https://github.com/talos-systems/talos/commit/93218687ec1a2a3116911d66a45d200461b02b02))
- **networkd:** Set hostname properly for dhcp when no hostname option is returned ([3dff2b2](https://github.com/talos-systems/talos/commit/3dff2b234d24392b81d2cb42dbd73006fd89d9cc))
- add Close func in remote generator ([0e47df0](https://github.com/talos-systems/talos/commit/0e47df01c9e7e32d50ccc6d891ce9b17cfdf53dc))
- check for installer image before proceeding with upgrade ([5e8cab4](https://github.com/talos-systems/talos/commit/5e8cab4dd54923907cd4dc1266063d6962f498ec))
- Ensure assets directory does not exist ([5f14dd3](https://github.com/talos-systems/talos/commit/5f14dd3246fe4384d5a88e224bf6735d2e541446))
- raise default NOFILE limit ([33777da](https://github.com/talos-systems/talos/commit/33777da05dc24a2044d5710eb838921b467e450d))
- refuse to upgrade if single master ([7719a67](https://github.com/talos-systems/talos/commit/7719a6783405db010df22d9da2f0b3265f0e6cf8)), closes [#1770](https://github.com/talos-systems/talos/issues/1770)
- set kube-dns labels ([5cac4f5](https://github.com/talos-systems/talos/commit/5cac4f5f39b9e30deaab0b61d181ec9b74bc26db))
- shutdown on button/power ACPI event ([825d821](https://github.com/talos-systems/talos/commit/825d8215106275bcd3a871e0176cf0f1ff028872))
- Update bootkube to include node ready check ([9566690](https://github.com/talos-systems/talos/commit/95666900a760b619c7a0d49a1e503dda6a2f4f98))
- update kernel version constant ([cb93646](https://github.com/talos-systems/talos/commit/cb93646c078951fa667611735d29718a80c0f949))

### Features

- add a basic architectural diagram and a call to action ([d6f5ff3](https://github.com/talos-systems/talos/commit/d6f5ff34148ce7914510fc89c666e49583689bc5))
- allow additional manifests to be provided to bootkube ([4b81907](https://github.com/talos-systems/talos/commit/4b81907bd36351b6119ee8ec418bd486de79fa4a))
- upgrade kubernetes version to 1.17.1 ([60260c8](https://github.com/talos-systems/talos/commit/60260c85d119e3e39b26111aaba66f6132f455d3))
- upgrade Linux to v5.4.10 ([7edd969](https://github.com/talos-systems/talos/commit/7edd96947a33a39e12ab2ffe2dc4c4712dbf9a03))
- upgrade Linux to v5.4.11 ([e66ac62](https://github.com/talos-systems/talos/commit/e66ac62877eb4637dd030de78ca1bd15f06a992a))

# [v0.4.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.0...v0.4.0-alpha.1) (2020-01-09)

### Bug Fixes

- make the CNI URL error better ([7fb8289](https://github.com/talos-systems/talos/commit/7fb8289a223984937ee74f9241c57bc088de81d6))

### Features

- enable DynamicKubeletConfiguration ([79878c1](https://github.com/talos-systems/talos/commit/79878c1d8d56fa9823806789f30d8b9166a15f8d))
- support configurable docker-based clusters ([75d9f7b](https://github.com/talos-systems/talos/commit/75d9f7b454cb956ac3659347884c20ffac2c4021))
- Upgrade bootkube ([0742e52](https://github.com/talos-systems/talos/commit/0742e5245a393f15f916f891ddf07c9fb8d256fc))
- upgrade linux to v5.4.8 ([4242acd](https://github.com/talos-systems/talos/commit/4242acd085a573b8d117f779a87e3c5bf375434a))

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
