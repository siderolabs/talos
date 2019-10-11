# [v0.3.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.0...v0.3.0-alpha.1) (2019-10-11)


### Bug Fixes

* always write the config to disk ([a799b05](https://github.com/talos-systems/talos/commit/a799b05012311603bee12c2ed1f89ad0455e13f5))
* catch panics in boot go routine ([89789fe](https://github.com/talos-systems/talos/commit/89789fe0a6f7f019332cf68d2a2012ee256619af))
* create etcd data directory ([ef86b3f](https://github.com/talos-systems/talos/commit/ef86b3f367b05fe6c7d9e90e059d325d83a36803))
* generate admin client certificate with 10 year expiration ([34599be](https://github.com/talos-systems/talos/commit/34599be9f2bc02efe8acca0e59a046d639268797))
* ignore case in install platform check ([877c8a0](https://github.com/talos-systems/talos/commit/877c8a0b173b23908f6c94f229d5a62cbd5b6357)), closes [#1249](https://github.com/talos-systems/talos/issues/1249)
* Make updating cert sans an append operation ([64bf429](https://github.com/talos-systems/talos/commit/64bf42960c38c0cd5a2b13b2471ea847537d71d2))
* marshal v1alpha1 config in String() method ([bf59264](https://github.com/talos-systems/talos/commit/bf592642284380186edb939f0c12a4987d6785d1))
* retry endpoint discovery ([1d09ae2](https://github.com/talos-systems/talos/commit/1d09ae2f5ad6ee75405b99bbbdf47d750fef21a8))
* set --cluster-dns kubelet flag properly ([edc21ea](https://github.com/talos-systems/talos/commit/edc21ea9109ebaa57b9a19ca1307c0d4f8fde65e))
* set kubelet-preferred-address-types to prioritize InternalIP ([d9287cd](https://github.com/talos-systems/talos/commit/d9287cdfb5123ee2ab377d4dc97992b17afd9f50))
* set target if specified on command line ([8286754](https://github.com/talos-systems/talos/commit/828675484da740c7432caa0a52af2637ba196584))
* update bootkube fork to fix pod-checkpointer ([9ff31cd](https://github.com/talos-systems/talos/commit/9ff31cd5d983d5da7c198537233459beee5928f6))
* update platform task to set hostname and cert SANs ([e1a50d3](https://github.com/talos-systems/talos/commit/e1a50d36a942339232a1acec1744548c81ac2cdd))
* Use correct names for kubelet config ([d3f20db](https://github.com/talos-systems/talos/commit/d3f20db0aa8d09a059148285eec2bb444269f403))


### Features

* add aescbcEncryptionSecret field to machine config ([4ff8824](https://github.com/talos-systems/talos/commit/4ff882418256da619b19a6eb8e3caff2ce09074b))
* add CNI, and pod and service CIDR to configurator ([04313bd](https://github.com/talos-systems/talos/commit/04313bd48cbb04c9a2a326b93fa1b56c5d191ad8))
* add configurator interface ([4ae8186](https://github.com/talos-systems/talos/commit/4ae818610782a7dc1841f40cf1e77fe9c37ac230))
* Add etcd ca generation to userdata.Generate ([0142696](https://github.com/talos-systems/talos/commit/01426964f684c96cfdb305e5ed5fc49925f26df2))
* add etcd service ([e8dbf10](https://github.com/talos-systems/talos/commit/e8dbf108e27bee28de77a5785b0560c61291bfbb))
* add etcd service to config ([eb8339b](https://github.com/talos-systems/talos/commit/eb8339bb0bb28d4c536e928b8a50888b328b225f))
* add external IP discovery for azure ([ee1b256](https://github.com/talos-systems/talos/commit/ee1b256e0f170583a336a4d3847aa356404f53e9))
* Add kubeadm flex on etcd if service is enabled ([6038c4e](https://github.com/talos-systems/talos/commit/6038c4efe0e27d6ad26a3f533b7a9e28f51f2d34))
* add retry package ([92de307](https://github.com/talos-systems/talos/commit/92de30715e407bedc5771fce28e3314182ea1f76))
* Allow env override of hack/qemu image location ([5686ba2](https://github.com/talos-systems/talos/commit/5686ba2db306ca9a33be0c64da56f296b9cbb70c)), closes [#1220](https://github.com/talos-systems/talos/issues/1220)
* allow Kubernetes version to be configured ([c44f766](https://github.com/talos-systems/talos/commit/c44f7669e552829b3047fb478b73d42c837923f1))
* default docker based cluster to 1 master ([4454afe](https://github.com/talos-systems/talos/commit/4454afef2fdf7265a39e615d8dc559744e47d6b1))
* discover control plane endpoints via Kubernetes ([9e9154b](https://github.com/talos-systems/talos/commit/9e9154b8f5c95898afa112ddc43349dce57f79f0))
* Discover platform external addresses ([3ba04cb](https://github.com/talos-systems/talos/commit/3ba04cb67b85741fff1e3ef99acd9b111880cddf))
* output cluster network info for all node types ([e36133b](https://github.com/talos-systems/talos/commit/e36133b3d3cfa7c4bafc93511816e16a9bef5749))
* use bootkube for cluster creation ([b29391f](https://github.com/talos-systems/talos/commit/b29391f0bed67e7de72bfa27866dc9642435da20))
* use kubeadm to distribute Kubernetes PKI ([607d680](https://github.com/talos-systems/talos/commit/607d68008c9b7ee046a6f8219a9980cee6e19bee))
* write audit policy instead of using trustd ([f244673](https://github.com/talos-systems/talos/commit/f2446738560a0eb46243517dacfab6d8df045df5))



# [v0.3.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.2.0-rc.0...v0.3.0-alpha.0) (2019-09-24)


### Bug Fixes

* **machined:** add nil checks to metal initializer ([1a64ece](https://github.com/talos-systems/talos/commit/1a64ece)), closes [#1186](https://github.com/talos-systems/talos/issues/1186)
* add kerenel config required by Cilium ([d4260f6](https://github.com/talos-systems/talos/commit/d4260f6))
* generate CA certificates with 1 year expiration ([fe4fe08](https://github.com/talos-systems/talos/commit/fe4fe08))
* generate CA certificates with 10 year expiration ([70eab14](https://github.com/talos-systems/talos/commit/70eab14))
* set extra kernel args for all platforms ([8f10647](https://github.com/talos-systems/talos/commit/8f10647))


### Features

* default processes command to one shot ([ead8ce2](https://github.com/talos-systems/talos/commit/ead8ce2))
* return a data structure in version RPC ([9230ff4](https://github.com/talos-systems/talos/commit/9230ff4))
* return a struct for processes RPC ([9ffa064](https://github.com/talos-systems/talos/commit/9ffa064))
* upgrade Kubernetes to v1.16.0 ([82c706a](https://github.com/talos-systems/talos/commit/82c706a))
