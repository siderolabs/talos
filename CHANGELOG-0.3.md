# [](https://github.com/andrewrynhard/talos/compare/v0.3.1...v) (2020-01-28)


### Bug Fixes

* parse correctly kernel command line missing DNS config ([9933f75](https://github.com/andrewrynhard/talos/commit/9933f7574a55c3fadcaeadeb41ae66d0ab41ccdd))
* re-enable control plane flags ([a4a1229](https://github.com/andrewrynhard/talos/commit/a4a12296ef71d64b5a40189db27067864ae71fe5)), closes [#1523](https://github.com/andrewrynhard/talos/issues/1523)
* retry system disk busy check ([c9cef6d](https://github.com/andrewrynhard/talos/commit/c9cef6dd57c27004ab76f17c62ba006bc1a9f079))



# [v0.3.1](https://github.com/talos-systems/talos/compare/v0.3.0...v0.3.1) (2020-01-22)

### Bug Fixes

- install sequence stuck on event bus ([485c9c7](https://github.com/talos-systems/talos/commit/485c9c7a75065a61364702eda537264e35a3df34))
- leave etcd after draining node ([a0066c7](https://github.com/talos-systems/talos/commit/a0066c7c9031cf4ac9bb0e9b0bb39585d61ae2bd))

# [v0.3.0](https://github.com/talos-systems/talos/compare/v0.3.0-rc.0...v0.3.0) (2020-01-21)

### Bug Fixes

- add Close func in remote generator ([a5184ea](https://github.com/talos-systems/talos/commit/a5184ead2b6f16164ec48d5797d6e08acc433dca))
- block when handling bus event ([4804033](https://github.com/talos-systems/talos/commit/4804033674da67c840c5d9bd8b567a187e4cae6a))
- Ensure assets directory does not exist ([a8ece47](https://github.com/talos-systems/talos/commit/a8ece47e9cd9004b010f0f131bd38f127230c787))
- refuse to upgrade if single master ([ed9d92f](https://github.com/talos-systems/talos/commit/ed9d92f8531ffcad17645f478136da97d6c74379)), closes [#1770](https://github.com/talos-systems/talos/issues/1770)
- stop race condition between kubelet and networkd ([995110d](https://github.com/talos-systems/talos/commit/995110d36431e766b178da7283e8a81fb0649d98))
- Update bootkube to include node ready check ([884d8da](https://github.com/talos-systems/talos/commit/884d8da693fa509f3ee93f75d858432b8c050461))
- update networkd permissions ([79b721b](https://github.com/talos-systems/talos/commit/79b721bc2f14d3dc6d53e77c5b23c418b88dccc5))
- **networkd:** Fix incorrect resolver settings ([4d78f17](https://github.com/talos-systems/talos/commit/4d78f17231d3f1a84e50d15e8b481e0c75d92575))
- **networkd:** Set hostname properly for dhcp when no hostname option is returned ([d5280a0](https://github.com/talos-systems/talos/commit/d5280a099dae7a476b0db8d411f60945122175dc))
- update kernel version constant ([749cbc7](https://github.com/talos-systems/talos/commit/749cbc7f1af1148e105cc529750665587ace9864))

### Features

- allow additional manifests to be provided to bootkube ([4cdeee4](https://github.com/talos-systems/talos/commit/4cdeee4dae5fe6ad56ca8ce8ae13d20037b6fe8a))
- upgrade kubernetes version to 1.17.1 ([c37e6a1](https://github.com/talos-systems/talos/commit/c37e6a1b929536fc8c258c5893aee01a3e3656bc))
- upgrade Linux to v5.4.11 ([c7dcfe3](https://github.com/talos-systems/talos/commit/c7dcfe384a0904dd45c0f7031c51ffbf766c038b))

# [v0.3.0-rc.0](https://github.com/talos-systems/talos/compare/v0.3.0-beta.3...v0.3.0-rc.0) (2020-01-11)

### Bug Fixes

- check for installer image before proceeding with upgrade ([198cd8b](https://github.com/talos-systems/talos/commit/198cd8b527800ff2272c7dfb06bb391da529ffa6))
- make the CNI URL error better ([b71aec5](https://github.com/talos-systems/talos/commit/b71aec549d98ee39399b4fe559191341809ad2f2))
- raise default NOFILE limit ([3fdf74c](https://github.com/talos-systems/talos/commit/3fdf74c2665fd27fcb0713fc78a6c0e4d0d18923))
- set kube-dns labels ([375d2af](https://github.com/talos-systems/talos/commit/375d2af16d68373a3154ea85355eae2a241f0096))
- shutdown on button/power ACPI event ([9fec814](https://github.com/talos-systems/talos/commit/9fec814db437365fae4fed00861357def801d24a))

### Features

- Upgrade bootkube ([1ae6c81](https://github.com/talos-systems/talos/commit/1ae6c81ea3e50f7bea12186b0076d919c400bc6e))
- upgrade Linux to v5.4.10 ([bfa70ff](https://github.com/talos-systems/talos/commit/bfa70ff480b13132893d5688b94c18be088c4fa3))
- upgrade linux to v5.4.8 ([e5cbbf7](https://github.com/talos-systems/talos/commit/e5cbbf73e7bf14423c7f83eb23b1b2248727fa16))

# [v0.3.0-beta.3](https://github.com/talos-systems/talos/compare/v0.3.0-beta.2...v0.3.0-beta.3) (2020-01-01)

### Bug Fixes

- set the correct kernel args for VMware ([f8d638d](https://github.com/talos-systems/talos/commit/f8d638d09580d048f861bb3c0fc63ecb5809c643))
- use the correct mf file name ([07731eb](https://github.com/talos-systems/talos/commit/07731eb22e8ada593b1e7515b86e48d2485e2fe3))
- use the correct TLD for the container version label ([c4d0fe1](https://github.com/talos-systems/talos/commit/c4d0fe148534b2d237af67715250eaf3f4f02497))
- **machined:** Add additional defaults for http transport ([7c57cd7](https://github.com/talos-systems/talos/commit/7c57cd72343bef08458af532a2c6aafaabfe4556)), closes [#1680](https://github.com/talos-systems/talos/issues/1680)
- don't log `token` metadata field in grpc request log ([b6e16cd](https://github.com/talos-systems/talos/commit/b6e16cd935f28ff487c821ae1bd9c75e1852f8e1))
- extend list of kmsg facilities ([3df7104](https://github.com/talos-systems/talos/commit/3df710427b17908a486217270910e29872e40c9c))
- **networkd:** Make better route scoping decisions ([df165ba](https://github.com/talos-systems/talos/commit/df165ba4ada6acba6ef9a515e360d602f8709ee4))
- fix output formats ([afc4bd2](https://github.com/talos-systems/talos/commit/afc4bd2b6fbd7c0df52dc1dfb647978815fd0a92))

### Features

- add installer command to installer container ([7a5141a](https://github.com/talos-systems/talos/commit/7a5141a64956dbe41536999ff5c5ba34ecc0ee7a))
- support specifying CIDR for docker network ([b795d7a](https://github.com/talos-systems/talos/commit/b795d7a260dd2f4750a1fe1ff44e08adf9033e33))

# [v0.3.0-beta.2](https://github.com/talos-systems/talos/compare/v0.3.0-beta.1...v0.3.0-beta.2) (2019-12-21)

### Bug Fixes

- **networkd:** Check for IFF_RUNNING on link up ([f7ca6d3](https://github.com/talos-systems/talos/commit/f7ca6d326ecf60a8bbc37056fe797da1ce8ce82d))
- Reset default http client to work around proxyEnv ([b342629](https://github.com/talos-systems/talos/commit/b34262973dcd6e9159323358aff11415e8c67661))

### Features

- add support for tailing logs ([8830788](https://github.com/talos-systems/talos/commit/8830788a7f6dddc14e1bb764fd5a6cf8aa67fcc5)), closes [#1564](https://github.com/talos-systems/talos/issues/1564)
- upgrade Linux to v5.4.5 ([bab9213](https://github.com/talos-systems/talos/commit/bab9213fb0877b36328cc923167a2cc934db9a2d))

# [v0.3.0-beta.1](https://github.com/talos-systems/talos/compare/v0.3.0-beta.0...v0.3.0-beta.1) (2019-12-19)

### Bug Fixes

- fail on muliple nodes for commands which don't support it ([462b01e](https://github.com/talos-systems/talos/commit/462b01e1f0e50cb631ecfefc55a2e45f6ea5b835)), closes [#1663](https://github.com/talos-systems/talos/issues/1663)
- issues discovered by lgtm tool ([f1b33b8](https://github.com/talos-systems/talos/commit/f1b33b8fbbc4d6d616cf8c9a15e9e7bf6fd7c1b7))
- update `osctl list` to report node name ([6eb5f33](https://github.com/talos-systems/talos/commit/6eb5f33281f2717cdadb859a781576d671eaa442))
- use specified kubelet and etcd images ([ad7c638](https://github.com/talos-systems/talos/commit/ad7c638f3498185598786fc0f2757cf4159d135d))

### Features

- humanize timestamp and size in `osctl list` output ([2b14182](https://github.com/talos-systems/talos/commit/2b14182208bcb90803dc91c484f820afc5ed12d5)), closes [#1565](https://github.com/talos-systems/talos/issues/1565)
- implement streaming mode of dmesg, parse messages ([2eb0937](https://github.com/talos-systems/talos/commit/2eb09372c21c012d52948550c02701673580fa69)), closes [#1563](https://github.com/talos-systems/talos/issues/1563)

# [v0.3.0-beta.0](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.10...v0.3.0-beta.0) (2019-12-11)

### Bug Fixes

- Add hostname setting to networkd ([e1651a8](https://github.com/talos-systems/talos/commit/e1651a8a986fb2bec938957930397353504a4334))
- add missing sysctl params required by containerd ([e8bb6b9](https://github.com/talos-systems/talos/commit/e8bb6b9119d2e8cf5e8bd964024813d1433d815e))
- allow initial-cluster-state to be set ([3725975](https://github.com/talos-systems/talos/commit/3725975df94c9ca1364389d92c822b4171be28cb))
- append domainname to DHCP-sourced hostname ([d8caa53](https://github.com/talos-systems/talos/commit/d8caa5316a9818cad23b62f9ecaacfc1a6ca217d)), closes [#1628](https://github.com/talos-systems/talos/issues/1628)
- close io.ReadCloser ([829c3d7](https://github.com/talos-systems/talos/commit/829c3d72aa43e086e36c995d4a7c2335b01b98ee))
- don't set br_netfilter sysctls in container mode ([e8a5c13](https://github.com/talos-systems/talos/commit/e8a5c132bdadb66be8b52f6dd3a6f9e018f4a9cc))
- don't use netrc ([d1c050d](https://github.com/talos-systems/talos/commit/d1c050d2daa391d1193aae95d7bead1c501bfe9a))
- error reporting in `osctl kubeconfig` ([b1d282a](https://github.com/talos-systems/talos/commit/b1d282adf3892a7342d6931a0ef72e1637c70a17))
- extract errors from API response ([10a40a1](https://github.com/talos-systems/talos/commit/10a40a15d964902ad2b678a166bc19db2a7bf074))
- improve the project site meta description ([9a2fd98](https://github.com/talos-systems/talos/commit/9a2fd989c9243ae94401ee7681361cc05be468b3))
- kill POD network mode pods first on upgrades ([fa515b8](https://github.com/talos-systems/talos/commit/fa515b81171059386ddff03280f2989e0ac1fd3b))
- make retry errors ordered ([6d8194b](https://github.com/talos-systems/talos/commit/6d8194be2154809d42ccd8c46864638de3a3397b))
- mount /run as shared in container mode ([9325f12](https://github.com/talos-systems/talos/commit/9325f124d7f26df4b48be7208e1455bd1235412a))
- mount as rshared ([f8c2f14](https://github.com/talos-systems/talos/commit/f8c2f14119b81f33c3d5d749787d9086aac14bdf))
- provide peer remote address for 'NODE': as default in osctl ([fc52025](https://github.com/talos-systems/talos/commit/fc52025490d357e79c38a7bfefcb02f3a193b7f6))
- response filtering for client API, RunE for osctl ([e907507](https://github.com/talos-systems/talos/commit/e907507aa690940dd5f23aaa47b06df72071aa94))
- **networkd:** Ignore loopback interface during hostname decision. ([653100d](https://github.com/talos-systems/talos/commit/653100dc3b6659f84ffe8af09a8727210053ad93))
- return a unique set of errors on retry failure ([66052d6](https://github.com/talos-systems/talos/commit/66052d6304c4a35c8f54336160d8eccde361ff8a))
- reverse preference order of network config ([9d9b958](https://github.com/talos-systems/talos/commit/9d9b958fba8c56dda640371fdc4441cb9a1d9cc1)), closes [#1588](https://github.com/talos-systems/talos/issues/1588)
- run go mod tidy ([4fa324a](https://github.com/talos-systems/talos/commit/4fa324a9bed0044882498bfdd189e0d2c3141a8b))
- strip line feed from domainname after read ([549db4d](https://github.com/talos-systems/talos/commit/549db4d3b18fa623804c77513d0abc1c08781748)), closes [#1624](https://github.com/talos-systems/talos/issues/1624)
- update kernel version constant ([7b6a1fd](https://github.com/talos-systems/talos/commit/7b6a1fdc94c4ccf90c8a7872313bea71ef390466))
- update node dependencies for project website ([343cba0](https://github.com/talos-systems/talos/commit/343cba04d3af8674a3250168543baff583cd3e0d))

### Features

- add ability to append to existing files with extrafiles ([84354c5](https://github.com/talos-systems/talos/commit/84354c59414b6795af94e7c62b7443a077064913)), closes [#1467](https://github.com/talos-systems/talos/issues/1467)
- add config nodes command ([f86465e](https://github.com/talos-systems/talos/commit/f86465ecae89557bd59e439041f57f5b86e4c153))
- add create and overwrite file operations ([fa4fb4d](https://github.com/talos-systems/talos/commit/fa4fb4d4448b1715ed05339dd1cfd200c618e00c))
- add domain search line to resolv.conf ([b597306](https://github.com/talos-systems/talos/commit/b597306989e6a72385bccf688450709b75f23492)), closes [#1626](https://github.com/talos-systems/talos/issues/1626)
- add security hardening settings ([09fbe2d](https://github.com/talos-systems/talos/commit/09fbe2d9ad23dec09cb08bf6092140dd352dceae))
- add support for `osctl logs -f` ([edb4043](https://github.com/talos-systems/talos/commit/edb40437ece722ceadb4f6a88b1aa7c51a347dc3))
- add universal TUN/TAP device driver support ([1f4c172](https://github.com/talos-systems/talos/commit/1f4c17269d2116f19535edafdb834785071beda8))
- allow ability to specify custom CNIs ([92b5bd9](https://github.com/talos-systems/talos/commit/92b5bd9b2be0a34303f88fe3a2754e731422e364)), closes [#1593](https://github.com/talos-systems/talos/issues/1593)
- allow configurable SANs for API ([e1ac4c4](https://github.com/talos-systems/talos/commit/e1ac4c4151dfe168efc2fb8dd63f469b88417372))
- allow deep-linking to specific docs pages ([4debea6](https://github.com/talos-systems/talos/commit/4debea685685aba2481f53dd2f8e5e9fd6806a15))
- make osd.Dmesg API streaming ([3a93e65](https://github.com/talos-systems/talos/commit/3a93e65b5480a02c22397244284417d4ee5c5b46))
- osctl logs now supports multiple targets ([5b316f7](https://github.com/talos-systems/talos/commit/5b316f7ea3bafea845e0b12dd8ba8bf6ad6e5e94))
- rename confusing target options, --endpoints, etc. ([399aeda](https://github.com/talos-systems/talos/commit/399aeda0b9470e4d3c7b14d701fb9ecdc64bbaf0)), closes [#1610](https://github.com/talos-systems/talos/issues/1610)
- support client only version for osctl ([190f0c6](https://github.com/talos-systems/talos/commit/190f0c6281881ae671b3275056fc86cf39838a46)), closes [#1363](https://github.com/talos-systems/talos/issues/1363)
- support output directory for osctl config generate ([739ce61](https://github.com/talos-systems/talos/commit/739ce61efa44917ce60aede56f1059695cbc93bc)), closes [#1509](https://github.com/talos-systems/talos/issues/1509)
- upgrade containerd to v1.3.2 ([43e6703](https://github.com/talos-systems/talos/commit/43e6703b8b92251756dc43d6ac503e67c04fe37b))
- Upgrade kubernetes to 1.17.0 ([9584b47](https://github.com/talos-systems/talos/commit/9584b47cd75c10c16da61ad608350c9209e3480c))
- upgrade Linux to v5.3.15 ([0347286](https://github.com/talos-systems/talos/commit/034728651156985b7732fbf41c11e14b9e16cf37))
- use containerd-shim-runc-v2 ([1d3cc00](https://github.com/talos-systems/talos/commit/1d3cc0038b5a090f64786705b0d38d280196101a))

# [v0.3.0-alpha.10](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.9...v0.3.0-alpha.10) (2019-12-02)

### Bug Fixes

- don't measure overlayfs ([4bec94f](https://github.com/talos-systems/talos/commit/4bec94f6552394ff811aa885e588d2bbd59d98c0))
- ensure etcd comes back up on reboot of all members ([f3882e7](https://github.com/talos-systems/talos/commit/f3882e7e0ac8b1d1dc00c48561c452e14582eaef))
- osctl panic when metadata is nil ([f0a080a](https://github.com/talos-systems/talos/commit/f0a080a34042fcdb281967bd18bbb588be526fea))
- prevent nil pointer panic ([aef38d0](https://github.com/talos-systems/talos/commit/aef38d0e1104b24d2441d7ab34efccbdb8c71c8e))
- provide a way for client TLS config to use Provider ([ad2f257](https://github.com/talos-systems/talos/commit/ad2f2574d7e769e3c9ea185c4184179a728b761e))
- recover control plane on reboot ([aaefcbd](https://github.com/talos-systems/talos/commit/aaefcbd8919b1ef132fefd6801c0e97d6698352a))
- require mode flag when validating ([c9a91b7](https://github.com/talos-systems/talos/commit/c9a91b7d9d5410ad615014965ddac86c4ba14030))
- update kernel version constant ([9745c3a](https://github.com/talos-systems/talos/commit/9745c3a504dc0d21ca2d61f6b92bc44c6d7fac2d))

### Features

- **networkd:** Add support for bonding ([119bf3e](https://github.com/talos-systems/talos/commit/119bf3e7bbc45630225c2d021ae1f5afd4e0e6ca))
- add Google Analytics tracking to the project website ([83d9e01](https://github.com/talos-systems/talos/commit/83d9e0121792f39d28482d0cba3bcc78c8dee409))
- add IMA policy ([031c65b](https://github.com/talos-systems/talos/commit/031c65be47ccb13af11e26861c565fd0d4b47359))
- enable aggregation layer ([48d5aac](https://github.com/talos-systems/talos/commit/48d5aac0fc34d3f05684b7dd81a825354e03bff3))
- enable IMA measurement and appraisal ([3f49a15](https://github.com/talos-systems/talos/commit/3f49a15c06b9e4c076be3a15979df29839e3da25))
- enable webhook authorization mode ([21c4aa8](https://github.com/talos-systems/talos/commit/21c4aa8aa6c8a8573cbfef104258981e680b63c5))
- support force flag for osctl kubeconfig ([c8f7336](https://github.com/talos-systems/talos/commit/c8f7336569049366c1c282a80e8bfedb4521df81))
- upgrade packages ([9ea041c](https://github.com/talos-systems/talos/commit/9ea041c7d9872d102d4d6dcc3fc9fdb2c1c5b8f6))
- use grpc-proxy in apid ([5b7bea2](https://github.com/talos-systems/talos/commit/5b7bea2471e823391a754efc44cd296e655fff1a))
- **networkd:** Add support for kernel nfsroot arguments. ([05c1659](https://github.com/talos-systems/talos/commit/05c1659126714063bf84af1d79bb4f0a44c1bba1))

# [v0.3.0-alpha.9](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.8...v0.3.0-alpha.9) (2019-11-25)

### Bug Fixes

- require arg length of 1 for kubeconfig command ([7b99d32](https://github.com/talos-systems/talos/commit/7b99d32f1e7001d465c1d7e22a93a36b267a5641))
- retry cordon and uncordon ([6a1a9fc](https://github.com/talos-systems/talos/commit/6a1a9fc8d97fed6c87c1c5fac6afb324947efb73))

### Features

- add read API ([ac089dc](https://github.com/talos-systems/talos/commit/ac089dc33049c31ab6380eef64ac2cde89954be4))
- allow sysctl writes ([43ad18f](https://github.com/talos-systems/talos/commit/43ad18fbeedbedc33156c2d46dafc85829c2c743))
- upgrade packages ([e78e165](https://github.com/talos-systems/talos/commit/e78e1655f1e2039e78c5dfb65c465104a6c2d2f6))

# [v0.3.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.7...v0.3.0-alpha.8) (2019-11-15)

### Bug Fixes

- honor the extraArgs option for the kubelet ([82c5936](https://github.com/talos-systems/talos/commit/82c59368af6eb4754d0836a376c4015c224648dd))
- make logging middleware first in the list, fix duration ([bb89d90](https://github.com/talos-systems/talos/commit/bb89d908b349312e21a370387ad6ec68abc3f7c7))
- set --upgrade flag properly on installs ([cbca760](https://github.com/talos-systems/talos/commit/cbca760562a92f4006ac11fd23c344e4c354cd78))
- use the config's cluster version for control plane image ([d2787db](https://github.com/talos-systems/talos/commit/d2787db99319fe3fd22dfb065bdd94dd4810adaa))

### Features

- Add context key to osctl ([83d5f4c](https://github.com/talos-systems/talos/commit/83d5f4c7210fb8cb553acbdbf63ba4c5cdbed28d))
- Add support for resetting the network during bootup ([d67fbf2](https://github.com/talos-systems/talos/commit/d67fbf269b58346ab49214e0bf0f9f04b422e2ce))
- allow extra arguments to be passed to etcd ([e1fc901](https://github.com/talos-systems/talos/commit/e1fc9017d2abbd44cfea0d8963b1e63792510634))

# [v0.3.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.6...v0.3.0-alpha.7) (2019-11-12)

### Bug Fixes

- conditionally create a new etcd cluster ([8ca4d49](https://github.com/talos-systems/talos/commit/8ca4d493479f9dae3d5790b0ad36cbc93aa0f2c2))
- mount extra disks after system disk ([34eb691](https://github.com/talos-systems/talos/commit/34eb691f81604fcbbb0683a35f27aadd2bd4f372))
- pass x509 options to NewCertificateFromCSR ([85638f5](https://github.com/talos-systems/talos/commit/85638f5d90985539b695a2a9588d6d60de425540))
- recover from panics in grpc servers ([add4a8d](https://github.com/talos-systems/talos/commit/add4a8d5abeeb3eb1589f85731ca223a712dbff7))
- remove duplicate line ([b3fd851](https://github.com/talos-systems/talos/commit/b3fd85174a14fcbe6aec7220ef5e47564f1f3b57))
- remove global variable in bootkube ([e2d9cc5](https://github.com/talos-systems/talos/commit/e2d9cc5438cb63ac8dceeeb5a9fef9bfe61175de))
- upgrade rtnetlink package ([9218fa8](https://github.com/talos-systems/talos/commit/9218fa8b2196491b8a584535e65b21005a088788))

### Features

- **networkd:** Add support for custom nameservers ([32fe629](https://github.com/talos-systems/talos/commit/32fe6297fe5b91e73a37d7460ddafacb92179251))
- Add meminfo api ([531e7d8](https://github.com/talos-systems/talos/commit/531e7d8144dfb4a0a1fbe35b6b4b0c8e9faaab8d))
- add metadata file to boot partition ([17cce54](https://github.com/talos-systems/talos/commit/17cce5468fa5fec32cd8dd5ba8c7e9434972f356))
- Add support for defining ntp servers via config ([e667a08](https://github.com/talos-systems/talos/commit/e667a08bf0813eac0b41d4cf2a5ac97c6111019f))
- Add support for setting container output to stdout ([6519c57](https://github.com/talos-systems/talos/commit/6519c575f85f2cb02316247774665c48db6100e0))
- Add support for streaming apis in apid ([7897374](https://github.com/talos-systems/talos/commit/7897374ff1844e2c2dc0730d00963bf12689d7e7))
- Disable networkd configuration if `ip` kernel parameter is specified ([8988c1c](https://github.com/talos-systems/talos/commit/8988c1c6a0b96a8cb59d540302a50d389e27b168))
- implement grpc request loggging ([e658c44](https://github.com/talos-systems/talos/commit/e658c442a668ae144d1dc1a98368d60269d6b694))

# [v0.3.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.5...v0.3.0-alpha.6) (2019-11-05)

### Bug Fixes

- add etcd member conditionally ([a82ed0c](https://github.com/talos-systems/talos/commit/a82ed0c5b705231e4a779d505b2871f3df27eaf9))
- Add host network namespace to networkd and ntpd ([db00c83](https://github.com/talos-systems/talos/commit/db00c83207f513760c3c8b149930f3c30c7054d0))
- Avoid running bootkube on reboots ([5abbb9b](https://github.com/talos-systems/talos/commit/5abbb9b04154dfa44401bd6cb8a5eca1de485784))
- be explicit about installs ([d15e226](https://github.com/talos-systems/talos/commit/d15e226998d6fe75a55a56481ce6360e655ca8c9))
- Disable support for proxy variables for apid. ([4b3cc34](https://github.com/talos-systems/talos/commit/4b3cc34ab04408efd46b61346126b7795628aae1))
- **osd:** Add additional capabilities for osd ([4653745](https://github.com/talos-systems/talos/commit/4653745acd844d6eaab02dd0c46c3e2d7eac7fb4))
- don't use 127.0.0.1 for etcd client ([33468f4](https://github.com/talos-systems/talos/commit/33468f4d6a4bd1df9e49c73a42aa56f9e40bcde6))
- retry BLKPG operations ([e9296be](https://github.com/talos-systems/talos/commit/e9296bed6e708abfcd0bf6c4a08dbd3f5d690119))
- send SIGKILL to hanging containers ([45a3406](https://github.com/talos-systems/talos/commit/45a3406fba3dd6c5df0d658b41ce2d81ecee5b5d))
- sleep in NTP query loop ([06009f6](https://github.com/talos-systems/talos/commit/06009f66c8ec9da1fb7dbeebb3a2de07f3613318))
- stop etcd and remove data-dir ([18f5c50](https://github.com/talos-systems/talos/commit/18f5c50a322bc392c3d0890a53e316248472bc7f))
- stop leaking file descriptors ([f411491](https://github.com/talos-systems/talos/commit/f4114914845a8daf5ba111c65c10dd61c4889ff8))
- use CRI to stop containers ([8f10462](https://github.com/talos-systems/talos/commit/8f10462795ec4c2b2a5d857a980e106349361361))
- verify system disk not in use ([7eb5b6b](https://github.com/talos-systems/talos/commit/7eb5b6b74832cc32b8775af8f7a4db47fafecfb9))
- verify that all etcd members are running before upgrading ([c973245](https://github.com/talos-systems/talos/commit/c9732458c120224c36a3827adf0a54539bccab24))

### Features

- add timestamp to installed file ([3ce6f34](https://github.com/talos-systems/talos/commit/3ce6f34995672e0248408fb0844180c9d85cf815))
- create cluster with default PSP ([dc38704](https://github.com/talos-systems/talos/commit/dc3870453bbe1c5d1f1327f408779b7d9b74ca2e))
- output machined logs to /dev/kmsg and file ([e81b3d1](https://github.com/talos-systems/talos/commit/e81b3d11a88a3d10c79fe0d8e28cf820894ea05a))

# [v0.3.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.4...v0.3.0-alpha.5) (2019-10-31)

### Bug Fixes

- check if endpoint is nil ([9933fc0](https://github.com/talos-systems/talos/commit/9933fc0fba17191a2eb57d2361450bd57810e585))

### Features

- Add support for creating VMware images ([ca76ccd](https://github.com/talos-systems/talos/commit/ca76ccd4afa865a9ce6d8b63bbd69dbd1fd3a6d1))
- lock down container permissions ([41619f9](https://github.com/talos-systems/talos/commit/41619f90160eab5229d7e1bee5c6fbb63f403a1e))
- upgrade Kubernetes to 1.16.2 ([3c6d013](https://github.com/talos-systems/talos/commit/3c6d0135d03c69b79a4bbc8887dcde04b83c1510))
- use Ed25519 public-key signature system ([82e43e0](https://github.com/talos-systems/talos/commit/82e43e05707fb0484d4e74b6b3bc9fac4cdc11f0))

# [v0.3.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.3...v0.3.0-alpha.4) (2019-10-28)

### Bug Fixes

- add cluster endpoint to certificate SANs ([2459ca1](https://github.com/talos-systems/talos/commit/2459ca14da55ad9b28a0053c296906ccd61c3d71))
- Fix osctl version output ([6de32dd](https://github.com/talos-systems/talos/commit/6de32dd30b4465b7c3235d529c92af7868c028d1))

### Features

- Add APId ([573cce8](https://github.com/talos-systems/talos/commit/573cce8d185db981805f08ff481d3d1d93d04a56))
- Add network api to apid ([457c641](https://github.com/talos-systems/talos/commit/457c6416a61c563fd14dab6cdb2f2268a3d98c51))
- Add retry on get kubeconfig ([c6e1e6f](https://github.com/talos-systems/talos/commit/c6e1e6f28f6878150fcf558b32b195ee1dd3158c))
- add support for Digital Ocean ([0d1c5ac](https://github.com/talos-systems/talos/commit/0d1c5ac30575f7157cd16d88305a31b76c97e680))
- Add time api to apid ([ee24e42](https://github.com/talos-systems/talos/commit/ee24e423196e8a87e036aa78c8baf74180bbdd1c))

# [v0.3.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.2...v0.3.0-alpha.3) (2019-10-25)

### Bug Fixes

- append localhost to cert sans if docker platform ([b615418](https://github.com/talos-systems/talos/commit/b615418e110411c6d82b4cfde3da5601bcd3ab0e))
- create external IP failures as non-fatal ([bccaa36](https://github.com/talos-systems/talos/commit/bccaa36b4419cf4f02bfff9db14c73d28da2264e))
- ensure control plane endpoint is set ([638d36b](https://github.com/talos-systems/talos/commit/638d36bce7abbf23a873ac80322ef11ec7f61930))

### Features

- Add node metadata wrapper to machine api ([251ab16](https://github.com/talos-systems/talos/commit/251ab16e075050d6d42253963f27a787eaed4c3e))
- detect gzipped machine configs ([d8db2bc](https://github.com/talos-systems/talos/commit/d8db2bc65bab8705889d88b7fbc8fb729d5ffb75))

# [v0.3.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.1...v0.3.0-alpha.2) (2019-10-21)

### Bug Fixes

- add slub_debug=P to ISO kernel args ([6c33547](https://github.com/talos-systems/talos/commit/6c335474524b60f0ea9f6aef02d082528bd68d8c))
- always run networkd ([3ded7e3](https://github.com/talos-systems/talos/commit/3ded7e3b2c75dc7244262627f1785d0c17d48f42))
- check if cluster network config is nil ([6c3b0ef](https://github.com/talos-systems/talos/commit/6c3b0ef442d5a8dc35643690d212d1d5796bf09f))
- run only essential services in container mode ([8b0bd34](https://github.com/talos-systems/talos/commit/8b0bd3408ce1c780fcf71a045540ee4278bbc461))
- set packet and metal platform mode to metal ([3343144](https://github.com/talos-systems/talos/commit/3343144a11695097e33b06ee1063f59ec79123ea))
- use localhost for osd endpoint on masters ([533b9f4](https://github.com/talos-systems/talos/commit/533b9f4757ec17afbdd78b63be0aacaf15e71715))
- use talos.config instead of talos.userdata ([792a35e](https://github.com/talos-systems/talos/commit/792a35e8ae6c182fb8546d50ea3209506d696696))

### Features

- add config validation task ([94c2865](https://github.com/talos-systems/talos/commit/94c28657d3df2e949a9a62ce6844fca817f0688c))
- add Runtime interface ([8153c2e](https://github.com/talos-systems/talos/commit/8153c2e2a9e45bb767703adf94e11e79a09873bb))
- allow specifcation of full url for endpoint ([d0111fe](https://github.com/talos-systems/talos/commit/d0111fe617bb510ec36f5401928e3402c1b34ebf))
- remove proxyd ([80e3876](https://github.com/talos-systems/talos/commit/80e3876df52ee8f599d10c173fb2d61f287bf7ff))
- use the unified pkgs repo artifacts ([fef1517](https://github.com/talos-systems/talos/commit/fef151748b8fd8432b940577177467a783a5f206))
- **osd:** Enable hitting multiple OSD endpoints ([e6bf92c](https://github.com/talos-systems/talos/commit/e6bf92ce31b95cf93c84e40a56de009d8b5c7b8b))

# [v0.3.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.0...v0.3.0-alpha.1) (2019-10-11)

### Bug Fixes

- always write the config to disk ([a799b05](https://github.com/talos-systems/talos/commit/a799b05012311603bee12c2ed1f89ad0455e13f5))
- catch panics in boot go routine ([89789fe](https://github.com/talos-systems/talos/commit/89789fe0a6f7f019332cf68d2a2012ee256619af))
- create etcd data directory ([ef86b3f](https://github.com/talos-systems/talos/commit/ef86b3f367b05fe6c7d9e90e059d325d83a36803))
- generate admin client certificate with 10 year expiration ([34599be](https://github.com/talos-systems/talos/commit/34599be9f2bc02efe8acca0e59a046d639268797))
- ignore case in install platform check ([877c8a0](https://github.com/talos-systems/talos/commit/877c8a0b173b23908f6c94f229d5a62cbd5b6357)), closes [#1249](https://github.com/talos-systems/talos/issues/1249)
- Make updating cert sans an append operation ([64bf429](https://github.com/talos-systems/talos/commit/64bf42960c38c0cd5a2b13b2471ea847537d71d2))
- marshal v1alpha1 config in String() method ([bf59264](https://github.com/talos-systems/talos/commit/bf592642284380186edb939f0c12a4987d6785d1))
- retry endpoint discovery ([1d09ae2](https://github.com/talos-systems/talos/commit/1d09ae2f5ad6ee75405b99bbbdf47d750fef21a8))
- set --cluster-dns kubelet flag properly ([edc21ea](https://github.com/talos-systems/talos/commit/edc21ea9109ebaa57b9a19ca1307c0d4f8fde65e))
- set kubelet-preferred-address-types to prioritize InternalIP ([d9287cd](https://github.com/talos-systems/talos/commit/d9287cdfb5123ee2ab377d4dc97992b17afd9f50))
- set target if specified on command line ([8286754](https://github.com/talos-systems/talos/commit/828675484da740c7432caa0a52af2637ba196584))
- update bootkube fork to fix pod-checkpointer ([9ff31cd](https://github.com/talos-systems/talos/commit/9ff31cd5d983d5da7c198537233459beee5928f6))
- update platform task to set hostname and cert SANs ([e1a50d3](https://github.com/talos-systems/talos/commit/e1a50d36a942339232a1acec1744548c81ac2cdd))
- Use correct names for kubelet config ([d3f20db](https://github.com/talos-systems/talos/commit/d3f20db0aa8d09a059148285eec2bb444269f403))

### Features

- add aescbcEncryptionSecret field to machine config ([4ff8824](https://github.com/talos-systems/talos/commit/4ff882418256da619b19a6eb8e3caff2ce09074b))
- add CNI, and pod and service CIDR to configurator ([04313bd](https://github.com/talos-systems/talos/commit/04313bd48cbb04c9a2a326b93fa1b56c5d191ad8))
- add configurator interface ([4ae8186](https://github.com/talos-systems/talos/commit/4ae818610782a7dc1841f40cf1e77fe9c37ac230))
- Add etcd ca generation to userdata.Generate ([0142696](https://github.com/talos-systems/talos/commit/01426964f684c96cfdb305e5ed5fc49925f26df2))
- add etcd service ([e8dbf10](https://github.com/talos-systems/talos/commit/e8dbf108e27bee28de77a5785b0560c61291bfbb))
- add etcd service to config ([eb8339b](https://github.com/talos-systems/talos/commit/eb8339bb0bb28d4c536e928b8a50888b328b225f))
- add external IP discovery for azure ([ee1b256](https://github.com/talos-systems/talos/commit/ee1b256e0f170583a336a4d3847aa356404f53e9))
- Add kubeadm flex on etcd if service is enabled ([6038c4e](https://github.com/talos-systems/talos/commit/6038c4efe0e27d6ad26a3f533b7a9e28f51f2d34))
- add retry package ([92de307](https://github.com/talos-systems/talos/commit/92de30715e407bedc5771fce28e3314182ea1f76))
- Allow env override of hack/qemu image location ([5686ba2](https://github.com/talos-systems/talos/commit/5686ba2db306ca9a33be0c64da56f296b9cbb70c)), closes [#1220](https://github.com/talos-systems/talos/issues/1220)
- allow Kubernetes version to be configured ([c44f766](https://github.com/talos-systems/talos/commit/c44f7669e552829b3047fb478b73d42c837923f1))
- default docker based cluster to 1 master ([4454afe](https://github.com/talos-systems/talos/commit/4454afef2fdf7265a39e615d8dc559744e47d6b1))
- discover control plane endpoints via Kubernetes ([9e9154b](https://github.com/talos-systems/talos/commit/9e9154b8f5c95898afa112ddc43349dce57f79f0))
- Discover platform external addresses ([3ba04cb](https://github.com/talos-systems/talos/commit/3ba04cb67b85741fff1e3ef99acd9b111880cddf))
- output cluster network info for all node types ([e36133b](https://github.com/talos-systems/talos/commit/e36133b3d3cfa7c4bafc93511816e16a9bef5749))
- use bootkube for cluster creation ([b29391f](https://github.com/talos-systems/talos/commit/b29391f0bed67e7de72bfa27866dc9642435da20))
- use kubeadm to distribute Kubernetes PKI ([607d680](https://github.com/talos-systems/talos/commit/607d68008c9b7ee046a6f8219a9980cee6e19bee))
- write audit policy instead of using trustd ([f244673](https://github.com/talos-systems/talos/commit/f2446738560a0eb46243517dacfab6d8df045df5))

# [v0.3.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.2.0-rc.0...v0.3.0-alpha.0) (2019-09-24)

### Bug Fixes

- **machined:** add nil checks to metal initializer ([1a64ece](https://github.com/talos-systems/talos/commit/1a64ece)), closes [#1186](https://github.com/talos-systems/talos/issues/1186)
- add kerenel config required by Cilium ([d4260f6](https://github.com/talos-systems/talos/commit/d4260f6))
- generate CA certificates with 1 year expiration ([fe4fe08](https://github.com/talos-systems/talos/commit/fe4fe08))
- generate CA certificates with 10 year expiration ([70eab14](https://github.com/talos-systems/talos/commit/70eab14))
- set extra kernel args for all platforms ([8f10647](https://github.com/talos-systems/talos/commit/8f10647))

### Features

- default processes command to one shot ([ead8ce2](https://github.com/talos-systems/talos/commit/ead8ce2))
- return a data structure in version RPC ([9230ff4](https://github.com/talos-systems/talos/commit/9230ff4))
- return a struct for processes RPC ([9ffa064](https://github.com/talos-systems/talos/commit/9ffa064))
- upgrade Kubernetes to v1.16.0 ([82c706a](https://github.com/talos-systems/talos/commit/82c706a))
