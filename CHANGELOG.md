## [Talos 0.10.0-alpha.1](https://github.com/talos-systems/talos/releases/tag/v0.10.0-alpha.1) (2021-03-31)

Welcome to the v0.10.0-alpha.1 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Install Disk Selector

Install section of the machine config now has `diskSelector` field that allows querying install disk using the list of qualifiers:

```yaml
...
  install:
    diskSelector:
      size: >= 500GB
      model: WDC*
...
```

`talosctl disks -n <node> -i` can be used to check allowed disk qualifiers when the node is running in the maintenance mode.


### Optmizations

* Talos `system` services now run without container images on initramfs from the single executable; this change reduces RAM usage, initramfs size and boot time..


### SBCs

* u-boot version was updated to fix the boot and USB issues on Raspberry Pi 4 8GiB version.
* added support for Rock Pi 4.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Spencer Smith
* Seán C McCord
* Andrew Rynhard
* Jorik Jonker
* bzub

### Changes
<details><summary>78 commits</summary>
<p>

* [`8309312a`](https://github.com/talos-systems/talos/commit/8309312a3db89cea17b673d0d1c73175db5258ac) chore: build components with race detector enabled in dev mode
* [`7d912584`](https://github.com/talos-systems/talos/commit/7d9125847506dfadc7e137a30bf0c93ab9ca0b50) test: fix data race in apply config tests
* [`204caf8e`](https://github.com/talos-systems/talos/commit/204caf8eb9c6c43a90c20ebaea8387584201e7f5) test: fix apply-config integration test, bump clusterctl version
* [`d812099d`](https://github.com/talos-systems/talos/commit/d812099df3d060ae74cd3d28405ddacbdd72ab15) fix: address several issues in TUI installer
* [`269c9ad0`](https://github.com/talos-systems/talos/commit/269c9ad0988f0f966a4e31a5ab744fed7d585385) fix: don't write to config object on access
* [`a9451f57`](https://github.com/talos-systems/talos/commit/a9451f57129b0b452825850bba9477ac3c536547) feat: update Kubernetes to 1.21.0-beta.1
* [`4b42ced4`](https://github.com/talos-systems/talos/commit/4b42ced4c2a300aa22f253435a4d6330770ec5c2) feat: add ability to disable comments in talosctl gen config
* [`a0dcfc3d`](https://github.com/talos-systems/talos/commit/a0dcfc3d5288e633db80bf3e32d31e41756cc90f) fix: workaround race in containerd runner with stdin pipe
* [`2ea20f59`](https://github.com/talos-systems/talos/commit/2ea20f598a01f3de95f633bdfaf5711738524ba2) feat: replace timed with time sync controller
* [`c38a161a`](https://github.com/talos-systems/talos/commit/c38a161ade34f00f7af52d9ae047d7936246e7f0) test: add unit-test for machine config validation
* [`a6106815`](https://github.com/talos-systems/talos/commit/a6106815b72efcb7f4df0caab6b93be49a7590ea) chore: bump dependencies via dependabot
* [`35598f39`](https://github.com/talos-systems/talos/commit/35598f391d5d0659e3390d4db67c7ed88c17b6eb) chore: refactor: extract ClusterConfig
* [`03285184`](https://github.com/talos-systems/talos/commit/032851844fdea4b1bde7507720025c981ee3b12c) fix: get rid of data race in encoder and fix concurrent map access
* [`4b3580aa`](https://github.com/talos-systems/talos/commit/4b3580aa57d83358434238ad953793070cfc67a7) fix: prevent panic in validate config if `machine.install` is missing
* [`d7e9f6d6`](https://github.com/talos-systems/talos/commit/d7e9f6d6a89143f0def74a270a21ed5e53556e07) chore: build integration tests with -race
* [`9f7d67ac`](https://github.com/talos-systems/talos/commit/9f7d67ac717834ed428b8f13d4061db5f33c81f9) chore: fix typo
* [`672c9707`](https://github.com/talos-systems/talos/commit/672c970739971dd0c558ad0319fe9fdbd66a741b) fix: allow `convert-k8s --remove-initialized-keys` with K8s cp is down
* [`fb605a0f`](https://github.com/talos-systems/talos/commit/fb605a0fc56e6df1ceae8c391524ac987bbba09d) chore: tweak nolintlint settings
* [`1f5a0c40`](https://github.com/talos-systems/talos/commit/1f5a0c4065e1fbd63ebe6d48c13e669bfb1dbeac) fix: resolve the issue with Kubernetes upgrade
* [`74b2b557`](https://github.com/talos-systems/talos/commit/74b2b5578cbe639a6f2663df6ab7a5e80b139fe0) docs: update AWS docs to ensure instances are tagged
* [`dc21d9b4`](https://github.com/talos-systems/talos/commit/dc21d9b4b0f5858fbe0d4072e8a47a934780c3dd) chore: remove old file
* [`966caf7a`](https://github.com/talos-systems/talos/commit/966caf7a674c20047c1184e64f3727abc0c54296) chore: remove unused module replace directives
* [`98b22f1e`](https://github.com/talos-systems/talos/commit/98b22f1e0b0f5e85b71d344041265efa95e1bb91) feat: show short options in talosctl kubeconfig
* [`51139d54`](https://github.com/talos-systems/talos/commit/51139d54d4ce4acf2e78f11ab0f384f91f86ff33) chore: cache go modules in the build
* [`65701aa7`](https://github.com/talos-systems/talos/commit/65701aa724130645fcabe521557225ff41b359b0) fix: resolve the issue with DHCP lease not being renewed
* [`711f5b23`](https://github.com/talos-systems/talos/commit/711f5b23be69665d6204dbb80064e0ab0d1468c0) fix: config validation: CNI should apply to cp nodes, encryption config
* [`5ff491d9`](https://github.com/talos-systems/talos/commit/5ff491d9686434a6208583dca97171bfbecf3f70) fix: allow empty list for CNI URLs
* [`946e74f0`](https://github.com/talos-systems/talos/commit/946e74f047f30180bf5f0554fd8ae1043e0d1f52) docs: update path for kernel downloads in qemu docs
* [`ed272e60`](https://github.com/talos-systems/talos/commit/ed272e604e67dc38557812e5f4dbcb8666c4b546) feat: update Kubernetes to 1.21.0-beta.0
* [`b0209fd2`](https://github.com/talos-systems/talos/commit/b0209fd29d3895d7a0b8806e505bbefcf2bba520) refactor: move networkd, timed APIs to machined, remove routerd
* [`6ffabe51`](https://github.com/talos-systems/talos/commit/6ffabe51691907b43f9f970f22d7aec4df19a6c3) feat: add ability to find disk by disk properties
* [`ac876470`](https://github.com/talos-systems/talos/commit/ac8764702f980a8dea5b6a67f0bc33b5203efecb) refactor: move apid, routerd, timed and trustd to single executable
* [`89a4b09f`](https://github.com/talos-systems/talos/commit/89a4b09fe8015e70f7074d9af72d47023ece2f1d) refactor: run networkd as a goroutine in machined
* [`f4a6a19c`](https://github.com/talos-systems/talos/commit/f4a6a19cd1bf1da7f2610276c00e8144a78f8694) chore: update sonobuoy
* [`dc294db1`](https://github.com/talos-systems/talos/commit/dc294db16c8bdb10e3f63987c87c0bbdf629b158) chore: bump dependencies via dependabot
* [`2b1641a3`](https://github.com/talos-systems/talos/commit/2b1641a3b543d736eb0d2e359d2a25dbc906e631) docs: add AMIs for Talos 0.9.0
* [`79ceb428`](https://github.com/talos-systems/talos/commit/79ceb428d4216a06418933058485ec2273474e3c) docs: make v0.9 the default docs
* [`a5b62f4d`](https://github.com/talos-systems/talos/commit/a5b62f4dc20da721b0f74c5fbb5082038e05e4f4) docs: add documentation for Talos 0.10
* [`ce795f1c`](https://github.com/talos-systems/talos/commit/ce795f1cea9d78c26edbcd4a40bb5d3637fde629) fix: command `etcd remove-member` shouldn't remove etcd data directory
* [`aab49a16`](https://github.com/talos-systems/talos/commit/aab49a167b1f1cd3974e3aa1244d636ba712f678) fix: repair zsh completion
* [`fc9c416a`](https://github.com/talos-systems/talos/commit/fc9c416a3c8425bb42892f740c910894610acd00) fix: build rockpi4 metal image as part of CI build
* [`125b86f4`](https://github.com/talos-systems/talos/commit/125b86f4efbc2ed3e0a4bdfc945e97b05f1cb82c) fix: upgrade-k8s bug with empty config values and provision script
* [`8b2d228d`](https://github.com/talos-systems/talos/commit/8b2d228dc42c196090aae1e6958683e265ebc05c) chore: add script for starting registry proxies
* [`f7d276b8`](https://github.com/talos-systems/talos/commit/f7d276b854c4c06f85155c517cc1de7109a53359) chore: remove old `osctl` reference
* [`5b14d6f2`](https://github.com/talos-systems/talos/commit/5b14d6f2b89c5b86f9ec2cb0271c6605272269d4) chore: fix `make help` output
* [`f0512dfc`](https://github.com/talos-systems/talos/commit/f0512dfce9443cf20790ef8b4fd8e87906cc5bda) feat: update Kubernetes to 1.20.5
* [`24cd0a20`](https://github.com/talos-systems/talos/commit/24cd0a20678f2728a0b36c1c401dd8af3d4932ed) feat: publish talosctl container image
* [`6e17102c`](https://github.com/talos-systems/talos/commit/6e17102c210dccd4bf78d347de07cfe2ba7737c4) chore: remove unused code
* [`88104407`](https://github.com/talos-systems/talos/commit/8810440744453550697ad39530633b81889d38b7) docs: add control plane in-depth guide
* [`ecf03449`](https://github.com/talos-systems/talos/commit/ecf034496e7450f89369140ad1791188580dee0d) chore: bump Go to 1.16.2
* [`cbc38418`](https://github.com/talos-systems/talos/commit/cbc38418d856a00ffb35d31676e1efb14fb6da36) release(v0.10.0-alpha.0): prepare release
* [`3455a8e8`](https://github.com/talos-systems/talos/commit/3455a8e8185ba25777784d392d6150a4a7e2d4a9) chore: use new release tool for changelogs and release notes
* [`08271ba9`](https://github.com/talos-systems/talos/commit/08271ba93178c17a7c495788fea00c5c380f8301) chore: use Go 1.16 language version
* [`7662d033`](https://github.com/talos-systems/talos/commit/7662d033bfc3d6e3878e2c2a2a1ec4d71dc2502e) fix: talosctl health should not check kube-proxy when it is disabled
* [`0dbaeb9e`](https://github.com/talos-systems/talos/commit/0dbaeb9e655acdc44f8b4db6d1bc6da2ddf6cc9d) chore: update tools, use new generators
* [`e31790f6`](https://github.com/talos-systems/talos/commit/e31790f6f548095fe3f1b9a5c88b47e70c197d2c) fix: properly format spec comments in the resources
* [`78d384eb`](https://github.com/talos-systems/talos/commit/78d384ebb6246cf41a73014312dfb0d86a8008d6) test: update aws cloud provider version
* [`3c5bfbb4`](https://github.com/talos-systems/talos/commit/3c5bfbb4736c86f493a665dbfe63a6e2d20acb3d) fix: don't touch any partitions on upgrade with --preserve
* [`891f90fe`](https://github.com/talos-systems/talos/commit/891f90fee9818f0f013878c0c77c1920e6427a91) chore: update Linux to 5.10.23
* [`d4d77882`](https://github.com/talos-systems/talos/commit/d4d77882e3f53f2449f50f54116a407726f41ede) chore: update dependencies via dependabot
* [`2e22f20b`](https://github.com/talos-systems/talos/commit/2e22f20bd876e4972bfdebd44fee13356b70b83f) docs: minor fixes to getting started
* [`ca8a5596`](https://github.com/talos-systems/talos/commit/ca8a5596c79f638e52601e850236b715f906e3d2) chore: fix provision tests after changes to build-container
* [`4aae924c`](https://github.com/talos-systems/talos/commit/4aae924c685ff578af06a1adceeec4f1938576a6) refactor: provide explicit logger for networkd
* [`22f37530`](https://github.com/talos-systems/talos/commit/22f375300c1cc1d95db540afd510a21b66d7c8a3) chore: update golanci-lint to 1.38.0
* [`83b4e7f7`](https://github.com/talos-systems/talos/commit/83b4e7f744e3a8ed21443642a9afcf5b1342c62b) feat: add Rock pi 4 support
* [`1362966f`](https://github.com/talos-systems/talos/commit/1362966ff546ee620c14e9312255616685743eed) docs: rewrite getting-started for ISO
* [`8e57fc4f`](https://github.com/talos-systems/talos/commit/8e57fc4f526096878213048658bae50cfac4cda8) fix: move containerd CRI config files under `/var/`
* [`6f7df3da`](https://github.com/talos-systems/talos/commit/6f7df3da1e147212e6d4b40a5de65e5ca8be84db) fix: update output of `convert-k8s` command
* [`dce6118c`](https://github.com/talos-systems/talos/commit/dce6118c290afe957e375586b6bbc5b10ef6ba09) docs: add guide for VIP
* [`ee5d9ffa`](https://github.com/talos-systems/talos/commit/ee5d9ffac60c93561874995d8926fc329e2b67dc) chore: bump Go to 1.16.1
* [`7c529e1c`](https://github.com/talos-systems/talos/commit/7c529e1cbd2be66d71e8496304781dd406495bdd) docs: fix links in the documentation
* [`f596c7f6`](https://github.com/talos-systems/talos/commit/f596c7f6be3880be994faf7c5361628024c6be7d) docs: add video for raspberry pi install
* [`47324dca`](https://github.com/talos-systems/talos/commit/47324dcaeaee94e4963eb3764fc01cd2d2d43041) docs: add guide on editing machine configuration
* [`99d5f894`](https://github.com/talos-systems/talos/commit/99d5f894e17f39004e61ee9d5b64d5a8139f33d0) chore: update website npm dependencies
* [`11056a80`](https://github.com/talos-systems/talos/commit/11056a80349e4c8df10a9ea98b6e3d53f96b971c) docs: add highlights for 0.9 release
* [`ae8bedb9`](https://github.com/talos-systems/talos/commit/ae8bedb9a0d999bfbe97b6e18dc2eff62f0fcb80) docs: add control plane conversion guide and 0.9 upgrade notes
* [`ed9673e5`](https://github.com/talos-systems/talos/commit/ed9673e50a7cb973fc49be9c2d659447a4c5bd62) docs: add troubleshooting control plane documentation
* [`485cb126`](https://github.com/talos-systems/talos/commit/485cb1262f97e982ea81597b49d173836c75558d) docs: update Kubernetes upgrade guide
</p>
</details>

### Changes since v0.10.0-alpha.0
<details><summary>50 commits</summary>
<p>

* [`8309312a`](https://github.com/talos-systems/talos/commit/8309312a3db89cea17b673d0d1c73175db5258ac) chore: build components with race detector enabled in dev mode
* [`7d912584`](https://github.com/talos-systems/talos/commit/7d9125847506dfadc7e137a30bf0c93ab9ca0b50) test: fix data race in apply config tests
* [`204caf8e`](https://github.com/talos-systems/talos/commit/204caf8eb9c6c43a90c20ebaea8387584201e7f5) test: fix apply-config integration test, bump clusterctl version
* [`d812099d`](https://github.com/talos-systems/talos/commit/d812099df3d060ae74cd3d28405ddacbdd72ab15) fix: address several issues in TUI installer
* [`269c9ad0`](https://github.com/talos-systems/talos/commit/269c9ad0988f0f966a4e31a5ab744fed7d585385) fix: don't write to config object on access
* [`a9451f57`](https://github.com/talos-systems/talos/commit/a9451f57129b0b452825850bba9477ac3c536547) feat: update Kubernetes to 1.21.0-beta.1
* [`4b42ced4`](https://github.com/talos-systems/talos/commit/4b42ced4c2a300aa22f253435a4d6330770ec5c2) feat: add ability to disable comments in talosctl gen config
* [`a0dcfc3d`](https://github.com/talos-systems/talos/commit/a0dcfc3d5288e633db80bf3e32d31e41756cc90f) fix: workaround race in containerd runner with stdin pipe
* [`2ea20f59`](https://github.com/talos-systems/talos/commit/2ea20f598a01f3de95f633bdfaf5711738524ba2) feat: replace timed with time sync controller
* [`c38a161a`](https://github.com/talos-systems/talos/commit/c38a161ade34f00f7af52d9ae047d7936246e7f0) test: add unit-test for machine config validation
* [`a6106815`](https://github.com/talos-systems/talos/commit/a6106815b72efcb7f4df0caab6b93be49a7590ea) chore: bump dependencies via dependabot
* [`35598f39`](https://github.com/talos-systems/talos/commit/35598f391d5d0659e3390d4db67c7ed88c17b6eb) chore: refactor: extract ClusterConfig
* [`03285184`](https://github.com/talos-systems/talos/commit/032851844fdea4b1bde7507720025c981ee3b12c) fix: get rid of data race in encoder and fix concurrent map access
* [`4b3580aa`](https://github.com/talos-systems/talos/commit/4b3580aa57d83358434238ad953793070cfc67a7) fix: prevent panic in validate config if `machine.install` is missing
* [`d7e9f6d6`](https://github.com/talos-systems/talos/commit/d7e9f6d6a89143f0def74a270a21ed5e53556e07) chore: build integration tests with -race
* [`9f7d67ac`](https://github.com/talos-systems/talos/commit/9f7d67ac717834ed428b8f13d4061db5f33c81f9) chore: fix typo
* [`672c9707`](https://github.com/talos-systems/talos/commit/672c970739971dd0c558ad0319fe9fdbd66a741b) fix: allow `convert-k8s --remove-initialized-keys` with K8s cp is down
* [`fb605a0f`](https://github.com/talos-systems/talos/commit/fb605a0fc56e6df1ceae8c391524ac987bbba09d) chore: tweak nolintlint settings
* [`1f5a0c40`](https://github.com/talos-systems/talos/commit/1f5a0c4065e1fbd63ebe6d48c13e669bfb1dbeac) fix: resolve the issue with Kubernetes upgrade
* [`74b2b557`](https://github.com/talos-systems/talos/commit/74b2b5578cbe639a6f2663df6ab7a5e80b139fe0) docs: update AWS docs to ensure instances are tagged
* [`dc21d9b4`](https://github.com/talos-systems/talos/commit/dc21d9b4b0f5858fbe0d4072e8a47a934780c3dd) chore: remove old file
* [`966caf7a`](https://github.com/talos-systems/talos/commit/966caf7a674c20047c1184e64f3727abc0c54296) chore: remove unused module replace directives
* [`98b22f1e`](https://github.com/talos-systems/talos/commit/98b22f1e0b0f5e85b71d344041265efa95e1bb91) feat: show short options in talosctl kubeconfig
* [`51139d54`](https://github.com/talos-systems/talos/commit/51139d54d4ce4acf2e78f11ab0f384f91f86ff33) chore: cache go modules in the build
* [`65701aa7`](https://github.com/talos-systems/talos/commit/65701aa724130645fcabe521557225ff41b359b0) fix: resolve the issue with DHCP lease not being renewed
* [`711f5b23`](https://github.com/talos-systems/talos/commit/711f5b23be69665d6204dbb80064e0ab0d1468c0) fix: config validation: CNI should apply to cp nodes, encryption config
* [`5ff491d9`](https://github.com/talos-systems/talos/commit/5ff491d9686434a6208583dca97171bfbecf3f70) fix: allow empty list for CNI URLs
* [`946e74f0`](https://github.com/talos-systems/talos/commit/946e74f047f30180bf5f0554fd8ae1043e0d1f52) docs: update path for kernel downloads in qemu docs
* [`ed272e60`](https://github.com/talos-systems/talos/commit/ed272e604e67dc38557812e5f4dbcb8666c4b546) feat: update Kubernetes to 1.21.0-beta.0
* [`b0209fd2`](https://github.com/talos-systems/talos/commit/b0209fd29d3895d7a0b8806e505bbefcf2bba520) refactor: move networkd, timed APIs to machined, remove routerd
* [`6ffabe51`](https://github.com/talos-systems/talos/commit/6ffabe51691907b43f9f970f22d7aec4df19a6c3) feat: add ability to find disk by disk properties
* [`ac876470`](https://github.com/talos-systems/talos/commit/ac8764702f980a8dea5b6a67f0bc33b5203efecb) refactor: move apid, routerd, timed and trustd to single executable
* [`89a4b09f`](https://github.com/talos-systems/talos/commit/89a4b09fe8015e70f7074d9af72d47023ece2f1d) refactor: run networkd as a goroutine in machined
* [`f4a6a19c`](https://github.com/talos-systems/talos/commit/f4a6a19cd1bf1da7f2610276c00e8144a78f8694) chore: update sonobuoy
* [`dc294db1`](https://github.com/talos-systems/talos/commit/dc294db16c8bdb10e3f63987c87c0bbdf629b158) chore: bump dependencies via dependabot
* [`2b1641a3`](https://github.com/talos-systems/talos/commit/2b1641a3b543d736eb0d2e359d2a25dbc906e631) docs: add AMIs for Talos 0.9.0
* [`79ceb428`](https://github.com/talos-systems/talos/commit/79ceb428d4216a06418933058485ec2273474e3c) docs: make v0.9 the default docs
* [`a5b62f4d`](https://github.com/talos-systems/talos/commit/a5b62f4dc20da721b0f74c5fbb5082038e05e4f4) docs: add documentation for Talos 0.10
* [`ce795f1c`](https://github.com/talos-systems/talos/commit/ce795f1cea9d78c26edbcd4a40bb5d3637fde629) fix: command `etcd remove-member` shouldn't remove etcd data directory
* [`aab49a16`](https://github.com/talos-systems/talos/commit/aab49a167b1f1cd3974e3aa1244d636ba712f678) fix: repair zsh completion
* [`fc9c416a`](https://github.com/talos-systems/talos/commit/fc9c416a3c8425bb42892f740c910894610acd00) fix: build rockpi4 metal image as part of CI build
* [`125b86f4`](https://github.com/talos-systems/talos/commit/125b86f4efbc2ed3e0a4bdfc945e97b05f1cb82c) fix: upgrade-k8s bug with empty config values and provision script
* [`8b2d228d`](https://github.com/talos-systems/talos/commit/8b2d228dc42c196090aae1e6958683e265ebc05c) chore: add script for starting registry proxies
* [`f7d276b8`](https://github.com/talos-systems/talos/commit/f7d276b854c4c06f85155c517cc1de7109a53359) chore: remove old `osctl` reference
* [`5b14d6f2`](https://github.com/talos-systems/talos/commit/5b14d6f2b89c5b86f9ec2cb0271c6605272269d4) chore: fix `make help` output
* [`f0512dfc`](https://github.com/talos-systems/talos/commit/f0512dfce9443cf20790ef8b4fd8e87906cc5bda) feat: update Kubernetes to 1.20.5
* [`24cd0a20`](https://github.com/talos-systems/talos/commit/24cd0a20678f2728a0b36c1c401dd8af3d4932ed) feat: publish talosctl container image
* [`6e17102c`](https://github.com/talos-systems/talos/commit/6e17102c210dccd4bf78d347de07cfe2ba7737c4) chore: remove unused code
* [`88104407`](https://github.com/talos-systems/talos/commit/8810440744453550697ad39530633b81889d38b7) docs: add control plane in-depth guide
* [`ecf03449`](https://github.com/talos-systems/talos/commit/ecf034496e7450f89369140ad1791188580dee0d) chore: bump Go to 1.16.2
</p>
</details>

### Changes from talos-systems/extras
<details><summary>2 commits</summary>
<p>

* [`c0fa0c0`](https://github.com/talos-systems/extras/commit/c0fa0c04641d8dfc418888c210788a6894e8d40c) feat: bump Go to 1.16.2
* [`5f89d77`](https://github.com/talos-systems/extras/commit/5f89d77a91f44d52146dae9c23b4654d219042b9) feat: bump Go to 1.16.1
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>1 commit</summary>
<p>

* [`776b37d`](https://github.com/talos-systems/go-blockdevice/commit/776b37d31de0781f098f5d9d1894fbea3f2dfa1d) feat: add options to probe disk by various sysblock parameters
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>6 commits</summary>
<p>

* [`fdf4866`](https://github.com/talos-systems/pkgs/commit/fdf48667851b4c80b0ca220c574d2fb57a943f64) feat: bump tools for Go 1.16.2
* [`35f9b6f`](https://github.com/talos-systems/pkgs/commit/35f9b6f22bbe094e93723559132b2a23f0853c2b) feat: update kernel to 5.10.23
* [`dbae83e`](https://github.com/talos-systems/pkgs/commit/dbae83e704da264066ceeca20e0fe66883b542ba) fix: do not use git-lfs for rockpi4 binaries
* [`1c6b9a3`](https://github.com/talos-systems/pkgs/commit/1c6b9a3a6ef91bce4f0cba18c466a9ece7b14750) feat: bump tools for Go 1.16.1
* [`c18073f`](https://github.com/talos-systems/pkgs/commit/c18073fe79b9d7ec36411c6f329fa60c580d4cea) feat: add u-boot for Rock Pi 4
* [`6b85a2b`](https://github.com/talos-systems/pkgs/commit/6b85a2bffbb144f25356eed6ed9dc8bb9a3fd392) feat: upgrade u-boot to 2021.04-rc3
</p>
</details>

### Changes from talos-systems/tools
<details><summary>4 commits</summary>
<p>

* [`41b8073`](https://github.com/talos-systems/tools/commit/41b807369779606f54d76e56038bfaf88d4f0f25) feat: bump protobuf-related tools
* [`f7bce92`](https://github.com/talos-systems/tools/commit/f7bce92febdf9f58f2940952d5138494b9232ea8) chore: bump Go to 1.16.2
* [`bcf3380`](https://github.com/talos-systems/tools/commit/bcf3380dd55810e556851acbe20e20cb4ddd5ef0) feat: bump protobuf deps, add protoc-gen-go-grpc
* [`b49c40e`](https://github.com/talos-systems/tools/commit/b49c40e0ad701f13192c1ad85ec616224343dc3f) feat: bump Go to 1.16.1
</p>
</details>

### Dependency Changes

* **github.com/coreos/go-semver**              v0.3.0 **_new_**
* **github.com/golang/protobuf**               v1.4.3 -> v1.5.1
* **github.com/google/go-cmp**                 v0.5.4 -> v0.5.5
* **github.com/hashicorp/go-multierror**       v1.1.0 -> v1.1.1
* **github.com/talos-systems/extras**          v0.2.0-1-g0db3328 -> v0.3.0-alpha.0-1-gc0fa0c0
* **github.com/talos-systems/go-blockdevice**  bb3ad73f6983 -> 776b37d31de0
* **github.com/talos-systems/pkgs**            v0.4.1-2-gd471b60 -> v0.5.0-alpha.0-3-gfdf4866
* **github.com/talos-systems/tools**           v0.4.0-1-g3b25a7e -> v0.5.0-alpha.0-3-g41b8073
* **google.golang.org/grpc**                   v1.36.0 -> v1.36.1
* **google.golang.org/protobuf**               v1.25.0 -> v1.26.0
* **k8s.io/api**                               v0.20.5 -> v0.21.0-rc.0
* **k8s.io/apimachinery**                      v0.20.5 -> v0.21.0-rc.0
* **k8s.io/apiserver**                         v0.20.5 -> v0.21.0-rc.0
* **k8s.io/client-go**                         v0.20.5 -> v0.21.0-rc.0
* **k8s.io/cri-api**                           v0.20.5 -> v0.21.0-rc.0
* **k8s.io/kubectl**                           v0.20.5 -> v0.21.0-rc.0
* **k8s.io/kubelet**                           v0.20.5 -> v0.21.0-rc.0

Previous release can be found at [v0.9.0](https://github.com/talos-systems/talos/releases/tag/v0.9.0)

## [Talos 0.10.0-alpha.0](https://github.com/talos-systems/talos/releases/tag/v0.10.0-alpha.0) (2021-03-17)

Welcome to the v0.10.0-alpha.0 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### SBCs

* u-boot version was updated to fix the boot and USB issues on Raspberry Pi 4 8GiB version.
* added support for Rock Pi 4.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Seán C McCord
* Spencer Smith
* Andrew Rynhard

### Changes
<details><summary>27 commits</summary>
<p>

* [`3455a8e8`](https://github.com/talos-systems/talos/commit/3455a8e8185ba25777784d392d6150a4a7e2d4a9) chore: use new release tool for changelogs and release notes
* [`08271ba9`](https://github.com/talos-systems/talos/commit/08271ba93178c17a7c495788fea00c5c380f8301) chore: use Go 1.16 language version
* [`7662d033`](https://github.com/talos-systems/talos/commit/7662d033bfc3d6e3878e2c2a2a1ec4d71dc2502e) fix: talosctl health should not check kube-proxy when it is disabled
* [`0dbaeb9e`](https://github.com/talos-systems/talos/commit/0dbaeb9e655acdc44f8b4db6d1bc6da2ddf6cc9d) chore: update tools, use new generators
* [`e31790f6`](https://github.com/talos-systems/talos/commit/e31790f6f548095fe3f1b9a5c88b47e70c197d2c) fix: properly format spec comments in the resources
* [`78d384eb`](https://github.com/talos-systems/talos/commit/78d384ebb6246cf41a73014312dfb0d86a8008d6) test: update aws cloud provider version
* [`3c5bfbb4`](https://github.com/talos-systems/talos/commit/3c5bfbb4736c86f493a665dbfe63a6e2d20acb3d) fix: don't touch any partitions on upgrade with --preserve
* [`891f90fe`](https://github.com/talos-systems/talos/commit/891f90fee9818f0f013878c0c77c1920e6427a91) chore: update Linux to 5.10.23
* [`d4d77882`](https://github.com/talos-systems/talos/commit/d4d77882e3f53f2449f50f54116a407726f41ede) chore: update dependencies via dependabot
* [`2e22f20b`](https://github.com/talos-systems/talos/commit/2e22f20bd876e4972bfdebd44fee13356b70b83f) docs: minor fixes to getting started
* [`ca8a5596`](https://github.com/talos-systems/talos/commit/ca8a5596c79f638e52601e850236b715f906e3d2) chore: fix provision tests after changes to build-container
* [`4aae924c`](https://github.com/talos-systems/talos/commit/4aae924c685ff578af06a1adceeec4f1938576a6) refactor: provide explicit logger for networkd
* [`22f37530`](https://github.com/talos-systems/talos/commit/22f375300c1cc1d95db540afd510a21b66d7c8a3) chore: update golanci-lint to 1.38.0
* [`83b4e7f7`](https://github.com/talos-systems/talos/commit/83b4e7f744e3a8ed21443642a9afcf5b1342c62b) feat: add Rock pi 4 support
* [`1362966f`](https://github.com/talos-systems/talos/commit/1362966ff546ee620c14e9312255616685743eed) docs: rewrite getting-started for ISO
* [`8e57fc4f`](https://github.com/talos-systems/talos/commit/8e57fc4f526096878213048658bae50cfac4cda8) fix: move containerd CRI config files under `/var/`
* [`6f7df3da`](https://github.com/talos-systems/talos/commit/6f7df3da1e147212e6d4b40a5de65e5ca8be84db) fix: update output of `convert-k8s` command
* [`dce6118c`](https://github.com/talos-systems/talos/commit/dce6118c290afe957e375586b6bbc5b10ef6ba09) docs: add guide for VIP
* [`ee5d9ffa`](https://github.com/talos-systems/talos/commit/ee5d9ffac60c93561874995d8926fc329e2b67dc) chore: bump Go to 1.16.1
* [`7c529e1c`](https://github.com/talos-systems/talos/commit/7c529e1cbd2be66d71e8496304781dd406495bdd) docs: fix links in the documentation
* [`f596c7f6`](https://github.com/talos-systems/talos/commit/f596c7f6be3880be994faf7c5361628024c6be7d) docs: add video for raspberry pi install
* [`47324dca`](https://github.com/talos-systems/talos/commit/47324dcaeaee94e4963eb3764fc01cd2d2d43041) docs: add guide on editing machine configuration
* [`99d5f894`](https://github.com/talos-systems/talos/commit/99d5f894e17f39004e61ee9d5b64d5a8139f33d0) chore: update website npm dependencies
* [`11056a80`](https://github.com/talos-systems/talos/commit/11056a80349e4c8df10a9ea98b6e3d53f96b971c) docs: add highlights for 0.9 release
* [`ae8bedb9`](https://github.com/talos-systems/talos/commit/ae8bedb9a0d999bfbe97b6e18dc2eff62f0fcb80) docs: add control plane conversion guide and 0.9 upgrade notes
* [`ed9673e5`](https://github.com/talos-systems/talos/commit/ed9673e50a7cb973fc49be9c2d659447a4c5bd62) docs: add troubleshooting control plane documentation
* [`485cb126`](https://github.com/talos-systems/talos/commit/485cb1262f97e982ea81597b49d173836c75558d) docs: update Kubernetes upgrade guide
</p>
</details>

### Changes since v0.10.0-alpha.0
<details><summary>0 commit</summary>
<p>

</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`5f89d77`](https://github.com/talos-systems/extras/commit/5f89d77a91f44d52146dae9c23b4654d219042b9) feat: bump Go to 1.16.1
</p>
</details>

### Changes from talos-systems/os-runtime
<details><summary>1 commit</summary>
<p>

* [`7b3d144`](https://github.com/talos-systems/os-runtime/commit/7b3d14457439d4fc10928cd6332c867b4acbae45) feat: use go-yaml fork and serialize spec as RawYAML objects
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>5 commits</summary>
<p>

* [`35f9b6f`](https://github.com/talos-systems/pkgs/commit/35f9b6f22bbe094e93723559132b2a23f0853c2b) feat: update kernel to 5.10.23
* [`dbae83e`](https://github.com/talos-systems/pkgs/commit/dbae83e704da264066ceeca20e0fe66883b542ba) fix: do not use git-lfs for rockpi4 binaries
* [`1c6b9a3`](https://github.com/talos-systems/pkgs/commit/1c6b9a3a6ef91bce4f0cba18c466a9ece7b14750) feat: bump tools for Go 1.16.1
* [`c18073f`](https://github.com/talos-systems/pkgs/commit/c18073fe79b9d7ec36411c6f329fa60c580d4cea) feat: add u-boot for Rock Pi 4
* [`6b85a2b`](https://github.com/talos-systems/pkgs/commit/6b85a2bffbb144f25356eed6ed9dc8bb9a3fd392) feat: upgrade u-boot to 2021.04-rc3
</p>
</details>

### Changes from talos-systems/tools
<details><summary>2 commits</summary>
<p>

* [`bcf3380`](https://github.com/talos-systems/tools/commit/bcf3380dd55810e556851acbe20e20cb4ddd5ef0) feat: bump protobuf deps, add protoc-gen-go-grpc
* [`b49c40e`](https://github.com/talos-systems/tools/commit/b49c40e0ad701f13192c1ad85ec616224343dc3f) feat: bump Go to 1.16.1
</p>
</details>

### Dependency Changes

* **github.com/hashicorp/go-multierror**   v1.1.0 -> v1.1.1
* **github.com/talos-systems/extras**      v0.2.0 -> v0.3.0-alpha.0
* **github.com/talos-systems/os-runtime**  84c3c875eb2b -> 7b3d14457439
* **github.com/talos-systems/pkgs**        v0.4.1 -> v0.5.0-alpha.0-2-g35f9b6f
* **github.com/talos-systems/tools**       v0.4.0 -> v0.5.0-alpha.0-1-gbcf3380

Previous release can be found at [v0.9.0-beta.0](https://github.com/talos-systems/talos/releases/tag/v0.9.0-beta.0)

<a name="v0.9.0-alpha.5"></a>
## [v0.9.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.4...v0.9.0-alpha.5) (2021-03-03)

### Chore

* bump Go module dependencies
* properly propagate context object in the controller

### Feat

* bypass lock if ACPI reboot/shutdown issued
* add `--on-reboot` flag to talosctl edit/patch machineConfig
* support JSON output in `talosctl get`, event types
* rename namespaces, resources, types etc

<a name="v0.9.0-alpha.4"></a>
## [v0.9.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.3...v0.9.0-alpha.4) (2021-03-02)

### Chore

* update provision/upgrade tests to 0.9.0-alpha.3

### Docs

* bump v0.8 release version in the SBCs guides
* add disk encryption guide

### Feat

* update linux kernel to 5.10.19

### Fix

* ignore 'ENOENT' (no such file directory) on mount
* move etcd to `cri` containerd runner

<a name="v0.9.0-alpha.3"></a>
## [v0.9.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.2...v0.9.0-alpha.3) (2021-03-01)

### Chore

* bump dependencies via dependabot
* build both Darwin and Linux versions of talosctl
* bump dependencies via dependabot
* switch CI to stop embedding local registry into the builds

### Docs

* update AMI images for 0.8.4

### Feat

* implement etcd remove-member cli command
* update etcd to 3.4.15
* talosctl: allow v-prefixed k8s versions
* implement simple layer 2 shared IP for CP
* implement talosctl edit and patch config commands
* bump etcd client library to 3.5.0-alpha.0

### Fix

* update in-cluster kubeconfig validity to match other certs
* add ApplyDynamicConfig call in the apply-config --immediate mode
* set hdmi_safe=1 on Raspberry Pi for maximum HDMI compatibility
* show stopped/exited containers via CRI inspector
* make ApplyDynamicConfig idempotent
* improve the drain function
* correctly set service state in the resource
* update the layout of the Disks API to match proxying requirements
* stop and clean up installer container  correctly
* sanitize volume name better in static pod extra volumes

### Refactor

* add context to the networkd
* split WithNetworkConfig into sub-options

### Test

* add integration test with Canal CNI and reset API
* upgrade master to master tests

<a name="v0.9.0-alpha.2"></a>
## [v0.9.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.1...v0.9.0-alpha.2) (2021-02-20)

### Chore

* add default cron pipeline to the list of pipelines
* run default pipeline as part of the `cron` pipeline

### Docs

* add link to GitHub Discussions as a support forum

### Feat

* u-boot 2021.01, ca-certificates update, Linux file ACLs
* support control plane upgrades with Talos managed control plane
* add support for extra volume mounts for control plane pods
* add a warning to boot log if running self-hosted control plane
* add an option to disable kube-proxy manifest
* update Kubernetes to 1.20.4
* add state encryption support

### Fix

* redirect warnings in manifest apply k8s client
* handle case when kubelet serving certificates are issued
* correctly escape extra args in kube-proxy manifest
* skip empty manifest YAML sub-documents

### Refactor

* split kubernetes/etcd resource generation into subresources

### Test

* enable disk encryption key rotation test
* update integration tests to use wrapped client for etcd APIs

<a name="v0.9.0-alpha.1"></a>
## [v0.9.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.0...v0.9.0-alpha.1) (2021-02-09)

### Chore

* update artifacts bucket name in Drone
* rework Drone pipelines
* update dependencies via dependabot
* **ci:** fix schedules in Drone pipelines
* **ci:** update gcp templates

### Docs

* update AMI list for 0.8.2
* fix typos

### Feat

* add a tool and package to convert self-hosted CP to static pods
* implement ephemeral partition encryption
* add resource watch API + CLI
* rename apply-config --no-reboot to --on-reboot
* skip filesystem for state and ephemeral partitions in the installer
* stop all pods before unmounting ephemeral partition
* bump Go to 1.15.8
* support version contract for Talos config generation
* update Linux to 5.10.14
* add an option to force upgrade without checks
* upgrade CoreDNS to 1.8.0
* implement IPv6 DHCP client in networkd

### Fix

* correctly unwrap responses for etcd commands
* drop cri dependency on etcd
* move versions to annotations in control plane static pods
* find master node IPs correctly in health checks
* add 3 seconds grub boot timeout
* don't use filename from URL when downloading manifest
* pass attributes when adding routes
* correct response structure for GenerateConfig API
* correctly extract wrapped error messages
* prevent crash in machined on apid service stop
* wait for time sync before generating Kubernetes certificates
* set proper hostname on docker nodes
* mount kubelet secrets from system instead of ephemeral
* allow loading of empty config files
* prefer configured nameservers, fix DHCP6 in container
* refresh control plane endpoints on worker apids on schedule
* update DHCP client to use Request-Ack sequence after an Offer

### Refactor

* extract go-cmd into a separate library

### Test

* trigger e2e on thrice daily
* update aws templates
* add support for IPv6 in talosctl cluster create

<a name="v0.9.0-alpha.0"></a>
## [v0.9.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.8.1...v0.9.0-alpha.0) (2021-02-01)

### Chore

* bump dependencies (via dependabot)
* fix import path for fsnotify
* add dependabot config
* enable virtio-balloon and monitor in QEMU provisioner
* update protobuf, grpc-go, prototool
* update upgrade test version used

### Docs

* update components.md
* add v0.9 docs
* add modes to validate command
* document omitting DiskPartition size
* update references to 0.8.0, add 0.8.0 AWS AMIs
* fix latest docs
* set latest docs to v0.8
* provide AMIs for 0.8.0-beta.0
* fix SBC docs to point to beta.0 instead of beta.1
* update Talos release for SBCs

### Feat

* move to ECDSA keys for all Kubernetes/etcd certs and keys
* update kernel
* mount hugetlbfs
* allow fqdn to be used when registering k8s node
* copy cryptsetup executable from pkgs
* use multi-arch images for k8s and Flannel CNI
* replace bootkube with Talos-managed control plane
* implement resource API in Talos
* update Linux to 5.10.7, musl-libc to 1.2.2
* update Kubernetes to 1.20.2
* support Wireguard networking
* bump pkgs for kernel with CONFIG_IPV6_MULTIPLE_TABLES
* support type filter in list API and CLI
* add commands to manage/query etcd cluster
* support disk image in talosctl cluster create
* update Kubernetes to 1.20.1

### Fix

* use hugetlbfs instead of none
* use grpc load-balancing when connecting to trustd
* lower memory usage a bit by disabling memory profiling
* don't probe disks in container mode
* prefix rendered Talos-owned static pod manifests
* bump timeout for worker apid waiting for kubelet client config
* kill all processes and umount all disk on reboot/shutdown
* open blockdevices with exclusive flock for partitioning
* list command unlimited recursion default behavior
* pick first interface valid hostname (vs. last one)
* allow 'console' argument in kernel args to be always overridden
* bring up bonded interfaces correctly on packet
* checkpoint controller-manager and scheduler
* correctly transport gRPC errors from apid
* use SetAll instead of AppendAll when building kernel args
* add more dependencies for bootstrap services
* pass disk image flags to e2e-qemu cluster create command
* ignore pods spun up from checkpoints in health checks
* leave etcd for staged upgrades
* ignore errors on stopping/removing pod sandboxes
* use the correct console on Banana Pi M64
* don't run LabelNodeAsMaster in two sequences

### Refactor

* update go-blockdevice and restructure disk interaction code
* define default kernel flags in machinery instead of procfs

### Test

* clear connection refused errors after reset
* skip etcd tests on non-HA clusters


<a name="v0.8.0-alpha.3"></a>
## [v0.8.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.2...v0.8.0-alpha.3) (2020-12-10)

### Chore

* update CONTRIBUTING.md
* limit unit-test run concurrency
* bump Go to 1.15.6
* bump dockerfile frontend version
* fix conform for releases

### Docs

* update Equinix Metal guide
* add architectural doc on the root file system layout
* add a note on caveats in container mode
* add storage doc
* add guide for custom CAs
* add docs for network connectivity
* improve SBC documentation

### Feat

* update kernel to 5.9.13, new KSPP requirements
* reset with system disk wipe spec
* add talosctl merge config command
* add talosctl config contexts
* update Kubernetes to 1.20.0
* implement "staged" (failsafe/backup) upgrades
* allow disabling NoSchedule taint on masters using TUI installer

### Fix

* remove kmsg ratelimiting on startup
* zero out partitions without filesystems on install
* make interactive installer work without endpoints provided

### Test

* add ISO test
* add support for mounting ISO in talosctl cluster create
* bump Talos release version for upgrade test to 0.7.1
* bump defaults for provision tests resources


<a name="v0.8.0-alpha.2"></a>
## [v0.8.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.1...v0.8.0-alpha.2) (2020-12-04)

### Chore

* publish Rock64 image
* enable thrice daily pipeline
* run integration test thrice daily
* output SBC images as compressed raw images
* build SBC images
* update module dependencies
* drop support for `docker load`
* fix metal image name
* use IMAGE_TAG instead of TAG for :latest pushes

### Docs

* fix typos
* add openstack docs
* ensure port for vbox and proxmox docs
* add console kernel arg to rpi_4 image generation
* add console kernel arg to libretech_all_h3_cc_h5 image generation

### Feat

* add support for the Pine64 Rock64
* add TUI for configuring network interfaces settings
* make GenerateConfiguration accept current time as a parameter
* introduce configpatcher package in machinery
* suggest fixed control plane endpoints in talosctl gen config
* update kubernetes to 1.20.0-rc.0
* allow boards to set kernel args
* add support for the Banana Pi M64
* stop including K8s version by default in `talosctl gen config`
* add support for the Raspberry Pi 4 Model B
* implement network interfaces list API
* bump package for kernel with CIFS support
* upgrade etcd to 3.4.14
* update Containerd and Linux
* add support for installing to SBCs
* add ability to choose CNI config

### Fix

* make default generate image arch dynamic based on arch
* stabilize serial console on RPi4, add video console
* make reset work again
* node taint doesn't contain value anymore
* defer resolving config context in client code
* remove value (change to empty) for `NoSchedule` taint
* prevent endless loop with DHCP requests in networkd
* skip `board` argument to the installer if it's not set
* use the dtb from kernel pkg for libretech_all_h3_cc_h5
* prevent crash in `talosctl config` commands
* update generated .ova manifest for raw disk size
* **security:** update Containerd to v1.4.3

### Release

* **v0.8.0-alpha.2:** prepare release


<a name="v0.8.0-alpha.1"></a>
## [v0.8.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.0...v0.8.0-alpha.1) (2020-11-26)

### Chore

* add cloud image uploader (AWS AMIs for now)
* bump K8s to 1.19.4 in e2e scripts with CABPT version
* build arm64 images in CI
* remove maintenance service interface and use machine service

### Docs

* provide list of AMIs on AWS documentation page
* add 0.8 docs for the upcoming release
* ensure we configure nodes in guides
* ensure gcp docs have firewall and node info
* add qemu diagram and video walkthrough
* graduate v0.7 docs
* improve configuration reference documentation
* fix small typo in talosctl processes cast
* update asciinemas with talosctl
* add proxmox doc
* add live walkthroughs where applicable

### Feat

* support openstack platform
* update Kubernetes to v1.20.0-beta.2
* change UI component for disks selector
* support cluster expansion in the interactive installer
* implement apply configuration without reboot
* make GenerateConfiguration API reuse current node auth
* sync time before installer runs
* set interface MTU in DHCP mode even if DHCP is not successful
* print hint about using interative installer in mainenance mode
* add TUI based talos interactive installer
* support ipv6 routes
* return client config as the second value in GenerateConfiguration
* correctly merge talosconfig (don't ever overwrite)
* drop to maintenance mode in cloud platforms if userdata is missing
* read config from extra guestinfo key (vmware)
* update Go to 1.15.5
* add generate config gRPC API
* upgrade Kubernetes default version to 1.19.4
* add example command in maintenance, enforce cert fingerprint
* add storage API

### Fix

* bump blockdevice library for `mmcblk` part name fix
* ignore 'not found' errors when stopping/removing CRI pods
* return hostname from packet platform
* make fingerprint clearly optional in a boot hint
* ensure packet nics get all IPs
* use ghcr.io/talos-systems/kubelet
* bump timeout for config downloading on bare metal

### Refactor

* drop osd compatibility layer

### Release

* **v0.8.0-alpha.1:** prepare release

### Test

* update integration test versions, clean up names
