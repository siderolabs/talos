<a name="0.1.0-alpha.11"></a>
# [0.1.0-alpha.11](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.10...v0.1.0-alpha.11) (2018-10-17)

### Bug Fixes

* **image:** align VERSION env var with pkg/version ([#168](https://github.com/autonomy/dianemo/issues/168)) ([04bb2da](https://github.com/autonomy/dianemo/commit/04bb2da))
* **init:** add /dev and /usr/libexec/kubernetes to docker service ([#160](https://github.com/autonomy/dianemo/issues/160)) ([7268e92](https://github.com/autonomy/dianemo/commit/7268e92))
* **init:** disable megacheck until it gains module support ([#167](https://github.com/autonomy/dianemo/issues/167)) ([9a6542f](https://github.com/autonomy/dianemo/commit/9a6542f))
* **kernel:** remove slub_debug kernel param ([#157](https://github.com/autonomy/dianemo/issues/157)) ([bbc3097](https://github.com/autonomy/dianemo/commit/bbc3097))


### Features

* upgrade Kubernetes to v1.13.0-alpha.1 ([#162](https://github.com/autonomy/dianemo/issues/162)) ([2c80522](https://github.com/autonomy/dianemo/commit/2c80522))
* **ami:** enable ena support ([#164](https://github.com/autonomy/dianemo/issues/164)) ([d542c83](https://github.com/autonomy/dianemo/commit/d542c83))
* **init:** mount partitions dynamically ([#169](https://github.com/autonomy/dianemo/issues/169)) ([453bc48](https://github.com/autonomy/dianemo/commit/453bc48))
* **kernel:** enable NVMe support ([#170](https://github.com/autonomy/dianemo/issues/170)) ([fc38380](https://github.com/autonomy/dianemo/commit/fc38380))



<a name="0.1.0-alpha.10"></a>
# [0.1.0-alpha.10](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.9...v0.1.0-alpha.10) (2018-10-13)


### Features

* upgrade all core components ([#153](https://github.com/autonomy/dianemo/issues/153)) ([92ef602](https://github.com/autonomy/dianemo/commit/92ef602))
* **kernel:** configure Kernel Self Protection Project recommendations ([#152](https://github.com/autonomy/dianemo/issues/152)) ([b34debe](https://github.com/autonomy/dianemo/commit/b34debe))



<a name="0.1.0-alpha.9"></a>
# [0.1.0-alpha.9](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.8...v0.1.0-alpha.9) (2018-09-20)


### Bug Fixes

* **init:** address linter error ([#146](https://github.com/autonomy/dianemo/issues/146)) ([46e895a](https://github.com/autonomy/dianemo/commit/46e895a))


### Features

* run system services via containerd ([#149](https://github.com/autonomy/dianemo/issues/149)) ([8f09202](https://github.com/autonomy/dianemo/commit/8f09202))
* **kernel:** upgrade Linux to v4.18.5 ([#147](https://github.com/autonomy/dianemo/issues/147)) ([80b5e36](https://github.com/autonomy/dianemo/commit/80b5e36))



<a name="0.1.0-alpha.8"></a>
# [0.1.0-alpha.8](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.7...v0.1.0-alpha.8) (2018-08-28)


### Features

* HA control plane ([#144](https://github.com/autonomy/dianemo/issues/144)) ([260d55c](https://github.com/autonomy/dianemo/commit/260d55c))
* list and restart processes ([#141](https://github.com/autonomy/dianemo/issues/141)) ([db0cb37](https://github.com/autonomy/dianemo/commit/db0cb37))
* **kernel:** upgrade Linux to v4.17.15 ([#140](https://github.com/autonomy/dianemo/issues/140)) ([aab4316](https://github.com/autonomy/dianemo/commit/aab4316))
* **osd:** node reset and reboot ([#142](https://github.com/autonomy/dianemo/issues/142)) ([0514ff4](https://github.com/autonomy/dianemo/commit/0514ff4))



<a name="0.1.0-alpha.7"></a>
# [0.1.0-alpha.7](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.6...v0.1.0-alpha.7) (2018-08-11)


### Bug Fixes

* **init:** make /etc/hosts writable ([#125](https://github.com/autonomy/dianemo/issues/125)) ([4014872](https://github.com/autonomy/dianemo/commit/4014872))
* **init:** read kubeadm env file ([#136](https://github.com/autonomy/dianemo/issues/136)) ([d8a3a79](https://github.com/autonomy/dianemo/commit/d8a3a79))
* **initramfs:** align go tests with upstream change ([#133](https://github.com/autonomy/dianemo/issues/133)) ([275ede7](https://github.com/autonomy/dianemo/commit/275ede7))


### Features

* upgrade Kubernetes to v1.11.2 ([#139](https://github.com/autonomy/dianemo/issues/139)) ([37df8a3](https://github.com/autonomy/dianemo/commit/37df8a3))
* **conformance:** add conformance image ([#126](https://github.com/autonomy/dianemo/issues/126)) ([6b661c3](https://github.com/autonomy/dianemo/commit/6b661c3))
* **conformance:** add quick mode config ([#129](https://github.com/autonomy/dianemo/issues/129)) ([6185ac5](https://github.com/autonomy/dianemo/commit/6185ac5))
* **hack:**  add CIS Kubernetes Benchmark script ([#134](https://github.com/autonomy/dianemo/issues/134)) ([deea44b](https://github.com/autonomy/dianemo/commit/deea44b))
* **hack:** use ubuntu 18.04 image in debug pod ([#135](https://github.com/autonomy/dianemo/issues/135)) ([73597c3](https://github.com/autonomy/dianemo/commit/73597c3))
* **image:** make AMI regions a variable ([#137](https://github.com/autonomy/dianemo/issues/137)) ([79bb464](https://github.com/autonomy/dianemo/commit/79bb464))
* **init:** add file creation option ([#132](https://github.com/autonomy/dianemo/issues/132)) ([5058b74](https://github.com/autonomy/dianemo/commit/5058b74))
* **init:** debug option ([#138](https://github.com/autonomy/dianemo/issues/138)) ([6058af2](https://github.com/autonomy/dianemo/commit/6058af2))
* **initramfs:** check for self-hosted-kube-apiserver label ([#130](https://github.com/autonomy/dianemo/issues/130)) ([5d0fa41](https://github.com/autonomy/dianemo/commit/5d0fa41))
* **kernel:** upgrade Linux to v4.17.10 ([#128](https://github.com/autonomy/dianemo/issues/128)) ([cb1a939](https://github.com/autonomy/dianemo/commit/cb1a939))



<a name="0.1.0-alpha.6"></a>
# [0.1.0-alpha.6](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.5...v0.1.0-alpha.6) (2018-07-24)


### Bug Fixes

* **rootfs:** don't remove the docker binary ([#119](https://github.com/autonomy/dianemo/issues/119)) ([eabd76c](https://github.com/autonomy/dianemo/commit/eabd76c))


### Features

* add a debug pod manifest ([#120](https://github.com/autonomy/dianemo/issues/120)) ([dc9e2fe](https://github.com/autonomy/dianemo/commit/dc9e2fe))
* run the kubelet in a container ([#122](https://github.com/autonomy/dianemo/issues/122)) ([90d3078](https://github.com/autonomy/dianemo/commit/90d3078))
* upgrade Kubernetes to v1.11.1 ([#123](https://github.com/autonomy/dianemo/issues/123)) ([b48884b](https://github.com/autonomy/dianemo/commit/b48884b))
* **image:** generate image ([#114](https://github.com/autonomy/dianemo/issues/114)) ([f6adabe](https://github.com/autonomy/dianemo/commit/f6adabe))
* **initramfs:** rewrite user data ([#121](https://github.com/autonomy/dianemo/issues/121)) ([0036bd1](https://github.com/autonomy/dianemo/commit/0036bd1))
* **initramfs:** set the platform explicitly ([#124](https://github.com/autonomy/dianemo/issues/124)) ([ca93ede](https://github.com/autonomy/dianemo/commit/ca93ede))



<a name="0.1.0-alpha.5"></a>
# [0.1.0-alpha.5](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.4...v0.1.0-alpha.5) (2018-07-02)


### Bug Fixes

* create build directory ([#108](https://github.com/autonomy/dianemo/issues/108)) ([9321d7a](https://github.com/autonomy/dianemo/commit/9321d7a))
* field tag should be yaml instead of json ([#100](https://github.com/autonomy/dianemo/issues/100)) ([c0e7996](https://github.com/autonomy/dianemo/commit/c0e7996))


### Features

* **init:** configurable kubelet arguments ([#99](https://github.com/autonomy/dianemo/issues/99)) ([5bd0879](https://github.com/autonomy/dianemo/commit/5bd0879))
* **init:** platform discovery ([#101](https://github.com/autonomy/dianemo/issues/101)) ([b1a7a82](https://github.com/autonomy/dianemo/commit/b1a7a82))
* **initramfs:** Kubernetes API reverse proxy ([#107](https://github.com/autonomy/dianemo/issues/107)) ([ea1edbb](https://github.com/autonomy/dianemo/commit/ea1edbb))
* **kernel:** enable Ceph ([#105](https://github.com/autonomy/dianemo/issues/105)) ([d5b6eca](https://github.com/autonomy/dianemo/commit/d5b6eca))
* **rootfs:** install cut ([#106](https://github.com/autonomy/dianemo/issues/106)) ([9823c35](https://github.com/autonomy/dianemo/commit/9823c35))
* **rootfs:** upgrade Docker to v17.03.2-ce ([#111](https://github.com/autonomy/dianemo/issues/111)) ([fa4f787](https://github.com/autonomy/dianemo/commit/fa4f787))
* **rootfs:** upgrade Kubernetes to v1.11.0-beta.1 ([#104](https://github.com/autonomy/dianemo/issues/104)) ([5519410](https://github.com/autonomy/dianemo/commit/5519410))



<a name="0.1.0-alpha.4"></a>
# [0.1.0-alpha.4](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.3...v0.1.0-alpha.4) (2018-05-20)


### Bug Fixes

* force the kernel to reread partition table ([#88](https://github.com/autonomy/dianemo/issues/88)) ([c843201](https://github.com/autonomy/dianemo/commit/c843201))
* use commit SHA on master and tag name on tags ([#98](https://github.com/autonomy/dianemo/issues/98)) ([2bd7b89](https://github.com/autonomy/dianemo/commit/2bd7b89))
* **init:** conditionally set version in /etc/os-release ([#97](https://github.com/autonomy/dianemo/issues/97)) ([65c2c32](https://github.com/autonomy/dianemo/commit/65c2c32))
* **init:** use /proc/net/pnp as resolv.conf ([#87](https://github.com/autonomy/dianemo/issues/87)) ([2aed515](https://github.com/autonomy/dianemo/commit/2aed515))
* **initramfs:** build variables ([#93](https://github.com/autonomy/dianemo/issues/93)) ([b55ce73](https://github.com/autonomy/dianemo/commit/b55ce73))
* **initramfs:** escape double quotes ([#96](https://github.com/autonomy/dianemo/issues/96)) ([63a0728](https://github.com/autonomy/dianemo/commit/63a0728))
* **initramfs:** invalid reference to template variable ([#94](https://github.com/autonomy/dianemo/issues/94)) ([3dc22fa](https://github.com/autonomy/dianemo/commit/3dc22fa))
* **initramfs:** quote -X flag ([#95](https://github.com/autonomy/dianemo/issues/95)) ([068017a](https://github.com/autonomy/dianemo/commit/068017a))


### Features

* add version command ([#85](https://github.com/autonomy/dianemo/issues/85)) ([a55daaf](https://github.com/autonomy/dianemo/commit/a55daaf))
* dynamic resolv.conf ([#86](https://github.com/autonomy/dianemo/issues/86)) ([325ae5c](https://github.com/autonomy/dianemo/commit/325ae5c))
* osctl configuration file ([#90](https://github.com/autonomy/dianemo/issues/90)) ([a16008e](https://github.com/autonomy/dianemo/commit/a16008e))
* upgrade kubernetes to v1.11.0-beta.0 ([#92](https://github.com/autonomy/dianemo/issues/92)) ([8701fcb](https://github.com/autonomy/dianemo/commit/8701fcb))
* **init:** verify EC2 PKCS7 signature ([#84](https://github.com/autonomy/dianemo/issues/84)) ([7bf0abd](https://github.com/autonomy/dianemo/commit/7bf0abd))



<a name="0.1.0-alpha.3"></a>
# [0.1.0-alpha.3](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.2...v0.1.0-alpha.3) (2018-05-15)


### Bug Fixes

* **generate:** use xvda instead of sda ([#77](https://github.com/autonomy/dianemo/issues/77)) ([e18cf83](https://github.com/autonomy/dianemo/commit/e18cf83))
* **init:** bad variable name and missing package ([#78](https://github.com/autonomy/dianemo/issues/78)) ([7c37272](https://github.com/autonomy/dianemo/commit/7c37272))


### Features

* automate signed certificates ([#81](https://github.com/autonomy/dianemo/issues/81)) ([d517737](https://github.com/autonomy/dianemo/commit/d517737))
* raw kubeadm configuration in user data ([#79](https://github.com/autonomy/dianemo/issues/79)) ([fc98614](https://github.com/autonomy/dianemo/commit/fc98614))
* **init:** don't print kubeadm token ([#74](https://github.com/autonomy/dianemo/issues/74)) ([2f48972](https://github.com/autonomy/dianemo/commit/2f48972))
* **kernel:** compile with Linux guest support ([#75](https://github.com/autonomy/dianemo/issues/75)) ([67e092a](https://github.com/autonomy/dianemo/commit/67e092a))



<a name="0.1.0-alpha.2"></a>
# [0.1.0-alpha.2](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.1...v0.1.0-alpha.2) (2018-05-09)


### Features

* upgrade Kubernetes to v1.10.2 ([#61](https://github.com/autonomy/dianemo/issues/61)) ([dcf3a71](https://github.com/autonomy/dianemo/commit/dcf3a71))
* **generate:** set RAW disk sizes dynamically ([#71](https://github.com/autonomy/dianemo/issues/71)) ([5701ea6](https://github.com/autonomy/dianemo/commit/5701ea6))
* **init:** gRPC with mutual TLS authentication ([#64](https://github.com/autonomy/dianemo/issues/64)) ([f6686bc](https://github.com/autonomy/dianemo/commit/f6686bc))
* **rootfs:** upgrade CRI-O to v1.10.1 ([#70](https://github.com/autonomy/dianemo/issues/70)) ([ff61573](https://github.com/autonomy/dianemo/commit/ff61573))



<a name="0.1.0-alpha.1"></a>
# [0.1.0-alpha.1](https://github.com/autonomy/dianemo/compare/v0.1.0-alpha.0...v0.1.0-alpha.1) (2018-04-20)


### Bug Fixes

* generate /etc/hosts and /etc/resolv.conf ([#54](https://github.com/autonomy/dianemo/issues/54)) ([5bd43ab](https://github.com/autonomy/dianemo/commit/5bd43ab))
* **init:** enable hierarchical accounting and reclaim ([#59](https://github.com/autonomy/dianemo/issues/59)) ([68d95c2](https://github.com/autonomy/dianemo/commit/68d95c2))
* **init:** missing parameter ([#55](https://github.com/autonomy/dianemo/issues/55)) ([1a89469](https://github.com/autonomy/dianemo/commit/1a89469))
* **init:** printf formatting ([#51](https://github.com/autonomy/dianemo/issues/51)) ([b0782b6](https://github.com/autonomy/dianemo/commit/b0782b6))
* **init:** remove unused code ([#56](https://github.com/autonomy/dianemo/issues/56)) ([0c62bda](https://github.com/autonomy/dianemo/commit/0c62bda))
* **init:** switch_root implementation ([#49](https://github.com/autonomy/dianemo/issues/49)) ([b614179](https://github.com/autonomy/dianemo/commit/b614179))


### Features

* docker as an optional container runtime ([#57](https://github.com/autonomy/dianemo/issues/57)) ([3a60bdc](https://github.com/autonomy/dianemo/commit/3a60bdc))
* upgrade to Kubernetes v1.10.1 ([#50](https://github.com/autonomy/dianemo/issues/50)) ([46616d1](https://github.com/autonomy/dianemo/commit/46616d1))
* **generate:** enable kernel logging ([#58](https://github.com/autonomy/dianemo/issues/58)) ([71d97c8](https://github.com/autonomy/dianemo/commit/71d97c8))
* **kernel:** use LTS kernel v4.14.34 ([#48](https://github.com/autonomy/dianemo/issues/48)) ([4c9a810](https://github.com/autonomy/dianemo/commit/4c9a810))



<a name="0.1.0-alpha.0"></a>
# [0.1.0-alpha.0](https://github.com/autonomy/dianemo/compare/aba4615...v0.1.0-alpha.0) (2018-04-03)


### Bug Fixes

* **init:** address crio errors and warns ([#40](https://github.com/autonomy/dianemo/issues/40)) ([7536d72](https://github.com/autonomy/dianemo/commit/7536d72))
* **init:** don't create CRI-O CNI configurations ([#36](https://github.com/autonomy/dianemo/issues/36)) ([8a7c424](https://github.com/autonomy/dianemo/commit/8a7c424))
* **init:** make log handling non-blocking ([#37](https://github.com/autonomy/dianemo/issues/37)) ([f244075](https://github.com/autonomy/dianemo/commit/f244075))
* **init:** typo in service subnet field; pin version of Kubernetes ([#10](https://github.com/autonomy/dianemo/issues/10)) ([8427ddf](https://github.com/autonomy/dianemo/commit/8427ddf))
* **rootfs:** install conntrack ([#27](https://github.com/autonomy/dianemo/issues/27)) ([1067958](https://github.com/autonomy/dianemo/commit/1067958))


### Features

* enable IPVS ([#42](https://github.com/autonomy/dianemo/issues/42)) ([168c598](https://github.com/autonomy/dianemo/commit/168c598))
* initial implementation ([#2](https://github.com/autonomy/dianemo/issues/2)) ([aba4615](https://github.com/autonomy/dianemo/commit/aba4615))
* mount ROOT partition as RO ([#11](https://github.com/autonomy/dianemo/issues/11)) ([29bdd6d](https://github.com/autonomy/dianemo/commit/29bdd6d))
* update Kubernetes to v1.10.0 ([#26](https://github.com/autonomy/dianemo/issues/26)) ([9a11837](https://github.com/autonomy/dianemo/commit/9a11837))
* update Kubernetes to v1.10.0-rc.1 ([#25](https://github.com/autonomy/dianemo/issues/25)) ([901461c](https://github.com/autonomy/dianemo/commit/901461c))
* update to linux 4.15.13 ([#30](https://github.com/autonomy/dianemo/issues/30)) ([e418d29](https://github.com/autonomy/dianemo/commit/e418d29))
* use CRI-O as the container runtime ([#12](https://github.com/autonomy/dianemo/issues/12)) ([7785d6f](https://github.com/autonomy/dianemo/commit/7785d6f))
* **init:** add node join functionality ([#38](https://github.com/autonomy/dianemo/issues/38)) ([0251868](https://github.com/autonomy/dianemo/commit/0251868))
* **init:** basic process managment ([#6](https://github.com/autonomy/dianemo/issues/6)) ([6c1038b](https://github.com/autonomy/dianemo/commit/6c1038b))
* **init:** provide and endpoint for getting logs of running processes ([#9](https://github.com/autonomy/dianemo/issues/9)) ([37d80cf](https://github.com/autonomy/dianemo/commit/37d80cf))
* **init:** set kubelet log level to 4 ([#13](https://github.com/autonomy/dianemo/issues/13)) ([9597b21](https://github.com/autonomy/dianemo/commit/9597b21))
* **init:** use CoreDNS by default ([#39](https://github.com/autonomy/dianemo/issues/39)) ([a8e3d50](https://github.com/autonomy/dianemo/commit/a8e3d50))
* **init:** user data ([#17](https://github.com/autonomy/dianemo/issues/17)) ([3ee01ae](https://github.com/autonomy/dianemo/commit/3ee01ae))
* **kernel:** enable nf_tables and ebtables modules ([#41](https://github.com/autonomy/dianemo/issues/41)) ([cf53a27](https://github.com/autonomy/dianemo/commit/cf53a27))
* **rootfs:** upgrade cri-o and cri-tools ([#35](https://github.com/autonomy/dianemo/issues/35)) ([0095227](https://github.com/autonomy/dianemo/commit/0095227))



