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
