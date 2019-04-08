# [v0.1.0-alpha.21](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.20...v0.1.0-alpha.21) (2019-04-08)


### Bug Fixes

* **osctl:** add missing flags ([#479](https://github.com/talos-systems/talos/issues/479)) ([380ba21](https://github.com/talos-systems/talos/commit/380ba21))
* check link state before bringing it up ([#497](https://github.com/talos-systems/talos/issues/497)) ([7fac0df](https://github.com/talos-systems/talos/commit/7fac0df))
* create GCE disk as disk.raw ([#498](https://github.com/talos-systems/talos/issues/498)) ([67d7abe](https://github.com/talos-systems/talos/commit/67d7abe))
* remove static resolv.conf ([#491](https://github.com/talos-systems/talos/issues/491)) ([0926e72](https://github.com/talos-systems/talos/commit/0926e72))


### Features

* add network configuration support ([#476](https://github.com/talos-systems/talos/issues/476)) ([7d4db80](https://github.com/talos-systems/talos/commit/7d4db80))



# [v0.1.0-alpha.20](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.19...v0.1.0-alpha.20) (2019-04-02)


### Bug Fixes

* revert runc to v1.0.0-rc.6 ([#470](https://github.com/talos-systems/talos/issues/470)) ([9bc2f8f](https://github.com/talos-systems/talos/commit/9bc2f8f))


### Features

* add power off functionality ([#462](https://github.com/talos-systems/talos/issues/462)) ([2e9a7ec](https://github.com/talos-systems/talos/commit/2e9a7ec))
* **initramfs:** add support for refreshing dhcp lease ([#454](https://github.com/talos-systems/talos/issues/454)) ([75d1d89](https://github.com/talos-systems/talos/commit/75d1d89))
* add basic ntp implementation ([#459](https://github.com/talos-systems/talos/issues/459)) ([3693cff](https://github.com/talos-systems/talos/commit/3693cff))
* add packet support ([#473](https://github.com/talos-systems/talos/issues/473)) ([19f712e](https://github.com/talos-systems/talos/commit/19f712e))
* dd bootloader components ([#438](https://github.com/talos-systems/talos/issues/438)) ([226697e](https://github.com/talos-systems/talos/commit/226697e))
* install bootloader to block device ([#455](https://github.com/talos-systems/talos/issues/455)) ([31a00ef](https://github.com/talos-systems/talos/commit/31a00ef))
* remove DenyEscalatingExec admission plugin ([#457](https://github.com/talos-systems/talos/issues/457)) ([6ae6118](https://github.com/talos-systems/talos/commit/6ae6118))
* upgrade containerd to v1.2.5 ([#463](https://github.com/talos-systems/talos/issues/463)) ([30774fc](https://github.com/talos-systems/talos/commit/30774fc))
* upgrade Kubernetes to v1.14.0 ([#466](https://github.com/talos-systems/talos/issues/466)) ([50253b8](https://github.com/talos-systems/talos/commit/50253b8))
* upgrade Linux to v4.19.31 ([#464](https://github.com/talos-systems/talos/issues/464)) ([da21b90](https://github.com/talos-systems/talos/commit/da21b90))
* upgrade runc to v1.0.0-rc.7 ([#469](https://github.com/talos-systems/talos/issues/469)) ([8dba7db](https://github.com/talos-systems/talos/commit/8dba7db))



# [v0.1.0-alpha.19](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.18...v0.1.0-alpha.19) (2019-03-11)


### Bug Fixes

* add initialization for userdata download ([#367](https://github.com/talos-systems/talos/issues/367)) ([12eeab2](https://github.com/talos-systems/talos/commit/12eeab2)), closes [#363](https://github.com/talos-systems/talos/issues/363)
* add iptables to rootfs ([#378](https://github.com/talos-systems/talos/issues/378)) ([eed7388](https://github.com/talos-systems/talos/commit/eed7388))
* add missing mounts and remove memory limits ([#442](https://github.com/talos-systems/talos/issues/442)) ([a2cee67](https://github.com/talos-systems/talos/commit/a2cee67))
* assign to existing target variable ([#436](https://github.com/talos-systems/talos/issues/436)) ([9f1e54c](https://github.com/talos-systems/talos/commit/9f1e54c))
* delay `gitmeta` until needed in Makefile ([#407](https://github.com/talos-systems/talos/issues/407)) ([0ed9bc8](https://github.com/talos-systems/talos/commit/0ed9bc8))
* distribute PKI from initial master to joining masters ([#426](https://github.com/talos-systems/talos/issues/426)) ([7528d89](https://github.com/talos-systems/talos/commit/7528d89))
* **initramfs:** fix case where we download a non archive file ([#421](https://github.com/talos-systems/talos/issues/421)) ([83d979d](https://github.com/talos-systems/talos/commit/83d979d))
* ensure DNS works in early boot ([#382](https://github.com/talos-systems/talos/issues/382)) ([078a664](https://github.com/talos-systems/talos/commit/078a664))
* fallback on IP address when DHCP reply has no hostname ([#432](https://github.com/talos-systems/talos/issues/432)) ([08ee6c4](https://github.com/talos-systems/talos/commit/08ee6c4))
* join masters in serial ([#437](https://github.com/talos-systems/talos/issues/437)) ([b6e6c46](https://github.com/talos-systems/talos/commit/b6e6c46))
* mount /dev/shm as tmpfs ([#445](https://github.com/talos-systems/talos/issues/445)) ([1ee326b](https://github.com/talos-systems/talos/commit/1ee326b))
* output userdata fails, ignore numcpu for kubeadm ([#398](https://github.com/talos-systems/talos/issues/398)) ([8e30f95](https://github.com/talos-systems/talos/commit/8e30f95))
* write config changes to specified config file ([#416](https://github.com/talos-systems/talos/issues/416)) ([6d8e94d](https://github.com/talos-systems/talos/commit/6d8e94d))


### Features

* add `docker-os` make target, Kubeadm.ExtraArgs, and a dev Makefile ([#446](https://github.com/talos-systems/talos/issues/446)) ([98e3920](https://github.com/talos-systems/talos/commit/98e3920))
* add arg to target nodes per command ([#435](https://github.com/talos-systems/talos/issues/435)) ([0cf8dda](https://github.com/talos-systems/talos/commit/0cf8dda))
* add automated PKI for joining nodes ([#406](https://github.com/talos-systems/talos/issues/406)) ([9e947c3](https://github.com/talos-systems/talos/commit/9e947c3))
* add config flag to osctl ([#413](https://github.com/talos-systems/talos/issues/413)) ([4d5350e](https://github.com/talos-systems/talos/commit/4d5350e))
* add container based deploy support to init ([#447](https://github.com/talos-systems/talos/issues/447)) ([b5f398d](https://github.com/talos-systems/talos/commit/b5f398d))
* add DHCP client ([#427](https://github.com/talos-systems/talos/issues/427)) ([ee232b8](https://github.com/talos-systems/talos/commit/ee232b8))
* add dosfstools to initramfs and rootfs ([#444](https://github.com/talos-systems/talos/issues/444)) ([d706803](https://github.com/talos-systems/talos/commit/d706803))
* add gcloud integration ([#385](https://github.com/talos-systems/talos/issues/385)) ([85e35d3](https://github.com/talos-systems/talos/commit/85e35d3))
* add hostname to node certificate SAN ([#415](https://github.com/talos-systems/talos/issues/415)) ([52d2660](https://github.com/talos-systems/talos/commit/52d2660))
* add osinstall cli utility ([#368](https://github.com/talos-systems/talos/issues/368)) ([8ee9022](https://github.com/talos-systems/talos/commit/8ee9022))
* add route printing to osctl ([#404](https://github.com/talos-systems/talos/issues/404)) ([a2704ee](https://github.com/talos-systems/talos/commit/a2704ee))
* add TALOSCONFIG env var ([#422](https://github.com/talos-systems/talos/issues/422)) ([c63ef44](https://github.com/talos-systems/talos/commit/c63ef44))
* allow user specified IP addresses in SANs ([#425](https://github.com/talos-systems/talos/issues/425)) ([b59f632](https://github.com/talos-systems/talos/commit/b59f632))
* change AWS instance type to t2.micro ([#399](https://github.com/talos-systems/talos/issues/399)) ([a55b84a](https://github.com/talos-systems/talos/commit/a55b84a))
* create certificates with all non-loopback addresses ([#424](https://github.com/talos-systems/talos/issues/424)) ([dce3e2c](https://github.com/talos-systems/talos/commit/dce3e2c))
* log to stdout when in container mode ([#450](https://github.com/talos-systems/talos/issues/450)) ([1f08961](https://github.com/talos-systems/talos/commit/1f08961))
* update gcc to 8.3.0, drop gcompat ([#433](https://github.com/talos-systems/talos/issues/433)) ([9de34cd](https://github.com/talos-systems/talos/commit/9de34cd))
* upgrade containerd to v1.2.4 ([#395](https://github.com/talos-systems/talos/issues/395)) ([b963f5a](https://github.com/talos-systems/talos/commit/b963f5a))
* upgrade linux to v4.19.23 ([#402](https://github.com/talos-systems/talos/issues/402)) ([c50b2e6](https://github.com/talos-systems/talos/commit/c50b2e6))
* upgrade musl to 1.1.21 ([#401](https://github.com/talos-systems/talos/issues/401)) ([d8594f4](https://github.com/talos-systems/talos/commit/d8594f4))
* **hack:** add osctl/kubelet dev tooling and document usage ([#449](https://github.com/talos-systems/talos/issues/449)) ([4f530e8](https://github.com/talos-systems/talos/commit/4f530e8))



# [0.1.0-alpha.18](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.17...v0.1.0-alpha.18) (2019-02-16)


### Bug Fixes

* add libblkid to the rootfs ([#345](https://github.com/talos-systems/talos/issues/345)) ([76bc58b](https://github.com/talos-systems/talos/commit/76bc58b))
* Minor adjustments to makefile ([#340](https://github.com/talos-systems/talos/issues/340)) ([eced2f2](https://github.com/talos-systems/talos/commit/eced2f2)), closes [#338](https://github.com/talos-systems/talos/issues/338)



# [0.1.0-alpha.17](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.16...v0.1.0-alpha.17) (2019-02-15)


### Features

* disable session tickets ([#334](https://github.com/talos-systems/talos/issues/334)) ([b226f5f](https://github.com/talos-systems/talos/commit/b226f5f))
* upgrade Kubernetes to v1.13.3 ([#335](https://github.com/talos-systems/talos/issues/335)) ([1219ae7](https://github.com/talos-systems/talos/commit/1219ae7))



# [0.1.0-alpha.16](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.15...v0.1.0-alpha.16) (2019-01-25)


### Bug Fixes

* raw image output ([#307](https://github.com/talos-systems/talos/issues/307)) ([8836577](https://github.com/talos-systems/talos/commit/8836577))
* use version tag for container tags ([#308](https://github.com/talos-systems/talos/issues/308)) ([07570a3](https://github.com/talos-systems/talos/commit/07570a3))
* **init:** update probe for NVMe ([#323](https://github.com/talos-systems/talos/issues/323)) ([d8bf727](https://github.com/talos-systems/talos/commit/d8bf727))
* **osctl:** compile static binary with CGO enabeld ([#328](https://github.com/talos-systems/talos/issues/328)) ([fa14741](https://github.com/talos-systems/talos/commit/fa14741))


### Features

* import core service containers from local store ([#309](https://github.com/talos-systems/talos/issues/309)) ([25fca3d](https://github.com/talos-systems/talos/commit/25fca3d))
* **osctl:** add stats command ([#314](https://github.com/talos-systems/talos/issues/314)) ([62bb226](https://github.com/talos-systems/talos/commit/62bb226))
* **osctl:** output namespace ([#312](https://github.com/talos-systems/talos/issues/312)) ([3c5f99f](https://github.com/talos-systems/talos/commit/3c5f99f))
* upgrade containerd to v1.2.2 ([#318](https://github.com/talos-systems/talos/issues/318)) ([a2b2e7e](https://github.com/talos-systems/talos/commit/a2b2e7e))
* upgrade Kubernetes to v1.13.2 ([#319](https://github.com/talos-systems/talos/issues/319)) ([5cadd83](https://github.com/talos-systems/talos/commit/5cadd83))
* use musl libc ([#316](https://github.com/talos-systems/talos/issues/316)) ([26c4418](https://github.com/talos-systems/talos/commit/26c4418))



# [0.1.0-alpha.15](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.14...v0.1.0-alpha.15) (2019-01-02)


### Bug Fixes

* **init:** don't create the EncryptionConfig if it exists ([#282](https://github.com/talos-systems/talos/issues/282)) ([0c32c95](https://github.com/talos-systems/talos/commit/0c32c95))
* **init:** no memory limit for container runtime ([#289](https://github.com/talos-systems/talos/issues/289)) ([fdac043](https://github.com/talos-systems/talos/commit/fdac043))
* symlink kubernetes libexec directory ([#294](https://github.com/talos-systems/talos/issues/294)) ([3de4323](https://github.com/talos-systems/talos/commit/3de4323))


### Features

* **image:** build AMI with random.trust_cpu=on ([#287](https://github.com/talos-systems/talos/issues/287)) ([648ce5b](https://github.com/talos-systems/talos/commit/648ce5b))
* **init:** reboot node on panic ([#284](https://github.com/talos-systems/talos/issues/284)) ([5140fbe](https://github.com/talos-systems/talos/commit/5140fbe))
* **initramfs:** retry userdata download ([#283](https://github.com/talos-systems/talos/issues/283)) ([028bdec](https://github.com/talos-systems/talos/commit/028bdec))
* **kernel:** upgrade Linux to v4.19.10 ([#293](https://github.com/talos-systems/talos/issues/293)) ([a8292cb](https://github.com/talos-systems/talos/commit/a8292cb))
* add filesystem probing library ([#298](https://github.com/talos-systems/talos/issues/298)) ([42b722b](https://github.com/talos-systems/talos/commit/42b722b))
* upgrade Kubernetes to v1.13.1 ([#291](https://github.com/talos-systems/talos/issues/291)) ([f5f948e](https://github.com/talos-systems/talos/commit/f5f948e))
* use Containerd as CRI ([#292](https://github.com/talos-systems/talos/issues/292)) ([23f7adb](https://github.com/talos-systems/talos/commit/23f7adb))



# [0.1.0-alpha.14](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.13...v0.1.0-alpha.14) (2018-12-05)


### Bug Fixes

* **gpt:** do not inform kernel of partition when writing ([#237](https://github.com/talos-systems/talos/issues/237)) ([fa9f77e](https://github.com/talos-systems/talos/commit/fa9f77e))
* **hack:** remove privileged options from debug manifest ([#224](https://github.com/talos-systems/talos/issues/224)) ([9c77b49](https://github.com/talos-systems/talos/commit/9c77b49))
* **image:** install gzip ([#272](https://github.com/talos-systems/talos/issues/272)) ([d4db548](https://github.com/talos-systems/talos/commit/d4db548))
* **init:** address linter errors ([#251](https://github.com/talos-systems/talos/issues/251)) ([ff83876](https://github.com/talos-systems/talos/commit/ff83876))
* **init:** allow custom image for kubeadm ([#212](https://github.com/talos-systems/talos/issues/212)) ([0bbd8a4](https://github.com/talos-systems/talos/commit/0bbd8a4))
* **init:** avoid kernel panic on recover ([#216](https://github.com/talos-systems/talos/issues/216)) ([74aafac](https://github.com/talos-systems/talos/commit/74aafac))
* **init:** ensure VMware user data is not empty ([#217](https://github.com/talos-systems/talos/issues/217)) ([f00e05a](https://github.com/talos-systems/talos/commit/f00e05a))
* **init:** log to kmsg after /dev is mounted ([#218](https://github.com/talos-systems/talos/issues/218)) ([fde2639](https://github.com/talos-systems/talos/commit/fde2639))
* **init:** retry mounts ([#220](https://github.com/talos-systems/talos/issues/220)) ([51118bd](https://github.com/talos-systems/talos/commit/51118bd))
* **init:** revert e94095b and fix bad attribute lookups ([#274](https://github.com/talos-systems/talos/issues/274)) ([b3f12a2](https://github.com/talos-systems/talos/commit/b3f12a2))
* **init:** unmount / last ([#249](https://github.com/talos-systems/talos/issues/249)) ([ee95933](https://github.com/talos-systems/talos/commit/ee95933))
* **init:** use PARTLABEL to identity Talos block devices ([#238](https://github.com/talos-systems/talos/issues/238)) ([a3dd113](https://github.com/talos-systems/talos/commit/a3dd113))
* **init:** use smaller default install sizes ([#240](https://github.com/talos-systems/talos/issues/240)) ([b50afcb](https://github.com/talos-systems/talos/commit/b50afcb))
* disable AlwaysPullImages admission plugin ([#273](https://github.com/talos-systems/talos/issues/273)) ([1bb002c](https://github.com/talos-systems/talos/commit/1bb002c))
* **init:** use text/template ([#228](https://github.com/talos-systems/talos/issues/228)) ([08dd81a](https://github.com/talos-systems/talos/commit/08dd81a))
* **init:** use the correct blkid lookup values ([#243](https://github.com/talos-systems/talos/issues/243)) ([e74f4c1](https://github.com/talos-systems/talos/commit/e74f4c1))
* **initramfs:** fix bare metal install ([#245](https://github.com/talos-systems/talos/issues/245)) ([c171c51](https://github.com/talos-systems/talos/commit/c171c51))
* **initramfs:** fix hardcoded version ([#275](https://github.com/talos-systems/talos/issues/275)) ([72eaa72](https://github.com/talos-systems/talos/commit/72eaa72))
* **initramfs:** fix printf statement ([#250](https://github.com/talos-systems/talos/issues/250)) ([678951b](https://github.com/talos-systems/talos/commit/678951b))
* **initramfs:** imports ([#276](https://github.com/talos-systems/talos/issues/276)) ([55fc13e](https://github.com/talos-systems/talos/commit/55fc13e))
* **initramfs:** minor fixes for booting from bare metal ([#241](https://github.com/talos-systems/talos/issues/241)) ([7564144](https://github.com/talos-systems/talos/commit/7564144))
* **kernel:** add missing kernel config options ([#236](https://github.com/talos-systems/talos/issues/236)) ([c48a2ef](https://github.com/talos-systems/talos/commit/c48a2ef))


### Features

* **init:** add calico support ([#223](https://github.com/talos-systems/talos/issues/223)) ([f16a130](https://github.com/talos-systems/talos/commit/f16a130))
* **init:** add label and force options for xfs ([#244](https://github.com/talos-systems/talos/issues/244)) ([e320fd1](https://github.com/talos-systems/talos/commit/e320fd1))
* **init:** add support for installing to a device ([#225](https://github.com/talos-systems/talos/issues/225)) ([79c96cf](https://github.com/talos-systems/talos/commit/79c96cf))
* **init:** add VMware support ([#200](https://github.com/talos-systems/talos/issues/200)) ([48b2ea3](https://github.com/talos-systems/talos/commit/48b2ea3))
* **init:** create CNI mounts ([#226](https://github.com/talos-systems/talos/issues/226)) ([aa08f15](https://github.com/talos-systems/talos/commit/aa08f15))
* **init:** enable PSP admission plugin ([#230](https://github.com/talos-systems/talos/issues/230)) ([d0a0d1f](https://github.com/talos-systems/talos/commit/d0a0d1f))
* **init:** log to /dev/kmsg ([#214](https://github.com/talos-systems/talos/issues/214)) ([b30ed5d](https://github.com/talos-systems/talos/commit/b30ed5d))
* **init:** service env var option ([#219](https://github.com/talos-systems/talos/issues/219)) ([0c80b7e](https://github.com/talos-systems/talos/commit/0c80b7e))
* **initramfs:** API for creating new partition tables ([#227](https://github.com/talos-systems/talos/issues/227)) ([374343a](https://github.com/talos-systems/talos/commit/374343a))
* **kernel:** add igb and ixgb drivers ([#221](https://github.com/talos-systems/talos/issues/221)) ([4696527](https://github.com/talos-systems/talos/commit/4696527))
* **kernel:** add low level SCSI support ([#215](https://github.com/talos-systems/talos/issues/215)) ([325de5b](https://github.com/talos-systems/talos/commit/325de5b))
* **kernel:** add raw iptables support ([#222](https://github.com/talos-systems/talos/issues/222)) ([86ef4fc](https://github.com/talos-systems/talos/commit/86ef4fc))
* **kernel:** add vmxnet3 support ([#213](https://github.com/talos-systems/talos/issues/213)) ([0244d18](https://github.com/talos-systems/talos/commit/0244d18))
* atomic partition table operations ([#234](https://github.com/talos-systems/talos/issues/234)) ([a2d079e](https://github.com/talos-systems/talos/commit/a2d079e))
* udevd service ([#231](https://github.com/talos-systems/talos/issues/231)) ([0c65fc6](https://github.com/talos-systems/talos/commit/0c65fc6))



# [0.1.0-alpha.13](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.12...v0.1.0-alpha.13) (2018-11-15)


### Bug Fixes

* **hack:** add /etc/kubernetes to CIS benchmark jobs ([#199](https://github.com/talos-systems/talos/issues/199)) ([fc84b62](https://github.com/talos-systems/talos/commit/fc84b62))
* **image:** VMDK generation ([#204](https://github.com/talos-systems/talos/issues/204)) ([9d4f791](https://github.com/talos-systems/talos/commit/9d4f791))
* **init:** node join ([#195](https://github.com/talos-systems/talos/issues/195)) ([157ef67](https://github.com/talos-systems/talos/commit/157ef67))
* **init:** use kubeadm experimental-control-plane ([#194](https://github.com/talos-systems/talos/issues/194)) ([2fd7112](https://github.com/talos-systems/talos/commit/2fd7112))
* **osctl:** build Linux binary with CGO ([#196](https://github.com/talos-systems/talos/issues/196)) ([ab82aa7](https://github.com/talos-systems/talos/commit/ab82aa7))
* **osctl:** nil pointer when injecting kubernetes PKI ([#187](https://github.com/talos-systems/talos/issues/187)) ([160702b](https://github.com/talos-systems/talos/commit/160702b))


### Features

* upgrade Containerd to v1.2.0 ([#190](https://github.com/talos-systems/talos/issues/190)) ([47787f7](https://github.com/talos-systems/talos/commit/47787f7))
* upgrade Kubernetes to v1.13.0-alpha.3 ([#189](https://github.com/talos-systems/talos/issues/189)) ([91825fa](https://github.com/talos-systems/talos/commit/91825fa))
* embed the kubeadm config ([#205](https://github.com/talos-systems/talos/issues/205)) ([160ce41](https://github.com/talos-systems/talos/commit/160ce41))
* **init:** add NoCloud user-data support ([#209](https://github.com/talos-systems/talos/issues/209)) ([b584904](https://github.com/talos-systems/talos/commit/b584904))
* **init:** enforce CIS requirements ([#198](https://github.com/talos-systems/talos/issues/198)) ([0c41de9](https://github.com/talos-systems/talos/commit/0c41de9))
* **init:** enforce use of hyperkube and Kubernetes version ([#207](https://github.com/talos-systems/talos/issues/207)) ([0081a89](https://github.com/talos-systems/talos/commit/0081a89))
* **kernel:** add virtio support ([#208](https://github.com/talos-systems/talos/issues/208)) ([ff97c8c](https://github.com/talos-systems/talos/commit/ff97c8c))
* **kernel:** upgrade Linux to v4.19.1 ([#192](https://github.com/talos-systems/talos/issues/192)) ([36b899b](https://github.com/talos-systems/talos/commit/36b899b))
* **rootfs:** upgrade crictl to v1.12.0 ([#191](https://github.com/talos-systems/talos/issues/191)) ([f7ad93c](https://github.com/talos-systems/talos/commit/f7ad93c))



# [0.1.0-alpha.12](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.11...v0.1.0-alpha.12) (2018-11-02)


### Features

* upgrade Kubernetes to v1.13.0-alpha.2 ([#173](https://github.com/talos-systems/talos/issues/173)) ([60adafb](https://github.com/talos-systems/talos/commit/60adafb))
* add blockd service ([#172](https://github.com/talos-systems/talos/issues/172)) ([aa65101](https://github.com/talos-systems/talos/commit/aa65101))



# [0.1.0-alpha.11](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.10...v0.1.0-alpha.11) (2018-10-18)


### Bug Fixes

* **image:** align VERSION env var with pkg/version ([#168](https://github.com/talos-systems/talos/issues/168)) ([04bb2da](https://github.com/talos-systems/talos/commit/04bb2da))
* **init:** add /dev and /usr/libexec/kubernetes to docker service ([#160](https://github.com/talos-systems/talos/issues/160)) ([7268e92](https://github.com/talos-systems/talos/commit/7268e92))
* **init:** disable megacheck until it gains module support ([#167](https://github.com/talos-systems/talos/issues/167)) ([9a6542f](https://github.com/talos-systems/talos/commit/9a6542f))
* **kernel:** remove slub_debug kernel param ([#157](https://github.com/talos-systems/talos/issues/157)) ([bbc3097](https://github.com/talos-systems/talos/commit/bbc3097))


### Features

* upgrade Kubernetes to v1.13.0-alpha.1 ([#162](https://github.com/talos-systems/talos/issues/162)) ([2c80522](https://github.com/talos-systems/talos/commit/2c80522))
* **ami:** enable ena support ([#164](https://github.com/talos-systems/talos/issues/164)) ([d542c83](https://github.com/talos-systems/talos/commit/d542c83))
* **init:** mount partitions dynamically ([#169](https://github.com/talos-systems/talos/issues/169)) ([453bc48](https://github.com/talos-systems/talos/commit/453bc48))
* **kernel:** enable NVMe support ([#170](https://github.com/talos-systems/talos/issues/170)) ([fc38380](https://github.com/talos-systems/talos/commit/fc38380))



# [0.1.0-alpha.10](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.9...v0.1.0-alpha.10) (2018-10-13)


### Features

* upgrade all core components ([#153](https://github.com/talos-systems/talos/issues/153)) ([92ef602](https://github.com/talos-systems/talos/commit/92ef602))
* **kernel:** configure Kernel Self Protection Project recommendations ([#152](https://github.com/talos-systems/talos/issues/152)) ([b34debe](https://github.com/talos-systems/talos/commit/b34debe))



# [0.1.0-alpha.9](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.8...v0.1.0-alpha.9) (2018-09-20)


### Bug Fixes

* **init:** address linter error ([#146](https://github.com/talos-systems/talos/issues/146)) ([46e895a](https://github.com/talos-systems/talos/commit/46e895a))


### Features

* run system services via containerd ([#149](https://github.com/talos-systems/talos/issues/149)) ([8f09202](https://github.com/talos-systems/talos/commit/8f09202))
* **kernel:** upgrade Linux to v4.18.5 ([#147](https://github.com/talos-systems/talos/issues/147)) ([80b5e36](https://github.com/talos-systems/talos/commit/80b5e36))



# [0.1.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.7...v0.1.0-alpha.8) (2018-08-28)


### Features

* HA control plane ([#144](https://github.com/talos-systems/talos/issues/144)) ([260d55c](https://github.com/talos-systems/talos/commit/260d55c))
* list and restart processes ([#141](https://github.com/talos-systems/talos/issues/141)) ([db0cb37](https://github.com/talos-systems/talos/commit/db0cb37))
* **kernel:** upgrade Linux to v4.17.15 ([#140](https://github.com/talos-systems/talos/issues/140)) ([aab4316](https://github.com/talos-systems/talos/commit/aab4316))
* **osd:** node reset and reboot ([#142](https://github.com/talos-systems/talos/issues/142)) ([0514ff4](https://github.com/talos-systems/talos/commit/0514ff4))



# [0.1.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.6...v0.1.0-alpha.7) (2018-08-11)


### Bug Fixes

* **init:** make /etc/hosts writable ([#125](https://github.com/talos-systems/talos/issues/125)) ([4014872](https://github.com/talos-systems/talos/commit/4014872))
* **init:** read kubeadm env file ([#136](https://github.com/talos-systems/talos/issues/136)) ([d8a3a79](https://github.com/talos-systems/talos/commit/d8a3a79))
* **initramfs:** align go tests with upstream change ([#133](https://github.com/talos-systems/talos/issues/133)) ([275ede7](https://github.com/talos-systems/talos/commit/275ede7))


### Features

* upgrade Kubernetes to v1.11.2 ([#139](https://github.com/talos-systems/talos/issues/139)) ([37df8a3](https://github.com/talos-systems/talos/commit/37df8a3))
* **conformance:** add conformance image ([#126](https://github.com/talos-systems/talos/issues/126)) ([6b661c3](https://github.com/talos-systems/talos/commit/6b661c3))
* **conformance:** add quick mode config ([#129](https://github.com/talos-systems/talos/issues/129)) ([6185ac5](https://github.com/talos-systems/talos/commit/6185ac5))
* **hack:**  add CIS Kubernetes Benchmark script ([#134](https://github.com/talos-systems/talos/issues/134)) ([deea44b](https://github.com/talos-systems/talos/commit/deea44b))
* **hack:** use ubuntu 18.04 image in debug pod ([#135](https://github.com/talos-systems/talos/issues/135)) ([73597c3](https://github.com/talos-systems/talos/commit/73597c3))
* **image:** make AMI regions a variable ([#137](https://github.com/talos-systems/talos/issues/137)) ([79bb464](https://github.com/talos-systems/talos/commit/79bb464))
* **init:** add file creation option ([#132](https://github.com/talos-systems/talos/issues/132)) ([5058b74](https://github.com/talos-systems/talos/commit/5058b74))
* **init:** debug option ([#138](https://github.com/talos-systems/talos/issues/138)) ([6058af2](https://github.com/talos-systems/talos/commit/6058af2))
* **initramfs:** check for self-hosted-kube-apiserver label ([#130](https://github.com/talos-systems/talos/issues/130)) ([5d0fa41](https://github.com/talos-systems/talos/commit/5d0fa41))
* **kernel:** upgrade Linux to v4.17.10 ([#128](https://github.com/talos-systems/talos/issues/128)) ([cb1a939](https://github.com/talos-systems/talos/commit/cb1a939))



# [0.1.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.5...v0.1.0-alpha.6) (2018-07-24)


### Bug Fixes

* **rootfs:** don't remove the docker binary ([#119](https://github.com/talos-systems/talos/issues/119)) ([eabd76c](https://github.com/talos-systems/talos/commit/eabd76c))


### Features

* add a debug pod manifest ([#120](https://github.com/talos-systems/talos/issues/120)) ([dc9e2fe](https://github.com/talos-systems/talos/commit/dc9e2fe))
* run the kubelet in a container ([#122](https://github.com/talos-systems/talos/issues/122)) ([90d3078](https://github.com/talos-systems/talos/commit/90d3078))
* upgrade Kubernetes to v1.11.1 ([#123](https://github.com/talos-systems/talos/issues/123)) ([b48884b](https://github.com/talos-systems/talos/commit/b48884b))
* **image:** generate image ([#114](https://github.com/talos-systems/talos/issues/114)) ([f6adabe](https://github.com/talos-systems/talos/commit/f6adabe))
* **initramfs:** rewrite user data ([#121](https://github.com/talos-systems/talos/issues/121)) ([0036bd1](https://github.com/talos-systems/talos/commit/0036bd1))
* **initramfs:** set the platform explicitly ([#124](https://github.com/talos-systems/talos/issues/124)) ([ca93ede](https://github.com/talos-systems/talos/commit/ca93ede))



# [0.1.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.4...v0.1.0-alpha.5) (2018-07-02)


### Bug Fixes

* create build directory ([#108](https://github.com/talos-systems/talos/issues/108)) ([9321d7a](https://github.com/talos-systems/talos/commit/9321d7a))
* field tag should be yaml instead of json ([#100](https://github.com/talos-systems/talos/issues/100)) ([c0e7996](https://github.com/talos-systems/talos/commit/c0e7996))


### Features

* **init:** configurable kubelet arguments ([#99](https://github.com/talos-systems/talos/issues/99)) ([5bd0879](https://github.com/talos-systems/talos/commit/5bd0879))
* **init:** platform discovery ([#101](https://github.com/talos-systems/talos/issues/101)) ([b1a7a82](https://github.com/talos-systems/talos/commit/b1a7a82))
* **initramfs:** Kubernetes API reverse proxy ([#107](https://github.com/talos-systems/talos/issues/107)) ([ea1edbb](https://github.com/talos-systems/talos/commit/ea1edbb))
* **kernel:** enable Ceph ([#105](https://github.com/talos-systems/talos/issues/105)) ([d5b6eca](https://github.com/talos-systems/talos/commit/d5b6eca))
* **rootfs:** install cut ([#106](https://github.com/talos-systems/talos/issues/106)) ([9823c35](https://github.com/talos-systems/talos/commit/9823c35))
* **rootfs:** upgrade Docker to v17.03.2-ce ([#111](https://github.com/talos-systems/talos/issues/111)) ([fa4f787](https://github.com/talos-systems/talos/commit/fa4f787))
* **rootfs:** upgrade Kubernetes to v1.11.0-beta.1 ([#104](https://github.com/talos-systems/talos/issues/104)) ([5519410](https://github.com/talos-systems/talos/commit/5519410))



# [0.1.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.3...v0.1.0-alpha.4) (2018-05-20)


### Bug Fixes

* force the kernel to reread partition table ([#88](https://github.com/talos-systems/talos/issues/88)) ([c843201](https://github.com/talos-systems/talos/commit/c843201))
* use commit SHA on master and tag name on tags ([#98](https://github.com/talos-systems/talos/issues/98)) ([2bd7b89](https://github.com/talos-systems/talos/commit/2bd7b89))
* **init:** conditionally set version in /etc/os-release ([#97](https://github.com/talos-systems/talos/issues/97)) ([65c2c32](https://github.com/talos-systems/talos/commit/65c2c32))
* **init:** use /proc/net/pnp as resolv.conf ([#87](https://github.com/talos-systems/talos/issues/87)) ([2aed515](https://github.com/talos-systems/talos/commit/2aed515))
* **initramfs:** build variables ([#93](https://github.com/talos-systems/talos/issues/93)) ([b55ce73](https://github.com/talos-systems/talos/commit/b55ce73))
* **initramfs:** escape double quotes ([#96](https://github.com/talos-systems/talos/issues/96)) ([63a0728](https://github.com/talos-systems/talos/commit/63a0728))
* **initramfs:** invalid reference to template variable ([#94](https://github.com/talos-systems/talos/issues/94)) ([3dc22fa](https://github.com/talos-systems/talos/commit/3dc22fa))
* **initramfs:** quote -X flag ([#95](https://github.com/talos-systems/talos/issues/95)) ([068017a](https://github.com/talos-systems/talos/commit/068017a))


### Features

* add version command ([#85](https://github.com/talos-systems/talos/issues/85)) ([a55daaf](https://github.com/talos-systems/talos/commit/a55daaf))
* dynamic resolv.conf ([#86](https://github.com/talos-systems/talos/issues/86)) ([325ae5c](https://github.com/talos-systems/talos/commit/325ae5c))
* osctl configuration file ([#90](https://github.com/talos-systems/talos/issues/90)) ([a16008e](https://github.com/talos-systems/talos/commit/a16008e))
* upgrade kubernetes to v1.11.0-beta.0 ([#92](https://github.com/talos-systems/talos/issues/92)) ([8701fcb](https://github.com/talos-systems/talos/commit/8701fcb))
* **init:** verify EC2 PKCS7 signature ([#84](https://github.com/talos-systems/talos/issues/84)) ([7bf0abd](https://github.com/talos-systems/talos/commit/7bf0abd))



# [0.1.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.2...v0.1.0-alpha.3) (2018-05-15)


### Bug Fixes

* **generate:** use xvda instead of sda ([#77](https://github.com/talos-systems/talos/issues/77)) ([e18cf83](https://github.com/talos-systems/talos/commit/e18cf83))
* **init:** bad variable name and missing package ([#78](https://github.com/talos-systems/talos/issues/78)) ([7c37272](https://github.com/talos-systems/talos/commit/7c37272))


### Features

* automate signed certificates ([#81](https://github.com/talos-systems/talos/issues/81)) ([d517737](https://github.com/talos-systems/talos/commit/d517737))
* raw kubeadm configuration in user data ([#79](https://github.com/talos-systems/talos/issues/79)) ([fc98614](https://github.com/talos-systems/talos/commit/fc98614))
* **init:** don't print kubeadm token ([#74](https://github.com/talos-systems/talos/issues/74)) ([2f48972](https://github.com/talos-systems/talos/commit/2f48972))
* **kernel:** compile with Linux guest support ([#75](https://github.com/talos-systems/talos/issues/75)) ([67e092a](https://github.com/talos-systems/talos/commit/67e092a))



# [0.1.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.1...v0.1.0-alpha.2) (2018-05-09)


### Features

* upgrade Kubernetes to v1.10.2 ([#61](https://github.com/talos-systems/talos/issues/61)) ([dcf3a71](https://github.com/talos-systems/talos/commit/dcf3a71))
* **generate:** set RAW disk sizes dynamically ([#71](https://github.com/talos-systems/talos/issues/71)) ([5701ea6](https://github.com/talos-systems/talos/commit/5701ea6))
* **init:** gRPC with mutual TLS authentication ([#64](https://github.com/talos-systems/talos/issues/64)) ([f6686bc](https://github.com/talos-systems/talos/commit/f6686bc))
* **rootfs:** upgrade CRI-O to v1.10.1 ([#70](https://github.com/talos-systems/talos/issues/70)) ([ff61573](https://github.com/talos-systems/talos/commit/ff61573))



# [0.1.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.0...v0.1.0-alpha.1) (2018-04-20)


### Bug Fixes

* generate /etc/hosts and /etc/resolv.conf ([#54](https://github.com/talos-systems/talos/issues/54)) ([5bd43ab](https://github.com/talos-systems/talos/commit/5bd43ab))
* **init:** enable hierarchical accounting and reclaim ([#59](https://github.com/talos-systems/talos/issues/59)) ([68d95c2](https://github.com/talos-systems/talos/commit/68d95c2))
* **init:** missing parameter ([#55](https://github.com/talos-systems/talos/issues/55)) ([1a89469](https://github.com/talos-systems/talos/commit/1a89469))
* **init:** printf formatting ([#51](https://github.com/talos-systems/talos/issues/51)) ([b0782b6](https://github.com/talos-systems/talos/commit/b0782b6))
* **init:** remove unused code ([#56](https://github.com/talos-systems/talos/issues/56)) ([0c62bda](https://github.com/talos-systems/talos/commit/0c62bda))
* **init:** switch_root implementation ([#49](https://github.com/talos-systems/talos/issues/49)) ([b614179](https://github.com/talos-systems/talos/commit/b614179))


### Features

* docker as an optional container runtime ([#57](https://github.com/talos-systems/talos/issues/57)) ([3a60bdc](https://github.com/talos-systems/talos/commit/3a60bdc))
* upgrade to Kubernetes v1.10.1 ([#50](https://github.com/talos-systems/talos/issues/50)) ([46616d1](https://github.com/talos-systems/talos/commit/46616d1))
* **generate:** enable kernel logging ([#58](https://github.com/talos-systems/talos/issues/58)) ([71d97c8](https://github.com/talos-systems/talos/commit/71d97c8))
* **kernel:** use LTS kernel v4.14.34 ([#48](https://github.com/talos-systems/talos/issues/48)) ([4c9a810](https://github.com/talos-systems/talos/commit/4c9a810))



# [0.1.0-alpha.0](https://github.com/talos-systems/talos/compare/aba4615...v0.1.0-alpha.0) (2018-04-03)


### Bug Fixes

* **init:** address crio errors and warns ([#40](https://github.com/talos-systems/talos/issues/40)) ([7536d72](https://github.com/talos-systems/talos/commit/7536d72))
* **init:** don't create CRI-O CNI configurations ([#36](https://github.com/talos-systems/talos/issues/36)) ([8a7c424](https://github.com/talos-systems/talos/commit/8a7c424))
* **init:** make log handling non-blocking ([#37](https://github.com/talos-systems/talos/issues/37)) ([f244075](https://github.com/talos-systems/talos/commit/f244075))
* **init:** typo in service subnet field; pin version of Kubernetes ([#10](https://github.com/talos-systems/talos/issues/10)) ([8427ddf](https://github.com/talos-systems/talos/commit/8427ddf))
* **rootfs:** install conntrack ([#27](https://github.com/talos-systems/talos/issues/27)) ([1067958](https://github.com/talos-systems/talos/commit/1067958))


### Features

* enable IPVS ([#42](https://github.com/talos-systems/talos/issues/42)) ([168c598](https://github.com/talos-systems/talos/commit/168c598))
* initial implementation ([#2](https://github.com/talos-systems/talos/issues/2)) ([aba4615](https://github.com/talos-systems/talos/commit/aba4615))
* mount ROOT partition as RO ([#11](https://github.com/talos-systems/talos/issues/11)) ([29bdd6d](https://github.com/talos-systems/talos/commit/29bdd6d))
* update Kubernetes to v1.10.0 ([#26](https://github.com/talos-systems/talos/issues/26)) ([9a11837](https://github.com/talos-systems/talos/commit/9a11837))
* update Kubernetes to v1.10.0-rc.1 ([#25](https://github.com/talos-systems/talos/issues/25)) ([901461c](https://github.com/talos-systems/talos/commit/901461c))
* update to linux 4.15.13 ([#30](https://github.com/talos-systems/talos/issues/30)) ([e418d29](https://github.com/talos-systems/talos/commit/e418d29))
* use CRI-O as the container runtime ([#12](https://github.com/talos-systems/talos/issues/12)) ([7785d6f](https://github.com/talos-systems/talos/commit/7785d6f))
* **init:** add node join functionality ([#38](https://github.com/talos-systems/talos/issues/38)) ([0251868](https://github.com/talos-systems/talos/commit/0251868))
* **init:** basic process managment ([#6](https://github.com/talos-systems/talos/issues/6)) ([6c1038b](https://github.com/talos-systems/talos/commit/6c1038b))
* **init:** provide and endpoint for getting logs of running processes ([#9](https://github.com/talos-systems/talos/issues/9)) ([37d80cf](https://github.com/talos-systems/talos/commit/37d80cf))
* **init:** set kubelet log level to 4 ([#13](https://github.com/talos-systems/talos/issues/13)) ([9597b21](https://github.com/talos-systems/talos/commit/9597b21))
* **init:** use CoreDNS by default ([#39](https://github.com/talos-systems/talos/issues/39)) ([a8e3d50](https://github.com/talos-systems/talos/commit/a8e3d50))
* **init:** user data ([#17](https://github.com/talos-systems/talos/issues/17)) ([3ee01ae](https://github.com/talos-systems/talos/commit/3ee01ae))
* **kernel:** enable nf_tables and ebtables modules ([#41](https://github.com/talos-systems/talos/issues/41)) ([cf53a27](https://github.com/talos-systems/talos/commit/cf53a27))
* **rootfs:** upgrade cri-o and cri-tools ([#35](https://github.com/talos-systems/talos/issues/35)) ([0095227](https://github.com/talos-systems/talos/commit/0095227))



