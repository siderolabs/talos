## [Talos 0.14.0-alpha.2](https://github.com/talos-systems/talos/releases/tag/v0.14.0-alpha.2) (2021-11-30)

Welcome to the v0.14.0-alpha.2 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Kexec and capabilities

When kexec support is disabled
Talos no longer drops Linux capabilities (`CAP_SYS_BOOT` and `CAP_SYS_MODULES`) for child processes.
That is helpful for advanced use-cases like Docker-in-Docker.

If you want to permanently disable kexec and capabilities dropping, pass `kexec_load_disabled=1` argument to the kernel.

For example:

```yaml
install:
  extraKernelArgs:
    - sysctl.kernel.kexec_load_disabled=1
```

Please note that capabilities are dropped before machine configuration is loaded,
so disabling kexec via `machine.sysctls` will not be enough.


### Cluster Discovery

[Cluster Discovery](https://www.talos.dev/docs/v0.14/guides/discovery/) is enabled by default for Talos 0.14.
Cluster Discovery can be disabled with `talosctl gen config --with-cluster-discovery=false`.


### Kubelet

Kubelet service can now be restarted with `talosctl service kubelet restart`.

Kubelet node IP configuration (`.machine.kubelet.nodeIP.validSubnets`) can now include negative subnet matches (prefixed with `!`).


### Log Shipping

Talos can now [ship system logs](https://www.talos.dev/docs/v0.14/guides/logging/)
to the configured destination using either JSON-over-UDP or JSON-over-TCP:
see `.machine.logging` machine configuration option.


### NTP Sync

Talos NTP sync process was improved to align better with kernel time adjustment periods and to filter out spikes.


### SideroLink

A set of Talos ehancements is going to unlock a number of exciting features in the upcoming release of [Sidero](https://www.sidero.dev/):

* `SideroLink`: a point-to-point Wireguard tunnel connecting Talos node back to the provisioning platform (Sidero).
* event sink (kernel arg `talos.event.sink=http://10.0.0.1:4000`) delivers Talos internal events to the specified destination.
* kmsg log delivery (kernel arg `talos.logging.kernel=tcp://10.0.0.1:4001`) sends kernel logs as JSON lines over TCP or UDP.


### `talosctl support`

`talosctl` CLI tool now has a new subcommand called `support`, that can gather all
cluster information that could help with future debugging in a single run.

Output of the command is a `zip` archive with all talos service logs, kubernetes pod logs and manifests,
talos resources manifests and so on.
Generated archive does not contain any secret information so it is safe to send it for analysis to a third party.


### Component Updates

* Linux: 5.15.5
* etcd: 3.5.1
* containerd: 1.5.8
* Kubernetes: 1.23.0-rc.0
* CoreDNS: 1.8.6

Talos is built with Go 1.17.3


### Kubernetes Upgrade Enhancements

`talosctl upgrade-k8s` now syncs all Talos manifest resources generated from templates.

So there is no need to update CoreDNS, Flannel container manually after running `upgrade-k8s` anymore.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Serge Logvinov
* Noel Georgi
* Nico Berlee
* Spencer Smith
* Alex Zero
* Andrew Rynhard
* Branden Cash
* David Haines
* Gerard de Leeuw
* Michael Fornaro
* Rui Lopes

### Changes
<details><summary>136 commits</summary>
<p>

* [`e9f4b7b2`](https://github.com/talos-systems/talos/commit/e9f4b7b2041223309467227fa8b99cf35b797c72) feat: update Linux to 5.15.5
* [`4d0a75a3`](https://github.com/talos-systems/talos/commit/4d0a75a3f0795d5a0537c3b59007f97423c072ab) docs: add documentation about logging
* [`8d1cbeef`](https://github.com/talos-systems/talos/commit/8d1cbeef9f2ae95d04035f5d999aa181fb88e9fc) chore: add API breaking changes detector
* [`ed7fb9db`](https://github.com/talos-systems/talos/commit/ed7fb9db14554ccc191cc0c989aba38021a59690) feat: move kubelet proccesses to /podruntime cgroup
* [`2cd3f9be`](https://github.com/talos-systems/talos/commit/2cd3f9be1f36dd3389ee528fa8f0b2548032c2f7) feat: filter out SideroLink addresses by default
* [`0f169bf9`](https://github.com/talos-systems/talos/commit/0f169bf9b15239bfd35f371832211c42caf4349c) chore: add API deprecations mechanism
* [`eaf6d472`](https://github.com/talos-systems/talos/commit/eaf6d4720383881c0dcf967dbc4e960d5ef49dd8) refactor: use random port listener in kernel log delivery tests
* [`bf4c81e7`](https://github.com/talos-systems/talos/commit/bf4c81e7da854b7e9491f4ecb6fce89b026f4a9f) feat: kernel log (kmsg) delivery controller
* [`f3149780`](https://github.com/talos-systems/talos/commit/f3149780e6663f7dc0fd0091cd6e3df605eac848) feat: update Kubernetes to 1.23.0-rc.0
* [`b824909d`](https://github.com/talos-systems/talos/commit/b824909d686b1f5a8cd20afe9ca5a4f291a6f12d) fix: disable kexec on RPi4
* [`3257751b`](https://github.com/talos-systems/talos/commit/3257751bc0a18e0d3bb7097191989440ae473ee6) fix: initialize Drainer properly
* [`e4bc68bf`](https://github.com/talos-systems/talos/commit/e4bc68bf026966a3326872a1d342ef3b9c05cc9d) fix: leave only a single IPv4/IPv6 address as kubelet's node IP
* [`e6d00741`](https://github.com/talos-systems/talos/commit/e6d007418efeb5d7f82eb82a35cddacc64ec99ba) feat: update pkgs - Linux 5.15.4, LibreSSL 3.2.7
* [`d5cbc364`](https://github.com/talos-systems/talos/commit/d5cbc3640256090e354b3896ffea72b8e58874bb) feat: add GCP ccm
* [`7433150f`](https://github.com/talos-systems/talos/commit/7433150fd84ef0935e1aad91ca654892dc522806) feat: implement events sink controller
* [`b4a406ae`](https://github.com/talos-systems/talos/commit/b4a406ae7c72e30ba488493682045495cd31dc4e) test: pin cluster API templates version to tag v1alpha4
* [`9427e78d`](https://github.com/talos-systems/talos/commit/9427e78dc6d581e752bf41a20f1e0379cc99d92d) fix: catch panics in network operator runs
* [`d1f55f90`](https://github.com/talos-systems/talos/commit/d1f55f90128859d41ada63159d6b2d12e83fabac) fix: update blockdevice library to properly handle absent GPT
* [`5ac64b2d`](https://github.com/talos-systems/talos/commit/5ac64b2d97c6e013c83a6618c6bece2e70dedd98) chore: set version in unit-tests
* [`20d39c0b`](https://github.com/talos-systems/talos/commit/20d39c0b48b64f237270e13df7f277abd262d10b) chore: format .proto files
* [`852bf4a7`](https://github.com/talos-systems/talos/commit/852bf4a7de815b75e2e632de18fae30bd1bc22be) feat: talosctl fish completion support
* [`6bb75150`](https://github.com/talos-systems/talos/commit/6bb75150a394ee1ef4a3677ab4d8e73f27172209) fix: allow add_key and request_key in kubelet seccomp profile
* [`6487b21f`](https://github.com/talos-systems/talos/commit/6487b21feb12291419c6fd1f6635a051b0a60afc) feat: update pkgs for u-boot, containerd, etc
* [`f7d1e777`](https://github.com/talos-systems/talos/commit/f7d1e7776917475507aa99847f88b9c22c9f7b95) feat: provide SideroLink client implementation
* [`58892cd6`](https://github.com/talos-systems/talos/commit/58892cd697676c19f830f55e8ba1d84cd6000621) fix: unblock events watch on context cancel
* [`caa76be2`](https://github.com/talos-systems/talos/commit/caa76be2c982d9d6bc8d3103f16b5915796f76b1) fix: containerd failed to load plugin
* [`1ffa8e04`](https://github.com/talos-systems/talos/commit/1ffa8e0480084264eee551ad177b2443ddb02ead) feat: add ULA prefix for SideroLink
* [`c6a67b86`](https://github.com/talos-systems/talos/commit/c6a67b8662bb3c6efbe912b19699ace19e70dd3f) fix: ignore not existing nodes on cordoning
* [`f7302525`](https://github.com/talos-systems/talos/commit/f730252579879df2e95878de292f17f791740804) feat: add new event types
* [`7c9b082f`](https://github.com/talos-systems/talos/commit/7c9b082f74f26349a0e309d9818d5bc55e672378) feat: update Kubernetes to 1.23.0-beta.0
* [`750e31c4`](https://github.com/talos-systems/talos/commit/750e31c4a46f2835eca9fc9a085d2bb64e582e40) fix: ignore EBUSY from `kexec_file_load`
* [`2d11b595`](https://github.com/talos-systems/talos/commit/2d11b59558c98f4cd07a50b25be29b5c355a4495) fix: ignore virtual IP as kubelet node IPs
* [`030fd349`](https://github.com/talos-systems/talos/commit/030fd349b1c0669d7059f8c6883c85096f6f9ef5) fix: don't run kexec prepare on shutdown and reset
* [`6dcce20e`](https://github.com/talos-systems/talos/commit/6dcce20e6fa088c3063aab728912731f5e827eb7) test: set proper pod CIDR for Cilium tests
* [`695300da`](https://github.com/talos-systems/talos/commit/695300dac46c114b8e7e40abdaeece25f7079c88) release(v0.14.0-alpha.1): prepare release
* [`753a8218`](https://github.com/talos-systems/talos/commit/753a82188f227da4f2f40da5f4d46ebe45774455) refactor: move pkg/resources to machinery
* [`0102a64a`](https://github.com/talos-systems/talos/commit/0102a64a5f6de2c3fe5d7792c2c5845fc737edff) refactor: remove pkg/resources dependencies on wgtypes, netx
* [`7462733b`](https://github.com/talos-systems/talos/commit/7462733bcb075b923b8c7ba4a763308c641c49a2) chore: update golangci-lint
* [`032c99a0`](https://github.com/talos-systems/talos/commit/032c99a0300ccb09105a07434884d2b1f57e537d) refactor: remove pkg/resources dependencies on k8s and base62
* [`4a5cff45`](https://github.com/talos-systems/talos/commit/4a5cff45f397ac29b7bfc390f11691c32d8615b2) perf: raspberry PIs clockspeed as fast as firmware allows
* [`a76f6d69`](https://github.com/talos-systems/talos/commit/a76f6d69dbfdf34e4383dd5d2ee9f8cca4661e87) feat: allow kubelet to be restarted and provide negative nodeIP subnets
* [`189221d5`](https://github.com/talos-systems/talos/commit/189221d589c1c9d4fc012dd9e31fd6d142d88dde) chore: update dependencies
* [`41f0aecc`](https://github.com/talos-systems/talos/commit/41f0aecc1d3c4afce96d034f160fa9f120c67e85) docs: update partition info
* [`95105071`](https://github.com/talos-systems/talos/commit/95105071de29f70552bd7c0881c2cc2e7c78c0ac) chore: fix simple issues found by golangci-lint
* [`d4b0ca21`](https://github.com/talos-systems/talos/commit/d4b0ca21a1ee1183b28738bb3d9ca251e1968fe7) test: retry upgrade mutex lock failures
* [`4357e9a8`](https://github.com/talos-systems/talos/commit/4357e9a849fcb7fb66378bdd767a926dde0c4318) docs: add Talos partions info
* [`8e8687d7`](https://github.com/talos-systems/talos/commit/8e8687d7592d4bc071981478491d70489e7dd4a9) fix: use temporary sonobuoy version
* [`e4e8e873`](https://github.com/talos-systems/talos/commit/e4e8e8737f564be47098e284706a63ef84636890) test: disable e2e-misc test with Canal CNI
* [`897da2f6`](https://github.com/talos-systems/talos/commit/897da2f6efc571a66d14722a67bbc401bad31887) docs: common typos
* [`a50483dd`](https://github.com/talos-systems/talos/commit/a50483dddfd9a742b998f509ee713af996a2484e) feat: update Linux to 5.15.1
* [`a2233bfe`](https://github.com/talos-systems/talos/commit/a2233bfe46bfb55d71cfc07174f6f22aee6d2651) fix: improve NTP sync process
* [`7efc1238`](https://github.com/talos-systems/talos/commit/7efc1238ee285d55c4619b6a40190b54ff953a66) fix: parse partition size correctly
* [`d6147eb1`](https://github.com/talos-systems/talos/commit/d6147eb17d2ebf263ca0537068bbbba6d3ced061) chore: update sonobuoy
* [`efbae785`](https://github.com/talos-systems/talos/commit/efbae7857d09aa7e5e704d5989efced5aa655259) fix: use etc folder for du cli tests
* [`198eea51`](https://github.com/talos-systems/talos/commit/198eea51a81bf041470c3c88cb6cb97af3a4e203) fix: wait for follow reader to start before writing to the file
* [`e7f715eb`](https://github.com/talos-systems/talos/commit/e7f715eb0ca0587a05949910cafdeb486654b577) chore: log KubeSpan IPs overlaps
* [`82a1ad16`](https://github.com/talos-systems/talos/commit/82a1ad1681bf262dcc68fc9cbac71ff2eb5639af) chore: bump dependencies
* [`e8fccbf5`](https://github.com/talos-systems/talos/commit/e8fccbf5351ec2481813553181cb73b8f16c915a) fix: clear time adjustment error when setting time to specific value
* [`e6f90bb4`](https://github.com/talos-systems/talos/commit/e6f90bb41a757b5173bbbf7554b6f85c08aaf58e) chore: remove unused parameters
* [`785161d1`](https://github.com/talos-systems/talos/commit/785161d19f68fb64451cf3d887b67f85a8bcb952) feat: update k8s to 1.23.0-alpha.4
* [`fe228d7c`](https://github.com/talos-systems/talos/commit/fe228d7c85a1f8437398061b18c090962adc9f29) fix: do not use yaml.v2 in the support cmd
* [`9b48ca21`](https://github.com/talos-systems/talos/commit/9b48ca21731cce53f0a61f05f74dcd264417d784) fix: endpoints and nodes in generated talosconfig
* [`6e16fd2f`](https://github.com/talos-systems/talos/commit/6e16fd2feeb3f8bf0b99e6cbe21047b7a5c1f05c) chore: update tools, pkgs, and extras
* [`261c497c`](https://github.com/talos-systems/talos/commit/261c497c71eb5ab5197bef05d8c209dbeb770d3f) feat: implement `talosctl support` command
* [`fc7dc454`](https://github.com/talos-systems/talos/commit/fc7dc454840e100d82bb036a7f065293234593f7) chore: check our API idiosyncrasies
* [`b1584429`](https://github.com/talos-systems/talos/commit/b15844298a6bfedca5acc0cc27061666481eb94b) feat: use GCP deployment manager
* [`3e7d4df9`](https://github.com/talos-systems/talos/commit/3e7d4df99019e3cc6d9a90920d377c73a76ac577) chore: bump dependencies
* [`88f24229`](https://github.com/talos-systems/talos/commit/88f2422955690d1eca1e21cd60a35e1d49141e3d) refactor: get rid of prometheus/procfs dependency in pkg/resources
* [`dd196d30`](https://github.com/talos-systems/talos/commit/dd196d3006d29ae5cae5d43b648da1ca2e5af236) refactor: prepare for move of pkg/resources to machinery
* [`f6110f80`](https://github.com/talos-systems/talos/commit/f6110f8036bc176188abb583bfa51296c4d3897d) fix: remove listening socket to fix Talos in a container restart
* [`53bbb13e`](https://github.com/talos-systems/talos/commit/53bbb13ed8592978dc27578fa79b3a2018941427) docs: update docs with emmc boot guide
* [`8329d211`](https://github.com/talos-systems/talos/commit/8329d21114abf841788be64765378343c12eaf69) chore: split polymorphic RootSecret resource into specific types
* [`c97becdd`](https://github.com/talos-systems/talos/commit/c97becdd9548d85b2b894a05765f93dcdf9ad803) chore: remove interfaces and routes APIs
* [`d798635d`](https://github.com/talos-systems/talos/commit/d798635d993a21392b8a7972a689c4be0728db32) feat: automatically limit kubelet node IP family based on service CIDRs
* [`205a8d6d`](https://github.com/talos-systems/talos/commit/205a8d6dc495e25af87bf0b920d0f55b8a27bbfd) chore: make nethelpers build on all OSes
* [`5b5dd49f`](https://github.com/talos-systems/talos/commit/5b5dd49f64bef584000655687e5b9c5d25af6a93) feat: extract JSON fields from more log messages
* [`eb4f1182`](https://github.com/talos-systems/talos/commit/eb4f11822dc0b35541e0576a75ca263ca96d4981) docs: create cluster in hetzner cloud
* [`728164e2`](https://github.com/talos-systems/talos/commit/728164e25a5705ae5194b416941f3607d592b140) docs: fix kexec_load_disabled param name in release notes
* [`f6328f09`](https://github.com/talos-systems/talos/commit/f6328f09a2bf8d233a48354dd548fb740e509341) fix: fix filename typo
* [`01b0f0ab`](https://github.com/talos-systems/talos/commit/01b0f0abb341b387f16d9b3a142af742f36c8c2b) release(v0.14.0-alpha.0): prepare release
* [`8b620653`](https://github.com/talos-systems/talos/commit/8b6206537a30be049f74f8c4c7350028e6e56c74) fix: skip generating empty `.machine.logging`
* [`60ad0063`](https://github.com/talos-systems/talos/commit/60ad006367e73f56fd69726e0044f1ce48f18a8b) fix: don't drop ability to use ambient capabilities
* [`b6b78e7f`](https://github.com/talos-systems/talos/commit/b6b78e7fef3f6ef0c566e1815d1e28f16f868c93) test: add cluster discovery integration tests
* [`97d64d16`](https://github.com/talos-systems/talos/commit/97d64d160ce7e71c3107adbd31404853f543f7cc) fix: hcloud network config changes
* [`4c76865d`](https://github.com/talos-systems/talos/commit/4c76865d0ecec726e801a4b8f87e09476481d808) feat: multiple logging improvements
* [`1d1e1df6`](https://github.com/talos-systems/talos/commit/1d1e1df643832478aaa715aea5f51ad2e61e2880) fix: handle skipped mounts correctly
* [`0a964d92`](https://github.com/talos-systems/talos/commit/0a964d921922a247293e36b5fecaab466b91d924) test: fix openstack unit-test stability
* [`72f62ac2`](https://github.com/talos-systems/talos/commit/72f62ac27b5d0a72db409fd003a7cf9c41a03d7c) chore: bump Go and Docker dependencies
* [`9c48ebe8`](https://github.com/talos-systems/talos/commit/9c48ebe8f94afa85921ee5f1c1e9315201905a92) fix: gcp fetching externalIP
* [`6c297268`](https://github.com/talos-systems/talos/commit/6c297268ce596c2a875b7c419c85317dc24d9f4f) test: fix e2e k8s version
* [`ae5af9d3`](https://github.com/talos-systems/talos/commit/ae5af9d3fad399dea95c316d94e3e66b124bfb24) feat: update Kubernetes to 1.23.0-alpha.3
* [`28d3a69e`](https://github.com/talos-systems/talos/commit/28d3a69e9d4ae7ffa231804e26af6d1f39c07afd) feat: openstack config-drive support
* [`2258bc49`](https://github.com/talos-systems/talos/commit/2258bc4918e89b3d6fcb841b2ad677f114ddba7e) test: update GCP e2e script to work with new templates
* [`36b6ace2`](https://github.com/talos-systems/talos/commit/36b6ace25378e8c4a607de6efb6b89a2d52f5cea) feat: update Linux to 5.10.75
* [`38516a54`](https://github.com/talos-systems/talos/commit/38516a5499d933a8038ce6768946ff096e7c6f98) test: update Talos versions in upgrade tests
* [`cff20ec7`](https://github.com/talos-systems/talos/commit/cff20ec78340b3855751e13f2ad0e54bd47e9989) fix: change services OOM score
* [`666a2b62`](https://github.com/talos-systems/talos/commit/666a2b6207d257edda20c9e0411b0d4cd4112aa6) feat: azure platform ipv6 support
* [`d32814e3`](https://github.com/talos-systems/talos/commit/d32814e302c370ec1e82aa2879186a034cd2a905) feat: extract JSON fields from log lines
* [`e77d81ff`](https://github.com/talos-systems/talos/commit/e77d81fff31d68f762da3741846f95a6d2303903) fix: treat literal 'unknown' as a valid machine type
* [`c8e404e3`](https://github.com/talos-systems/talos/commit/c8e404e356878f6cd819a33386b351c1c152c3f5) test: update vars for AWS cluster
* [`ad23891b`](https://github.com/talos-systems/talos/commit/ad23891b1f6b33409721528c6771304b7ab94b2c) feat: update CoreDNS version 1.8.6
* [`41299cae`](https://github.com/talos-systems/talos/commit/41299cae9961665c2bf2a642290f8309683f040d) feat: udev rules support
* [`5237fdc9`](https://github.com/talos-systems/talos/commit/5237fdc957efbb018649b866bfb756f280f589a2) feat: send JSON logs over UDP
* [`6d44587a`](https://github.com/talos-systems/talos/commit/6d44587a4d4c16defa6bb06329cdfc6e39c95188) feat: coredns service dualstack
* [`12f7888b`](https://github.com/talos-systems/talos/commit/12f7888b75fa2498e0f8305f5d6910cecad5c65c) feat: feed control plane endpoints on workers from cluster discovery
* [`431e4fb4`](https://github.com/talos-systems/talos/commit/431e4fb4b690fa4955c407d8dd8156bdecd9a2c5) chore: bump Go and Docker dependencies
* [`89f3b9f8`](https://github.com/talos-systems/talos/commit/89f3b9f8d41e33c4cb736917f418ab5cfb9edd83) feat: update etcd to 3.5.1
* [`e60469a3`](https://github.com/talos-systems/talos/commit/e60469a38cb81ace2039bae1927eb6c5f1f0ad1f) feat: initial support for JSON logging
* [`68c420e3`](https://github.com/talos-systems/talos/commit/68c420e3c96a0fdc3b3e6cd75be24cc797c48e09) feat: enable cluster discovery by default
* [`3e100aa9`](https://github.com/talos-systems/talos/commit/3e100aa97734ea809563e23fc36e19bdd3df1920) test: workaround EventsWatch test flakiness
* [`9bd4838a`](https://github.com/talos-systems/talos/commit/9bd4838ac10abbd4760da4fb905d7639a1c26f9f) chore: stop using sonobuoy CLI
* [`6ad45951`](https://github.com/talos-systems/talos/commit/6ad45951975aac48fdcc282e5a0e31344058d07e) docs: fix field names for bonding configuration
* [`d7a3b7b5`](https://github.com/talos-systems/talos/commit/d7a3b7b5b70293884d2e19c6a59b14ebcfa24397) chore: use discovery-client and discovery-api modules
* [`d6309eed`](https://github.com/talos-systems/talos/commit/d6309eed6618abd1b4efd0e3cd18a6c0df39378f) docs: create docs for Talos 0.14
* [`c0fda643`](https://github.com/talos-systems/talos/commit/c0fda6436ae27d8bbc210ee74a1128968108f6a6) fix: attempt to clean up tasks in containerd runner
* [`8cf442da`](https://github.com/talos-systems/talos/commit/8cf442daa60d911caff59d1c2c05dd77652c8b51) chore: bump tools, pkgs, extras
* [`0dad5f4d`](https://github.com/talos-systems/talos/commit/0dad5f4d7846f3fb41ff4ba27395023d33796a61) chore: small cleanup
* [`e3e2113a`](https://github.com/talos-systems/talos/commit/e3e2113adc058940725b1041827d7adb8895c6cf) feat: upgrade CoreDNS during `upgrade-k8s` call
* [`d92c98e1`](https://github.com/talos-systems/talos/commit/d92c98e19a054472bff3e0d646756f16c5e65bbf) docs: fix discovery service documentation link
* [`e44b11c5`](https://github.com/talos-systems/talos/commit/e44b11c595e4cab796128a932843b90734ff6d1d) feat: update containerd to 1.5.7, bump Go dependencies
* [`24129307`](https://github.com/talos-systems/talos/commit/24129307a14d6e59c6bc0d3586c0c95969bde679) docs: make Talos 0.13 docs latest, update documentation
* [`31b6e39e`](https://github.com/talos-systems/talos/commit/31b6e39e58a27e1f2c1be500fca8636971bfa5c6) fix: delete expired affiliates from the discovery service
* [`877a2b6f`](https://github.com/talos-systems/talos/commit/877a2b6fc00eaa7574349f9086d78c04df163840) test: bump CAPI components to v1alpha4
* [`2ba0e0ac`](https://github.com/talos-systems/talos/commit/2ba0e0ac4ad460409101f5f2374e66698adbba4c) docs: add KubeSpan documentation
* [`997873b6`](https://github.com/talos-systems/talos/commit/997873b6d3116b59ebb46df66b8aa1cee06df92f) fix: use ECDSA-SHA512 when generating certs for Talos < 0.13
* [`7137166d`](https://github.com/talos-systems/talos/commit/7137166d1d5817e2d44ead4a01796275f92a9d4a) fix: allow overriding `audit-policy-file` in `kube-apiserver` static pod
* [`8fcd4219`](https://github.com/talos-systems/talos/commit/8fcd4219671a9359880ba344a2ec7fd65dfe5e2a) chore: fix integration-qemu-race
* [`91a858b5`](https://github.com/talos-systems/talos/commit/91a858b53704ede86392fe3c155ce9ab3c2d406f) fix: sort output of the argument builder
* [`657f7a56`](https://github.com/talos-systems/talos/commit/657f7a56b10089e0dc551e178bc85b28d8003243) fix: use ECDSA-SHA256 signature algorithm for Kubernetes certs
* [`983d2459`](https://github.com/talos-systems/talos/commit/983d2459e2aa036774828f773bbaba5697665ae7) feat: suppress logging NTP sync to the console
* [`022c7335`](https://github.com/talos-systems/talos/commit/022c7335f3063675ab744454a2ad4b2c0c19bfbc) fix: add interface route if DHCP4 router is not directly routeable
* [`66a1579e`](https://github.com/talos-systems/talos/commit/66a1579ea7d2a9c4fdf15b762cd024c54b3e8ffb) fix: don't enable 'no new privs' on the system level
* [`423861cf`](https://github.com/talos-systems/talos/commit/423861cf9f99eaf034a4f0cb243d73d1275c3f38) feat: don't drop capabilities if kexec is disabled
* [`facc8c38`](https://github.com/talos-systems/talos/commit/facc8c38a021610da900a45f397aea8ddfc74f1c) docs: fix documentation for cluster discovery
* [`ce65ca4e`](https://github.com/talos-systems/talos/commit/ce65ca4e4a2994f901f01ce5ca269d6df86f0de8) chore: build using only amd64 builders
* [`e9b0f010`](https://github.com/talos-systems/talos/commit/e9b0f010d2855b968a5d8b8b5fbcd268e06ba302) chore: update docker image in the pipeline
</p>
</details>

### Changes since v0.14.0-alpha.1
<details><summary>34 commits</summary>
<p>

* [`e9f4b7b2`](https://github.com/talos-systems/talos/commit/e9f4b7b2041223309467227fa8b99cf35b797c72) feat: update Linux to 5.15.5
* [`4d0a75a3`](https://github.com/talos-systems/talos/commit/4d0a75a3f0795d5a0537c3b59007f97423c072ab) docs: add documentation about logging
* [`8d1cbeef`](https://github.com/talos-systems/talos/commit/8d1cbeef9f2ae95d04035f5d999aa181fb88e9fc) chore: add API breaking changes detector
* [`ed7fb9db`](https://github.com/talos-systems/talos/commit/ed7fb9db14554ccc191cc0c989aba38021a59690) feat: move kubelet proccesses to /podruntime cgroup
* [`2cd3f9be`](https://github.com/talos-systems/talos/commit/2cd3f9be1f36dd3389ee528fa8f0b2548032c2f7) feat: filter out SideroLink addresses by default
* [`0f169bf9`](https://github.com/talos-systems/talos/commit/0f169bf9b15239bfd35f371832211c42caf4349c) chore: add API deprecations mechanism
* [`eaf6d472`](https://github.com/talos-systems/talos/commit/eaf6d4720383881c0dcf967dbc4e960d5ef49dd8) refactor: use random port listener in kernel log delivery tests
* [`bf4c81e7`](https://github.com/talos-systems/talos/commit/bf4c81e7da854b7e9491f4ecb6fce89b026f4a9f) feat: kernel log (kmsg) delivery controller
* [`f3149780`](https://github.com/talos-systems/talos/commit/f3149780e6663f7dc0fd0091cd6e3df605eac848) feat: update Kubernetes to 1.23.0-rc.0
* [`b824909d`](https://github.com/talos-systems/talos/commit/b824909d686b1f5a8cd20afe9ca5a4f291a6f12d) fix: disable kexec on RPi4
* [`3257751b`](https://github.com/talos-systems/talos/commit/3257751bc0a18e0d3bb7097191989440ae473ee6) fix: initialize Drainer properly
* [`e4bc68bf`](https://github.com/talos-systems/talos/commit/e4bc68bf026966a3326872a1d342ef3b9c05cc9d) fix: leave only a single IPv4/IPv6 address as kubelet's node IP
* [`e6d00741`](https://github.com/talos-systems/talos/commit/e6d007418efeb5d7f82eb82a35cddacc64ec99ba) feat: update pkgs - Linux 5.15.4, LibreSSL 3.2.7
* [`d5cbc364`](https://github.com/talos-systems/talos/commit/d5cbc3640256090e354b3896ffea72b8e58874bb) feat: add GCP ccm
* [`7433150f`](https://github.com/talos-systems/talos/commit/7433150fd84ef0935e1aad91ca654892dc522806) feat: implement events sink controller
* [`b4a406ae`](https://github.com/talos-systems/talos/commit/b4a406ae7c72e30ba488493682045495cd31dc4e) test: pin cluster API templates version to tag v1alpha4
* [`9427e78d`](https://github.com/talos-systems/talos/commit/9427e78dc6d581e752bf41a20f1e0379cc99d92d) fix: catch panics in network operator runs
* [`d1f55f90`](https://github.com/talos-systems/talos/commit/d1f55f90128859d41ada63159d6b2d12e83fabac) fix: update blockdevice library to properly handle absent GPT
* [`5ac64b2d`](https://github.com/talos-systems/talos/commit/5ac64b2d97c6e013c83a6618c6bece2e70dedd98) chore: set version in unit-tests
* [`20d39c0b`](https://github.com/talos-systems/talos/commit/20d39c0b48b64f237270e13df7f277abd262d10b) chore: format .proto files
* [`852bf4a7`](https://github.com/talos-systems/talos/commit/852bf4a7de815b75e2e632de18fae30bd1bc22be) feat: talosctl fish completion support
* [`6bb75150`](https://github.com/talos-systems/talos/commit/6bb75150a394ee1ef4a3677ab4d8e73f27172209) fix: allow add_key and request_key in kubelet seccomp profile
* [`6487b21f`](https://github.com/talos-systems/talos/commit/6487b21feb12291419c6fd1f6635a051b0a60afc) feat: update pkgs for u-boot, containerd, etc
* [`f7d1e777`](https://github.com/talos-systems/talos/commit/f7d1e7776917475507aa99847f88b9c22c9f7b95) feat: provide SideroLink client implementation
* [`58892cd6`](https://github.com/talos-systems/talos/commit/58892cd697676c19f830f55e8ba1d84cd6000621) fix: unblock events watch on context cancel
* [`caa76be2`](https://github.com/talos-systems/talos/commit/caa76be2c982d9d6bc8d3103f16b5915796f76b1) fix: containerd failed to load plugin
* [`1ffa8e04`](https://github.com/talos-systems/talos/commit/1ffa8e0480084264eee551ad177b2443ddb02ead) feat: add ULA prefix for SideroLink
* [`c6a67b86`](https://github.com/talos-systems/talos/commit/c6a67b8662bb3c6efbe912b19699ace19e70dd3f) fix: ignore not existing nodes on cordoning
* [`f7302525`](https://github.com/talos-systems/talos/commit/f730252579879df2e95878de292f17f791740804) feat: add new event types
* [`7c9b082f`](https://github.com/talos-systems/talos/commit/7c9b082f74f26349a0e309d9818d5bc55e672378) feat: update Kubernetes to 1.23.0-beta.0
* [`750e31c4`](https://github.com/talos-systems/talos/commit/750e31c4a46f2835eca9fc9a085d2bb64e582e40) fix: ignore EBUSY from `kexec_file_load`
* [`2d11b595`](https://github.com/talos-systems/talos/commit/2d11b59558c98f4cd07a50b25be29b5c355a4495) fix: ignore virtual IP as kubelet node IPs
* [`030fd349`](https://github.com/talos-systems/talos/commit/030fd349b1c0669d7059f8c6883c85096f6f9ef5) fix: don't run kexec prepare on shutdown and reset
* [`6dcce20e`](https://github.com/talos-systems/talos/commit/6dcce20e6fa088c3063aab728912731f5e827eb7) test: set proper pod CIDR for Cilium tests
</p>
</details>

### Changes from talos-systems/discovery-api
<details><summary>2 commits</summary>
<p>

* [`db279ef`](https://github.com/talos-systems/discovery-api/commit/db279ef42a1fad2e1feb4902150b4969f7082c81) feat: initial set of APIs and generated files
* [`ac52a37`](https://github.com/talos-systems/discovery-api/commit/ac52a378211475ebd281dcbb00954eec42459778) chore: initial commit
</p>
</details>

### Changes from talos-systems/discovery-client
<details><summary>2 commits</summary>
<p>

* [`a9a5e9b`](https://github.com/talos-systems/discovery-client/commit/a9a5e9bfddaa670e0fb4f57510167d377cf09b07) feat: initial client code
* [`98eb999`](https://github.com/talos-systems/discovery-client/commit/98eb9999c0c76d2f93378108b7e22de6bcae6e81) chore: initial commit
</p>
</details>

### Changes from talos-systems/extras
<details><summary>2 commits</summary>
<p>

* [`2bb2efc`](https://github.com/talos-systems/extras/commit/2bb2efcbe68bcce2172b9ac7771dde1d0d2b6d3c) chore: update pkgs and tools
* [`d6e8b3a`](https://github.com/talos-systems/extras/commit/d6e8b3a78e9a3371472753286c559627932466c3) chore: update pkgs and tools
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>2 commits</summary>
<p>

* [`15b182d`](https://github.com/talos-systems/go-blockdevice/commit/15b182db0cd233b163ed83d1724c7e28cf29d71a) fix: return partition table not exist when trying to read an empty dev
* [`b9517d5`](https://github.com/talos-systems/go-blockdevice/commit/b9517d51120d385f97b0026f99ce3c4782940c37) fix: resize partition
</p>
</details>

### Changes from talos-systems/go-smbios
<details><summary>1 commit</summary>
<p>

* [`fd5ec8c`](https://github.com/talos-systems/go-smbios/commit/fd5ec8ce4873790b7fbd46dba9d7f49c9de7176a) fix: remove useless (?) goroutines leading to data race error
</p>
</details>

### Changes from talos-systems/net
<details><summary>2 commits</summary>
<p>

* [`b4b7181`](https://github.com/talos-systems/net/commit/b4b718179a1aa68e4f54422baf08ca3761723d2d) feat: add a way to filter list of IPs for the machine
* [`0abe5bd`](https://github.com/talos-systems/net/commit/0abe5bdae8f85e4e976bc4d90e95dcb4be8fb853) feat: implement FilterIPs function
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>22 commits</summary>
<p>

* [`422276d`](https://github.com/talos-systems/pkgs/commit/422276d5c06b00e63ec0ba0c70b642eedd13eea6) feat: update Linux to 5.15.5
* [`d385e24`](https://github.com/talos-systems/pkgs/commit/d385e24e868682ca68c74ecdae94af2bd17b4a28) chore: update LibreSSL to 3.2.7
* [`39a3b76`](https://github.com/talos-systems/pkgs/commit/39a3b76c3dd6d50b266b594c8ee2cc2d5537a5e0) feat: update Linux to 5.15.4
* [`ca30b50`](https://github.com/talos-systems/pkgs/commit/ca30b509b4cd3ab591a27eb6f76d5c2fd9da3b7e) feat: update u-boot to 2021.10
* [`cea93f1`](https://github.com/talos-systems/pkgs/commit/cea93f1163d79cea8b44e2f9f0bd645aa2404003) chore: add conformance
* [`79d16b8`](https://github.com/talos-systems/pkgs/commit/79d16b87f31759a1fbb3eab0a06728382983fce1) feat: update containerd to 1.5.8
* [`1c76107`](https://github.com/talos-systems/pkgs/commit/1c761077176f46388c4335bb7ceb9f58cc456c44) feat: add mdraid 1/0/10
* [`740da24`](https://github.com/talos-systems/pkgs/commit/740da24d801cc5a8f47f6badf788faea872a6e72) feat: bump raspberrypi-firmware to 1.20211029
* [`832dae4`](https://github.com/talos-systems/pkgs/commit/832dae4175d7cd3336fb3637134833e7b9fe1719) fix: enable CONFIG_DM_SNAPSHOT
* [`f307e64`](https://github.com/talos-systems/pkgs/commit/f307e64e6b2e8a29cff5508ae2da4ae92286771e) feat: update Linux to 5.15.1
* [`4f0f238`](https://github.com/talos-systems/pkgs/commit/4f0f238decfb93561c5871207da8dd3b1d350961) chore: update tools
* [`932c3cf`](https://github.com/talos-systems/pkgs/commit/932c3cfff9fcb9ffb671d7e5e10b0ca7c290c058) feat: update libseccomp to 2.5.3
* [`7f3311e`](https://github.com/talos-systems/pkgs/commit/7f3311e2a3d2ef759d9f909f9872e5b98f9682a5) feat: update cpu governor to schedutil
* [`b4cdb99`](https://github.com/talos-systems/pkgs/commit/b4cdb991a4aa60b1fa859b44efcd57705d89e9ee) fix: update containerd shas
* [`80a63d4`](https://github.com/talos-systems/pkgs/commit/80a63d4cf2231383266f244f608a958b94872a99) feat: update Linux to 5.10.75
* [`5c98efd`](https://github.com/talos-systems/pkgs/commit/5c98efd95d2e2e036d845c63b6268583d853d3fd) feat: add QLogic QED 25/40/100Gb Ethernet NIC driver
* [`bfb2365`](https://github.com/talos-systems/pkgs/commit/bfb2365b04aa7f92ef87799c47ffde6bc2395785) feat: enable driver for SuperMicro raid controller
* [`657e16b`](https://github.com/talos-systems/pkgs/commit/657e16b3976ba376401797277e85dd67c9b7e64e) feat: enable Intel VMD driver
* [`f7d9d72`](https://github.com/talos-systems/pkgs/commit/f7d9d728d468b9e3af2552595c9fb145f9008ef3) feat: enable smarpqi driver and related options
* [`bca3be0`](https://github.com/talos-systems/pkgs/commit/bca3be04e22367585a60afa421e78707d2c6a1de) feat: enable aqtion device driver
* [`b88127a`](https://github.com/talos-systems/pkgs/commit/b88127afec39d3039e93dfd6bc20a62415d396f0) chore: update tools
* [`971735f`](https://github.com/talos-systems/pkgs/commit/971735f4b1914cb1c8f2575aeda9b354ecf842f6) feat: update containerd to 1.5.7
</p>
</details>

### Changes from talos-systems/siderolink
<details><summary>6 commits</summary>
<p>

* [`d0612a7`](https://github.com/talos-systems/siderolink/commit/d0612a724a1b1336a2bc6a99ed3178e3e40f6d9b) refactor: pass in listener to the log receiver
* [`d86cdd5`](https://github.com/talos-systems/siderolink/commit/d86cdd59ee7a0e0504b739a913991c272c7fb3f5) feat: implement logreceiver for kernel logs
* [`f7cadbc`](https://github.com/talos-systems/siderolink/commit/f7cadbcdfbb84d367e27b5af32e89c138d72d9d7) fix: handle duplicate peer updates
* [`0755b24`](https://github.com/talos-systems/siderolink/commit/0755b24d4682410b251a2a9d662960da15153106) feat: initial implementation of SideroLink
* [`ee73ea9`](https://github.com/talos-systems/siderolink/commit/ee73ea9575a81be7685f24936b2c48a4508a159e) feat: add Talos events sink proto files and the reference implementation
* [`1e2cd9d`](https://github.com/talos-systems/siderolink/commit/1e2cd9d38621234a0a6010e33b1bab264f4d9bdf) Initial commit
</p>
</details>

### Changes from talos-systems/tools
<details><summary>6 commits</summary>
<p>

* [`96e0231`](https://github.com/talos-systems/tools/commit/96e0231c0f01a9fe6120a941b21c40e1a37bab36) feat: update squashfs-tools to 4.5
* [`2c9c826`](https://github.com/talos-systems/tools/commit/2c9c826201dc9f4d869fc00ffac63ee10f5e1101) feat: update libseccomp to 2.5.3
* [`f713a7c`](https://github.com/talos-systems/tools/commit/f713a7cd96fb1176687f5a6c4ec7d1345bb5e568) feat: update protobuf to 3.19.1, grpc-go to 1.42.0
* [`972c5ef`](https://github.com/talos-systems/tools/commit/972c5ef413f721ab5ad835dab526850620d05003) feat: update Go to 1.17.3
* [`f63848c`](https://github.com/talos-systems/tools/commit/f63848c1a756807879f22c752155fe1f36ccec32) feat: update PCRE version and source host
* [`fab7532`](https://github.com/talos-systems/tools/commit/fab7532fd59519d62a3985684a250273a14f1893) feat: update Go to 1.17.2
</p>
</details>

### Dependency Changes

* **github.com/AlekSi/pointer**                      v1.1.0 -> v1.2.0
* **github.com/cenkalti/backoff/v4**                 v4.1.2 **_new_**
* **github.com/containerd/cgroups**                  v1.0.1 -> v1.0.2
* **github.com/containerd/containerd**               v1.5.5 -> v1.5.8
* **github.com/docker/docker**                       v20.10.8 -> v20.10.11
* **github.com/evanphx/json-patch**                  v4.11.0 -> v5.6.0
* **github.com/gosuri/uiprogress**                   v0.0.1 **_new_**
* **github.com/hashicorp/go-getter**                 v1.5.8 -> v1.5.9
* **github.com/hetznercloud/hcloud-go**              v1.32.0 -> v1.33.1
* **github.com/insomniacslk/dhcp**                   b95caade3eac -> ad197bcd36fd
* **github.com/jsimonetti/rtnetlink**                435639c8e6a8 -> 93da33804786
* **github.com/jxskiss/base62**                      4f11678b909b -> v1.0.0
* **github.com/mdlayher/ethtool**                    2b88debcdd43 -> 288d040e9d60
* **github.com/rivo/tview**                          ee97a7ab3975 -> badfa0f0b301
* **github.com/talos-systems/discovery-api**         v0.1.0 **_new_**
* **github.com/talos-systems/discovery-client**      v0.1.0 **_new_**
* **github.com/talos-systems/extras**                v0.6.0 -> v0.7.0-alpha.0-1-g2bb2efc
* **github.com/talos-systems/go-blockdevice**        v0.2.4 -> 15b182db0cd2
* **github.com/talos-systems/go-smbios**             v0.1.0 -> fd5ec8ce4873
* **github.com/talos-systems/net**                   v0.3.0 -> b4b718179a1a
* **github.com/talos-systems/pkgs**                  v0.8.0 -> v0.9.0-alpha.0-21-g422276d
* **github.com/talos-systems/siderolink**            v0.1.0 **_new_**
* **github.com/talos-systems/talos/pkg/machinery**   v0.13.0 -> 1ffa8e048008
* **github.com/talos-systems/tools**                 v0.8.0 -> v0.9.0-alpha.0-5-g96e0231
* **github.com/vmware-tanzu/sonobuoy**               v0.53.2 -> v0.55.1
* **github.com/vmware/govmomi**                      v0.26.1 -> v0.27.2
* **github.com/vmware/vmw-guestinfo**                687661b8bd8e -> cc1fd90d572c
* **go.etcd.io/etcd/api/v3**                         v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/pkg/v3**                  v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/v3**                      v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/etcdutl/v3**                     v3.5.0 -> v3.5.1
* **go.uber.org/atomic**                             v1.7.0 **_new_**
* **golang.org/x/net**                               3ad01bbaa167 -> d83791d6bcd9
* **golang.org/x/sys**                               39ccf1dd6fa6 -> fe61309f8881
* **golang.org/x/term**                              140adaaadfaf -> 03fcf44c2211
* **golang.org/x/time**                              1f47c861a9ac -> f0f3c7e86c11
* **golang.zx2c4.com/wireguard/wgctrl**              0a2f4901cba6 -> dd7407c86d22
* **google.golang.org/grpc**                         v1.41.0 -> v1.42.0
* **inet.af/netaddr**                                85fa6c94624e -> c74959edd3b6
* **k8s.io/api**                                     v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/apimachinery**                            v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/client-go**                               v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/cri-api**                                 v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/kubectl**                                 v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/kubelet**                                 v0.22.2 -> v0.23.0-alpha.4
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.59 -> v1.2.61
* **sigs.k8s.io/yaml**                               v1.3.0 **_new_**

Previous release can be found at [v0.13.0](https://github.com/talos-systems/talos/releases/tag/v0.13.0)

## [Talos 0.14.0-alpha.1](https://github.com/talos-systems/talos/releases/tag/v0.14.0-alpha.1) (2021-11-15)

Welcome to the v0.14.0-alpha.1 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Kexec and capabilities

When kexec support is disabled
Talos no longer drops Linux capabilities (`CAP_SYS_BOOT` and `CAP_SYS_MODULES`) for child processes.
That is helpful for advanced use-cases like Docker-in-Docker.

If you want to permanently disable kexec and capabilities dropping, pass `kexec_load_disabled=1` argument to the kernel.

For example:

```yaml
install:
  extraKernelArgs:
    - sysctl.kernel.kexec_load_disabled=1
```

Please note that capabilities are dropped before machine configuration is loaded,
so disabling kexec via `machine.sysctls` will not be enough.


### Cluster Discovery

Cluster Discovery is enabled by default for Talos 0.14.
Cluster Discovery can be disabled with `talosctl gen config --with-cluster-discovery=false`.


### Kubelet

Kubelet service can now be restarted with `talosctl service kubelet restart`.

Kubelet node IP configuration (`.machine.kubelet.nodeIP.validSubnets`) can now include negative subnet matches (prefixed with `!`).


### Log Shipping

Talos can now ship system logs to the configured destination using either JSON-over-UDP or JSON-over-TCP:
see `.machine.logging` machine configuration option.


### `talosctl support`

`talosctl` CLI tool now has a new subcommand called `support`, that can gather all
cluster information that could help with future debugging in a single run.

Output of the command is a `zip` archive with all talos service logs, kubernetes pod logs and manifests,
talos resources manifests and so on.
Generated archive does not contain any secret information so it is safe to send it for analysis to a third party.


### Component Updates

* Linux: 5.15.1
* etcd: 3.5.1
* containerd: 1.5.7
* Kubernetes: 1.23.0-alpha.4
* CoreDNS: 1.8.6

Talos is built with Go 1.17.2


### Kubernetes Upgrade Enhancements

`talosctl upgrade-k8s` now syncs all Talos manifest resources generated from templates.

So there is no need to update CoreDNS, Flannel container manually after running `upgrade-k8s` anymore.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Serge Logvinov
* Noel Georgi
* Spencer Smith
* Nico Berlee
* Alex Zero
* Andrew Rynhard
* Branden Cash
* David Haines
* Gerard de Leeuw
* Michael Fornaro
* Rui Lopes

### Changes
<details><summary>101 commits</summary>
<p>

* [`753a8218`](https://github.com/talos-systems/talos/commit/753a82188f227da4f2f40da5f4d46ebe45774455) refactor: move pkg/resources to machinery
* [`0102a64a`](https://github.com/talos-systems/talos/commit/0102a64a5f6de2c3fe5d7792c2c5845fc737edff) refactor: remove pkg/resources dependencies on wgtypes, netx
* [`7462733b`](https://github.com/talos-systems/talos/commit/7462733bcb075b923b8c7ba4a763308c641c49a2) chore: update golangci-lint
* [`032c99a0`](https://github.com/talos-systems/talos/commit/032c99a0300ccb09105a07434884d2b1f57e537d) refactor: remove pkg/resources dependencies on k8s and base62
* [`4a5cff45`](https://github.com/talos-systems/talos/commit/4a5cff45f397ac29b7bfc390f11691c32d8615b2) perf: raspberry PIs clockspeed as fast as firmware allows
* [`a76f6d69`](https://github.com/talos-systems/talos/commit/a76f6d69dbfdf34e4383dd5d2ee9f8cca4661e87) feat: allow kubelet to be restarted and provide negative nodeIP subnets
* [`189221d5`](https://github.com/talos-systems/talos/commit/189221d589c1c9d4fc012dd9e31fd6d142d88dde) chore: update dependencies
* [`41f0aecc`](https://github.com/talos-systems/talos/commit/41f0aecc1d3c4afce96d034f160fa9f120c67e85) docs: update partition info
* [`95105071`](https://github.com/talos-systems/talos/commit/95105071de29f70552bd7c0881c2cc2e7c78c0ac) chore: fix simple issues found by golangci-lint
* [`d4b0ca21`](https://github.com/talos-systems/talos/commit/d4b0ca21a1ee1183b28738bb3d9ca251e1968fe7) test: retry upgrade mutex lock failures
* [`4357e9a8`](https://github.com/talos-systems/talos/commit/4357e9a849fcb7fb66378bdd767a926dde0c4318) docs: add Talos partions info
* [`8e8687d7`](https://github.com/talos-systems/talos/commit/8e8687d7592d4bc071981478491d70489e7dd4a9) fix: use temporary sonobuoy version
* [`e4e8e873`](https://github.com/talos-systems/talos/commit/e4e8e8737f564be47098e284706a63ef84636890) test: disable e2e-misc test with Canal CNI
* [`897da2f6`](https://github.com/talos-systems/talos/commit/897da2f6efc571a66d14722a67bbc401bad31887) docs: common typos
* [`a50483dd`](https://github.com/talos-systems/talos/commit/a50483dddfd9a742b998f509ee713af996a2484e) feat: update Linux to 5.15.1
* [`a2233bfe`](https://github.com/talos-systems/talos/commit/a2233bfe46bfb55d71cfc07174f6f22aee6d2651) fix: improve NTP sync process
* [`7efc1238`](https://github.com/talos-systems/talos/commit/7efc1238ee285d55c4619b6a40190b54ff953a66) fix: parse partition size correctly
* [`d6147eb1`](https://github.com/talos-systems/talos/commit/d6147eb17d2ebf263ca0537068bbbba6d3ced061) chore: update sonobuoy
* [`efbae785`](https://github.com/talos-systems/talos/commit/efbae7857d09aa7e5e704d5989efced5aa655259) fix: use etc folder for du cli tests
* [`198eea51`](https://github.com/talos-systems/talos/commit/198eea51a81bf041470c3c88cb6cb97af3a4e203) fix: wait for follow reader to start before writing to the file
* [`e7f715eb`](https://github.com/talos-systems/talos/commit/e7f715eb0ca0587a05949910cafdeb486654b577) chore: log KubeSpan IPs overlaps
* [`82a1ad16`](https://github.com/talos-systems/talos/commit/82a1ad1681bf262dcc68fc9cbac71ff2eb5639af) chore: bump dependencies
* [`e8fccbf5`](https://github.com/talos-systems/talos/commit/e8fccbf5351ec2481813553181cb73b8f16c915a) fix: clear time adjustment error when setting time to specific value
* [`e6f90bb4`](https://github.com/talos-systems/talos/commit/e6f90bb41a757b5173bbbf7554b6f85c08aaf58e) chore: remove unused parameters
* [`785161d1`](https://github.com/talos-systems/talos/commit/785161d19f68fb64451cf3d887b67f85a8bcb952) feat: update k8s to 1.23.0-alpha.4
* [`fe228d7c`](https://github.com/talos-systems/talos/commit/fe228d7c85a1f8437398061b18c090962adc9f29) fix: do not use yaml.v2 in the support cmd
* [`9b48ca21`](https://github.com/talos-systems/talos/commit/9b48ca21731cce53f0a61f05f74dcd264417d784) fix: endpoints and nodes in generated talosconfig
* [`6e16fd2f`](https://github.com/talos-systems/talos/commit/6e16fd2feeb3f8bf0b99e6cbe21047b7a5c1f05c) chore: update tools, pkgs, and extras
* [`261c497c`](https://github.com/talos-systems/talos/commit/261c497c71eb5ab5197bef05d8c209dbeb770d3f) feat: implement `talosctl support` command
* [`fc7dc454`](https://github.com/talos-systems/talos/commit/fc7dc454840e100d82bb036a7f065293234593f7) chore: check our API idiosyncrasies
* [`b1584429`](https://github.com/talos-systems/talos/commit/b15844298a6bfedca5acc0cc27061666481eb94b) feat: use GCP deployment manager
* [`3e7d4df9`](https://github.com/talos-systems/talos/commit/3e7d4df99019e3cc6d9a90920d377c73a76ac577) chore: bump dependencies
* [`88f24229`](https://github.com/talos-systems/talos/commit/88f2422955690d1eca1e21cd60a35e1d49141e3d) refactor: get rid of prometheus/procfs dependency in pkg/resources
* [`dd196d30`](https://github.com/talos-systems/talos/commit/dd196d3006d29ae5cae5d43b648da1ca2e5af236) refactor: prepare for move of pkg/resources to machinery
* [`f6110f80`](https://github.com/talos-systems/talos/commit/f6110f8036bc176188abb583bfa51296c4d3897d) fix: remove listening socket to fix Talos in a container restart
* [`53bbb13e`](https://github.com/talos-systems/talos/commit/53bbb13ed8592978dc27578fa79b3a2018941427) docs: update docs with emmc boot guide
* [`8329d211`](https://github.com/talos-systems/talos/commit/8329d21114abf841788be64765378343c12eaf69) chore: split polymorphic RootSecret resource into specific types
* [`c97becdd`](https://github.com/talos-systems/talos/commit/c97becdd9548d85b2b894a05765f93dcdf9ad803) chore: remove interfaces and routes APIs
* [`d798635d`](https://github.com/talos-systems/talos/commit/d798635d993a21392b8a7972a689c4be0728db32) feat: automatically limit kubelet node IP family based on service CIDRs
* [`205a8d6d`](https://github.com/talos-systems/talos/commit/205a8d6dc495e25af87bf0b920d0f55b8a27bbfd) chore: make nethelpers build on all OSes
* [`5b5dd49f`](https://github.com/talos-systems/talos/commit/5b5dd49f64bef584000655687e5b9c5d25af6a93) feat: extract JSON fields from more log messages
* [`eb4f1182`](https://github.com/talos-systems/talos/commit/eb4f11822dc0b35541e0576a75ca263ca96d4981) docs: create cluster in hetzner cloud
* [`728164e2`](https://github.com/talos-systems/talos/commit/728164e25a5705ae5194b416941f3607d592b140) docs: fix kexec_load_disabled param name in release notes
* [`f6328f09`](https://github.com/talos-systems/talos/commit/f6328f09a2bf8d233a48354dd548fb740e509341) fix: fix filename typo
* [`01b0f0ab`](https://github.com/talos-systems/talos/commit/01b0f0abb341b387f16d9b3a142af742f36c8c2b) release(v0.14.0-alpha.0): prepare release
* [`8b620653`](https://github.com/talos-systems/talos/commit/8b6206537a30be049f74f8c4c7350028e6e56c74) fix: skip generating empty `.machine.logging`
* [`60ad0063`](https://github.com/talos-systems/talos/commit/60ad006367e73f56fd69726e0044f1ce48f18a8b) fix: don't drop ability to use ambient capabilities
* [`b6b78e7f`](https://github.com/talos-systems/talos/commit/b6b78e7fef3f6ef0c566e1815d1e28f16f868c93) test: add cluster discovery integration tests
* [`97d64d16`](https://github.com/talos-systems/talos/commit/97d64d160ce7e71c3107adbd31404853f543f7cc) fix: hcloud network config changes
* [`4c76865d`](https://github.com/talos-systems/talos/commit/4c76865d0ecec726e801a4b8f87e09476481d808) feat: multiple logging improvements
* [`1d1e1df6`](https://github.com/talos-systems/talos/commit/1d1e1df643832478aaa715aea5f51ad2e61e2880) fix: handle skipped mounts correctly
* [`0a964d92`](https://github.com/talos-systems/talos/commit/0a964d921922a247293e36b5fecaab466b91d924) test: fix openstack unit-test stability
* [`72f62ac2`](https://github.com/talos-systems/talos/commit/72f62ac27b5d0a72db409fd003a7cf9c41a03d7c) chore: bump Go and Docker dependencies
* [`9c48ebe8`](https://github.com/talos-systems/talos/commit/9c48ebe8f94afa85921ee5f1c1e9315201905a92) fix: gcp fetching externalIP
* [`6c297268`](https://github.com/talos-systems/talos/commit/6c297268ce596c2a875b7c419c85317dc24d9f4f) test: fix e2e k8s version
* [`ae5af9d3`](https://github.com/talos-systems/talos/commit/ae5af9d3fad399dea95c316d94e3e66b124bfb24) feat: update Kubernetes to 1.23.0-alpha.3
* [`28d3a69e`](https://github.com/talos-systems/talos/commit/28d3a69e9d4ae7ffa231804e26af6d1f39c07afd) feat: openstack config-drive support
* [`2258bc49`](https://github.com/talos-systems/talos/commit/2258bc4918e89b3d6fcb841b2ad677f114ddba7e) test: update GCP e2e script to work with new templates
* [`36b6ace2`](https://github.com/talos-systems/talos/commit/36b6ace25378e8c4a607de6efb6b89a2d52f5cea) feat: update Linux to 5.10.75
* [`38516a54`](https://github.com/talos-systems/talos/commit/38516a5499d933a8038ce6768946ff096e7c6f98) test: update Talos versions in upgrade tests
* [`cff20ec7`](https://github.com/talos-systems/talos/commit/cff20ec78340b3855751e13f2ad0e54bd47e9989) fix: change services OOM score
* [`666a2b62`](https://github.com/talos-systems/talos/commit/666a2b6207d257edda20c9e0411b0d4cd4112aa6) feat: azure platform ipv6 support
* [`d32814e3`](https://github.com/talos-systems/talos/commit/d32814e302c370ec1e82aa2879186a034cd2a905) feat: extract JSON fields from log lines
* [`e77d81ff`](https://github.com/talos-systems/talos/commit/e77d81fff31d68f762da3741846f95a6d2303903) fix: treat literal 'unknown' as a valid machine type
* [`c8e404e3`](https://github.com/talos-systems/talos/commit/c8e404e356878f6cd819a33386b351c1c152c3f5) test: update vars for AWS cluster
* [`ad23891b`](https://github.com/talos-systems/talos/commit/ad23891b1f6b33409721528c6771304b7ab94b2c) feat: update CoreDNS version 1.8.6
* [`41299cae`](https://github.com/talos-systems/talos/commit/41299cae9961665c2bf2a642290f8309683f040d) feat: udev rules support
* [`5237fdc9`](https://github.com/talos-systems/talos/commit/5237fdc957efbb018649b866bfb756f280f589a2) feat: send JSON logs over UDP
* [`6d44587a`](https://github.com/talos-systems/talos/commit/6d44587a4d4c16defa6bb06329cdfc6e39c95188) feat: coredns service dualstack
* [`12f7888b`](https://github.com/talos-systems/talos/commit/12f7888b75fa2498e0f8305f5d6910cecad5c65c) feat: feed control plane endpoints on workers from cluster discovery
* [`431e4fb4`](https://github.com/talos-systems/talos/commit/431e4fb4b690fa4955c407d8dd8156bdecd9a2c5) chore: bump Go and Docker dependencies
* [`89f3b9f8`](https://github.com/talos-systems/talos/commit/89f3b9f8d41e33c4cb736917f418ab5cfb9edd83) feat: update etcd to 3.5.1
* [`e60469a3`](https://github.com/talos-systems/talos/commit/e60469a38cb81ace2039bae1927eb6c5f1f0ad1f) feat: initial support for JSON logging
* [`68c420e3`](https://github.com/talos-systems/talos/commit/68c420e3c96a0fdc3b3e6cd75be24cc797c48e09) feat: enable cluster discovery by default
* [`3e100aa9`](https://github.com/talos-systems/talos/commit/3e100aa97734ea809563e23fc36e19bdd3df1920) test: workaround EventsWatch test flakiness
* [`9bd4838a`](https://github.com/talos-systems/talos/commit/9bd4838ac10abbd4760da4fb905d7639a1c26f9f) chore: stop using sonobuoy CLI
* [`6ad45951`](https://github.com/talos-systems/talos/commit/6ad45951975aac48fdcc282e5a0e31344058d07e) docs: fix field names for bonding configuration
* [`d7a3b7b5`](https://github.com/talos-systems/talos/commit/d7a3b7b5b70293884d2e19c6a59b14ebcfa24397) chore: use discovery-client and discovery-api modules
* [`d6309eed`](https://github.com/talos-systems/talos/commit/d6309eed6618abd1b4efd0e3cd18a6c0df39378f) docs: create docs for Talos 0.14
* [`c0fda643`](https://github.com/talos-systems/talos/commit/c0fda6436ae27d8bbc210ee74a1128968108f6a6) fix: attempt to clean up tasks in containerd runner
* [`8cf442da`](https://github.com/talos-systems/talos/commit/8cf442daa60d911caff59d1c2c05dd77652c8b51) chore: bump tools, pkgs, extras
* [`0dad5f4d`](https://github.com/talos-systems/talos/commit/0dad5f4d7846f3fb41ff4ba27395023d33796a61) chore: small cleanup
* [`e3e2113a`](https://github.com/talos-systems/talos/commit/e3e2113adc058940725b1041827d7adb8895c6cf) feat: upgrade CoreDNS during `upgrade-k8s` call
* [`d92c98e1`](https://github.com/talos-systems/talos/commit/d92c98e19a054472bff3e0d646756f16c5e65bbf) docs: fix discovery service documentation link
* [`e44b11c5`](https://github.com/talos-systems/talos/commit/e44b11c595e4cab796128a932843b90734ff6d1d) feat: update containerd to 1.5.7, bump Go dependencies
* [`24129307`](https://github.com/talos-systems/talos/commit/24129307a14d6e59c6bc0d3586c0c95969bde679) docs: make Talos 0.13 docs latest, update documentation
* [`31b6e39e`](https://github.com/talos-systems/talos/commit/31b6e39e58a27e1f2c1be500fca8636971bfa5c6) fix: delete expired affiliates from the discovery service
* [`877a2b6f`](https://github.com/talos-systems/talos/commit/877a2b6fc00eaa7574349f9086d78c04df163840) test: bump CAPI components to v1alpha4
* [`2ba0e0ac`](https://github.com/talos-systems/talos/commit/2ba0e0ac4ad460409101f5f2374e66698adbba4c) docs: add KubeSpan documentation
* [`997873b6`](https://github.com/talos-systems/talos/commit/997873b6d3116b59ebb46df66b8aa1cee06df92f) fix: use ECDSA-SHA512 when generating certs for Talos < 0.13
* [`7137166d`](https://github.com/talos-systems/talos/commit/7137166d1d5817e2d44ead4a01796275f92a9d4a) fix: allow overriding `audit-policy-file` in `kube-apiserver` static pod
* [`8fcd4219`](https://github.com/talos-systems/talos/commit/8fcd4219671a9359880ba344a2ec7fd65dfe5e2a) chore: fix integration-qemu-race
* [`91a858b5`](https://github.com/talos-systems/talos/commit/91a858b53704ede86392fe3c155ce9ab3c2d406f) fix: sort output of the argument builder
* [`657f7a56`](https://github.com/talos-systems/talos/commit/657f7a56b10089e0dc551e178bc85b28d8003243) fix: use ECDSA-SHA256 signature algorithm for Kubernetes certs
* [`983d2459`](https://github.com/talos-systems/talos/commit/983d2459e2aa036774828f773bbaba5697665ae7) feat: suppress logging NTP sync to the console
* [`022c7335`](https://github.com/talos-systems/talos/commit/022c7335f3063675ab744454a2ad4b2c0c19bfbc) fix: add interface route if DHCP4 router is not directly routeable
* [`66a1579e`](https://github.com/talos-systems/talos/commit/66a1579ea7d2a9c4fdf15b762cd024c54b3e8ffb) fix: don't enable 'no new privs' on the system level
* [`423861cf`](https://github.com/talos-systems/talos/commit/423861cf9f99eaf034a4f0cb243d73d1275c3f38) feat: don't drop capabilities if kexec is disabled
* [`facc8c38`](https://github.com/talos-systems/talos/commit/facc8c38a021610da900a45f397aea8ddfc74f1c) docs: fix documentation for cluster discovery
* [`ce65ca4e`](https://github.com/talos-systems/talos/commit/ce65ca4e4a2994f901f01ce5ca269d6df86f0de8) chore: build using only amd64 builders
* [`e9b0f010`](https://github.com/talos-systems/talos/commit/e9b0f010d2855b968a5d8b8b5fbcd268e06ba302) chore: update docker image in the pipeline
</p>
</details>

### Changes since v0.14.0-alpha.0
<details><summary>44 commits</summary>
<p>

* [`753a8218`](https://github.com/talos-systems/talos/commit/753a82188f227da4f2f40da5f4d46ebe45774455) refactor: move pkg/resources to machinery
* [`0102a64a`](https://github.com/talos-systems/talos/commit/0102a64a5f6de2c3fe5d7792c2c5845fc737edff) refactor: remove pkg/resources dependencies on wgtypes, netx
* [`7462733b`](https://github.com/talos-systems/talos/commit/7462733bcb075b923b8c7ba4a763308c641c49a2) chore: update golangci-lint
* [`032c99a0`](https://github.com/talos-systems/talos/commit/032c99a0300ccb09105a07434884d2b1f57e537d) refactor: remove pkg/resources dependencies on k8s and base62
* [`4a5cff45`](https://github.com/talos-systems/talos/commit/4a5cff45f397ac29b7bfc390f11691c32d8615b2) perf: raspberry PIs clockspeed as fast as firmware allows
* [`a76f6d69`](https://github.com/talos-systems/talos/commit/a76f6d69dbfdf34e4383dd5d2ee9f8cca4661e87) feat: allow kubelet to be restarted and provide negative nodeIP subnets
* [`189221d5`](https://github.com/talos-systems/talos/commit/189221d589c1c9d4fc012dd9e31fd6d142d88dde) chore: update dependencies
* [`41f0aecc`](https://github.com/talos-systems/talos/commit/41f0aecc1d3c4afce96d034f160fa9f120c67e85) docs: update partition info
* [`95105071`](https://github.com/talos-systems/talos/commit/95105071de29f70552bd7c0881c2cc2e7c78c0ac) chore: fix simple issues found by golangci-lint
* [`d4b0ca21`](https://github.com/talos-systems/talos/commit/d4b0ca21a1ee1183b28738bb3d9ca251e1968fe7) test: retry upgrade mutex lock failures
* [`4357e9a8`](https://github.com/talos-systems/talos/commit/4357e9a849fcb7fb66378bdd767a926dde0c4318) docs: add Talos partions info
* [`8e8687d7`](https://github.com/talos-systems/talos/commit/8e8687d7592d4bc071981478491d70489e7dd4a9) fix: use temporary sonobuoy version
* [`e4e8e873`](https://github.com/talos-systems/talos/commit/e4e8e8737f564be47098e284706a63ef84636890) test: disable e2e-misc test with Canal CNI
* [`897da2f6`](https://github.com/talos-systems/talos/commit/897da2f6efc571a66d14722a67bbc401bad31887) docs: common typos
* [`a50483dd`](https://github.com/talos-systems/talos/commit/a50483dddfd9a742b998f509ee713af996a2484e) feat: update Linux to 5.15.1
* [`a2233bfe`](https://github.com/talos-systems/talos/commit/a2233bfe46bfb55d71cfc07174f6f22aee6d2651) fix: improve NTP sync process
* [`7efc1238`](https://github.com/talos-systems/talos/commit/7efc1238ee285d55c4619b6a40190b54ff953a66) fix: parse partition size correctly
* [`d6147eb1`](https://github.com/talos-systems/talos/commit/d6147eb17d2ebf263ca0537068bbbba6d3ced061) chore: update sonobuoy
* [`efbae785`](https://github.com/talos-systems/talos/commit/efbae7857d09aa7e5e704d5989efced5aa655259) fix: use etc folder for du cli tests
* [`198eea51`](https://github.com/talos-systems/talos/commit/198eea51a81bf041470c3c88cb6cb97af3a4e203) fix: wait for follow reader to start before writing to the file
* [`e7f715eb`](https://github.com/talos-systems/talos/commit/e7f715eb0ca0587a05949910cafdeb486654b577) chore: log KubeSpan IPs overlaps
* [`82a1ad16`](https://github.com/talos-systems/talos/commit/82a1ad1681bf262dcc68fc9cbac71ff2eb5639af) chore: bump dependencies
* [`e8fccbf5`](https://github.com/talos-systems/talos/commit/e8fccbf5351ec2481813553181cb73b8f16c915a) fix: clear time adjustment error when setting time to specific value
* [`e6f90bb4`](https://github.com/talos-systems/talos/commit/e6f90bb41a757b5173bbbf7554b6f85c08aaf58e) chore: remove unused parameters
* [`785161d1`](https://github.com/talos-systems/talos/commit/785161d19f68fb64451cf3d887b67f85a8bcb952) feat: update k8s to 1.23.0-alpha.4
* [`fe228d7c`](https://github.com/talos-systems/talos/commit/fe228d7c85a1f8437398061b18c090962adc9f29) fix: do not use yaml.v2 in the support cmd
* [`9b48ca21`](https://github.com/talos-systems/talos/commit/9b48ca21731cce53f0a61f05f74dcd264417d784) fix: endpoints and nodes in generated talosconfig
* [`6e16fd2f`](https://github.com/talos-systems/talos/commit/6e16fd2feeb3f8bf0b99e6cbe21047b7a5c1f05c) chore: update tools, pkgs, and extras
* [`261c497c`](https://github.com/talos-systems/talos/commit/261c497c71eb5ab5197bef05d8c209dbeb770d3f) feat: implement `talosctl support` command
* [`fc7dc454`](https://github.com/talos-systems/talos/commit/fc7dc454840e100d82bb036a7f065293234593f7) chore: check our API idiosyncrasies
* [`b1584429`](https://github.com/talos-systems/talos/commit/b15844298a6bfedca5acc0cc27061666481eb94b) feat: use GCP deployment manager
* [`3e7d4df9`](https://github.com/talos-systems/talos/commit/3e7d4df99019e3cc6d9a90920d377c73a76ac577) chore: bump dependencies
* [`88f24229`](https://github.com/talos-systems/talos/commit/88f2422955690d1eca1e21cd60a35e1d49141e3d) refactor: get rid of prometheus/procfs dependency in pkg/resources
* [`dd196d30`](https://github.com/talos-systems/talos/commit/dd196d3006d29ae5cae5d43b648da1ca2e5af236) refactor: prepare for move of pkg/resources to machinery
* [`f6110f80`](https://github.com/talos-systems/talos/commit/f6110f8036bc176188abb583bfa51296c4d3897d) fix: remove listening socket to fix Talos in a container restart
* [`53bbb13e`](https://github.com/talos-systems/talos/commit/53bbb13ed8592978dc27578fa79b3a2018941427) docs: update docs with emmc boot guide
* [`8329d211`](https://github.com/talos-systems/talos/commit/8329d21114abf841788be64765378343c12eaf69) chore: split polymorphic RootSecret resource into specific types
* [`c97becdd`](https://github.com/talos-systems/talos/commit/c97becdd9548d85b2b894a05765f93dcdf9ad803) chore: remove interfaces and routes APIs
* [`d798635d`](https://github.com/talos-systems/talos/commit/d798635d993a21392b8a7972a689c4be0728db32) feat: automatically limit kubelet node IP family based on service CIDRs
* [`205a8d6d`](https://github.com/talos-systems/talos/commit/205a8d6dc495e25af87bf0b920d0f55b8a27bbfd) chore: make nethelpers build on all OSes
* [`5b5dd49f`](https://github.com/talos-systems/talos/commit/5b5dd49f64bef584000655687e5b9c5d25af6a93) feat: extract JSON fields from more log messages
* [`eb4f1182`](https://github.com/talos-systems/talos/commit/eb4f11822dc0b35541e0576a75ca263ca96d4981) docs: create cluster in hetzner cloud
* [`728164e2`](https://github.com/talos-systems/talos/commit/728164e25a5705ae5194b416941f3607d592b140) docs: fix kexec_load_disabled param name in release notes
* [`f6328f09`](https://github.com/talos-systems/talos/commit/f6328f09a2bf8d233a48354dd548fb740e509341) fix: fix filename typo
</p>
</details>

### Changes from talos-systems/discovery-api
<details><summary>2 commits</summary>
<p>

* [`db279ef`](https://github.com/talos-systems/discovery-api/commit/db279ef42a1fad2e1feb4902150b4969f7082c81) feat: initial set of APIs and generated files
* [`ac52a37`](https://github.com/talos-systems/discovery-api/commit/ac52a378211475ebd281dcbb00954eec42459778) chore: initial commit
</p>
</details>

### Changes from talos-systems/discovery-client
<details><summary>2 commits</summary>
<p>

* [`a9a5e9b`](https://github.com/talos-systems/discovery-client/commit/a9a5e9bfddaa670e0fb4f57510167d377cf09b07) feat: initial client code
* [`98eb999`](https://github.com/talos-systems/discovery-client/commit/98eb9999c0c76d2f93378108b7e22de6bcae6e81) chore: initial commit
</p>
</details>

### Changes from talos-systems/extras
<details><summary>2 commits</summary>
<p>

* [`2bb2efc`](https://github.com/talos-systems/extras/commit/2bb2efcbe68bcce2172b9ac7771dde1d0d2b6d3c) chore: update pkgs and tools
* [`d6e8b3a`](https://github.com/talos-systems/extras/commit/d6e8b3a78e9a3371472753286c559627932466c3) chore: update pkgs and tools
</p>
</details>

### Changes from talos-systems/net
<details><summary>1 commit</summary>
<p>

* [`0abe5bd`](https://github.com/talos-systems/net/commit/0abe5bdae8f85e4e976bc4d90e95dcb4be8fb853) feat: implement FilterIPs function
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>15 commits</summary>
<p>

* [`740da24`](https://github.com/talos-systems/pkgs/commit/740da24d801cc5a8f47f6badf788faea872a6e72) feat: bump raspberrypi-firmware to 1.20211029
* [`832dae4`](https://github.com/talos-systems/pkgs/commit/832dae4175d7cd3336fb3637134833e7b9fe1719) fix: enable CONFIG_DM_SNAPSHOT
* [`f307e64`](https://github.com/talos-systems/pkgs/commit/f307e64e6b2e8a29cff5508ae2da4ae92286771e) feat: update Linux to 5.15.1
* [`4f0f238`](https://github.com/talos-systems/pkgs/commit/4f0f238decfb93561c5871207da8dd3b1d350961) chore: update tools
* [`932c3cf`](https://github.com/talos-systems/pkgs/commit/932c3cfff9fcb9ffb671d7e5e10b0ca7c290c058) feat: update libseccomp to 2.5.3
* [`7f3311e`](https://github.com/talos-systems/pkgs/commit/7f3311e2a3d2ef759d9f909f9872e5b98f9682a5) feat: update cpu governor to schedutil
* [`b4cdb99`](https://github.com/talos-systems/pkgs/commit/b4cdb991a4aa60b1fa859b44efcd57705d89e9ee) fix: update containerd shas
* [`80a63d4`](https://github.com/talos-systems/pkgs/commit/80a63d4cf2231383266f244f608a958b94872a99) feat: update Linux to 5.10.75
* [`5c98efd`](https://github.com/talos-systems/pkgs/commit/5c98efd95d2e2e036d845c63b6268583d853d3fd) feat: add QLogic QED 25/40/100Gb Ethernet NIC driver
* [`bfb2365`](https://github.com/talos-systems/pkgs/commit/bfb2365b04aa7f92ef87799c47ffde6bc2395785) feat: enable driver for SuperMicro raid controller
* [`657e16b`](https://github.com/talos-systems/pkgs/commit/657e16b3976ba376401797277e85dd67c9b7e64e) feat: enable Intel VMD driver
* [`f7d9d72`](https://github.com/talos-systems/pkgs/commit/f7d9d728d468b9e3af2552595c9fb145f9008ef3) feat: enable smarpqi driver and related options
* [`bca3be0`](https://github.com/talos-systems/pkgs/commit/bca3be04e22367585a60afa421e78707d2c6a1de) feat: enable aqtion device driver
* [`b88127a`](https://github.com/talos-systems/pkgs/commit/b88127afec39d3039e93dfd6bc20a62415d396f0) chore: update tools
* [`971735f`](https://github.com/talos-systems/pkgs/commit/971735f4b1914cb1c8f2575aeda9b354ecf842f6) feat: update containerd to 1.5.7
</p>
</details>

### Changes from talos-systems/tools
<details><summary>6 commits</summary>
<p>

* [`96e0231`](https://github.com/talos-systems/tools/commit/96e0231c0f01a9fe6120a941b21c40e1a37bab36) feat: update squashfs-tools to 4.5
* [`2c9c826`](https://github.com/talos-systems/tools/commit/2c9c826201dc9f4d869fc00ffac63ee10f5e1101) feat: update libseccomp to 2.5.3
* [`f713a7c`](https://github.com/talos-systems/tools/commit/f713a7cd96fb1176687f5a6c4ec7d1345bb5e568) feat: update protobuf to 3.19.1, grpc-go to 1.42.0
* [`972c5ef`](https://github.com/talos-systems/tools/commit/972c5ef413f721ab5ad835dab526850620d05003) feat: update Go to 1.17.3
* [`f63848c`](https://github.com/talos-systems/tools/commit/f63848c1a756807879f22c752155fe1f36ccec32) feat: update PCRE version and source host
* [`fab7532`](https://github.com/talos-systems/tools/commit/fab7532fd59519d62a3985684a250273a14f1893) feat: update Go to 1.17.2
</p>
</details>

### Dependency Changes

* **github.com/AlekSi/pointer**                      v1.1.0 -> v1.2.0
* **github.com/containerd/cgroups**                  v1.0.1 -> v1.0.2
* **github.com/containerd/containerd**               v1.5.5 -> v1.5.7
* **github.com/docker/docker**                       v20.10.8 -> v20.10.10
* **github.com/evanphx/json-patch**                  v4.11.0 -> v4.12.0
* **github.com/gosuri/uiprogress**                   v0.0.1 **_new_**
* **github.com/hashicorp/go-getter**                 v1.5.8 -> v1.5.9
* **github.com/hetznercloud/hcloud-go**              v1.32.0 -> v1.33.1
* **github.com/insomniacslk/dhcp**                   b95caade3eac -> ad197bcd36fd
* **github.com/jsimonetti/rtnetlink**                435639c8e6a8 -> 93da33804786
* **github.com/jxskiss/base62**                      4f11678b909b -> v1.0.0
* **github.com/mdlayher/ethtool**                    2b88debcdd43 -> 288d040e9d60
* **github.com/rivo/tview**                          ee97a7ab3975 -> badfa0f0b301
* **github.com/talos-systems/discovery-api**         v0.1.0 **_new_**
* **github.com/talos-systems/discovery-client**      v0.1.0 **_new_**
* **github.com/talos-systems/extras**                v0.6.0 -> v0.7.0-alpha.0-1-g2bb2efc
* **github.com/talos-systems/net**                   v0.3.0 -> 0abe5bdae8f8
* **github.com/talos-systems/pkgs**                  v0.8.0 -> v0.9.0-alpha.0-14-g740da24
* **github.com/talos-systems/talos/pkg/machinery**   v0.13.0 -> 000000000000
* **github.com/talos-systems/tools**                 v0.8.0 -> v0.9.0-alpha.0-5-g96e0231
* **github.com/vmware-tanzu/sonobuoy**               v0.53.2 -> v0.55.0
* **github.com/vmware/govmomi**                      v0.26.1 -> v0.27.1
* **github.com/vmware/vmw-guestinfo**                687661b8bd8e -> cc1fd90d572c
* **go.etcd.io/etcd/api/v3**                         v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/pkg/v3**                  v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/v3**                      v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/etcdutl/v3**                     v3.5.0 -> v3.5.1
* **golang.org/x/net**                               3ad01bbaa167 -> 69e39bad7dc2
* **golang.org/x/sys**                               39ccf1dd6fa6 -> 0c823b97ae02
* **golang.org/x/term**                              140adaaadfaf -> 03fcf44c2211
* **golang.zx2c4.com/wireguard/wgctrl**              0a2f4901cba6 -> 0073765f69ba
* **google.golang.org/grpc**                         v1.41.0 -> v1.42.0
* **inet.af/netaddr**                                85fa6c94624e -> c74959edd3b6
* **k8s.io/api**                                     v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/apimachinery**                            v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/client-go**                               v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/cri-api**                                 v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/kubectl**                                 v0.22.2 -> v0.23.0-alpha.4
* **k8s.io/kubelet**                                 v0.22.2 -> v0.23.0-alpha.4
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.59 -> v1.2.60
* **sigs.k8s.io/yaml**                               v1.3.0 **_new_**

Previous release can be found at [v0.13.0](https://github.com/talos-systems/talos/releases/tag/v0.13.0)

## [Talos 0.14.0-alpha.0](https://github.com/talos-systems/talos/releases/tag/v0.14.0-alpha.0) (2021-10-25)

Welcome to the v0.14.0-alpha.0 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Kexec and capabilities

When kexec support is disabled
Talos no longer drops Linux capabilities (`CAP_SYS_BOOT` and `CAP_SYS_MODULES`) for child processes.
That is helpful for advanced use-cases like Docker-in-Docker.

If you want to permanently disable kexec and capabilities dropping, pass `kexec_load_disabled=1` argument to the kernel.

For example:

```yaml
install:
  extraKernelArgs:
    - kexec_load_disabled=1
```

Please note that capabilities are dropped before machine configuration is loaded,
so disabling kexec via `machine.sysctls` will not be enough.


### Cluster Discovery

Cluster Discovery is enabled by default for Talos 0.14.
Cluster Discovery can be disabled with `talosctl gen config --with-cluster-discovery=false`.


### Log Shipping

Talos can now ship system logs to the configured destination using either JSON-over-UDP or JSON-over-TCP:
see `.machine.logging` machine configuration option.


### Component Updates

* Linux: 5.10.75
* etcd: 3.5.1
* containerd: 1.5.7
* Kubernetes: 1.23.0-alpha.0
* CoreDNS: 1.8.6

Talos is built with Go 1.17.2


### Kubernetes Upgrade Enhancements

`talosctl upgrade-k8s` now syncs all Talos manifest resources generated from templates.

So there is no need to update CoreDNS, Flannel container manually after running `upgrade-k8s` anymore.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Serge Logvinov
* Artem Chernyshev
* Spencer Smith
* Andrew Rynhard
* Branden Cash
* Gerard de Leeuw

### Changes
<details><summary>56 commits</summary>
<p>

* [`8b620653`](https://github.com/talos-systems/talos/commit/8b6206537a30be049f74f8c4c7350028e6e56c74) fix: skip generating empty `.machine.logging`
* [`60ad0063`](https://github.com/talos-systems/talos/commit/60ad006367e73f56fd69726e0044f1ce48f18a8b) fix: don't drop ability to use ambient capabilities
* [`b6b78e7f`](https://github.com/talos-systems/talos/commit/b6b78e7fef3f6ef0c566e1815d1e28f16f868c93) test: add cluster discovery integration tests
* [`97d64d16`](https://github.com/talos-systems/talos/commit/97d64d160ce7e71c3107adbd31404853f543f7cc) fix: hcloud network config changes
* [`4c76865d`](https://github.com/talos-systems/talos/commit/4c76865d0ecec726e801a4b8f87e09476481d808) feat: multiple logging improvements
* [`1d1e1df6`](https://github.com/talos-systems/talos/commit/1d1e1df643832478aaa715aea5f51ad2e61e2880) fix: handle skipped mounts correctly
* [`0a964d92`](https://github.com/talos-systems/talos/commit/0a964d921922a247293e36b5fecaab466b91d924) test: fix openstack unit-test stability
* [`72f62ac2`](https://github.com/talos-systems/talos/commit/72f62ac27b5d0a72db409fd003a7cf9c41a03d7c) chore: bump Go and Docker dependencies
* [`9c48ebe8`](https://github.com/talos-systems/talos/commit/9c48ebe8f94afa85921ee5f1c1e9315201905a92) fix: gcp fetching externalIP
* [`6c297268`](https://github.com/talos-systems/talos/commit/6c297268ce596c2a875b7c419c85317dc24d9f4f) test: fix e2e k8s version
* [`ae5af9d3`](https://github.com/talos-systems/talos/commit/ae5af9d3fad399dea95c316d94e3e66b124bfb24) feat: update Kubernetes to 1.23.0-alpha.3
* [`28d3a69e`](https://github.com/talos-systems/talos/commit/28d3a69e9d4ae7ffa231804e26af6d1f39c07afd) feat: openstack config-drive support
* [`2258bc49`](https://github.com/talos-systems/talos/commit/2258bc4918e89b3d6fcb841b2ad677f114ddba7e) test: update GCP e2e script to work with new templates
* [`36b6ace2`](https://github.com/talos-systems/talos/commit/36b6ace25378e8c4a607de6efb6b89a2d52f5cea) feat: update Linux to 5.10.75
* [`38516a54`](https://github.com/talos-systems/talos/commit/38516a5499d933a8038ce6768946ff096e7c6f98) test: update Talos versions in upgrade tests
* [`cff20ec7`](https://github.com/talos-systems/talos/commit/cff20ec78340b3855751e13f2ad0e54bd47e9989) fix: change services OOM score
* [`666a2b62`](https://github.com/talos-systems/talos/commit/666a2b6207d257edda20c9e0411b0d4cd4112aa6) feat: azure platform ipv6 support
* [`d32814e3`](https://github.com/talos-systems/talos/commit/d32814e302c370ec1e82aa2879186a034cd2a905) feat: extract JSON fields from log lines
* [`e77d81ff`](https://github.com/talos-systems/talos/commit/e77d81fff31d68f762da3741846f95a6d2303903) fix: treat literal 'unknown' as a valid machine type
* [`c8e404e3`](https://github.com/talos-systems/talos/commit/c8e404e356878f6cd819a33386b351c1c152c3f5) test: update vars for AWS cluster
* [`ad23891b`](https://github.com/talos-systems/talos/commit/ad23891b1f6b33409721528c6771304b7ab94b2c) feat: update CoreDNS version 1.8.6
* [`41299cae`](https://github.com/talos-systems/talos/commit/41299cae9961665c2bf2a642290f8309683f040d) feat: udev rules support
* [`5237fdc9`](https://github.com/talos-systems/talos/commit/5237fdc957efbb018649b866bfb756f280f589a2) feat: send JSON logs over UDP
* [`6d44587a`](https://github.com/talos-systems/talos/commit/6d44587a4d4c16defa6bb06329cdfc6e39c95188) feat: coredns service dualstack
* [`12f7888b`](https://github.com/talos-systems/talos/commit/12f7888b75fa2498e0f8305f5d6910cecad5c65c) feat: feed control plane endpoints on workers from cluster discovery
* [`431e4fb4`](https://github.com/talos-systems/talos/commit/431e4fb4b690fa4955c407d8dd8156bdecd9a2c5) chore: bump Go and Docker dependencies
* [`89f3b9f8`](https://github.com/talos-systems/talos/commit/89f3b9f8d41e33c4cb736917f418ab5cfb9edd83) feat: update etcd to 3.5.1
* [`e60469a3`](https://github.com/talos-systems/talos/commit/e60469a38cb81ace2039bae1927eb6c5f1f0ad1f) feat: initial support for JSON logging
* [`68c420e3`](https://github.com/talos-systems/talos/commit/68c420e3c96a0fdc3b3e6cd75be24cc797c48e09) feat: enable cluster discovery by default
* [`3e100aa9`](https://github.com/talos-systems/talos/commit/3e100aa97734ea809563e23fc36e19bdd3df1920) test: workaround EventsWatch test flakiness
* [`9bd4838a`](https://github.com/talos-systems/talos/commit/9bd4838ac10abbd4760da4fb905d7639a1c26f9f) chore: stop using sonobuoy CLI
* [`6ad45951`](https://github.com/talos-systems/talos/commit/6ad45951975aac48fdcc282e5a0e31344058d07e) docs: fix field names for bonding configuration
* [`d7a3b7b5`](https://github.com/talos-systems/talos/commit/d7a3b7b5b70293884d2e19c6a59b14ebcfa24397) chore: use discovery-client and discovery-api modules
* [`d6309eed`](https://github.com/talos-systems/talos/commit/d6309eed6618abd1b4efd0e3cd18a6c0df39378f) docs: create docs for Talos 0.14
* [`c0fda643`](https://github.com/talos-systems/talos/commit/c0fda6436ae27d8bbc210ee74a1128968108f6a6) fix: attempt to clean up tasks in containerd runner
* [`8cf442da`](https://github.com/talos-systems/talos/commit/8cf442daa60d911caff59d1c2c05dd77652c8b51) chore: bump tools, pkgs, extras
* [`0dad5f4d`](https://github.com/talos-systems/talos/commit/0dad5f4d7846f3fb41ff4ba27395023d33796a61) chore: small cleanup
* [`e3e2113a`](https://github.com/talos-systems/talos/commit/e3e2113adc058940725b1041827d7adb8895c6cf) feat: upgrade CoreDNS during `upgrade-k8s` call
* [`d92c98e1`](https://github.com/talos-systems/talos/commit/d92c98e19a054472bff3e0d646756f16c5e65bbf) docs: fix discovery service documentation link
* [`e44b11c5`](https://github.com/talos-systems/talos/commit/e44b11c595e4cab796128a932843b90734ff6d1d) feat: update containerd to 1.5.7, bump Go dependencies
* [`24129307`](https://github.com/talos-systems/talos/commit/24129307a14d6e59c6bc0d3586c0c95969bde679) docs: make Talos 0.13 docs latest, update documentation
* [`31b6e39e`](https://github.com/talos-systems/talos/commit/31b6e39e58a27e1f2c1be500fca8636971bfa5c6) fix: delete expired affiliates from the discovery service
* [`877a2b6f`](https://github.com/talos-systems/talos/commit/877a2b6fc00eaa7574349f9086d78c04df163840) test: bump CAPI components to v1alpha4
* [`2ba0e0ac`](https://github.com/talos-systems/talos/commit/2ba0e0ac4ad460409101f5f2374e66698adbba4c) docs: add KubeSpan documentation
* [`997873b6`](https://github.com/talos-systems/talos/commit/997873b6d3116b59ebb46df66b8aa1cee06df92f) fix: use ECDSA-SHA512 when generating certs for Talos < 0.13
* [`7137166d`](https://github.com/talos-systems/talos/commit/7137166d1d5817e2d44ead4a01796275f92a9d4a) fix: allow overriding `audit-policy-file` in `kube-apiserver` static pod
* [`8fcd4219`](https://github.com/talos-systems/talos/commit/8fcd4219671a9359880ba344a2ec7fd65dfe5e2a) chore: fix integration-qemu-race
* [`91a858b5`](https://github.com/talos-systems/talos/commit/91a858b53704ede86392fe3c155ce9ab3c2d406f) fix: sort output of the argument builder
* [`657f7a56`](https://github.com/talos-systems/talos/commit/657f7a56b10089e0dc551e178bc85b28d8003243) fix: use ECDSA-SHA256 signature algorithm for Kubernetes certs
* [`983d2459`](https://github.com/talos-systems/talos/commit/983d2459e2aa036774828f773bbaba5697665ae7) feat: suppress logging NTP sync to the console
* [`022c7335`](https://github.com/talos-systems/talos/commit/022c7335f3063675ab744454a2ad4b2c0c19bfbc) fix: add interface route if DHCP4 router is not directly routeable
* [`66a1579e`](https://github.com/talos-systems/talos/commit/66a1579ea7d2a9c4fdf15b762cd024c54b3e8ffb) fix: don't enable 'no new privs' on the system level
* [`423861cf`](https://github.com/talos-systems/talos/commit/423861cf9f99eaf034a4f0cb243d73d1275c3f38) feat: don't drop capabilities if kexec is disabled
* [`facc8c38`](https://github.com/talos-systems/talos/commit/facc8c38a021610da900a45f397aea8ddfc74f1c) docs: fix documentation for cluster discovery
* [`ce65ca4e`](https://github.com/talos-systems/talos/commit/ce65ca4e4a2994f901f01ce5ca269d6df86f0de8) chore: build using only amd64 builders
* [`e9b0f010`](https://github.com/talos-systems/talos/commit/e9b0f010d2855b968a5d8b8b5fbcd268e06ba302) chore: update docker image in the pipeline
</p>
</details>

### Changes from talos-systems/discovery-api
<details><summary>2 commits</summary>
<p>

* [`db279ef`](https://github.com/talos-systems/discovery-api/commit/db279ef42a1fad2e1feb4902150b4969f7082c81) feat: initial set of APIs and generated files
* [`ac52a37`](https://github.com/talos-systems/discovery-api/commit/ac52a378211475ebd281dcbb00954eec42459778) chore: initial commit
</p>
</details>

### Changes from talos-systems/discovery-client
<details><summary>2 commits</summary>
<p>

* [`a9a5e9b`](https://github.com/talos-systems/discovery-client/commit/a9a5e9bfddaa670e0fb4f57510167d377cf09b07) feat: initial client code
* [`98eb999`](https://github.com/talos-systems/discovery-client/commit/98eb9999c0c76d2f93378108b7e22de6bcae6e81) chore: initial commit
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`d6e8b3a`](https://github.com/talos-systems/extras/commit/d6e8b3a78e9a3371472753286c559627932466c3) chore: update pkgs and tools
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>8 commits</summary>
<p>

* [`80a63d4`](https://github.com/talos-systems/pkgs/commit/80a63d4cf2231383266f244f608a958b94872a99) feat: update Linux to 5.10.75
* [`5c98efd`](https://github.com/talos-systems/pkgs/commit/5c98efd95d2e2e036d845c63b6268583d853d3fd) feat: add QLogic QED 25/40/100Gb Ethernet NIC driver
* [`bfb2365`](https://github.com/talos-systems/pkgs/commit/bfb2365b04aa7f92ef87799c47ffde6bc2395785) feat: enable driver for SuperMicro raid controller
* [`657e16b`](https://github.com/talos-systems/pkgs/commit/657e16b3976ba376401797277e85dd67c9b7e64e) feat: enable Intel VMD driver
* [`f7d9d72`](https://github.com/talos-systems/pkgs/commit/f7d9d728d468b9e3af2552595c9fb145f9008ef3) feat: enable smarpqi driver and related options
* [`bca3be0`](https://github.com/talos-systems/pkgs/commit/bca3be04e22367585a60afa421e78707d2c6a1de) feat: enable aqtion device driver
* [`b88127a`](https://github.com/talos-systems/pkgs/commit/b88127afec39d3039e93dfd6bc20a62415d396f0) chore: update tools
* [`971735f`](https://github.com/talos-systems/pkgs/commit/971735f4b1914cb1c8f2575aeda9b354ecf842f6) feat: update containerd to 1.5.7
</p>
</details>

### Changes from talos-systems/tools
<details><summary>1 commit</summary>
<p>

* [`fab7532`](https://github.com/talos-systems/tools/commit/fab7532fd59519d62a3985684a250273a14f1893) feat: update Go to 1.17.2
</p>
</details>

### Dependency Changes

* **github.com/AlekSi/pointer**                      v1.1.0 -> v1.2.0
* **github.com/containerd/cgroups**                  v1.0.1 -> v1.0.2
* **github.com/containerd/containerd**               v1.5.5 -> v1.5.7
* **github.com/docker/docker**                       v20.10.8 -> v20.10.9
* **github.com/hashicorp/go-getter**                 v1.5.8 -> v1.5.9
* **github.com/insomniacslk/dhcp**                   b95caade3eac -> 509557e9f781
* **github.com/jsimonetti/rtnetlink**                435639c8e6a8 -> e34540a94caa
* **github.com/jxskiss/base62**                      4f11678b909b -> v1.0.0
* **github.com/rivo/tview**                          ee97a7ab3975 -> 5508f4b00266
* **github.com/talos-systems/discovery-api**         v0.1.0 **_new_**
* **github.com/talos-systems/discovery-client**      v0.1.0 **_new_**
* **github.com/talos-systems/extras**                v0.6.0 -> v0.7.0-alpha.0
* **github.com/talos-systems/pkgs**                  v0.8.0 -> v0.9.0-alpha.0-7-g80a63d4
* **github.com/talos-systems/talos/pkg/machinery**   v0.13.0 -> 000000000000
* **github.com/talos-systems/tools**                 v0.8.0 -> v0.9.0-alpha.0
* **github.com/vmware-tanzu/sonobuoy**               v0.53.2 -> v0.54.0
* **github.com/vmware/govmomi**                      v0.26.1 -> v0.27.1
* **github.com/vmware/vmw-guestinfo**                687661b8bd8e -> cc1fd90d572c
* **go.etcd.io/etcd/api/v3**                         v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/pkg/v3**                  v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/client/v3**                      v3.5.0 -> v3.5.1
* **go.etcd.io/etcd/etcdutl/v3**                     v3.5.0 -> v3.5.1
* **golang.org/x/net**                               3ad01bbaa167 -> d418f374d309
* **golang.org/x/sys**                               39ccf1dd6fa6 -> d6a326fbbf70
* **golang.org/x/term**                              140adaaadfaf -> 03fcf44c2211
* **golang.zx2c4.com/wireguard/wgctrl**              0a2f4901cba6 -> 5be1d6054c42
* **k8s.io/api**                                     v0.22.2 -> v0.23.0-alpha.3
* **k8s.io/apimachinery**                            v0.22.2 -> v0.23.0-alpha.3
* **k8s.io/client-go**                               v0.22.2 -> v0.23.0-alpha.3
* **k8s.io/cri-api**                                 v0.22.2 -> v0.23.0-alpha.3
* **k8s.io/kubectl**                                 v0.22.2 -> v0.23.0-alpha.3
* **k8s.io/kubelet**                                 v0.22.2 -> v0.23.0-alpha.3
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.59 -> v1.2.60
* **sigs.k8s.io/yaml**                               v1.3.0 **_new_**

Previous release can be found at [v0.13.0](https://github.com/talos-systems/talos/releases/tag/v0.13.0)

## [Talos 0.13.0-alpha.3](https://github.com/talos-systems/talos/releases/tag/v0.13.0-alpha.3) (2021-09-29)

Welcome to the v0.13.0-alpha.3 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Hetzner, Scaleway, Upcloud and Vultr

Talos now natively supports three new cloud platforms:

* [Hetzner](https://www.hetzner.com/), including VIP support
* [Scaleway](https://www.scaleway.com/en/)
* [Upcloud](https://upcloud.com/)
* [Vultr](https://www.vultr.com/)

Also generic `cloud-init` `nocloud` platform is supported in both networking and storage-based modes.


### etcd Advertised Address

The address advertised by etcd can now be controlled with new machine configuration option `machine.etcd.subnet`.


### Reboots via kexec

Talos now reboots by default via kexec syscall which means BIOS POST process is skipped.
On bare-metal hardware BIOS POST process might take 10-15 minutes, so Talos reboots 10-15 minutes faster on bare-metal.

Kexec support can be disabled with the following change to the machine configuration:

```
machine:
  sysctls:
    kernel.kexec_load_disabled: "1"
```


### Cluster Discovery and KubeSpan

This release of Talos provides initial support for cluster membership discovery and [KubeSpan](https://www.talos-systems.com/kubespan/).

These new features are not enabled by default, to enable them please make following changes to the machine configuration:

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```



### Windows Suport

CLI tool talosctl is now built for Windows and published as part of the release.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Seán C McCord
* Serge Logvinov
* Alexey Palazhchenko
* Andrew Rynhard
* Olli Janatuinen
* Andrey Smirnov
* Lennard Klein
* Rui Lopes
* Spencer Smith

### Changes
<details><summary>100 commits</summary>
<p>

* [`4044372e`](https://github.com/talos-systems/talos/commit/4044372e12ff5308ba9cb9178a7e6b3b32955aab) feat: harvest discovered endpoints and push them via discovery svc
* [`9a51aa83`](https://github.com/talos-systems/talos/commit/9a51aa83581b25bdb0604904027a4cedf21b8123) feat: add an option to skip downed peers in KubeSpan
* [`cbbd7c68`](https://github.com/talos-systems/talos/commit/cbbd7c68219808a4f4b0d805203326019ce14ec9) feat: publish node's ExternalIPs as node addresses
* [`0f60ef6d`](https://github.com/talos-systems/talos/commit/0f60ef6d38f9f5978a19e0ca4c6729af03a11f0e) fix: reset inputs back to initial state in secrets.APIController
* [`64cb873e`](https://github.com/talos-systems/talos/commit/64cb873ec4421d43b291acb8afe75f65728d5732) feat: override static pods default args by extra Args
* [`ecdd7757`](https://github.com/talos-systems/talos/commit/ecdd7757fb5906d6fa904581efff74a16b22ae4b) test: workaround race in the tests with zaptest package
* [`9c67fde7`](https://github.com/talos-systems/talos/commit/9c67fde759de1e2a9f2b4406d85485d3d71c3d99) release(v0.13.0-alpha.2): prepare release
* [`30ae7142`](https://github.com/talos-systems/talos/commit/30ae714243379aaa3fb1e93023c2249ff3c3b4e3) feat: implement integration with Discovery Service
* [`353d632a`](https://github.com/talos-systems/talos/commit/353d632ae5d944a8662f0746ff8e757a67ffca53) feat: add nocloud platform support
* [`628fbf9b`](https://github.com/talos-systems/talos/commit/628fbf9b48d98df1063285b14958c94d246ce102) chore: update Linux to 5.10.69
* [`62acd625`](https://github.com/talos-systems/talos/commit/62acd6251637250dbea7d408d8cd4d5eb1f18713) fix: check trustd API CA on worker nodes
* [`ba27bc36`](https://github.com/talos-systems/talos/commit/ba27bc366fb3166b22f1bda909b9ede486ad8c7d) feat: implement Hetzner Cloud support for virtual (shared) IP
* [`95f440ea`](https://github.com/talos-systems/talos/commit/95f440eaa06d2a558fc828c11b451b6aed8d5855) test: add fuzz test for configloader
* [`d2cf021d`](https://github.com/talos-systems/talos/commit/d2cf021d8ffb6d6188b2d50f1f7b9c24df0aac84) chore: remove deprecated "join" term
* [`0e18e280`](https://github.com/talos-systems/talos/commit/0e18e2800fc038a86ed2fd9b042278ae29070bb5) chore: bump dependencies
* [`b450b7ce`](https://github.com/talos-systems/talos/commit/b450b7cef0d84a9ad975d8b50b93854bb0645173) chore: deprecate Interfaces and Routes APIs
* [`cddcb962`](https://github.com/talos-systems/talos/commit/cddcb9622bce7ae3626b8b9dce8c622a0e30ba66) fix: find devices without partition table
* [`b1b6d613`](https://github.com/talos-systems/talos/commit/b1b6d61365c900c4ebfc377b86067ddbe4fe8353) fix: check for existence of dhcp6 FQDN first
* [`519999b8`](https://github.com/talos-systems/talos/commit/519999b8462ff4931ed12323417b9a9c8c20b369) fix: use readonly mode when probing devices with `All` lookup
* [`2b520420`](https://github.com/talos-systems/talos/commit/2b5204200a4bd22aa78245b201c471136016ce3a) feat: enable resource API in the maintenance mode
* [`452893c2`](https://github.com/talos-systems/talos/commit/452893c260b920c601b0fc22ff018dc2d4341fca) fix: make probe open blockdevice in readonly mode
* [`96bccdd3`](https://github.com/talos-systems/talos/commit/96bccdd3b625f0edefd685cadf5f2cd46e3111f5) test: update CABPT provider to 0.3 release
* [`d9eb18bf`](https://github.com/talos-systems/talos/commit/d9eb18bfddf69a61712d930b53aec489a806394a) fix: containerd log symlink
* [`efa7f48e`](https://github.com/talos-systems/talos/commit/efa7f48e08382249609e0ecd3241c01a2e46df73) docs: quicklinks on landing page
* [`1cb9f282`](https://github.com/talos-systems/talos/commit/1cb9f282b541505f2d61ae0a57655cba9ae62843) fix: don't marshal clock with SecretsBundle
* [`b27c75b3`](https://github.com/talos-systems/talos/commit/b27c75b30f689dafa7d4effd0c2eaf8f0f3f8caf) release(v0.13.0-alpha.1): prepare release
* [`9d803d75`](https://github.com/talos-systems/talos/commit/9d803d75bfbe788fa5c2ef2ae0639de31e172c7b) chore: bump dependencies and drop firecracker support
* [`50a24104`](https://github.com/talos-systems/talos/commit/50a24104820e26bb99e66ab68be2bd9a6c17b0be) feat: add operating system version field to discovery
* [`085c61b2`](https://github.com/talos-systems/talos/commit/085c61b2ec432c586daa77464910e967a223ebe0) chore: add a special condition to check for kubeconfig readiness
* [`21cdd854`](https://github.com/talos-systems/talos/commit/21cdd854036498fbeb9f6e4d058a0edd55ed4856) fix: add node address to the list of allowed IPs (kubespan)
* [`fdd80a12`](https://github.com/talos-systems/talos/commit/fdd80a1234dc993cc01daa7764ba5a9db2fdc275) feat: add an option to continue booting on NTP timeout
* [`ef368498`](https://github.com/talos-systems/talos/commit/ef36849899b18bbb35c6116fdf35aa580a50a5e5) feat: add routes, routing rules and nftables rules for KubeSpan
* [`ed12379f`](https://github.com/talos-systems/talos/commit/ed12379f2f49fcbca84080f1066cf52dc202bd2d) fix: patch multi nodes support
* [`d943bb0e`](https://github.com/talos-systems/talos/commit/d943bb0e280e90f3592d9f7b67813b7a15818c84) feat: update Kubernetes to 1.22.2
* [`d0585fb6`](https://github.com/talos-systems/talos/commit/d0585fb6b303dfdd7fc80a76024915df31c72389) feat: reboot via kexec
* [`3de505c8`](https://github.com/talos-systems/talos/commit/3de505c894274bfd5248b6c597f6e3a53f873ba1) fix: skip bad cloud-config in OpenStack platform
* [`a394d1e2`](https://github.com/talos-systems/talos/commit/a394d1e20ba82de7d05e4d3f91823a98362ac9ee) fix: tear down control plane static pods when etcd is stopped
* [`1c05089b`](https://github.com/talos-systems/talos/commit/1c05089bb22c7c1050e95cf8d7bea8b763a0e86f) feat: implement KubeSpan manager for Wireguard peer state
* [`ec7f44ef`](https://github.com/talos-systems/talos/commit/ec7f44efe4f89e7ed207cbd5fe3748953ccfdf28) fix: completely prevent editing resources other than mc
* [`19a8ae97`](https://github.com/talos-systems/talos/commit/19a8ae97c69949f7c2421154b2ae4e52a905ff63) feat: add vultr.com cloud support
* [`0ff4c7cd`](https://github.com/talos-systems/talos/commit/0ff4c7cdb2b9505823f4c4504ec9bf4d7fddf5c5) fix: write KubernetesCACert chmodded 0400 instead of 0500
* [`a1c9d649`](https://github.com/talos-systems/talos/commit/a1c9d64907cce75bcb566f3ee394734e29b3932d) fix: update the way results are retrieved for certified conformance
* [`a0594540`](https://github.com/talos-systems/talos/commit/a0594540451a7636f8cd4bbe835913d31f66d0de) chore: build using Go 1.17
* [`7c5045bd`](https://github.com/talos-systems/talos/commit/7c5045bd929fcf5028cae3840970e692ef3bc7c9) release(v0.13.0-alpha.0): prepare release
* [`ee2dce6c`](https://github.com/talos-systems/talos/commit/ee2dce6c1a0e8838e587a9136afd1b7381000432) chore: bump dependencies
* [`ef022959`](https://github.com/talos-systems/talos/commit/ef022959280f156d6311836ef9cc2d01e5e3ae7d) fix: print etcd member ID in hex
* [`5ca1fb82`](https://github.com/talos-systems/talos/commit/5ca1fb822125483be290e79d8828bba246fda51c) fix: multiple fixes for KubeSpan and Wireguard implementation
* [`b1bd6425`](https://github.com/talos-systems/talos/commit/b1bd64250820df3fcb5214368ce9c8cf4634970a) fix: build platform images
* [`3b5f4038`](https://github.com/talos-systems/talos/commit/3b5f4038de2f855b3b634e4abb1c564da624e2fc) feat: add scaleway.com cloud support
* [`f156ab18`](https://github.com/talos-systems/talos/commit/f156ab1847f2ad1ca2a2548b299a713ee5fe0fcd) feat: add upcloud.com cloud support
* [`c3b2429c`](https://github.com/talos-systems/talos/commit/c3b2429ce91edc4f8f9e720a4b144bc941046fc3) fix: suppress spurious Kubernetes API server cert updates
* [`ff90b575`](https://github.com/talos-systems/talos/commit/ff90b5751e17a60fc6ca4274f35da7ddcca44fea) feat: implement KubeSpan peer generation controller
* [`14c69df5`](https://github.com/talos-systems/talos/commit/14c69df5063e71765b9316ae37657fda2388c60e) fix: correctly parse multiple pod/service CIDRs
* [`69897dbb`](https://github.com/talos-systems/talos/commit/69897dbba402812403c0c15d6cb8d2a771ea5a88) feat: drop some capabilities to be never available
* [`51e9836b`](https://github.com/talos-systems/talos/commit/51e9836b01926d1619d662e6e08df29210ff94e5) docs: promote 0.12 docs to be the latest
* [`812d59c7`](https://github.com/talos-systems/talos/commit/812d59c70085b54136e3b56127b0efea7ddb60af) feat: add hetzner.com cloud support
* [`d53e9e89`](https://github.com/talos-systems/talos/commit/d53e9e89633258d85c2232b85855535ebb42c417) chore: use named constants
* [`2dfe7f1f`](https://github.com/talos-systems/talos/commit/2dfe7f1fc654c8bec83b632a98dbaa8d1b90a521) chore: bump tools to the latest version
* [`82b130e7`](https://github.com/talos-systems/talos/commit/82b130e789aa4376e1f0e2d086233e630b410f74) docs: document required options for extraMounts
* [`af662210`](https://github.com/talos-systems/talos/commit/af6622109faecdf03aed43b047035904110c7580) feat: implement Kubernetes cluster discovery registry
* [`2c66e1b3`](https://github.com/talos-systems/talos/commit/2c66e1b3c5d4c34c5d4cdc155c32f2808a5f1c69) feat: provide building of local `Affiliate` structure (for the node)
* [`d69bd2af`](https://github.com/talos-systems/talos/commit/d69bd2af3e3d3bf12b6d74078e9eedf3dc8752fc) chore: enable GPG identity check for Talos
* [`8dbd851f`](https://github.com/talos-systems/talos/commit/8dbd851fde3febb5999df694a079121b43519aa9) chore: update tools/pkgs/extras to the new version
* [`0b347570`](https://github.com/talos-systems/talos/commit/0b347570a7aca0a133d6b6e6cc8d3e0355630480) feat: use dynamic NodeAddresses/HostnameStatus in Kubernetes certs
* [`bd5b9c96`](https://github.com/talos-systems/talos/commit/bd5b9c96e2563249a5633433703493b292b83ee9) fix: correctly define example for `extraMounts`
* [`01cca099`](https://github.com/talos-systems/talos/commit/01cca099f40ec75d1e047a84c89692eb254e8adf) docs: update docs for Talos 0.12 release
* [`668627d5`](https://github.com/talos-systems/talos/commit/668627d5b8ec79ec955eb1254732b1cc031d3aec) feat: add subnet filter for etcd address
* [`3c3c281b`](https://github.com/talos-systems/talos/commit/3c3c281bff8481f680feca9cf01af413a38e6973) chore: bump dependencies via dependabot
* [`f8bebba2`](https://github.com/talos-systems/talos/commit/f8bebba2de3999b7a36fecb2d6b90e583372c98f) fix: ignore error on duplicate for `MountStatus`
* [`6956edd0`](https://github.com/talos-systems/talos/commit/6956edd0bfae6c6c5d6eba00a22bc3a4cb7f54ea) feat: add node address filters, filter out k8s addresses for Talos API
* [`caee24bf`](https://github.com/talos-systems/talos/commit/caee24bf61136daecb095991a6e439f7fbf40da2) feat: implement KubeSpan identity controller
* [`da0f6e7e`](https://github.com/talos-systems/talos/commit/da0f6e7e1d295dce0c44c1854363528a6ffedde1) fix: allow updating diskSelector option
* [`761ccaf3`](https://github.com/talos-systems/talos/commit/761ccaf32348d8664eb0d5d1a51f6abb19ca52a6) feat: provide machine configuration for KubeSpan and cluster discovery
* [`a81e30cb`](https://github.com/talos-systems/talos/commit/a81e30cb46326fbdd433f37dc37549b588a2bc7a) docs: add bootstrap command to VMware docs
* [`97da354c`](https://github.com/talos-systems/talos/commit/97da354cc0e4a965e14b8939c426150d5c12f228) fix: do not panic on invalid machine configs
* [`c4048e26`](https://github.com/talos-systems/talos/commit/c4048e263d22682142f12fc4af6ac58c679273f0) fix: don't extract nil IPs in the GCP platform
* [`ba169c6f`](https://github.com/talos-systems/talos/commit/ba169c6f91948cf057251236fa7a727a05253639) feat: provide talosctl.exe for Windows
* [`6312f473`](https://github.com/talos-systems/talos/commit/6312f473e63df50287e6801c079242e2311a23e6) fix: properly handle omitempty fields in the validator
* [`7f22879a`](https://github.com/talos-systems/talos/commit/7f22879af0882af4cdebe9c84afb96ae68eb9f20) feat: provide random node identity
* [`032e7c6b`](https://github.com/talos-systems/talos/commit/032e7c6b863b5ca02cfa16df79c88950544dbffb) chore: import yaml.v3 consistently
* [`80b5f0e7`](https://github.com/talos-systems/talos/commit/80b5f0e7f78f09a11ed249f9f1dc7b05ea275ab0) fix: validate IP address returned as HTTP response in platform code
* [`c9af8f7f`](https://github.com/talos-systems/talos/commit/c9af8f7ff17facc18f10675879ed04982a000f6f) docs: fork docs for 0.13
* [`85cda1b9`](https://github.com/talos-systems/talos/commit/85cda1b956b042ba20696637248999d46f63ccc9) feat: provide MountStatus resource for system partition mounts
* [`950f122c`](https://github.com/talos-systems/talos/commit/950f122c95e225858e77083f2490481ed8d21aef) chore: update versions in upgrade tests
* [`83fdb772`](https://github.com/talos-systems/talos/commit/83fdb7721f45aa075898caf05a4b6856d3c5f330) feat: provide first NIC hardware addr as a resource
* [`5f5ac12f`](https://github.com/talos-systems/talos/commit/5f5ac12f1dc8aeb3a8598e57d965471e93fe3724) fix: properly case the VMware name
* [`0a6048f4`](https://github.com/talos-systems/talos/commit/0a6048f469da02efad7e84eb237e6fdeb85b7e33) fix: don't allow bootstrap if etcd data directory is not empty
* [`e24b93b4`](https://github.com/talos-systems/talos/commit/e24b93b4e120448f37109599f3e9eb15954b147a) fix: cgroup delegate
* [`751f64f9`](https://github.com/talos-systems/talos/commit/751f64f9bc10e9ad8508ade9e3a6a14aaaa54d57) docs: add release notes for 0.12, support matrix
* [`57a77696`](https://github.com/talos-systems/talos/commit/57a77696ef2b255a59ee4ed213a1a3971a5e2943) feat: update Kubernetes to 1.22.1
* [`244b08cc`](https://github.com/talos-systems/talos/commit/244b08cc198a8ba676bb9acadcbdd23a161b0876) chore: bump dependencies
* [`576ba195`](https://github.com/talos-systems/talos/commit/576ba195784abf275256c861d5f811ab1f7b1102) fix: do not set KSPP kernel params in container mode
* [`b8c92ede`](https://github.com/talos-systems/talos/commit/b8c92ede52ed515dba68abf4fb1cc6494d510827) fix: don't support cgroups nesting in process runner
* [`9bb0b797`](https://github.com/talos-systems/talos/commit/9bb0b79709a502ab49ea9bacd7e54617554d4cc3) test: adapt tests to the cgroupsv2
* [`1abc12be`](https://github.com/talos-systems/talos/commit/1abc12be13208ad1da03492a1b88d2c1ec0d5d33) fix: extramount should have `yaml:",inline"` tag
* [`2b614e43`](https://github.com/talos-systems/talos/commit/2b614e430e478cc111db018996ab2c8f763e4f92) feat: check if cluster has deprecated resources versions
* [`0b86edab`](https://github.com/talos-systems/talos/commit/0b86edab80cf4dd01f330d7721b130f5017d84a5) fix: don't panic if the machine config doesn't have network (EM)
* [`8bef41e4`](https://github.com/talos-systems/talos/commit/8bef41e4bacc4190976657ae5021afecd2d6e001) fix: make sure file mode is same (reproducibility issue)
* [`fcfca55a`](https://github.com/talos-systems/talos/commit/fcfca55a059e92fcda198baa321c4c63bda1f0a4) chore: do not check that go mod tidy gives empty output
* [`5ce92ca5`](https://github.com/talos-systems/talos/commit/5ce92ca5163616fcd7abe16c4efc3a100953b246) docs: ensure azure VMs are 0 indexed
</p>
</details>

### Changes since v0.13.0-alpha.2
<details><summary>6 commits</summary>
<p>

* [`4044372e`](https://github.com/talos-systems/talos/commit/4044372e12ff5308ba9cb9178a7e6b3b32955aab) feat: harvest discovered endpoints and push them via discovery svc
* [`9a51aa83`](https://github.com/talos-systems/talos/commit/9a51aa83581b25bdb0604904027a4cedf21b8123) feat: add an option to skip downed peers in KubeSpan
* [`cbbd7c68`](https://github.com/talos-systems/talos/commit/cbbd7c68219808a4f4b0d805203326019ce14ec9) feat: publish node's ExternalIPs as node addresses
* [`0f60ef6d`](https://github.com/talos-systems/talos/commit/0f60ef6d38f9f5978a19e0ca4c6729af03a11f0e) fix: reset inputs back to initial state in secrets.APIController
* [`64cb873e`](https://github.com/talos-systems/talos/commit/64cb873ec4421d43b291acb8afe75f65728d5732) feat: override static pods default args by extra Args
* [`ecdd7757`](https://github.com/talos-systems/talos/commit/ecdd7757fb5906d6fa904581efff74a16b22ae4b) test: workaround race in the tests with zaptest package
</p>
</details>

### Changes from talos-systems/discovery-service
<details><summary>17 commits</summary>
<p>

* [`b2e2079`](https://github.com/talos-systems/discovery-service/commit/b2e2079088a5b08d0466b36e0f56927fe4524d78) fix: properly encrypt IPv6 endpoints
* [`e9d5dfa`](https://github.com/talos-systems/discovery-service/commit/e9d5dfa15e92fc1bdee620b2687eecd2ff929ac3) fix: enable connections to endpoints with public certs
* [`509e9b2`](https://github.com/talos-systems/discovery-service/commit/509e9b2ced989631224afd3bfd5e8bdcbcf13137) feat: implement client wrapper around discovery service API
* [`6195466`](https://github.com/talos-systems/discovery-service/commit/619546696afe99c1b58b46ca819bb32e74560f5b) feat: enable vtprotobuf, watch batching, more limits
* [`7174ec1`](https://github.com/talos-systems/discovery-service/commit/7174ec10426ff17172aaa195423e2588b23aa868) feat: implement new discovery service
* [`1a43970`](https://github.com/talos-systems/discovery-service/commit/1a4397082659a2b6bca6fd53a5080e109d978627) feat: add node and cluster validation
* [`6454cfc`](https://github.com/talos-systems/discovery-service/commit/6454cfcb726bba64823f25d962bc04007d057a8b) refactor: kresify, fix linter and rename to Kubespan manager
* [`d782452`](https://github.com/talos-systems/discovery-service/commit/d782452e86bf94c1194f806384fc339835a96d2d) add redis database backend
* [`924fed4`](https://github.com/talos-systems/discovery-service/commit/924fed4ecc939887c55a4a8a5f5dfe737ca29c78) refactor to flexible addresses
* [`cd02b5a`](https://github.com/talos-systems/discovery-service/commit/cd02b5a36783454f7650df5709984f01ccc8bd6e) revert to string IDs
* [`576288f`](https://github.com/talos-systems/discovery-service/commit/576288fc7559f9eac1966627b6c1e2c731e0c322) add self-reported IPs
* [`6ad15ca`](https://github.com/talos-systems/discovery-service/commit/6ad15ca825bdf4636855f2b77535945d07462a17) strong typing and known endpoint API
* [`3437ff2`](https://github.com/talos-systems/discovery-service/commit/3437ff28fb9ab6efb96cc79da1c75bd81bfb50cc) fixes from testing
* [`d3fd1f3`](https://github.com/talos-systems/discovery-service/commit/d3fd1f324d7af60f2ddf425c9c2cf9b9743e6d62) add Name to Node
* [`eb0e8ba`](https://github.com/talos-systems/discovery-service/commit/eb0e8baadacd702044ca419f933dd409ae58977e) add simple client pkg
* [`5e0c1df`](https://github.com/talos-systems/discovery-service/commit/5e0c1df0f1823cce1b92ccffb1e18fb1e540117d) add cluster hash grouping
* [`f982696`](https://github.com/talos-systems/discovery-service/commit/f982696ec1b45f8d7e6d2dfa1d780b3b0d515dbb) initial commit
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`52b27da`](https://github.com/talos-systems/extras/commit/52b27dad5aeeb5d14225a99e4b5902614c993022) chore: update pkgs and tools to 0.8.0-alpha.0
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>6 commits</summary>
<p>

* [`70d2865`](https://github.com/talos-systems/go-blockdevice/commit/70d28650b398a14469cbb5356417355b0ba62956) fix: try to find cdrom disks
* [`667bf53`](https://github.com/talos-systems/go-blockdevice/commit/667bf539b99ac34b629a0103ef7a7278a5a5f35d) fix: revert gpt partition not found
* [`d7d4cdd`](https://github.com/talos-systems/go-blockdevice/commit/d7d4cdd7ac56c82caab19246b5decd59f12195eb) fix: gpt partition not found
* [`33afba3`](https://github.com/talos-systems/go-blockdevice/commit/33afba347c0dce38a436c46a0aac26d2f99427c1) fix: also open in readonly mode when running `All` lookup method
* [`e367f9d`](https://github.com/talos-systems/go-blockdevice/commit/e367f9dc7fa935f11672de0fdc8a89429285a07a) feat: make probe always open blockdevices in readonly mode
* [`d981156`](https://github.com/talos-systems/go-blockdevice/commit/d9811569588ba44be878a00ce316f59a37abed8b) fix: allow Build for Windows
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>7 commits</summary>
<p>

* [`28cda67`](https://github.com/talos-systems/pkgs/commit/28cda67f9b518cb4d4a299bac62be1c3ed3cddda) feat: update Linux kernel to 5.10.69
* [`db90f93`](https://github.com/talos-systems/pkgs/commit/db90f93c0b462dcaefa081c18f8adebae5d5744a) chore: update tools
* [`ca38c59`](https://github.com/talos-systems/pkgs/commit/ca38c599b71334d5a108021c7e7ccec12701ff18) feat: enable KEXEC_FILE_LOAD in the kernel
* [`982bc18`](https://github.com/talos-systems/pkgs/commit/982bc18a4ecf5d4e15a73c350abb97f02adb8871) chore: update tools
* [`a243ab8`](https://github.com/talos-systems/pkgs/commit/a243ab8a9345b8bc39cc65254015b6eb07605f61) feat: add /usr/src to FHS
* [`428abdb`](https://github.com/talos-systems/pkgs/commit/428abdbfd303fce69cf583f5a8a4f5ed43253807) chore: support builds with HTTP_PROXY
* [`13151c5`](https://github.com/talos-systems/pkgs/commit/13151c59b5b29541ed5828aa9c75a061ec920ff1) chore: update bldr version, update tools
</p>
</details>

### Changes from talos-systems/tools
<details><summary>5 commits</summary>
<p>

* [`2790b55`](https://github.com/talos-systems/tools/commit/2790b5586e810c7dfc0a197ef9d1e6d77a646e3b) feat: update Go to 1.17.1
* [`5b9d214`](https://github.com/talos-systems/tools/commit/5b9d214c38515a55232ce36591036748fd8c49cc) fix: restore static library for ncurses
* [`01104e5`](https://github.com/talos-systems/tools/commit/01104e562efdbff34fb2d597d4cf27d04ba44ea6) chore: reproducible builds
* [`53fe146`](https://github.com/talos-systems/tools/commit/53fe146ca8ba55c959fee04302a5ce215a927f1d) chore: update bldr with new version
* [`bf4540d`](https://github.com/talos-systems/tools/commit/bf4540d0ed0728cd7751e0c3ab3bb4b8927e334c) chore: add patch dependency
</p>
</details>

### Dependency Changes

* **github.com/containerd/go-cni**                   v1.0.2 -> v1.1.0
* **github.com/containernetworking/cni**             v0.8.1 -> v1.0.1
* **github.com/containernetworking/plugins**         v0.9.1 -> v1.0.1
* **github.com/cosi-project/runtime**                25f235cd0682 -> 5cb7f5002d77
* **github.com/fatih/color**                         v1.12.0 -> v1.13.0
* **github.com/fsnotify/fsnotify**                   v1.4.9 -> v1.5.1
* **github.com/gdamore/tcell/v2**                    v2.4.0 -> f057f0a857a1
* **github.com/google/nftables**                     16a134723a96 **_new_**
* **github.com/hashicorp/go-getter**                 v1.5.7 -> v1.5.8
* **github.com/hetznercloud/hcloud-go**              v1.32.0 **_new_**
* **github.com/insomniacslk/dhcp**                   1cac67f12b1e -> b95caade3eac
* **github.com/jsimonetti/rtnetlink**                9c52e516c709 -> 435639c8e6a8
* **github.com/jxskiss/base62**                      4f11678b909b **_new_**
* **github.com/mattn/go-isatty**                     v0.0.13 -> v0.0.14
* **github.com/mdlayher/netx**                       669a06fde734 **_new_**
* **github.com/packethost/packngo**                  v0.19.0 -> v0.19.1
* **github.com/prometheus/procfs**                   v0.7.2 -> v0.7.3
* **github.com/rivo/tview**                          29d673af0ce2 -> ee97a7ab3975
* **github.com/scaleway/scaleway-sdk-go**            v1.0.0-beta.7 **_new_**
* **github.com/talos-systems/discovery-service**     b2e2079088a5 **_new_**
* **github.com/talos-systems/extras**                v0.5.0 -> v0.6.0-alpha.0
* **github.com/talos-systems/go-blockdevice**        v0.2.3 -> 70d28650b398
* **github.com/talos-systems/pkgs**                  v0.7.0 -> v0.8.0-alpha.0-4-g28cda67
* **github.com/talos-systems/tools**                 v0.7.0-1-ga33ccc1 -> v0.8.0-alpha.0-3-g2790b55
* **github.com/vishvananda/netlink**                 f5de75959ad5 **_new_**
* **github.com/vmware-tanzu/sonobuoy**               v0.53.1 -> v0.53.2
* **github.com/vmware/govmomi**                      v0.26.0 -> v0.26.1
* **github.com/vultr/metadata**                      v1.0.3 **_new_**
* **go.uber.org/zap**                                v1.19.0 -> v1.19.1
* **golang.org/x/net**                               853a461950ff -> 3ad01bbaa167
* **golang.org/x/sys**                               0f9fa26af87c -> 39ccf1dd6fa6
* **golang.org/x/term**                              6886f2dfbf5b -> 140adaaadfaf
* **golang.zx2c4.com/wireguard/wgctrl**              92e472f520a5 -> 0a2f4901cba6
* **google.golang.org/grpc**                         v1.40.0 -> v1.41.0
* **inet.af/netaddr**                                ce7a8ad02cc1 -> 85fa6c94624e
* **k8s.io/api**                                     v0.22.1 -> v0.22.2
* **k8s.io/apimachinery**                            v0.22.1 -> v0.22.2
* **k8s.io/client-go**                               v0.22.1 -> v0.22.2
* **k8s.io/kubectl**                                 v0.22.1 -> v0.22.2
* **k8s.io/kubelet**                                 v0.22.1 -> v0.22.2
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.59 **_new_**

Previous release can be found at [v0.12.0](https://github.com/talos-systems/talos/releases/tag/v0.12.0)

## [Talos 0.13.0-alpha.2](https://github.com/talos-systems/talos/releases/tag/v0.13.0-alpha.2) (2021-09-28)

Welcome to the v0.13.0-alpha.2 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Hetzner, Scaleway, Upcloud and Vultr

Talos now natively supports three new cloud platforms:

* [Hetzner](https://www.hetzner.com/), including VIP support
* [Scaleway](https://www.scaleway.com/en/)
* [Upcloud](https://upcloud.com/)
* [Vultr](https://www.vultr.com/)

Also generic `cloud-init` `nocloud` platform is supported in both networking and storage-based modes.


### etcd Advertised Address

The address advertised by etcd can now be controlled with new machine configuration option `machine.etcd.subnet`.


### Reboots via kexec

Talos now reboots by default via kexec syscall which means BIOS POST process is skipped.
On bare-metal hardware BIOS POST process might take 10-15 minutes, so Talos reboots 10-15 minutes faster on bare-metal.

Kexec support can be disabled with the following change to the machine configuration:

```
machine:
  sysctls:
    kernel.kexec_load_disabled: "1"
```


### Cluster Discovery and KubeSpan

This release of Talos provides initial support for cluster membership discovery and [KubeSpan](https://www.talos-systems.com/kubespan/).

These new features are not enabled by default, to enable them please make following changes to the machine configuration:

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```



### Windows Suport

CLI tool talosctl is now built for Windows and published as part of the release.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Seán C McCord
* Serge Logvinov
* Alexey Palazhchenko
* Andrew Rynhard
* Olli Janatuinen
* Andrey Smirnov
* Lennard Klein
* Rui Lopes
* Spencer Smith

### Changes
<details><summary>93 commits</summary>
<p>

* [`30ae7142`](https://github.com/talos-systems/talos/commit/30ae714243379aaa3fb1e93023c2249ff3c3b4e3) feat: implement integration with Discovery Service
* [`353d632a`](https://github.com/talos-systems/talos/commit/353d632ae5d944a8662f0746ff8e757a67ffca53) feat: add nocloud platform support
* [`628fbf9b`](https://github.com/talos-systems/talos/commit/628fbf9b48d98df1063285b14958c94d246ce102) chore: update Linux to 5.10.69
* [`62acd625`](https://github.com/talos-systems/talos/commit/62acd6251637250dbea7d408d8cd4d5eb1f18713) fix: check trustd API CA on worker nodes
* [`ba27bc36`](https://github.com/talos-systems/talos/commit/ba27bc366fb3166b22f1bda909b9ede486ad8c7d) feat: implement Hetzner Cloud support for virtual (shared) IP
* [`95f440ea`](https://github.com/talos-systems/talos/commit/95f440eaa06d2a558fc828c11b451b6aed8d5855) test: add fuzz test for configloader
* [`d2cf021d`](https://github.com/talos-systems/talos/commit/d2cf021d8ffb6d6188b2d50f1f7b9c24df0aac84) chore: remove deprecated "join" term
* [`0e18e280`](https://github.com/talos-systems/talos/commit/0e18e2800fc038a86ed2fd9b042278ae29070bb5) chore: bump dependencies
* [`b450b7ce`](https://github.com/talos-systems/talos/commit/b450b7cef0d84a9ad975d8b50b93854bb0645173) chore: deprecate Interfaces and Routes APIs
* [`cddcb962`](https://github.com/talos-systems/talos/commit/cddcb9622bce7ae3626b8b9dce8c622a0e30ba66) fix: find devices without partition table
* [`b1b6d613`](https://github.com/talos-systems/talos/commit/b1b6d61365c900c4ebfc377b86067ddbe4fe8353) fix: check for existence of dhcp6 FQDN first
* [`519999b8`](https://github.com/talos-systems/talos/commit/519999b8462ff4931ed12323417b9a9c8c20b369) fix: use readonly mode when probing devices with `All` lookup
* [`2b520420`](https://github.com/talos-systems/talos/commit/2b5204200a4bd22aa78245b201c471136016ce3a) feat: enable resource API in the maintenance mode
* [`452893c2`](https://github.com/talos-systems/talos/commit/452893c260b920c601b0fc22ff018dc2d4341fca) fix: make probe open blockdevice in readonly mode
* [`96bccdd3`](https://github.com/talos-systems/talos/commit/96bccdd3b625f0edefd685cadf5f2cd46e3111f5) test: update CABPT provider to 0.3 release
* [`d9eb18bf`](https://github.com/talos-systems/talos/commit/d9eb18bfddf69a61712d930b53aec489a806394a) fix: containerd log symlink
* [`efa7f48e`](https://github.com/talos-systems/talos/commit/efa7f48e08382249609e0ecd3241c01a2e46df73) docs: quicklinks on landing page
* [`1cb9f282`](https://github.com/talos-systems/talos/commit/1cb9f282b541505f2d61ae0a57655cba9ae62843) fix: don't marshal clock with SecretsBundle
* [`b27c75b3`](https://github.com/talos-systems/talos/commit/b27c75b30f689dafa7d4effd0c2eaf8f0f3f8caf) release(v0.13.0-alpha.1): prepare release
* [`9d803d75`](https://github.com/talos-systems/talos/commit/9d803d75bfbe788fa5c2ef2ae0639de31e172c7b) chore: bump dependencies and drop firecracker support
* [`50a24104`](https://github.com/talos-systems/talos/commit/50a24104820e26bb99e66ab68be2bd9a6c17b0be) feat: add operating system version field to discovery
* [`085c61b2`](https://github.com/talos-systems/talos/commit/085c61b2ec432c586daa77464910e967a223ebe0) chore: add a special condition to check for kubeconfig readiness
* [`21cdd854`](https://github.com/talos-systems/talos/commit/21cdd854036498fbeb9f6e4d058a0edd55ed4856) fix: add node address to the list of allowed IPs (kubespan)
* [`fdd80a12`](https://github.com/talos-systems/talos/commit/fdd80a1234dc993cc01daa7764ba5a9db2fdc275) feat: add an option to continue booting on NTP timeout
* [`ef368498`](https://github.com/talos-systems/talos/commit/ef36849899b18bbb35c6116fdf35aa580a50a5e5) feat: add routes, routing rules and nftables rules for KubeSpan
* [`ed12379f`](https://github.com/talos-systems/talos/commit/ed12379f2f49fcbca84080f1066cf52dc202bd2d) fix: patch multi nodes support
* [`d943bb0e`](https://github.com/talos-systems/talos/commit/d943bb0e280e90f3592d9f7b67813b7a15818c84) feat: update Kubernetes to 1.22.2
* [`d0585fb6`](https://github.com/talos-systems/talos/commit/d0585fb6b303dfdd7fc80a76024915df31c72389) feat: reboot via kexec
* [`3de505c8`](https://github.com/talos-systems/talos/commit/3de505c894274bfd5248b6c597f6e3a53f873ba1) fix: skip bad cloud-config in OpenStack platform
* [`a394d1e2`](https://github.com/talos-systems/talos/commit/a394d1e20ba82de7d05e4d3f91823a98362ac9ee) fix: tear down control plane static pods when etcd is stopped
* [`1c05089b`](https://github.com/talos-systems/talos/commit/1c05089bb22c7c1050e95cf8d7bea8b763a0e86f) feat: implement KubeSpan manager for Wireguard peer state
* [`ec7f44ef`](https://github.com/talos-systems/talos/commit/ec7f44efe4f89e7ed207cbd5fe3748953ccfdf28) fix: completely prevent editing resources other than mc
* [`19a8ae97`](https://github.com/talos-systems/talos/commit/19a8ae97c69949f7c2421154b2ae4e52a905ff63) feat: add vultr.com cloud support
* [`0ff4c7cd`](https://github.com/talos-systems/talos/commit/0ff4c7cdb2b9505823f4c4504ec9bf4d7fddf5c5) fix: write KubernetesCACert chmodded 0400 instead of 0500
* [`a1c9d649`](https://github.com/talos-systems/talos/commit/a1c9d64907cce75bcb566f3ee394734e29b3932d) fix: update the way results are retrieved for certified conformance
* [`a0594540`](https://github.com/talos-systems/talos/commit/a0594540451a7636f8cd4bbe835913d31f66d0de) chore: build using Go 1.17
* [`7c5045bd`](https://github.com/talos-systems/talos/commit/7c5045bd929fcf5028cae3840970e692ef3bc7c9) release(v0.13.0-alpha.0): prepare release
* [`ee2dce6c`](https://github.com/talos-systems/talos/commit/ee2dce6c1a0e8838e587a9136afd1b7381000432) chore: bump dependencies
* [`ef022959`](https://github.com/talos-systems/talos/commit/ef022959280f156d6311836ef9cc2d01e5e3ae7d) fix: print etcd member ID in hex
* [`5ca1fb82`](https://github.com/talos-systems/talos/commit/5ca1fb822125483be290e79d8828bba246fda51c) fix: multiple fixes for KubeSpan and Wireguard implementation
* [`b1bd6425`](https://github.com/talos-systems/talos/commit/b1bd64250820df3fcb5214368ce9c8cf4634970a) fix: build platform images
* [`3b5f4038`](https://github.com/talos-systems/talos/commit/3b5f4038de2f855b3b634e4abb1c564da624e2fc) feat: add scaleway.com cloud support
* [`f156ab18`](https://github.com/talos-systems/talos/commit/f156ab1847f2ad1ca2a2548b299a713ee5fe0fcd) feat: add upcloud.com cloud support
* [`c3b2429c`](https://github.com/talos-systems/talos/commit/c3b2429ce91edc4f8f9e720a4b144bc941046fc3) fix: suppress spurious Kubernetes API server cert updates
* [`ff90b575`](https://github.com/talos-systems/talos/commit/ff90b5751e17a60fc6ca4274f35da7ddcca44fea) feat: implement KubeSpan peer generation controller
* [`14c69df5`](https://github.com/talos-systems/talos/commit/14c69df5063e71765b9316ae37657fda2388c60e) fix: correctly parse multiple pod/service CIDRs
* [`69897dbb`](https://github.com/talos-systems/talos/commit/69897dbba402812403c0c15d6cb8d2a771ea5a88) feat: drop some capabilities to be never available
* [`51e9836b`](https://github.com/talos-systems/talos/commit/51e9836b01926d1619d662e6e08df29210ff94e5) docs: promote 0.12 docs to be the latest
* [`812d59c7`](https://github.com/talos-systems/talos/commit/812d59c70085b54136e3b56127b0efea7ddb60af) feat: add hetzner.com cloud support
* [`d53e9e89`](https://github.com/talos-systems/talos/commit/d53e9e89633258d85c2232b85855535ebb42c417) chore: use named constants
* [`2dfe7f1f`](https://github.com/talos-systems/talos/commit/2dfe7f1fc654c8bec83b632a98dbaa8d1b90a521) chore: bump tools to the latest version
* [`82b130e7`](https://github.com/talos-systems/talos/commit/82b130e789aa4376e1f0e2d086233e630b410f74) docs: document required options for extraMounts
* [`af662210`](https://github.com/talos-systems/talos/commit/af6622109faecdf03aed43b047035904110c7580) feat: implement Kubernetes cluster discovery registry
* [`2c66e1b3`](https://github.com/talos-systems/talos/commit/2c66e1b3c5d4c34c5d4cdc155c32f2808a5f1c69) feat: provide building of local `Affiliate` structure (for the node)
* [`d69bd2af`](https://github.com/talos-systems/talos/commit/d69bd2af3e3d3bf12b6d74078e9eedf3dc8752fc) chore: enable GPG identity check for Talos
* [`8dbd851f`](https://github.com/talos-systems/talos/commit/8dbd851fde3febb5999df694a079121b43519aa9) chore: update tools/pkgs/extras to the new version
* [`0b347570`](https://github.com/talos-systems/talos/commit/0b347570a7aca0a133d6b6e6cc8d3e0355630480) feat: use dynamic NodeAddresses/HostnameStatus in Kubernetes certs
* [`bd5b9c96`](https://github.com/talos-systems/talos/commit/bd5b9c96e2563249a5633433703493b292b83ee9) fix: correctly define example for `extraMounts`
* [`01cca099`](https://github.com/talos-systems/talos/commit/01cca099f40ec75d1e047a84c89692eb254e8adf) docs: update docs for Talos 0.12 release
* [`668627d5`](https://github.com/talos-systems/talos/commit/668627d5b8ec79ec955eb1254732b1cc031d3aec) feat: add subnet filter for etcd address
* [`3c3c281b`](https://github.com/talos-systems/talos/commit/3c3c281bff8481f680feca9cf01af413a38e6973) chore: bump dependencies via dependabot
* [`f8bebba2`](https://github.com/talos-systems/talos/commit/f8bebba2de3999b7a36fecb2d6b90e583372c98f) fix: ignore error on duplicate for `MountStatus`
* [`6956edd0`](https://github.com/talos-systems/talos/commit/6956edd0bfae6c6c5d6eba00a22bc3a4cb7f54ea) feat: add node address filters, filter out k8s addresses for Talos API
* [`caee24bf`](https://github.com/talos-systems/talos/commit/caee24bf61136daecb095991a6e439f7fbf40da2) feat: implement KubeSpan identity controller
* [`da0f6e7e`](https://github.com/talos-systems/talos/commit/da0f6e7e1d295dce0c44c1854363528a6ffedde1) fix: allow updating diskSelector option
* [`761ccaf3`](https://github.com/talos-systems/talos/commit/761ccaf32348d8664eb0d5d1a51f6abb19ca52a6) feat: provide machine configuration for KubeSpan and cluster discovery
* [`a81e30cb`](https://github.com/talos-systems/talos/commit/a81e30cb46326fbdd433f37dc37549b588a2bc7a) docs: add bootstrap command to VMware docs
* [`97da354c`](https://github.com/talos-systems/talos/commit/97da354cc0e4a965e14b8939c426150d5c12f228) fix: do not panic on invalid machine configs
* [`c4048e26`](https://github.com/talos-systems/talos/commit/c4048e263d22682142f12fc4af6ac58c679273f0) fix: don't extract nil IPs in the GCP platform
* [`ba169c6f`](https://github.com/talos-systems/talos/commit/ba169c6f91948cf057251236fa7a727a05253639) feat: provide talosctl.exe for Windows
* [`6312f473`](https://github.com/talos-systems/talos/commit/6312f473e63df50287e6801c079242e2311a23e6) fix: properly handle omitempty fields in the validator
* [`7f22879a`](https://github.com/talos-systems/talos/commit/7f22879af0882af4cdebe9c84afb96ae68eb9f20) feat: provide random node identity
* [`032e7c6b`](https://github.com/talos-systems/talos/commit/032e7c6b863b5ca02cfa16df79c88950544dbffb) chore: import yaml.v3 consistently
* [`80b5f0e7`](https://github.com/talos-systems/talos/commit/80b5f0e7f78f09a11ed249f9f1dc7b05ea275ab0) fix: validate IP address returned as HTTP response in platform code
* [`c9af8f7f`](https://github.com/talos-systems/talos/commit/c9af8f7ff17facc18f10675879ed04982a000f6f) docs: fork docs for 0.13
* [`85cda1b9`](https://github.com/talos-systems/talos/commit/85cda1b956b042ba20696637248999d46f63ccc9) feat: provide MountStatus resource for system partition mounts
* [`950f122c`](https://github.com/talos-systems/talos/commit/950f122c95e225858e77083f2490481ed8d21aef) chore: update versions in upgrade tests
* [`83fdb772`](https://github.com/talos-systems/talos/commit/83fdb7721f45aa075898caf05a4b6856d3c5f330) feat: provide first NIC hardware addr as a resource
* [`5f5ac12f`](https://github.com/talos-systems/talos/commit/5f5ac12f1dc8aeb3a8598e57d965471e93fe3724) fix: properly case the VMware name
* [`0a6048f4`](https://github.com/talos-systems/talos/commit/0a6048f469da02efad7e84eb237e6fdeb85b7e33) fix: don't allow bootstrap if etcd data directory is not empty
* [`e24b93b4`](https://github.com/talos-systems/talos/commit/e24b93b4e120448f37109599f3e9eb15954b147a) fix: cgroup delegate
* [`751f64f9`](https://github.com/talos-systems/talos/commit/751f64f9bc10e9ad8508ade9e3a6a14aaaa54d57) docs: add release notes for 0.12, support matrix
* [`57a77696`](https://github.com/talos-systems/talos/commit/57a77696ef2b255a59ee4ed213a1a3971a5e2943) feat: update Kubernetes to 1.22.1
* [`244b08cc`](https://github.com/talos-systems/talos/commit/244b08cc198a8ba676bb9acadcbdd23a161b0876) chore: bump dependencies
* [`576ba195`](https://github.com/talos-systems/talos/commit/576ba195784abf275256c861d5f811ab1f7b1102) fix: do not set KSPP kernel params in container mode
* [`b8c92ede`](https://github.com/talos-systems/talos/commit/b8c92ede52ed515dba68abf4fb1cc6494d510827) fix: don't support cgroups nesting in process runner
* [`9bb0b797`](https://github.com/talos-systems/talos/commit/9bb0b79709a502ab49ea9bacd7e54617554d4cc3) test: adapt tests to the cgroupsv2
* [`1abc12be`](https://github.com/talos-systems/talos/commit/1abc12be13208ad1da03492a1b88d2c1ec0d5d33) fix: extramount should have `yaml:",inline"` tag
* [`2b614e43`](https://github.com/talos-systems/talos/commit/2b614e430e478cc111db018996ab2c8f763e4f92) feat: check if cluster has deprecated resources versions
* [`0b86edab`](https://github.com/talos-systems/talos/commit/0b86edab80cf4dd01f330d7721b130f5017d84a5) fix: don't panic if the machine config doesn't have network (EM)
* [`8bef41e4`](https://github.com/talos-systems/talos/commit/8bef41e4bacc4190976657ae5021afecd2d6e001) fix: make sure file mode is same (reproducibility issue)
* [`fcfca55a`](https://github.com/talos-systems/talos/commit/fcfca55a059e92fcda198baa321c4c63bda1f0a4) chore: do not check that go mod tidy gives empty output
* [`5ce92ca5`](https://github.com/talos-systems/talos/commit/5ce92ca5163616fcd7abe16c4efc3a100953b246) docs: ensure azure VMs are 0 indexed
</p>
</details>

### Changes since v0.13.0-alpha.1
<details><summary>18 commits</summary>
<p>

* [`30ae7142`](https://github.com/talos-systems/talos/commit/30ae714243379aaa3fb1e93023c2249ff3c3b4e3) feat: implement integration with Discovery Service
* [`353d632a`](https://github.com/talos-systems/talos/commit/353d632ae5d944a8662f0746ff8e757a67ffca53) feat: add nocloud platform support
* [`628fbf9b`](https://github.com/talos-systems/talos/commit/628fbf9b48d98df1063285b14958c94d246ce102) chore: update Linux to 5.10.69
* [`62acd625`](https://github.com/talos-systems/talos/commit/62acd6251637250dbea7d408d8cd4d5eb1f18713) fix: check trustd API CA on worker nodes
* [`ba27bc36`](https://github.com/talos-systems/talos/commit/ba27bc366fb3166b22f1bda909b9ede486ad8c7d) feat: implement Hetzner Cloud support for virtual (shared) IP
* [`95f440ea`](https://github.com/talos-systems/talos/commit/95f440eaa06d2a558fc828c11b451b6aed8d5855) test: add fuzz test for configloader
* [`d2cf021d`](https://github.com/talos-systems/talos/commit/d2cf021d8ffb6d6188b2d50f1f7b9c24df0aac84) chore: remove deprecated "join" term
* [`0e18e280`](https://github.com/talos-systems/talos/commit/0e18e2800fc038a86ed2fd9b042278ae29070bb5) chore: bump dependencies
* [`b450b7ce`](https://github.com/talos-systems/talos/commit/b450b7cef0d84a9ad975d8b50b93854bb0645173) chore: deprecate Interfaces and Routes APIs
* [`cddcb962`](https://github.com/talos-systems/talos/commit/cddcb9622bce7ae3626b8b9dce8c622a0e30ba66) fix: find devices without partition table
* [`b1b6d613`](https://github.com/talos-systems/talos/commit/b1b6d61365c900c4ebfc377b86067ddbe4fe8353) fix: check for existence of dhcp6 FQDN first
* [`519999b8`](https://github.com/talos-systems/talos/commit/519999b8462ff4931ed12323417b9a9c8c20b369) fix: use readonly mode when probing devices with `All` lookup
* [`2b520420`](https://github.com/talos-systems/talos/commit/2b5204200a4bd22aa78245b201c471136016ce3a) feat: enable resource API in the maintenance mode
* [`452893c2`](https://github.com/talos-systems/talos/commit/452893c260b920c601b0fc22ff018dc2d4341fca) fix: make probe open blockdevice in readonly mode
* [`96bccdd3`](https://github.com/talos-systems/talos/commit/96bccdd3b625f0edefd685cadf5f2cd46e3111f5) test: update CABPT provider to 0.3 release
* [`d9eb18bf`](https://github.com/talos-systems/talos/commit/d9eb18bfddf69a61712d930b53aec489a806394a) fix: containerd log symlink
* [`efa7f48e`](https://github.com/talos-systems/talos/commit/efa7f48e08382249609e0ecd3241c01a2e46df73) docs: quicklinks on landing page
* [`1cb9f282`](https://github.com/talos-systems/talos/commit/1cb9f282b541505f2d61ae0a57655cba9ae62843) fix: don't marshal clock with SecretsBundle
</p>
</details>

### Changes from talos-systems/discovery-service
<details><summary>16 commits</summary>
<p>

* [`e9d5dfa`](https://github.com/talos-systems/discovery-service/commit/e9d5dfa15e92fc1bdee620b2687eecd2ff929ac3) fix: enable connections to endpoints with public certs
* [`509e9b2`](https://github.com/talos-systems/discovery-service/commit/509e9b2ced989631224afd3bfd5e8bdcbcf13137) feat: implement client wrapper around discovery service API
* [`6195466`](https://github.com/talos-systems/discovery-service/commit/619546696afe99c1b58b46ca819bb32e74560f5b) feat: enable vtprotobuf, watch batching, more limits
* [`7174ec1`](https://github.com/talos-systems/discovery-service/commit/7174ec10426ff17172aaa195423e2588b23aa868) feat: implement new discovery service
* [`1a43970`](https://github.com/talos-systems/discovery-service/commit/1a4397082659a2b6bca6fd53a5080e109d978627) feat: add node and cluster validation
* [`6454cfc`](https://github.com/talos-systems/discovery-service/commit/6454cfcb726bba64823f25d962bc04007d057a8b) refactor: kresify, fix linter and rename to Kubespan manager
* [`d782452`](https://github.com/talos-systems/discovery-service/commit/d782452e86bf94c1194f806384fc339835a96d2d) add redis database backend
* [`924fed4`](https://github.com/talos-systems/discovery-service/commit/924fed4ecc939887c55a4a8a5f5dfe737ca29c78) refactor to flexible addresses
* [`cd02b5a`](https://github.com/talos-systems/discovery-service/commit/cd02b5a36783454f7650df5709984f01ccc8bd6e) revert to string IDs
* [`576288f`](https://github.com/talos-systems/discovery-service/commit/576288fc7559f9eac1966627b6c1e2c731e0c322) add self-reported IPs
* [`6ad15ca`](https://github.com/talos-systems/discovery-service/commit/6ad15ca825bdf4636855f2b77535945d07462a17) strong typing and known endpoint API
* [`3437ff2`](https://github.com/talos-systems/discovery-service/commit/3437ff28fb9ab6efb96cc79da1c75bd81bfb50cc) fixes from testing
* [`d3fd1f3`](https://github.com/talos-systems/discovery-service/commit/d3fd1f324d7af60f2ddf425c9c2cf9b9743e6d62) add Name to Node
* [`eb0e8ba`](https://github.com/talos-systems/discovery-service/commit/eb0e8baadacd702044ca419f933dd409ae58977e) add simple client pkg
* [`5e0c1df`](https://github.com/talos-systems/discovery-service/commit/5e0c1df0f1823cce1b92ccffb1e18fb1e540117d) add cluster hash grouping
* [`f982696`](https://github.com/talos-systems/discovery-service/commit/f982696ec1b45f8d7e6d2dfa1d780b3b0d515dbb) initial commit
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`52b27da`](https://github.com/talos-systems/extras/commit/52b27dad5aeeb5d14225a99e4b5902614c993022) chore: update pkgs and tools to 0.8.0-alpha.0
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>6 commits</summary>
<p>

* [`70d2865`](https://github.com/talos-systems/go-blockdevice/commit/70d28650b398a14469cbb5356417355b0ba62956) fix: try to find cdrom disks
* [`667bf53`](https://github.com/talos-systems/go-blockdevice/commit/667bf539b99ac34b629a0103ef7a7278a5a5f35d) fix: revert gpt partition not found
* [`d7d4cdd`](https://github.com/talos-systems/go-blockdevice/commit/d7d4cdd7ac56c82caab19246b5decd59f12195eb) fix: gpt partition not found
* [`33afba3`](https://github.com/talos-systems/go-blockdevice/commit/33afba347c0dce38a436c46a0aac26d2f99427c1) fix: also open in readonly mode when running `All` lookup method
* [`e367f9d`](https://github.com/talos-systems/go-blockdevice/commit/e367f9dc7fa935f11672de0fdc8a89429285a07a) feat: make probe always open blockdevices in readonly mode
* [`d981156`](https://github.com/talos-systems/go-blockdevice/commit/d9811569588ba44be878a00ce316f59a37abed8b) fix: allow Build for Windows
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>7 commits</summary>
<p>

* [`28cda67`](https://github.com/talos-systems/pkgs/commit/28cda67f9b518cb4d4a299bac62be1c3ed3cddda) feat: update Linux kernel to 5.10.69
* [`db90f93`](https://github.com/talos-systems/pkgs/commit/db90f93c0b462dcaefa081c18f8adebae5d5744a) chore: update tools
* [`ca38c59`](https://github.com/talos-systems/pkgs/commit/ca38c599b71334d5a108021c7e7ccec12701ff18) feat: enable KEXEC_FILE_LOAD in the kernel
* [`982bc18`](https://github.com/talos-systems/pkgs/commit/982bc18a4ecf5d4e15a73c350abb97f02adb8871) chore: update tools
* [`a243ab8`](https://github.com/talos-systems/pkgs/commit/a243ab8a9345b8bc39cc65254015b6eb07605f61) feat: add /usr/src to FHS
* [`428abdb`](https://github.com/talos-systems/pkgs/commit/428abdbfd303fce69cf583f5a8a4f5ed43253807) chore: support builds with HTTP_PROXY
* [`13151c5`](https://github.com/talos-systems/pkgs/commit/13151c59b5b29541ed5828aa9c75a061ec920ff1) chore: update bldr version, update tools
</p>
</details>

### Changes from talos-systems/tools
<details><summary>5 commits</summary>
<p>

* [`2790b55`](https://github.com/talos-systems/tools/commit/2790b5586e810c7dfc0a197ef9d1e6d77a646e3b) feat: update Go to 1.17.1
* [`5b9d214`](https://github.com/talos-systems/tools/commit/5b9d214c38515a55232ce36591036748fd8c49cc) fix: restore static library for ncurses
* [`01104e5`](https://github.com/talos-systems/tools/commit/01104e562efdbff34fb2d597d4cf27d04ba44ea6) chore: reproducible builds
* [`53fe146`](https://github.com/talos-systems/tools/commit/53fe146ca8ba55c959fee04302a5ce215a927f1d) chore: update bldr with new version
* [`bf4540d`](https://github.com/talos-systems/tools/commit/bf4540d0ed0728cd7751e0c3ab3bb4b8927e334c) chore: add patch dependency
</p>
</details>

### Dependency Changes

* **github.com/containerd/go-cni**                   v1.0.2 -> v1.1.0
* **github.com/containernetworking/cni**             v0.8.1 -> v1.0.1
* **github.com/containernetworking/plugins**         v0.9.1 -> v1.0.1
* **github.com/cosi-project/runtime**                25f235cd0682 -> 5cb7f5002d77
* **github.com/fatih/color**                         v1.12.0 -> v1.13.0
* **github.com/fsnotify/fsnotify**                   v1.4.9 -> v1.5.1
* **github.com/gdamore/tcell/v2**                    v2.4.0 -> f057f0a857a1
* **github.com/google/nftables**                     16a134723a96 **_new_**
* **github.com/hashicorp/go-getter**                 v1.5.7 -> v1.5.8
* **github.com/hetznercloud/hcloud-go**              v1.32.0 **_new_**
* **github.com/insomniacslk/dhcp**                   1cac67f12b1e -> b95caade3eac
* **github.com/jsimonetti/rtnetlink**                9c52e516c709 -> 435639c8e6a8
* **github.com/jxskiss/base62**                      4f11678b909b **_new_**
* **github.com/mattn/go-isatty**                     v0.0.13 -> v0.0.14
* **github.com/mdlayher/netx**                       669a06fde734 **_new_**
* **github.com/packethost/packngo**                  v0.19.0 -> v0.19.1
* **github.com/prometheus/procfs**                   v0.7.2 -> v0.7.3
* **github.com/rivo/tview**                          29d673af0ce2 -> ee97a7ab3975
* **github.com/scaleway/scaleway-sdk-go**            v1.0.0-beta.7 **_new_**
* **github.com/talos-systems/discovery-service**     e9d5dfa15e92 **_new_**
* **github.com/talos-systems/extras**                v0.5.0 -> v0.6.0-alpha.0
* **github.com/talos-systems/go-blockdevice**        v0.2.3 -> 70d28650b398
* **github.com/talos-systems/pkgs**                  v0.7.0 -> v0.8.0-alpha.0-4-g28cda67
* **github.com/talos-systems/tools**                 v0.7.0-1-ga33ccc1 -> v0.8.0-alpha.0-3-g2790b55
* **github.com/vishvananda/netlink**                 f5de75959ad5 **_new_**
* **github.com/vmware-tanzu/sonobuoy**               v0.53.1 -> v0.53.2
* **github.com/vmware/govmomi**                      v0.26.0 -> v0.26.1
* **github.com/vultr/metadata**                      v1.0.3 **_new_**
* **go.uber.org/zap**                                v1.19.0 -> v1.19.1
* **golang.org/x/net**                               853a461950ff -> 3ad01bbaa167
* **golang.org/x/sys**                               0f9fa26af87c -> 39ccf1dd6fa6
* **golang.org/x/term**                              6886f2dfbf5b -> 140adaaadfaf
* **golang.zx2c4.com/wireguard/wgctrl**              92e472f520a5 -> 0a2f4901cba6
* **google.golang.org/grpc**                         v1.40.0 -> v1.41.0
* **inet.af/netaddr**                                ce7a8ad02cc1 -> 85fa6c94624e
* **k8s.io/api**                                     v0.22.1 -> v0.22.2
* **k8s.io/apimachinery**                            v0.22.1 -> v0.22.2
* **k8s.io/client-go**                               v0.22.1 -> v0.22.2
* **k8s.io/kubectl**                                 v0.22.1 -> v0.22.2
* **k8s.io/kubelet**                                 v0.22.1 -> v0.22.2
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.59 **_new_**

Previous release can be found at [v0.12.0](https://github.com/talos-systems/talos/releases/tag/v0.12.0)

## [Talos 0.13.0-alpha.1](https://github.com/talos-systems/talos/releases/tag/v0.13.0-alpha.1) (2021-09-20)

Welcome to the v0.13.0-alpha.1 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Hetzner, Scaleway, Upcloud and Vultr

Talos now natively supports three new cloud platforms:

* [Hetzner](https://www.hetzner.com/)
* [Scaleway](https://www.scaleway.com/en/)
* [Upcloud](https://upcloud.com/)
* [Vultr](https://www.vultr.com/)


### etcd Advertised Address

The address advertised by etcd can now be controlled with new machine configuration option `machine.etcd.subnet`.


### Reboots via kexec

Talos now reboots by default via kexec syscall which means BIOS POST process is skipped.
On bare-metal hardware BIOS POST process might take 10-15 minutes, so Talos reboots 10-15 minutes faster on bare-metal.

Kexec support can be disabled with the following change to the machine configuration:

```
machine:
  sysctls:
    kernel.kexec_load_disabled: "1"
```


### Cluster Discovery and KubeSpan

This release of Talos provides initial support for cluster membership discovery and [KubeSpan](https://www.talos-systems.com/kubespan/).

These new features are not enabled by default, to enable them please make following changes to the machine configuration:

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```



### Windows Suport

CLI tool talosctl is now built for Windows and published as part of the release.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Serge Logvinov
* Andrew Rynhard
* Olli Janatuinen
* Andrey Smirnov
* Lennard Klein
* Rui Lopes
* Spencer Smith

### Changes
<details><summary>74 commits</summary>
<p>

* [`9d803d75`](https://github.com/talos-systems/talos/commit/9d803d75bfbe788fa5c2ef2ae0639de31e172c7b) chore: bump dependencies and drop firecracker support
* [`50a24104`](https://github.com/talos-systems/talos/commit/50a24104820e26bb99e66ab68be2bd9a6c17b0be) feat: add operating system version field to discovery
* [`085c61b2`](https://github.com/talos-systems/talos/commit/085c61b2ec432c586daa77464910e967a223ebe0) chore: add a special condition to check for kubeconfig readiness
* [`21cdd854`](https://github.com/talos-systems/talos/commit/21cdd854036498fbeb9f6e4d058a0edd55ed4856) fix: add node address to the list of allowed IPs (kubespan)
* [`fdd80a12`](https://github.com/talos-systems/talos/commit/fdd80a1234dc993cc01daa7764ba5a9db2fdc275) feat: add an option to continue booting on NTP timeout
* [`ef368498`](https://github.com/talos-systems/talos/commit/ef36849899b18bbb35c6116fdf35aa580a50a5e5) feat: add routes, routing rules and nftables rules for KubeSpan
* [`ed12379f`](https://github.com/talos-systems/talos/commit/ed12379f2f49fcbca84080f1066cf52dc202bd2d) fix: patch multi nodes support
* [`d943bb0e`](https://github.com/talos-systems/talos/commit/d943bb0e280e90f3592d9f7b67813b7a15818c84) feat: update Kubernetes to 1.22.2
* [`d0585fb6`](https://github.com/talos-systems/talos/commit/d0585fb6b303dfdd7fc80a76024915df31c72389) feat: reboot via kexec
* [`3de505c8`](https://github.com/talos-systems/talos/commit/3de505c894274bfd5248b6c597f6e3a53f873ba1) fix: skip bad cloud-config in OpenStack platform
* [`a394d1e2`](https://github.com/talos-systems/talos/commit/a394d1e20ba82de7d05e4d3f91823a98362ac9ee) fix: tear down control plane static pods when etcd is stopped
* [`1c05089b`](https://github.com/talos-systems/talos/commit/1c05089bb22c7c1050e95cf8d7bea8b763a0e86f) feat: implement KubeSpan manager for Wireguard peer state
* [`ec7f44ef`](https://github.com/talos-systems/talos/commit/ec7f44efe4f89e7ed207cbd5fe3748953ccfdf28) fix: completely prevent editing resources other than mc
* [`19a8ae97`](https://github.com/talos-systems/talos/commit/19a8ae97c69949f7c2421154b2ae4e52a905ff63) feat: add vultr.com cloud support
* [`0ff4c7cd`](https://github.com/talos-systems/talos/commit/0ff4c7cdb2b9505823f4c4504ec9bf4d7fddf5c5) fix: write KubernetesCACert chmodded 0400 instead of 0500
* [`a1c9d649`](https://github.com/talos-systems/talos/commit/a1c9d64907cce75bcb566f3ee394734e29b3932d) fix: update the way results are retrieved for certified conformance
* [`a0594540`](https://github.com/talos-systems/talos/commit/a0594540451a7636f8cd4bbe835913d31f66d0de) chore: build using Go 1.17
* [`7c5045bd`](https://github.com/talos-systems/talos/commit/7c5045bd929fcf5028cae3840970e692ef3bc7c9) release(v0.13.0-alpha.0): prepare release
* [`ee2dce6c`](https://github.com/talos-systems/talos/commit/ee2dce6c1a0e8838e587a9136afd1b7381000432) chore: bump dependencies
* [`ef022959`](https://github.com/talos-systems/talos/commit/ef022959280f156d6311836ef9cc2d01e5e3ae7d) fix: print etcd member ID in hex
* [`5ca1fb82`](https://github.com/talos-systems/talos/commit/5ca1fb822125483be290e79d8828bba246fda51c) fix: multiple fixes for KubeSpan and Wireguard implementation
* [`b1bd6425`](https://github.com/talos-systems/talos/commit/b1bd64250820df3fcb5214368ce9c8cf4634970a) fix: build platform images
* [`3b5f4038`](https://github.com/talos-systems/talos/commit/3b5f4038de2f855b3b634e4abb1c564da624e2fc) feat: add scaleway.com cloud support
* [`f156ab18`](https://github.com/talos-systems/talos/commit/f156ab1847f2ad1ca2a2548b299a713ee5fe0fcd) feat: add upcloud.com cloud support
* [`c3b2429c`](https://github.com/talos-systems/talos/commit/c3b2429ce91edc4f8f9e720a4b144bc941046fc3) fix: suppress spurious Kubernetes API server cert updates
* [`ff90b575`](https://github.com/talos-systems/talos/commit/ff90b5751e17a60fc6ca4274f35da7ddcca44fea) feat: implement KubeSpan peer generation controller
* [`14c69df5`](https://github.com/talos-systems/talos/commit/14c69df5063e71765b9316ae37657fda2388c60e) fix: correctly parse multiple pod/service CIDRs
* [`69897dbb`](https://github.com/talos-systems/talos/commit/69897dbba402812403c0c15d6cb8d2a771ea5a88) feat: drop some capabilities to be never available
* [`51e9836b`](https://github.com/talos-systems/talos/commit/51e9836b01926d1619d662e6e08df29210ff94e5) docs: promote 0.12 docs to be the latest
* [`812d59c7`](https://github.com/talos-systems/talos/commit/812d59c70085b54136e3b56127b0efea7ddb60af) feat: add hetzner.com cloud support
* [`d53e9e89`](https://github.com/talos-systems/talos/commit/d53e9e89633258d85c2232b85855535ebb42c417) chore: use named constants
* [`2dfe7f1f`](https://github.com/talos-systems/talos/commit/2dfe7f1fc654c8bec83b632a98dbaa8d1b90a521) chore: bump tools to the latest version
* [`82b130e7`](https://github.com/talos-systems/talos/commit/82b130e789aa4376e1f0e2d086233e630b410f74) docs: document required options for extraMounts
* [`af662210`](https://github.com/talos-systems/talos/commit/af6622109faecdf03aed43b047035904110c7580) feat: implement Kubernetes cluster discovery registry
* [`2c66e1b3`](https://github.com/talos-systems/talos/commit/2c66e1b3c5d4c34c5d4cdc155c32f2808a5f1c69) feat: provide building of local `Affiliate` structure (for the node)
* [`d69bd2af`](https://github.com/talos-systems/talos/commit/d69bd2af3e3d3bf12b6d74078e9eedf3dc8752fc) chore: enable GPG identity check for Talos
* [`8dbd851f`](https://github.com/talos-systems/talos/commit/8dbd851fde3febb5999df694a079121b43519aa9) chore: update tools/pkgs/extras to the new version
* [`0b347570`](https://github.com/talos-systems/talos/commit/0b347570a7aca0a133d6b6e6cc8d3e0355630480) feat: use dynamic NodeAddresses/HostnameStatus in Kubernetes certs
* [`bd5b9c96`](https://github.com/talos-systems/talos/commit/bd5b9c96e2563249a5633433703493b292b83ee9) fix: correctly define example for `extraMounts`
* [`01cca099`](https://github.com/talos-systems/talos/commit/01cca099f40ec75d1e047a84c89692eb254e8adf) docs: update docs for Talos 0.12 release
* [`668627d5`](https://github.com/talos-systems/talos/commit/668627d5b8ec79ec955eb1254732b1cc031d3aec) feat: add subnet filter for etcd address
* [`3c3c281b`](https://github.com/talos-systems/talos/commit/3c3c281bff8481f680feca9cf01af413a38e6973) chore: bump dependencies via dependabot
* [`f8bebba2`](https://github.com/talos-systems/talos/commit/f8bebba2de3999b7a36fecb2d6b90e583372c98f) fix: ignore error on duplicate for `MountStatus`
* [`6956edd0`](https://github.com/talos-systems/talos/commit/6956edd0bfae6c6c5d6eba00a22bc3a4cb7f54ea) feat: add node address filters, filter out k8s addresses for Talos API
* [`caee24bf`](https://github.com/talos-systems/talos/commit/caee24bf61136daecb095991a6e439f7fbf40da2) feat: implement KubeSpan identity controller
* [`da0f6e7e`](https://github.com/talos-systems/talos/commit/da0f6e7e1d295dce0c44c1854363528a6ffedde1) fix: allow updating diskSelector option
* [`761ccaf3`](https://github.com/talos-systems/talos/commit/761ccaf32348d8664eb0d5d1a51f6abb19ca52a6) feat: provide machine configuration for KubeSpan and cluster discovery
* [`a81e30cb`](https://github.com/talos-systems/talos/commit/a81e30cb46326fbdd433f37dc37549b588a2bc7a) docs: add bootstrap command to VMware docs
* [`97da354c`](https://github.com/talos-systems/talos/commit/97da354cc0e4a965e14b8939c426150d5c12f228) fix: do not panic on invalid machine configs
* [`c4048e26`](https://github.com/talos-systems/talos/commit/c4048e263d22682142f12fc4af6ac58c679273f0) fix: don't extract nil IPs in the GCP platform
* [`ba169c6f`](https://github.com/talos-systems/talos/commit/ba169c6f91948cf057251236fa7a727a05253639) feat: provide talosctl.exe for Windows
* [`6312f473`](https://github.com/talos-systems/talos/commit/6312f473e63df50287e6801c079242e2311a23e6) fix: properly handle omitempty fields in the validator
* [`7f22879a`](https://github.com/talos-systems/talos/commit/7f22879af0882af4cdebe9c84afb96ae68eb9f20) feat: provide random node identity
* [`032e7c6b`](https://github.com/talos-systems/talos/commit/032e7c6b863b5ca02cfa16df79c88950544dbffb) chore: import yaml.v3 consistently
* [`80b5f0e7`](https://github.com/talos-systems/talos/commit/80b5f0e7f78f09a11ed249f9f1dc7b05ea275ab0) fix: validate IP address returned as HTTP response in platform code
* [`c9af8f7f`](https://github.com/talos-systems/talos/commit/c9af8f7ff17facc18f10675879ed04982a000f6f) docs: fork docs for 0.13
* [`85cda1b9`](https://github.com/talos-systems/talos/commit/85cda1b956b042ba20696637248999d46f63ccc9) feat: provide MountStatus resource for system partition mounts
* [`950f122c`](https://github.com/talos-systems/talos/commit/950f122c95e225858e77083f2490481ed8d21aef) chore: update versions in upgrade tests
* [`83fdb772`](https://github.com/talos-systems/talos/commit/83fdb7721f45aa075898caf05a4b6856d3c5f330) feat: provide first NIC hardware addr as a resource
* [`5f5ac12f`](https://github.com/talos-systems/talos/commit/5f5ac12f1dc8aeb3a8598e57d965471e93fe3724) fix: properly case the VMware name
* [`0a6048f4`](https://github.com/talos-systems/talos/commit/0a6048f469da02efad7e84eb237e6fdeb85b7e33) fix: don't allow bootstrap if etcd data directory is not empty
* [`e24b93b4`](https://github.com/talos-systems/talos/commit/e24b93b4e120448f37109599f3e9eb15954b147a) fix: cgroup delegate
* [`751f64f9`](https://github.com/talos-systems/talos/commit/751f64f9bc10e9ad8508ade9e3a6a14aaaa54d57) docs: add release notes for 0.12, support matrix
* [`57a77696`](https://github.com/talos-systems/talos/commit/57a77696ef2b255a59ee4ed213a1a3971a5e2943) feat: update Kubernetes to 1.22.1
* [`244b08cc`](https://github.com/talos-systems/talos/commit/244b08cc198a8ba676bb9acadcbdd23a161b0876) chore: bump dependencies
* [`576ba195`](https://github.com/talos-systems/talos/commit/576ba195784abf275256c861d5f811ab1f7b1102) fix: do not set KSPP kernel params in container mode
* [`b8c92ede`](https://github.com/talos-systems/talos/commit/b8c92ede52ed515dba68abf4fb1cc6494d510827) fix: don't support cgroups nesting in process runner
* [`9bb0b797`](https://github.com/talos-systems/talos/commit/9bb0b79709a502ab49ea9bacd7e54617554d4cc3) test: adapt tests to the cgroupsv2
* [`1abc12be`](https://github.com/talos-systems/talos/commit/1abc12be13208ad1da03492a1b88d2c1ec0d5d33) fix: extramount should have `yaml:",inline"` tag
* [`2b614e43`](https://github.com/talos-systems/talos/commit/2b614e430e478cc111db018996ab2c8f763e4f92) feat: check if cluster has deprecated resources versions
* [`0b86edab`](https://github.com/talos-systems/talos/commit/0b86edab80cf4dd01f330d7721b130f5017d84a5) fix: don't panic if the machine config doesn't have network (EM)
* [`8bef41e4`](https://github.com/talos-systems/talos/commit/8bef41e4bacc4190976657ae5021afecd2d6e001) fix: make sure file mode is same (reproducibility issue)
* [`fcfca55a`](https://github.com/talos-systems/talos/commit/fcfca55a059e92fcda198baa321c4c63bda1f0a4) chore: do not check that go mod tidy gives empty output
* [`5ce92ca5`](https://github.com/talos-systems/talos/commit/5ce92ca5163616fcd7abe16c4efc3a100953b246) docs: ensure azure VMs are 0 indexed
</p>
</details>

### Changes since v0.13.0-alpha.0
<details><summary>17 commits</summary>
<p>

* [`9d803d75`](https://github.com/talos-systems/talos/commit/9d803d75bfbe788fa5c2ef2ae0639de31e172c7b) chore: bump dependencies and drop firecracker support
* [`50a24104`](https://github.com/talos-systems/talos/commit/50a24104820e26bb99e66ab68be2bd9a6c17b0be) feat: add operating system version field to discovery
* [`085c61b2`](https://github.com/talos-systems/talos/commit/085c61b2ec432c586daa77464910e967a223ebe0) chore: add a special condition to check for kubeconfig readiness
* [`21cdd854`](https://github.com/talos-systems/talos/commit/21cdd854036498fbeb9f6e4d058a0edd55ed4856) fix: add node address to the list of allowed IPs (kubespan)
* [`fdd80a12`](https://github.com/talos-systems/talos/commit/fdd80a1234dc993cc01daa7764ba5a9db2fdc275) feat: add an option to continue booting on NTP timeout
* [`ef368498`](https://github.com/talos-systems/talos/commit/ef36849899b18bbb35c6116fdf35aa580a50a5e5) feat: add routes, routing rules and nftables rules for KubeSpan
* [`ed12379f`](https://github.com/talos-systems/talos/commit/ed12379f2f49fcbca84080f1066cf52dc202bd2d) fix: patch multi nodes support
* [`d943bb0e`](https://github.com/talos-systems/talos/commit/d943bb0e280e90f3592d9f7b67813b7a15818c84) feat: update Kubernetes to 1.22.2
* [`d0585fb6`](https://github.com/talos-systems/talos/commit/d0585fb6b303dfdd7fc80a76024915df31c72389) feat: reboot via kexec
* [`3de505c8`](https://github.com/talos-systems/talos/commit/3de505c894274bfd5248b6c597f6e3a53f873ba1) fix: skip bad cloud-config in OpenStack platform
* [`a394d1e2`](https://github.com/talos-systems/talos/commit/a394d1e20ba82de7d05e4d3f91823a98362ac9ee) fix: tear down control plane static pods when etcd is stopped
* [`1c05089b`](https://github.com/talos-systems/talos/commit/1c05089bb22c7c1050e95cf8d7bea8b763a0e86f) feat: implement KubeSpan manager for Wireguard peer state
* [`ec7f44ef`](https://github.com/talos-systems/talos/commit/ec7f44efe4f89e7ed207cbd5fe3748953ccfdf28) fix: completely prevent editing resources other than mc
* [`19a8ae97`](https://github.com/talos-systems/talos/commit/19a8ae97c69949f7c2421154b2ae4e52a905ff63) feat: add vultr.com cloud support
* [`0ff4c7cd`](https://github.com/talos-systems/talos/commit/0ff4c7cdb2b9505823f4c4504ec9bf4d7fddf5c5) fix: write KubernetesCACert chmodded 0400 instead of 0500
* [`a1c9d649`](https://github.com/talos-systems/talos/commit/a1c9d64907cce75bcb566f3ee394734e29b3932d) fix: update the way results are retrieved for certified conformance
* [`a0594540`](https://github.com/talos-systems/talos/commit/a0594540451a7636f8cd4bbe835913d31f66d0de) chore: build using Go 1.17
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`52b27da`](https://github.com/talos-systems/extras/commit/52b27dad5aeeb5d14225a99e4b5902614c993022) chore: update pkgs and tools to 0.8.0-alpha.0
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>1 commit</summary>
<p>

* [`d981156`](https://github.com/talos-systems/go-blockdevice/commit/d9811569588ba44be878a00ce316f59a37abed8b) fix: allow Build for Windows
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>6 commits</summary>
<p>

* [`db90f93`](https://github.com/talos-systems/pkgs/commit/db90f93c0b462dcaefa081c18f8adebae5d5744a) chore: update tools
* [`ca38c59`](https://github.com/talos-systems/pkgs/commit/ca38c599b71334d5a108021c7e7ccec12701ff18) feat: enable KEXEC_FILE_LOAD in the kernel
* [`982bc18`](https://github.com/talos-systems/pkgs/commit/982bc18a4ecf5d4e15a73c350abb97f02adb8871) chore: update tools
* [`a243ab8`](https://github.com/talos-systems/pkgs/commit/a243ab8a9345b8bc39cc65254015b6eb07605f61) feat: add /usr/src to FHS
* [`428abdb`](https://github.com/talos-systems/pkgs/commit/428abdbfd303fce69cf583f5a8a4f5ed43253807) chore: support builds with HTTP_PROXY
* [`13151c5`](https://github.com/talos-systems/pkgs/commit/13151c59b5b29541ed5828aa9c75a061ec920ff1) chore: update bldr version, update tools
</p>
</details>

### Changes from talos-systems/tools
<details><summary>5 commits</summary>
<p>

* [`2790b55`](https://github.com/talos-systems/tools/commit/2790b5586e810c7dfc0a197ef9d1e6d77a646e3b) feat: update Go to 1.17.1
* [`5b9d214`](https://github.com/talos-systems/tools/commit/5b9d214c38515a55232ce36591036748fd8c49cc) fix: restore static library for ncurses
* [`01104e5`](https://github.com/talos-systems/tools/commit/01104e562efdbff34fb2d597d4cf27d04ba44ea6) chore: reproducible builds
* [`53fe146`](https://github.com/talos-systems/tools/commit/53fe146ca8ba55c959fee04302a5ce215a927f1d) chore: update bldr with new version
* [`bf4540d`](https://github.com/talos-systems/tools/commit/bf4540d0ed0728cd7751e0c3ab3bb4b8927e334c) chore: add patch dependency
</p>
</details>

### Dependency Changes

* **github.com/containerd/go-cni**                   v1.0.2 -> v1.1.0
* **github.com/containernetworking/cni**             v0.8.1 -> v1.0.1
* **github.com/containernetworking/plugins**         v0.9.1 -> v1.0.1
* **github.com/cosi-project/runtime**                25f235cd0682 -> 5cb7f5002d77
* **github.com/fsnotify/fsnotify**                   v1.4.9 -> v1.5.1
* **github.com/gdamore/tcell/v2**                    v2.4.0 -> f057f0a857a1
* **github.com/google/nftables**                     16a134723a96 **_new_**
* **github.com/hashicorp/go-getter**                 v1.5.7 -> v1.5.8
* **github.com/insomniacslk/dhcp**                   1cac67f12b1e -> b95caade3eac
* **github.com/jsimonetti/rtnetlink**                9c52e516c709 -> 4cc3c1489576
* **github.com/jxskiss/base62**                      4f11678b909b **_new_**
* **github.com/mattn/go-isatty**                     v0.0.13 -> v0.0.14
* **github.com/mdlayher/netx**                       669a06fde734 **_new_**
* **github.com/packethost/packngo**                  v0.19.0 -> v0.19.1
* **github.com/prometheus/procfs**                   v0.7.2 -> v0.7.3
* **github.com/rivo/tview**                          29d673af0ce2 -> f7430b878d17
* **github.com/scaleway/scaleway-sdk-go**            v1.0.0-beta.7 **_new_**
* **github.com/talos-systems/extras**                v0.5.0 -> v0.6.0-alpha.0
* **github.com/talos-systems/go-blockdevice**        v0.2.3 -> d9811569588b
* **github.com/talos-systems/pkgs**                  v0.7.0 -> v0.8.0-alpha.0-3-gdb90f93
* **github.com/talos-systems/tools**                 v0.7.0-1-ga33ccc1 -> v0.8.0-alpha.0-3-g2790b55
* **github.com/vishvananda/netlink**                 f5de75959ad5 **_new_**
* **github.com/vmware-tanzu/sonobuoy**               v0.53.1 -> v0.53.2
* **github.com/vmware/govmomi**                      v0.26.0 -> v0.26.1
* **github.com/vultr/metadata**                      v1.0.3 **_new_**
* **go.uber.org/zap**                                v1.19.0 -> v1.19.1
* **golang.org/x/net**                               853a461950ff -> 978cfadd31cf
* **golang.org/x/sys**                               0f9fa26af87c -> d61c044b1678
* **golang.org/x/term**                              6886f2dfbf5b -> 140adaaadfaf
* **golang.zx2c4.com/wireguard/wgctrl**              92e472f520a5 -> 91d1988e44de
* **inet.af/netaddr**                                ce7a8ad02cc1 -> 85fa6c94624e
* **k8s.io/api**                                     v0.22.1 -> v0.22.2
* **k8s.io/apimachinery**                            v0.22.1 -> v0.22.2
* **k8s.io/client-go**                               v0.22.1 -> v0.22.2
* **k8s.io/kubectl**                                 v0.22.1 -> v0.22.2
* **k8s.io/kubelet**                                 v0.22.1 -> v0.22.2
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.58 **_new_**

Previous release can be found at [v0.12.0](https://github.com/talos-systems/talos/releases/tag/v0.12.0)

## [Talos 0.13.0-alpha.0](https://github.com/talos-systems/talos/releases/tag/v0.13.0-alpha.0) (2021-09-13)

Welcome to the v0.13.0-alpha.0 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Hetzner, Scaleway and Upcloud

Talos now natively supports three new cloud platforms:

* [Hetzner](https://www.hetzner.com/)
* [Scaleway](https://www.scaleway.com/en/)
* [Upcloud](https://upcloud.com/)


### etcd Advertised Address

The address advertised by etcd can now be controlled with new machine configuration option `machine.etcd.subnet`.


### Cluster Discovery and KubeSpan

This release of Talos provides some initial support for cluster membership discovery and [KubeSpan](https://www.talos-systems.com/kubespan/).

These new features are not enabled by default.


### Windows Suport

CLI tool talosctl is now built for Windows and published as part of the release.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Alexey Palazhchenko
* Serge Logvinov
* Andrew Rynhard
* Olli Janatuinen
* Andrey Smirnov
* Rui Lopes
* Spencer Smith

### Changes
<details><summary>55 commits</summary>
<p>

* [`ef022959`](https://github.com/talos-systems/talos/commit/ef022959280f156d6311836ef9cc2d01e5e3ae7d) fix: print etcd member ID in hex
* [`5ca1fb82`](https://github.com/talos-systems/talos/commit/5ca1fb822125483be290e79d8828bba246fda51c) fix: multiple fixes for KubeSpan and Wireguard implementation
* [`b1bd6425`](https://github.com/talos-systems/talos/commit/b1bd64250820df3fcb5214368ce9c8cf4634970a) fix: build platform images
* [`3b5f4038`](https://github.com/talos-systems/talos/commit/3b5f4038de2f855b3b634e4abb1c564da624e2fc) feat: add scaleway.com cloud support
* [`f156ab18`](https://github.com/talos-systems/talos/commit/f156ab1847f2ad1ca2a2548b299a713ee5fe0fcd) feat: add upcloud.com cloud support
* [`c3b2429c`](https://github.com/talos-systems/talos/commit/c3b2429ce91edc4f8f9e720a4b144bc941046fc3) fix: suppress spurious Kubernetes API server cert updates
* [`ff90b575`](https://github.com/talos-systems/talos/commit/ff90b5751e17a60fc6ca4274f35da7ddcca44fea) feat: implement KubeSpan peer generation controller
* [`14c69df5`](https://github.com/talos-systems/talos/commit/14c69df5063e71765b9316ae37657fda2388c60e) fix: correctly parse multiple pod/service CIDRs
* [`69897dbb`](https://github.com/talos-systems/talos/commit/69897dbba402812403c0c15d6cb8d2a771ea5a88) feat: drop some capabilities to be never available
* [`51e9836b`](https://github.com/talos-systems/talos/commit/51e9836b01926d1619d662e6e08df29210ff94e5) docs: promote 0.12 docs to be the latest
* [`812d59c7`](https://github.com/talos-systems/talos/commit/812d59c70085b54136e3b56127b0efea7ddb60af) feat: add hetzner.com cloud support
* [`d53e9e89`](https://github.com/talos-systems/talos/commit/d53e9e89633258d85c2232b85855535ebb42c417) chore: use named constants
* [`2dfe7f1f`](https://github.com/talos-systems/talos/commit/2dfe7f1fc654c8bec83b632a98dbaa8d1b90a521) chore: bump tools to the latest version
* [`82b130e7`](https://github.com/talos-systems/talos/commit/82b130e789aa4376e1f0e2d086233e630b410f74) docs: document required options for extraMounts
* [`af662210`](https://github.com/talos-systems/talos/commit/af6622109faecdf03aed43b047035904110c7580) feat: implement Kubernetes cluster discovery registry
* [`2c66e1b3`](https://github.com/talos-systems/talos/commit/2c66e1b3c5d4c34c5d4cdc155c32f2808a5f1c69) feat: provide building of local `Affiliate` structure (for the node)
* [`d69bd2af`](https://github.com/talos-systems/talos/commit/d69bd2af3e3d3bf12b6d74078e9eedf3dc8752fc) chore: enable GPG identity check for Talos
* [`8dbd851f`](https://github.com/talos-systems/talos/commit/8dbd851fde3febb5999df694a079121b43519aa9) chore: update tools/pkgs/extras to the new version
* [`0b347570`](https://github.com/talos-systems/talos/commit/0b347570a7aca0a133d6b6e6cc8d3e0355630480) feat: use dynamic NodeAddresses/HostnameStatus in Kubernetes certs
* [`bd5b9c96`](https://github.com/talos-systems/talos/commit/bd5b9c96e2563249a5633433703493b292b83ee9) fix: correctly define example for `extraMounts`
* [`01cca099`](https://github.com/talos-systems/talos/commit/01cca099f40ec75d1e047a84c89692eb254e8adf) docs: update docs for Talos 0.12 release
* [`668627d5`](https://github.com/talos-systems/talos/commit/668627d5b8ec79ec955eb1254732b1cc031d3aec) feat: add subnet filter for etcd address
* [`3c3c281b`](https://github.com/talos-systems/talos/commit/3c3c281bff8481f680feca9cf01af413a38e6973) chore: bump dependencies via dependabot
* [`f8bebba2`](https://github.com/talos-systems/talos/commit/f8bebba2de3999b7a36fecb2d6b90e583372c98f) fix: ignore error on duplicate for `MountStatus`
* [`6956edd0`](https://github.com/talos-systems/talos/commit/6956edd0bfae6c6c5d6eba00a22bc3a4cb7f54ea) feat: add node address filters, filter out k8s addresses for Talos API
* [`caee24bf`](https://github.com/talos-systems/talos/commit/caee24bf61136daecb095991a6e439f7fbf40da2) feat: implement KubeSpan identity controller
* [`da0f6e7e`](https://github.com/talos-systems/talos/commit/da0f6e7e1d295dce0c44c1854363528a6ffedde1) fix: allow updating diskSelector option
* [`761ccaf3`](https://github.com/talos-systems/talos/commit/761ccaf32348d8664eb0d5d1a51f6abb19ca52a6) feat: provide machine configuration for KubeSpan and cluster discovery
* [`a81e30cb`](https://github.com/talos-systems/talos/commit/a81e30cb46326fbdd433f37dc37549b588a2bc7a) docs: add bootstrap command to VMware docs
* [`97da354c`](https://github.com/talos-systems/talos/commit/97da354cc0e4a965e14b8939c426150d5c12f228) fix: do not panic on invalid machine configs
* [`c4048e26`](https://github.com/talos-systems/talos/commit/c4048e263d22682142f12fc4af6ac58c679273f0) fix: don't extract nil IPs in the GCP platform
* [`ba169c6f`](https://github.com/talos-systems/talos/commit/ba169c6f91948cf057251236fa7a727a05253639) feat: provide talosctl.exe for Windows
* [`6312f473`](https://github.com/talos-systems/talos/commit/6312f473e63df50287e6801c079242e2311a23e6) fix: properly handle omitempty fields in the validator
* [`7f22879a`](https://github.com/talos-systems/talos/commit/7f22879af0882af4cdebe9c84afb96ae68eb9f20) feat: provide random node identity
* [`032e7c6b`](https://github.com/talos-systems/talos/commit/032e7c6b863b5ca02cfa16df79c88950544dbffb) chore: import yaml.v3 consistently
* [`80b5f0e7`](https://github.com/talos-systems/talos/commit/80b5f0e7f78f09a11ed249f9f1dc7b05ea275ab0) fix: validate IP address returned as HTTP response in platform code
* [`c9af8f7f`](https://github.com/talos-systems/talos/commit/c9af8f7ff17facc18f10675879ed04982a000f6f) docs: fork docs for 0.13
* [`85cda1b9`](https://github.com/talos-systems/talos/commit/85cda1b956b042ba20696637248999d46f63ccc9) feat: provide MountStatus resource for system partition mounts
* [`950f122c`](https://github.com/talos-systems/talos/commit/950f122c95e225858e77083f2490481ed8d21aef) chore: update versions in upgrade tests
* [`83fdb772`](https://github.com/talos-systems/talos/commit/83fdb7721f45aa075898caf05a4b6856d3c5f330) feat: provide first NIC hardware addr as a resource
* [`5f5ac12f`](https://github.com/talos-systems/talos/commit/5f5ac12f1dc8aeb3a8598e57d965471e93fe3724) fix: properly case the VMware name
* [`0a6048f4`](https://github.com/talos-systems/talos/commit/0a6048f469da02efad7e84eb237e6fdeb85b7e33) fix: don't allow bootstrap if etcd data directory is not empty
* [`e24b93b4`](https://github.com/talos-systems/talos/commit/e24b93b4e120448f37109599f3e9eb15954b147a) fix: cgroup delegate
* [`751f64f9`](https://github.com/talos-systems/talos/commit/751f64f9bc10e9ad8508ade9e3a6a14aaaa54d57) docs: add release notes for 0.12, support matrix
* [`57a77696`](https://github.com/talos-systems/talos/commit/57a77696ef2b255a59ee4ed213a1a3971a5e2943) feat: update Kubernetes to 1.22.1
* [`244b08cc`](https://github.com/talos-systems/talos/commit/244b08cc198a8ba676bb9acadcbdd23a161b0876) chore: bump dependencies
* [`576ba195`](https://github.com/talos-systems/talos/commit/576ba195784abf275256c861d5f811ab1f7b1102) fix: do not set KSPP kernel params in container mode
* [`b8c92ede`](https://github.com/talos-systems/talos/commit/b8c92ede52ed515dba68abf4fb1cc6494d510827) fix: don't support cgroups nesting in process runner
* [`9bb0b797`](https://github.com/talos-systems/talos/commit/9bb0b79709a502ab49ea9bacd7e54617554d4cc3) test: adapt tests to the cgroupsv2
* [`1abc12be`](https://github.com/talos-systems/talos/commit/1abc12be13208ad1da03492a1b88d2c1ec0d5d33) fix: extramount should have `yaml:",inline"` tag
* [`2b614e43`](https://github.com/talos-systems/talos/commit/2b614e430e478cc111db018996ab2c8f763e4f92) feat: check if cluster has deprecated resources versions
* [`0b86edab`](https://github.com/talos-systems/talos/commit/0b86edab80cf4dd01f330d7721b130f5017d84a5) fix: don't panic if the machine config doesn't have network (EM)
* [`8bef41e4`](https://github.com/talos-systems/talos/commit/8bef41e4bacc4190976657ae5021afecd2d6e001) fix: make sure file mode is same (reproducibility issue)
* [`fcfca55a`](https://github.com/talos-systems/talos/commit/fcfca55a059e92fcda198baa321c4c63bda1f0a4) chore: do not check that go mod tidy gives empty output
* [`5ce92ca5`](https://github.com/talos-systems/talos/commit/5ce92ca5163616fcd7abe16c4efc3a100953b246) docs: ensure azure VMs are 0 indexed
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`52b27da`](https://github.com/talos-systems/extras/commit/52b27dad5aeeb5d14225a99e4b5902614c993022) chore: update pkgs and tools to 0.8.0-alpha.0
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>1 commit</summary>
<p>

* [`d981156`](https://github.com/talos-systems/go-blockdevice/commit/d9811569588ba44be878a00ce316f59a37abed8b) fix: allow Build for Windows
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>3 commits</summary>
<p>

* [`a243ab8`](https://github.com/talos-systems/pkgs/commit/a243ab8a9345b8bc39cc65254015b6eb07605f61) feat: add /usr/src to FHS
* [`428abdb`](https://github.com/talos-systems/pkgs/commit/428abdbfd303fce69cf583f5a8a4f5ed43253807) chore: support builds with HTTP_PROXY
* [`13151c5`](https://github.com/talos-systems/pkgs/commit/13151c59b5b29541ed5828aa9c75a061ec920ff1) chore: update bldr version, update tools
</p>
</details>

### Changes from talos-systems/tools
<details><summary>4 commits</summary>
<p>

* [`5b9d214`](https://github.com/talos-systems/tools/commit/5b9d214c38515a55232ce36591036748fd8c49cc) fix: restore static library for ncurses
* [`01104e5`](https://github.com/talos-systems/tools/commit/01104e562efdbff34fb2d597d4cf27d04ba44ea6) chore: reproducible builds
* [`53fe146`](https://github.com/talos-systems/tools/commit/53fe146ca8ba55c959fee04302a5ce215a927f1d) chore: update bldr with new version
* [`bf4540d`](https://github.com/talos-systems/tools/commit/bf4540d0ed0728cd7751e0c3ab3bb4b8927e334c) chore: add patch dependency
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**                25f235cd0682 -> 57b048cd66b0
* **github.com/fsnotify/fsnotify**                   v1.4.9 -> v1.5.1
* **github.com/insomniacslk/dhcp**                   1cac67f12b1e -> d82598001386
* **github.com/jxskiss/base62**                      4f11678b909b **_new_**
* **github.com/mdlayher/netx**                       669a06fde734 **_new_**
* **github.com/prometheus/procfs**                   v0.7.2 -> v0.7.3
* **github.com/scaleway/scaleway-sdk-go**            v1.0.0-beta.7 **_new_**
* **github.com/talos-systems/extras**                v0.5.0 -> v0.6.0-alpha.0
* **github.com/talos-systems/go-blockdevice**        v0.2.3 -> d9811569588b
* **github.com/talos-systems/pkgs**                  v0.7.0 -> v0.8.0-alpha.0
* **github.com/talos-systems/tools**                 v0.7.0-1-ga33ccc1 -> v0.8.0-alpha.0-2-g5b9d214
* **github.com/vmware-tanzu/sonobuoy**               v0.53.1 -> v0.53.2
* **github.com/vmware/govmomi**                      v0.26.0 -> v0.26.1
* **golang.org/x/net**                               853a461950ff -> 60bc85c4be6d
* **golang.org/x/sys**                               0f9fa26af87c -> 63515b42dcdf
* **kernel.org/pub/linux/libs/security/libcap/cap**  v1.2.56 **_new_**

Previous release can be found at [v0.12.0](https://github.com/talos-systems/talos/releases/tag/v0.12.0)

## [Talos 0.12.0-alpha.1](https://github.com/talos-systems/talos/releases/tag/v0.12.0-alpha.1) (2021-08-13)

Welcome to the v0.12.0-alpha.1 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Support for Self-hosted Control Plane Dropped

> **Note**: This item only applies to clusters bootstrapped with Talos <= 0.8.

Talos 0.12 completely removes support for self-hosted Kubernetes control plane (bootkube-based).
Talos 0.9 introduced support for Talos-managed control plane and provided migration path to convert self-hosted control plane
to Talos-managed static pods.
Automated and manual conversion process is available in Talos from 0.9.x to 0.11.x.
For clusters bootstrapped with bootkube (Talos <= 0.8), please make sure control plane is converted to Talos-managed before
before upgrading to Talos 0.12.
Current control plane status can be checked with `talosctl get bootstrapstatus` before performing upgrade to Talos 0.12.


### Cluster API v0.3.x

Cluster API v0.3.x (v1alpha3) is not compatible with Kubernetes 1.22 used by default in Talos 0.12.
Talos can be configued to use Kubernetes 1.21 or CAPI v0.4.x components can be used instead.


### Machine Config Validation

Unknown keys in the machine config now make the config invalid,
so any attempt to apply/edit the configuration with the unknown keys will lead into an error.


### Sysctl Configuration

Sysctl Kernel Params configuration was completely rewritten to be based on controllers and resources,
which makes it possible to apply `.machine.sysctls` in immediate mode (without a reboot).
`talosctl get kernelparams` returns merged list of KSPP, Kubernetes and user defined params along with
the default values overwritten by Talos.


### Equinix Metal

Added support for Equinix Metal IPs for the Talos virtual (shared) IP (option `equnixMetal` under `vip` in the machine configuration).
Talos automatically re-assigns IP using the Equinix Metal API when leadership changes.


### etcd

New etcd cluster members are now joined in [learner mode](https://etcd.io/docs/v3.4/learning/design-learner/), which improves cluster resiliency
to member join issues.


### Join Node Type

Node type `join` was renamed to `worker` for clarity. The old value is still accepted in the machine configuration but deprecated.
`talosctl gen config` now generates `worker.yaml` instead of `join.yaml`.


### Networking

* multiple static addresses can be specified for the interface with new `.addresses` field (old `.cidr` field is deprecated now)
* static addresses can be set on interfaces configured with DHCP


### Performance

* machined uses less memory and CPU time
* more disk encryption options are exposed via the machine configuration
* disk partitions are now aligned properly with minimum I/O size
* Talos system processes are moved under proper cgroups, resource metrics are now available via the kubelet
* OOM score is set on the system processes making sure they are killed last under memory pressure


### Security

* etcd PKI moved to `/system/secrets`
* kubelet bootstrap CSR auto-signing scoped to kubelet bootstrap tokens only
* enforce default seccomp profile on all system containers
* run system services apid, trustd, and etcd as non-root users


### Component Updates

* Linux: 5.10.57
* Kubernetes: 1.22.0
* containerd: 1.5.5
* runc: 1.0.1
* GRUB: 2.06
* Talos is built with Go 1.16.7


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Andrey Smirnov
* Serge Logvinov
* Artem Chernyshev
* Spencer Smith
* Alexey Palazhchenko
* dependabot[bot]
* Andrew Rynhard
* Artem Chernyshev
* Rui Lopes
* Caleb Woodbine
* Seán C McCord

### Changes
<details><summary>109 commits</summary>
<p>

* [`1ed5e545`](https://github.com/talos-systems/talos/commit/1ed5e545385e160fe3b61e6dbbcaa8a701437b62) feat: add ClusterID and ClusterSecret
* [`228b3761`](https://github.com/talos-systems/talos/commit/228b376163597cd825e4a142e6b4bdea0f870365) chore: run etcd as non-root user
* [`3518219b`](https://github.com/talos-systems/talos/commit/3518219bff44f71a60ad8e448e518844d1b933fd) chore: drop deprecated `--no-reboot` param and KernelCurrentRoot const
* [`33d1c3e4`](https://github.com/talos-systems/talos/commit/33d1c3e42582649f25a44fc3c86007bcebbc80b3) chore: run apid and trustd services as non-root user
* [`dadaa65d`](https://github.com/talos-systems/talos/commit/dadaa65d542171d25317840fcf35fa3979cf0632) feat: print uid/gid for the files in `ls -l`
* [`e6fa401b`](https://github.com/talos-systems/talos/commit/e6fa401b663d0ebd4374c9e47a7ca6150a4756cd) fix: enable seccomp default profile by default
* [`8ddbcc96`](https://github.com/talos-systems/talos/commit/8ddbcc9643113c15de538fc070b7053d1c6efdfc) feat: validate if extra fields present in the decoder
* [`5b57a980`](https://github.com/talos-systems/talos/commit/5b57a98008c64d7cb07729fd9b31a0e3493c289c) chore: update Go to 1.16.7, Linux to 5.10.57
* [`eefe1c21`](https://github.com/talos-systems/talos/commit/eefe1c21c30fa2cd281fc5524b2e88553f6fdfcc) feat: add new etcd members in learner mode
* [`b1c66fba`](https://github.com/talos-systems/talos/commit/b1c66fbad113400729cf4db806e30192bf7e0462) feat: implement Equinix Metal support for virtual (shared) IP
* [`62242f97`](https://github.com/talos-systems/talos/commit/62242f979e1921ed8abfa06a26564ea0bf8a5fb3) chore: require GPG signatures
* [`faecae44`](https://github.com/talos-systems/talos/commit/faecae44fde60fc626ccb01da3b221519a9d41d7) feat: make ISO builds reproducible
* [`887c2326`](https://github.com/talos-systems/talos/commit/887c2326a4f81c846e3aa3bd1787bc840877e494) release(v0.12.0-alpha.0): prepare release
* [`a15f0184`](https://github.com/talos-systems/talos/commit/a15f01844fdaf0d3e2dad2750d9353d03e18dea2) fix: move etcd PKI under /system/secrets
* [`eb02afe1`](https://github.com/talos-systems/talos/commit/eb02afe18be63bf483a0467f655611561aef10f6) fix: match correctly routes on the address family
* [`cb948acc`](https://github.com/talos-systems/talos/commit/cb948accfeca13c57b3b512dc8a06425989294f9) feat: allow multiple addresses per interface
* [`e030b2e8`](https://github.com/talos-systems/talos/commit/e030b2e8bb0a65abf4e1f7b5f27348631210ebc4) chore: use k8s 1.21.3 in CAPI tests for now
* [`e08b4f8f`](https://github.com/talos-systems/talos/commit/e08b4f8f9e72f8db1116b4bbe395d49b4bccb460) feat: implement sysctl controllers
* [`fdf6b243`](https://github.com/talos-systems/talos/commit/fdf6b2433c40613bcb039852a96196dbe9b7b5e2) chore: revert "improve artifacts generation reproducibility"
* [`b68ed1eb`](https://github.com/talos-systems/talos/commit/b68ed1eb896039ec1319db2e3d6d364034c86863) fix: make route resources ID match closer routing table primary key
* [`585f6337`](https://github.com/talos-systems/talos/commit/585f633710abb7a6d863b54c37aa65c50a3c7312) fix: correctly handle nodoc for struct fields
* [`f2d394dc`](https://github.com/talos-systems/talos/commit/f2d394dc42f9ec704050db0a8a928a889483ce3e) docs: add AMIs for v0.11.5
* [`d0970cbf`](https://github.com/talos-systems/talos/commit/d0970cbfd696b28b201b232a03da2119f664afbd) feat: bootstrap token limit
* [`5285a46d`](https://github.com/talos-systems/talos/commit/5285a46d78ef2fc76594aad4ad4acb75312bc0a7) fix: maintenance mode reason message
* [`009d15e8`](https://github.com/talos-systems/talos/commit/009d15e8dc6e75eca6b5963dddf8063941099f14) chore: use etcd client TryLock function on upgrade
* [`4dae9ea5`](https://github.com/talos-systems/talos/commit/4dae9ea55c087c28a9d7a8d241e0ec3a7a1b8ca3) chore: use vtprotobuf compiled marshaling in Talos API
* [`7ca5749a`](https://github.com/talos-systems/talos/commit/7ca5749ad4267701ce639d0f0d91c10a7f9c1d3d) chore: bump dependencies via dependabot
* [`b2507b41`](https://github.com/talos-systems/talos/commit/b2507b41d250b989b9c13ad23e16202cd53a18d2) chore: improve artifacts generation reproducibility
* [`1f7dad23`](https://github.com/talos-systems/talos/commit/1f7dad234b480c7a5e3484ccf10180747c979036) chore: update PKGS version (512 cpus, new ca-certficates)
* [`1a2e78a2`](https://github.com/talos-systems/talos/commit/1a2e78a24e997241c4cd18dfac3c2d971ba78116) fix: update go-blockdevice
* [`6d6ed117`](https://github.com/talos-systems/talos/commit/6d6ed1170f3f28e7f559ccdf64e7c34dfee022a0) chore: use parallel xz with higher compression level
* [`571f7db1`](https://github.com/talos-systems/talos/commit/571f7db1bb44a0dcb5e373f9c37396d50eb0e8f4) chore: workaround GitHub new release notes limit
* [`09d70b7e`](https://github.com/talos-systems/talos/commit/09d70b7eafb18343eb4ca57d7f8b84e4ccd2fcfb) feat: update Kubernetes to v1.22.0
* [`f25f10e7`](https://github.com/talos-systems/talos/commit/f25f10e73ec534acd7cc483f254d612d8a7c1858) feat: add an option to disable PSP
* [`7c6e4cf2`](https://github.com/talos-systems/talos/commit/7c6e4cf230ba1f30da664374c41c934d1e6620bc) feat: allow both DHCP and static addressing for the interface
* [`3c566dbc`](https://github.com/talos-systems/talos/commit/3c566dbc30595467a3789707c6e993aa92f36df6) fix: remove admission plugins enabled by default from the list
* [`69ead373`](https://github.com/talos-systems/talos/commit/69ead37353b7e3aa7f089c70073037a6eba37767) fix: preserve PMBR bootable flag correctly
* [`dee63051`](https://github.com/talos-systems/talos/commit/dee63051702d49f495bfb28b4be74ed8b39143ad) fix: align partitions with minimal I/O size
* [`62890229`](https://github.com/talos-systems/talos/commit/628902297d2efe93e6388377b2ea6d4beda83095) feat: update GRUB to 2.06
* [`b9d04928`](https://github.com/talos-systems/talos/commit/b9d04928d960f9d576671c6f3511cf242ff31cb7) feat: move system processes to cgroups
* [`0b8681b4`](https://github.com/talos-systems/talos/commit/0b8681b4b49ab109b8863792d48c2f551d1ceeb5) fix: resolve several issues with Wireguard link specs
* [`f8f4bf3b`](https://github.com/talos-systems/talos/commit/f8f4bf3baef31d4ac957ec68cd869adea1e931cd) docs: add disk encryptions examples
* [`79b8fa64`](https://github.com/talos-systems/talos/commit/79b8fa64b9453917860faae3df5d14647186b9ba) feat: update containerd to 1.5.5
* [`539f4209`](https://github.com/talos-systems/talos/commit/539f42090e436921a23087296cde6eaf7e495b5e) chore: bump dependencies via dependabot
* [`0c7ce1cd`](https://github.com/talos-systems/talos/commit/0c7ce1cd814354213a1a6c7a9251b166ee58c493) feat: remove remnants of bootkube support
* [`d4f9804f`](https://github.com/talos-systems/talos/commit/d4f9804f8659562f6152ae73cb1788f6f6d6ad89) chore: fix typos
* [`5f027615`](https://github.com/talos-systems/talos/commit/5f027615ffac68e0a484a5da4827a6589bae3880) feat: expose more encryption options to the machine config
* [`585152a0`](https://github.com/talos-systems/talos/commit/585152a0be051accd4cb8b7c2f130c5a92dfd32d) chore: bump dependencies
* [`fc66ec59`](https://github.com/talos-systems/talos/commit/fc66ec59691fb1b9d00b27e1f7b34c870a09d717) feat: set oom score for main processes
* [`df54584a`](https://github.com/talos-systems/talos/commit/df54584a33d88de13deadcb87a5cfa9c1f9b3961) fix: drop linux capabilities
* [`f65d0b73`](https://github.com/talos-systems/talos/commit/f65d0b739bd36a57979f9bf26c3092ac544e607c) docs: add 0.11.3 AMIs
* [`7332d636`](https://github.com/talos-systems/talos/commit/7332d63695074dd5eef35ad545d48aff857fbde8) fix: bump pkgs for new kernel 5.10.52
* [`70d2505b`](https://github.com/talos-systems/talos/commit/70d2505b7c8807cb5d4f8a017f9f6200757e13e0) fix: do not require ToVersion to be set when detecting version
* [`0953b199`](https://github.com/talos-systems/talos/commit/0953b1998579f855adffff4b83db917f26687a7b) chore: update extras to bring a new CNI bundle
* [`b6c47f86`](https://github.com/talos-systems/talos/commit/b6c47f866a57bafb60f85fb1ce10428ed3f52c4a) fix: set the /etc/os-release HOME_URL parameter
* [`c780821d`](https://github.com/talos-systems/talos/commit/c780821d0b8fda0b3ef6d33b63b595e40970a897) feat: update containerd to 1.5.3, runc to 1.0.1
* [`f8f1c83a`](https://github.com/talos-systems/talos/commit/f8f1c83a757f5a729896174f95f83c6d804d4858) feat: detect the lowest Kubernetes version in upgrade-k8s CLI command
* [`55e17ccd`](https://github.com/talos-systems/talos/commit/55e17ccdd1df789466ccfb0c9cfe55a62b437f77) chore: bump dependencies
* [`da6f786c`](https://github.com/talos-systems/talos/commit/da6f786cab80cbacb886d34b7c5e0ed957cc24c9) fix: kuberentes => kubernetes typo
* [`2e463348`](https://github.com/talos-systems/talos/commit/2e463348b26fb8b36657b8cb6871e4bce8030b0b) fix: pass all logs through the options.Log method
* [`4e9c5afb`](https://github.com/talos-systems/talos/commit/4e9c5afb6dd6bdedb4032b7cf4a24b6f1bf88144) fix: make ethtool optional in link status controller
* [`bf61c2cc`](https://github.com/talos-systems/talos/commit/bf61c2cc4a51d290fe98aaeb80224bdd52bb7ac5) fix: write upgrade logs only to the LogOutput if it's defined
* [`9c73257c`](https://github.com/talos-systems/talos/commit/9c73257cb128a76459b7d4442b56a50feed089d6) feat: update Go to 1.16.6
* [`23ef1d40`](https://github.com/talos-systems/talos/commit/23ef1d40af44b873d60337d691f878e2cfe0fe8d) chore: add ability to redirect talos upgrade module logs to io.Writer
* [`33e9d6c9`](https://github.com/talos-systems/talos/commit/33e9d6c984f82af24ad79e002758841935e60a6a) chore: bump github.com/aws/aws-sdk-go in /hack/cloud-image-uploader
* [`604434c4`](https://github.com/talos-systems/talos/commit/604434c43eb63aa760cd2176aa1041b653c9bd75) chore: bump github.com/prometheus/procfs from 0.6.0 to 0.7.0
* [`2ea28f62`](https://github.com/talos-systems/talos/commit/2ea28f62d8dcac3280d7a133ae6532f3ca5709cc) chore: bump node from 16.3.0-alpine to 16.4.2-alpine
* [`b358a189`](https://github.com/talos-systems/talos/commit/b358a189bcbaa480d1bb3fbcc58eecd1b61f447d) fix: correctly pick route scope for link-local destination
* [`6848d431`](https://github.com/talos-systems/talos/commit/6848d431427636e415436cdda95543a9a0da5676) feat: can change clusterdns ip lists
* [`72b76abf`](https://github.com/talos-systems/talos/commit/72b76abfd43d04aa7a9283669925bd49498dc05f) fix: workaround issues when IPv6 is fully or partially disabled
* [`679b08f4`](https://github.com/talos-systems/talos/commit/679b08f4fabd098311786551e75e38c2a027bd31) docs: update docs for 0.12
* [`6fbec9e0`](https://github.com/talos-systems/talos/commit/6fbec9e0cb656f411cceb986560473b1a40b6a45) fix: cache etcd client used for healthchecks
* [`eea750de`](https://github.com/talos-systems/talos/commit/eea750de2c11a9883f343c65a36e30712b987f89) chore: rename "join" type to "worker"
* [`951493ac`](https://github.com/talos-systems/talos/commit/951493ac8356a414ff85fce25e30e4bd808b412c) docs: update what's new for Talos 0.11
* [`b47d1098`](https://github.com/talos-systems/talos/commit/b47d1098b1f1cbd21c501266ffc4a38711ed213f) docs: promote 0.11 docs to be the latest
* [`d930a265`](https://github.com/talos-systems/talos/commit/d930a26502759cebccb05d9b78741e1fc147b30b) chore: implement DeepCopy for machine configuration
* [`fe4ed3c7`](https://github.com/talos-systems/talos/commit/fe4ed3c734e5713b2fa1d639bd80bffc7888d7e7) chore: ignore tags which don't look like semantic version
* [`b969e772`](https://github.com/talos-systems/talos/commit/b969e7720ebcb0103e94494533d819a91dba59f5) chore: update references to old protobuf package
* [`2ba8ac9a`](https://github.com/talos-systems/talos/commit/2ba8ac9ab4b24572512c2a877acd26b912b5423a) docs: add documentation directory for 0.12
* [`011e2885`](https://github.com/talos-systems/talos/commit/011e2885e7f88a3a92f3f495fdc1d3be6ed0c877) fix: validate bond slaves addressing
* [`10c28758`](https://github.com/talos-systems/talos/commit/10c28758a4fc50a5e5a29097769b4a3a92ed249a) fix: ignore DeadlineExceeded error correctly on bootstrap
* [`77fabace`](https://github.com/talos-systems/talos/commit/77fabaceca242f89949d4bf231e9754b4d04eb5e) chore: ignore future pkg/machinery/vX.Y.Z tags
* [`6b661114`](https://github.com/talos-systems/talos/commit/6b661114d03a7cd1ddd8939ea323d4fe2ce9976c) fix: make COSI runtime history depth smaller
* [`9bf899bd`](https://github.com/talos-systems/talos/commit/9bf899bdd852befbb4aa5ac4f3ceecb3c33502c8) fix: make forfeit leadership connect to the right node
* [`4708beae`](https://github.com/talos-systems/talos/commit/4708beaee53e3aacbeec07c38cdd2c7316d16a4c) feat: implement `talosctl config info` command
* [`6d13d2cf`](https://github.com/talos-systems/talos/commit/6d13d2cf9243adce739673f1982cbc1f12252ef1) fix: close Kubernetes API client
* [`aaa36f3b`](https://github.com/talos-systems/talos/commit/aaa36f3b4fb250d2921f35c09bcb01b6c31ad423) fix: ignore 'not a leader' error on forfeit leadership
* [`22a41936`](https://github.com/talos-systems/talos/commit/22a4193678d2245b4c24b7e173d4cfd5fa876e95) fix: workaround 'Unauthorized' errors when accessing Kubernetes API
* [`71c6f700`](https://github.com/talos-systems/talos/commit/71c6f7004e28c8a72410652d7d38f770bcf8a5f8) chore: bump go.mod dependencies
* [`915cd8fe`](https://github.com/talos-systems/talos/commit/915cd8fe20c55112cc1fa7776c115ac85c7f3da9) docs: add guide for RBAC
* [`f5721050`](https://github.com/talos-systems/talos/commit/f5721050deffe61f892a9fca2d20b3fccb5021a6) fix: controlplane keyusage
* [`3d772661`](https://github.com/talos-systems/talos/commit/3d7726613ca5c5e6b14b4854564d71ee3644d32e) fix: fill uuid argument correctly in the config download URL
* [`d8602025`](https://github.com/talos-systems/talos/commit/d8602025c828189fa15350a15bf3ccefe39bd0ce) chore: update containerd config version 2
* [`5949ec4e`](https://github.com/talos-systems/talos/commit/5949ec4e6e05ada904d69a24c9d21e20cc7dea85) docs: describe the new network configuration subsystem
* [`444d72b4`](https://github.com/talos-systems/talos/commit/444d72b4d7cff7b38c8e3a483bbe10c74251448a) feat: update pkgs version
* [`e883c12b`](https://github.com/talos-systems/talos/commit/e883c12b31e2ddc3860abc04e7c0867701f46026) fix: make output of `upgrade-k8s` command less scary
* [`7f8e50de`](https://github.com/talos-systems/talos/commit/7f8e50de4d9a36dae9de7783d71a981fb6a72854) fix: restart the merge controllers on conflict
* [`60d73609`](https://github.com/talos-systems/talos/commit/60d7360944ff6fc1e75f98e37a754f3bb2962144) fix: ignore deadline exceeded errors on bootstrap
* [`ee06dd69`](https://github.com/talos-systems/talos/commit/ee06dd69fc39d5df720a88991caaf3646c6fa349) fix: don't print git sha of the release twice in the dashboard
* [`07fb61e5`](https://github.com/talos-systems/talos/commit/07fb61e5d22da86b434d30f12b84b845ac1a4df7) fix: issue worker apid certs properly on renewal
* [`84817f73`](https://github.com/talos-systems/talos/commit/84817f733458cbd35549eebc72df6a5df202b299) chore: bump Talos version in upgrade tests
* [`2fa54107`](https://github.com/talos-systems/talos/commit/2fa54107b2c84cabe948ace5d70836dd4be95799) chore: fix tests for disabled RBAC
* [`78583ba9`](https://github.com/talos-systems/talos/commit/78583ba985fa2b90ec610d148b2cbeb0b92d646b) fix: don't set bond delay options if miimon is not enabled
* [`bbf1c091`](https://github.com/talos-systems/talos/commit/bbf1c091d4cea0b4610bce7165a98c7572423b01) feat: add RBAC to `talosctl version` output
* [`5f6ec3ef`](https://github.com/talos-systems/talos/commit/5f6ec3ef66c8bf2cb334e02b5aa9869330c985d8) fix: handle cases when merged resource re-appears before being destroyed
* [`1e9a0e74`](https://github.com/talos-systems/talos/commit/1e9a0e745db73bd45ec0881aa19e43d7badb5914) fix: documentation typos
* [`f228af40`](https://github.com/talos-systems/talos/commit/f228af4061e2025531c953fdb7f8bf83de4bf8b0) chore: bump go.mod dependencies
* [`2060ceaa`](https://github.com/talos-systems/talos/commit/2060ceaa0b16be04a61a00e0085e25889ffe613a) chore: add CAPI version to CI setup
* [`ad047a7d`](https://github.com/talos-systems/talos/commit/ad047a7dee4c0ac26c01862bdaa923fab93cc2e1) chore: small RBAC improvements
</p>
</details>

### Changes since v0.12.0-alpha.0
<details><summary>12 commits</summary>
<p>

* [`1ed5e545`](https://github.com/talos-systems/talos/commit/1ed5e545385e160fe3b61e6dbbcaa8a701437b62) feat: add ClusterID and ClusterSecret
* [`228b3761`](https://github.com/talos-systems/talos/commit/228b376163597cd825e4a142e6b4bdea0f870365) chore: run etcd as non-root user
* [`3518219b`](https://github.com/talos-systems/talos/commit/3518219bff44f71a60ad8e448e518844d1b933fd) chore: drop deprecated `--no-reboot` param and KernelCurrentRoot const
* [`33d1c3e4`](https://github.com/talos-systems/talos/commit/33d1c3e42582649f25a44fc3c86007bcebbc80b3) chore: run apid and trustd services as non-root user
* [`dadaa65d`](https://github.com/talos-systems/talos/commit/dadaa65d542171d25317840fcf35fa3979cf0632) feat: print uid/gid for the files in `ls -l`
* [`e6fa401b`](https://github.com/talos-systems/talos/commit/e6fa401b663d0ebd4374c9e47a7ca6150a4756cd) fix: enable seccomp default profile by default
* [`8ddbcc96`](https://github.com/talos-systems/talos/commit/8ddbcc9643113c15de538fc070b7053d1c6efdfc) feat: validate if extra fields present in the decoder
* [`5b57a980`](https://github.com/talos-systems/talos/commit/5b57a98008c64d7cb07729fd9b31a0e3493c289c) chore: update Go to 1.16.7, Linux to 5.10.57
* [`eefe1c21`](https://github.com/talos-systems/talos/commit/eefe1c21c30fa2cd281fc5524b2e88553f6fdfcc) feat: add new etcd members in learner mode
* [`b1c66fba`](https://github.com/talos-systems/talos/commit/b1c66fbad113400729cf4db806e30192bf7e0462) feat: implement Equinix Metal support for virtual (shared) IP
* [`62242f97`](https://github.com/talos-systems/talos/commit/62242f979e1921ed8abfa06a26564ea0bf8a5fb3) chore: require GPG signatures
* [`faecae44`](https://github.com/talos-systems/talos/commit/faecae44fde60fc626ccb01da3b221519a9d41d7) feat: make ISO builds reproducible
</p>
</details>

### Changes from talos-systems/crypto
<details><summary>1 commit</summary>
<p>

* [`deec8d4`](https://github.com/talos-systems/crypto/commit/deec8d47700e10e3ea813bdce01377bd93c83367) chore: implement DeepCopy methods for PEMEncoded* types
</p>
</details>

### Changes from talos-systems/extras
<details><summary>3 commits</summary>
<p>

* [`8ce17e5`](https://github.com/talos-systems/extras/commit/8ce17e5e5d60dce7b46cf87555400f7951fe9fda) chore: bump tools and packages for Go 1.16.7
* [`4957f3c`](https://github.com/talos-systems/extras/commit/4957f3c64bc5fd1574fe3d3f251f52e914e78e41) chore: update pkgs to use CNI plugins v0.9.1
* [`233716a`](https://github.com/talos-systems/extras/commit/233716a04f1e4e1762101b279308630caa46d17d) feat: update Go to 1.16.6
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>4 commits</summary>
<p>

* [`fe24303`](https://github.com/talos-systems/go-blockdevice/commit/fe2430349e9d734ce6dbf4e7b2e0f8a37bb22679) fix: perform correct PMBR partition calculations
* [`2ec0c3c`](https://github.com/talos-systems/go-blockdevice/commit/2ec0c3cc0ff5ff705ed5c910ca1bcd5d93c7b102) fix: preserve the PMBR bootable flag when opening GPT partition
* [`87816a8`](https://github.com/talos-systems/go-blockdevice/commit/87816a81cefc728cfe3cb221b476d8ed4b609fd8) feat: align partition to minimum I/O size
* [`c34b59f`](https://github.com/talos-systems/go-blockdevice/commit/c34b59fb33a7ad8be18bb19bc8c8d8294b4b3a78) feat: expose more encryption options in the LUKS module
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>17 commits</summary>
<p>

* [`da4ac04`](https://github.com/talos-systems/pkgs/commit/da4ac04969924256df4ebc66d3bf435a52e30cb7) chore: bump tools for Go 1.16.7
* [`10275fb`](https://github.com/talos-systems/pkgs/commit/10275fbf737aaa0ac41cc7220d824f5d68d3b0fa) feat: update Linux to 5.10.57
* [`875c7ec`](https://github.com/talos-systems/pkgs/commit/875c7ecaacc9e999416a2ba17bea3130261120eb) chore: patch grub with support for reproducible ISO builds
* [`12856ce`](https://github.com/talos-systems/pkgs/commit/12856ce15d6d72814a2f40bbaf3f8ab6efb849f9) feat: increase number of CPUs supported by the kernel to 512
* [`cbfabac`](https://github.com/talos-systems/pkgs/commit/cbfabaca6a3faf20914aae5c535e44a393a4f422) chore: update ca-certificates to 2021-07-05
* [`0c011c0`](https://github.com/talos-systems/pkgs/commit/0c011c088068e5fdb55066008b526ca3ef69f218) feat: update GRUB to 2.06
* [`5090d14`](https://github.com/talos-systems/pkgs/commit/5090d149a669f7eb3cc922196b7e82869c152dae) chore: update containerd to v1.5.5
* [`6653902`](https://github.com/talos-systems/pkgs/commit/66539021daf1037782b1c4009dd96544057628d3) feat: add kernel drivers for fusion and scsi-isci
* [`9b4041f`](https://github.com/talos-systems/pkgs/commit/9b4041fb79d9c5d8e18391f1e2f4843a88d26c19) chore: update containerd to v1.5.4
* [`7b6cc05`](https://github.com/talos-systems/pkgs/commit/7b6cc05ceee8c24e746afa7ed105f9f55fef589b) feat: update kernel to latest 5.10.52
* [`65159fb`](https://github.com/talos-systems/pkgs/commit/65159fb19c3138ec612cdca507e5cc795b657a7d) chore: update runc and CNI plugins
* [`514ba34`](https://github.com/talos-systems/pkgs/commit/514ba3420a0773ac7305d00e8b582858f9685953) feat: disable aufs, devmapper, zfs
* [`6bc118f`](https://github.com/talos-systems/pkgs/commit/6bc118f37cfd018183952b9feb009c54f1a3c215) chore: update runc and containerd
* [`b6fca88`](https://github.com/talos-systems/pkgs/commit/b6fca88d22436a0fb78b8a4e06792b7af1a22ef5) feat: update Go to 1.16.6
* [`fd56852`](https://github.com/talos-systems/pkgs/commit/fd568520e8c77bd8d96f96efb47dd2bdd2f36c1a) chore: update `open-isns` and `open-iscsi`
* [`d779204`](https://github.com/talos-systems/pkgs/commit/d779204c0d9e9c8e90f32b1f68eb9ff4b030b83c) chore: update dosfstools to v4.2
* [`bc7c0d7`](https://github.com/talos-systems/pkgs/commit/bc7c0d7c6afaec8226c2a52299981ac519b5e595) feat: add support for hotplug of PCIE devices
</p>
</details>

### Changes from talos-systems/tools
<details><summary>5 commits</summary>
<p>

* [`2368154`](https://github.com/talos-systems/tools/commit/23681542fc7e29ede59b3775e04089c5b1a0f666) feat: update Go and protoc-gen-go tools
* [`7172a5d`](https://github.com/talos-systems/tools/commit/7172a5db9d361527aa7bd9c7af407b9d578e2e02) feat: update Go to 1.16.6
* [`1de34d7`](https://github.com/talos-systems/tools/commit/1de34d7961c7ac86f369217dea4ce69cdde04122) chore: update musl
* [`76979a1`](https://github.com/talos-systems/tools/commit/76979a1c194c74c25db22c9ec90ec36f97179e3f) chore: update protobuf deps
* [`0846c64`](https://github.com/talos-systems/tools/commit/0846c6493316b5d00ecc241b7051ced1bac1cf7e) chore: update expat
</p>
</details>

### Dependency Changes

* **github.com/BurntSushi/toml**               v0.3.1 -> v0.4.1
* **github.com/aws/aws-sdk-go**                v1.38.66 -> v1.40.2
* **github.com/containerd/containerd**         v1.5.2 -> v1.5.5
* **github.com/cosi-project/runtime**          93ead370bf57 -> 25f235cd0682
* **github.com/docker/docker**                 v20.10.7 -> v20.10.8
* **github.com/google/uuid**                   v1.2.0 -> v1.3.0
* **github.com/hashicorp/go-getter**           v1.5.4 -> v1.5.6
* **github.com/opencontainers/runtime-spec**   e6143ca7d51d -> 1c3f411f0417
* **github.com/packethost/packngo**            v0.19.0 **_new_**
* **github.com/prometheus/procfs**             v0.6.0 -> v0.7.2
* **github.com/rivo/tview**                    d4fb0348227b -> 29d673af0ce2
* **github.com/spf13/cobra**                   v1.1.3 -> v1.2.1
* **github.com/talos-systems/crypto**          v0.3.1 -> deec8d47700e
* **github.com/talos-systems/extras**          v0.4.0 -> v0.5.0-alpha.0-2-g8ce17e5
* **github.com/talos-systems/go-blockdevice**  v0.2.1 -> v0.2.3
* **github.com/talos-systems/pkgs**            v0.6.0-1-g7b2e126 -> v0.7.0-alpha.0-16-gda4ac04
* **github.com/talos-systems/tools**           v0.6.0 -> v0.7.0-alpha.0-3-g2368154
* **github.com/vmware-tanzu/sonobuoy**         v0.52.0 -> v0.53.0
* **go.uber.org/zap**                          v1.17.0 -> v1.18.1
* **golang.org/x/net**                         04defd469f4e -> 853a461950ff
* **golang.org/x/sys**                         59db8d763f22 -> 0f9fa26af87c
* **golang.org/x/time**                        38a9dc6acbc6 -> 1f47c861a9ac
* **google.golang.org/grpc**                   v1.38.0 -> v1.39.1
* **google.golang.org/protobuf**               v1.26.0 -> v1.27.1
* **inet.af/netaddr**                          bf05d8b52dda -> ce7a8ad02cc1
* **k8s.io/api**                               v0.21.2 -> v0.22.0
* **k8s.io/apimachinery**                      v0.21.2 -> v0.22.0
* **k8s.io/apiserver**                         v0.21.2 -> v0.22.0
* **k8s.io/client-go**                         v0.21.2 -> v0.22.0
* **k8s.io/cri-api**                           v0.21.2 -> v0.22.0
* **k8s.io/kubectl**                           v0.21.2 -> v0.22.0
* **k8s.io/kubelet**                           v0.21.2 -> v0.22.0

Previous release can be found at [v0.11.0](https://github.com/talos-systems/talos/releases/tag/v0.11.0)

## [Talos 0.12.0-alpha.0](https://github.com/talos-systems/talos/releases/tag/v0.12.0-alpha.0) (2021-08-11)

Welcome to the v0.12.0-alpha.0 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Support for Self-hosted Control Plane Dropped

> **Note**: This item only applies to clusters bootstrapped with Talos <= 0.8.

Talos 0.12 completely removes support for self-hosted Kubernetes control plane (bootkube-based).
Talos 0.9 introduced support for Talos-managed control plane and provided migration path to convert self-hosted control plane
to Talos-managed static pods.
Automated and manual conversion process is available in Talos from 0.9.x to 0.11.x.
For clusters bootstrapped with bootkube (Talos <= 0.8), please make sure control plane is converted to Talos-managed before
before upgrading to Talos 0.12.
Current control plane status can be checked with `talosctl get bootstrapstatus` before performing upgrade to Talos 0.12.


### Cluster API v0.3.x

Cluster API v0.3.x (v1alpha3) is not compatible with Kubernetes 1.22 used by default in Talos 0.12.
Talos can be configued to use Kubernetes 1.21 or CAPI v0.4.x components can be used instead.


### Sysctl Configuration

Sysctl Kernel Params configuration was completely rewritten to be based on controllers and resources,
which makes it possible to apply `.machine.sysctls` in immediate mode (without a reboot).
`talosctl get kernelparams` returns merged list of KSPP, Kubernetes and user defined params along with
the default values overwritten by Talos.


### Join Node Type

Node type `join` was renamed to `worker` for clarity. The old value is still accepted in the machine configuration but deprecated.
`talosctl gen config` now generates `worker.yaml` instead of `join.yaml`.


### Networking

* multiple static addresses can be specified for the interface with new `.addresses` field (old `.cidr` field is deprecated now)
* static addresses can be set on interfaces configured with DHCP


### Performance

* machined uses less memory and CPU time
* more disk encryption options are exposed via the machine configuration
* disk partitions are now aligned properly with minimum I/O size
* Talos system processes are moved under proper cgroups, resource metrics are now available via the kubelet
* OOM score is set on the system processes making sure they are killed last under memory pressure


### Security

* etcd PKI moved to `/system/secrets`
* kubelet bootstrap CSR auto-signing scoped to kubelet bootstrap tokens only


### Component Updates

* Linux: 5.10.52
* Kubernetes: 1.22.0
* containerd: 1.5.5
* runc: 1.0.1
* GRUB: 2.06
* Talos is built with Go 1.16.6


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Serge Logvinov
* Andrey Smirnov
* Artem Chernyshev
* Spencer Smith
* Alexey Palazhchenko
* dependabot[bot]
* Rui Lopes
* Andrew Rynhard
* Caleb Woodbine

### Changes
<details><summary>96 commits</summary>
<p>

* [`a15f0184`](https://github.com/talos-systems/talos/commit/a15f01844fdaf0d3e2dad2750d9353d03e18dea2) fix: move etcd PKI under /system/secrets
* [`eb02afe1`](https://github.com/talos-systems/talos/commit/eb02afe18be63bf483a0467f655611561aef10f6) fix: match correctly routes on the address family
* [`cb948acc`](https://github.com/talos-systems/talos/commit/cb948accfeca13c57b3b512dc8a06425989294f9) feat: allow multiple addresses per interface
* [`e030b2e8`](https://github.com/talos-systems/talos/commit/e030b2e8bb0a65abf4e1f7b5f27348631210ebc4) chore: use k8s 1.21.3 in CAPI tests for now
* [`e08b4f8f`](https://github.com/talos-systems/talos/commit/e08b4f8f9e72f8db1116b4bbe395d49b4bccb460) feat: implement sysctl controllers
* [`fdf6b243`](https://github.com/talos-systems/talos/commit/fdf6b2433c40613bcb039852a96196dbe9b7b5e2) chore: revert "improve artifacts generation reproducibility"
* [`b68ed1eb`](https://github.com/talos-systems/talos/commit/b68ed1eb896039ec1319db2e3d6d364034c86863) fix: make route resources ID match closer routing table primary key
* [`585f6337`](https://github.com/talos-systems/talos/commit/585f633710abb7a6d863b54c37aa65c50a3c7312) fix: correctly handle nodoc for struct fields
* [`f2d394dc`](https://github.com/talos-systems/talos/commit/f2d394dc42f9ec704050db0a8a928a889483ce3e) docs: add AMIs for v0.11.5
* [`d0970cbf`](https://github.com/talos-systems/talos/commit/d0970cbfd696b28b201b232a03da2119f664afbd) feat: bootstrap token limit
* [`5285a46d`](https://github.com/talos-systems/talos/commit/5285a46d78ef2fc76594aad4ad4acb75312bc0a7) fix: maintenance mode reason message
* [`009d15e8`](https://github.com/talos-systems/talos/commit/009d15e8dc6e75eca6b5963dddf8063941099f14) chore: use etcd client TryLock function on upgrade
* [`4dae9ea5`](https://github.com/talos-systems/talos/commit/4dae9ea55c087c28a9d7a8d241e0ec3a7a1b8ca3) chore: use vtprotobuf compiled marshaling in Talos API
* [`7ca5749a`](https://github.com/talos-systems/talos/commit/7ca5749ad4267701ce639d0f0d91c10a7f9c1d3d) chore: bump dependencies via dependabot
* [`b2507b41`](https://github.com/talos-systems/talos/commit/b2507b41d250b989b9c13ad23e16202cd53a18d2) chore: improve artifacts generation reproducibility
* [`1f7dad23`](https://github.com/talos-systems/talos/commit/1f7dad234b480c7a5e3484ccf10180747c979036) chore: update PKGS version (512 cpus, new ca-certficates)
* [`1a2e78a2`](https://github.com/talos-systems/talos/commit/1a2e78a24e997241c4cd18dfac3c2d971ba78116) fix: update go-blockdevice
* [`6d6ed117`](https://github.com/talos-systems/talos/commit/6d6ed1170f3f28e7f559ccdf64e7c34dfee022a0) chore: use parallel xz with higher compression level
* [`571f7db1`](https://github.com/talos-systems/talos/commit/571f7db1bb44a0dcb5e373f9c37396d50eb0e8f4) chore: workaround GitHub new release notes limit
* [`09d70b7e`](https://github.com/talos-systems/talos/commit/09d70b7eafb18343eb4ca57d7f8b84e4ccd2fcfb) feat: update Kubernetes to v1.22.0
* [`f25f10e7`](https://github.com/talos-systems/talos/commit/f25f10e73ec534acd7cc483f254d612d8a7c1858) feat: add an option to disable PSP
* [`7c6e4cf2`](https://github.com/talos-systems/talos/commit/7c6e4cf230ba1f30da664374c41c934d1e6620bc) feat: allow both DHCP and static addressing for the interface
* [`3c566dbc`](https://github.com/talos-systems/talos/commit/3c566dbc30595467a3789707c6e993aa92f36df6) fix: remove admission plugins enabled by default from the list
* [`69ead373`](https://github.com/talos-systems/talos/commit/69ead37353b7e3aa7f089c70073037a6eba37767) fix: preserve PMBR bootable flag correctly
* [`dee63051`](https://github.com/talos-systems/talos/commit/dee63051702d49f495bfb28b4be74ed8b39143ad) fix: align partitions with minimal I/O size
* [`62890229`](https://github.com/talos-systems/talos/commit/628902297d2efe93e6388377b2ea6d4beda83095) feat: update GRUB to 2.06
* [`b9d04928`](https://github.com/talos-systems/talos/commit/b9d04928d960f9d576671c6f3511cf242ff31cb7) feat: move system processes to cgroups
* [`0b8681b4`](https://github.com/talos-systems/talos/commit/0b8681b4b49ab109b8863792d48c2f551d1ceeb5) fix: resolve several issues with Wireguard link specs
* [`f8f4bf3b`](https://github.com/talos-systems/talos/commit/f8f4bf3baef31d4ac957ec68cd869adea1e931cd) docs: add disk encryptions examples
* [`79b8fa64`](https://github.com/talos-systems/talos/commit/79b8fa64b9453917860faae3df5d14647186b9ba) feat: update containerd to 1.5.5
* [`539f4209`](https://github.com/talos-systems/talos/commit/539f42090e436921a23087296cde6eaf7e495b5e) chore: bump dependencies via dependabot
* [`0c7ce1cd`](https://github.com/talos-systems/talos/commit/0c7ce1cd814354213a1a6c7a9251b166ee58c493) feat: remove remnants of bootkube support
* [`d4f9804f`](https://github.com/talos-systems/talos/commit/d4f9804f8659562f6152ae73cb1788f6f6d6ad89) chore: fix typos
* [`5f027615`](https://github.com/talos-systems/talos/commit/5f027615ffac68e0a484a5da4827a6589bae3880) feat: expose more encryption options to the machine config
* [`585152a0`](https://github.com/talos-systems/talos/commit/585152a0be051accd4cb8b7c2f130c5a92dfd32d) chore: bump dependencies
* [`fc66ec59`](https://github.com/talos-systems/talos/commit/fc66ec59691fb1b9d00b27e1f7b34c870a09d717) feat: set oom score for main processes
* [`df54584a`](https://github.com/talos-systems/talos/commit/df54584a33d88de13deadcb87a5cfa9c1f9b3961) fix: drop linux capabilities
* [`f65d0b73`](https://github.com/talos-systems/talos/commit/f65d0b739bd36a57979f9bf26c3092ac544e607c) docs: add 0.11.3 AMIs
* [`7332d636`](https://github.com/talos-systems/talos/commit/7332d63695074dd5eef35ad545d48aff857fbde8) fix: bump pkgs for new kernel 5.10.52
* [`70d2505b`](https://github.com/talos-systems/talos/commit/70d2505b7c8807cb5d4f8a017f9f6200757e13e0) fix: do not require ToVersion to be set when detecting version
* [`0953b199`](https://github.com/talos-systems/talos/commit/0953b1998579f855adffff4b83db917f26687a7b) chore: update extras to bring a new CNI bundle
* [`b6c47f86`](https://github.com/talos-systems/talos/commit/b6c47f866a57bafb60f85fb1ce10428ed3f52c4a) fix: set the /etc/os-release HOME_URL parameter
* [`c780821d`](https://github.com/talos-systems/talos/commit/c780821d0b8fda0b3ef6d33b63b595e40970a897) feat: update containerd to 1.5.3, runc to 1.0.1
* [`f8f1c83a`](https://github.com/talos-systems/talos/commit/f8f1c83a757f5a729896174f95f83c6d804d4858) feat: detect the lowest Kubernetes version in upgrade-k8s CLI command
* [`55e17ccd`](https://github.com/talos-systems/talos/commit/55e17ccdd1df789466ccfb0c9cfe55a62b437f77) chore: bump dependencies
* [`da6f786c`](https://github.com/talos-systems/talos/commit/da6f786cab80cbacb886d34b7c5e0ed957cc24c9) fix: kuberentes => kubernetes typo
* [`2e463348`](https://github.com/talos-systems/talos/commit/2e463348b26fb8b36657b8cb6871e4bce8030b0b) fix: pass all logs through the options.Log method
* [`4e9c5afb`](https://github.com/talos-systems/talos/commit/4e9c5afb6dd6bdedb4032b7cf4a24b6f1bf88144) fix: make ethtool optional in link status controller
* [`bf61c2cc`](https://github.com/talos-systems/talos/commit/bf61c2cc4a51d290fe98aaeb80224bdd52bb7ac5) fix: write upgrade logs only to the LogOutput if it's defined
* [`9c73257c`](https://github.com/talos-systems/talos/commit/9c73257cb128a76459b7d4442b56a50feed089d6) feat: update Go to 1.16.6
* [`23ef1d40`](https://github.com/talos-systems/talos/commit/23ef1d40af44b873d60337d691f878e2cfe0fe8d) chore: add ability to redirect talos upgrade module logs to io.Writer
* [`33e9d6c9`](https://github.com/talos-systems/talos/commit/33e9d6c984f82af24ad79e002758841935e60a6a) chore: bump github.com/aws/aws-sdk-go in /hack/cloud-image-uploader
* [`604434c4`](https://github.com/talos-systems/talos/commit/604434c43eb63aa760cd2176aa1041b653c9bd75) chore: bump github.com/prometheus/procfs from 0.6.0 to 0.7.0
* [`2ea28f62`](https://github.com/talos-systems/talos/commit/2ea28f62d8dcac3280d7a133ae6532f3ca5709cc) chore: bump node from 16.3.0-alpine to 16.4.2-alpine
* [`b358a189`](https://github.com/talos-systems/talos/commit/b358a189bcbaa480d1bb3fbcc58eecd1b61f447d) fix: correctly pick route scope for link-local destination
* [`6848d431`](https://github.com/talos-systems/talos/commit/6848d431427636e415436cdda95543a9a0da5676) feat: can change clusterdns ip lists
* [`72b76abf`](https://github.com/talos-systems/talos/commit/72b76abfd43d04aa7a9283669925bd49498dc05f) fix: workaround issues when IPv6 is fully or partially disabled
* [`679b08f4`](https://github.com/talos-systems/talos/commit/679b08f4fabd098311786551e75e38c2a027bd31) docs: update docs for 0.12
* [`6fbec9e0`](https://github.com/talos-systems/talos/commit/6fbec9e0cb656f411cceb986560473b1a40b6a45) fix: cache etcd client used for healthchecks
* [`eea750de`](https://github.com/talos-systems/talos/commit/eea750de2c11a9883f343c65a36e30712b987f89) chore: rename "join" type to "worker"
* [`951493ac`](https://github.com/talos-systems/talos/commit/951493ac8356a414ff85fce25e30e4bd808b412c) docs: update what's new for Talos 0.11
* [`b47d1098`](https://github.com/talos-systems/talos/commit/b47d1098b1f1cbd21c501266ffc4a38711ed213f) docs: promote 0.11 docs to be the latest
* [`d930a265`](https://github.com/talos-systems/talos/commit/d930a26502759cebccb05d9b78741e1fc147b30b) chore: implement DeepCopy for machine configuration
* [`fe4ed3c7`](https://github.com/talos-systems/talos/commit/fe4ed3c734e5713b2fa1d639bd80bffc7888d7e7) chore: ignore tags which don't look like semantic version
* [`b969e772`](https://github.com/talos-systems/talos/commit/b969e7720ebcb0103e94494533d819a91dba59f5) chore: update references to old protobuf package
* [`2ba8ac9a`](https://github.com/talos-systems/talos/commit/2ba8ac9ab4b24572512c2a877acd26b912b5423a) docs: add documentation directory for 0.12
* [`011e2885`](https://github.com/talos-systems/talos/commit/011e2885e7f88a3a92f3f495fdc1d3be6ed0c877) fix: validate bond slaves addressing
* [`10c28758`](https://github.com/talos-systems/talos/commit/10c28758a4fc50a5e5a29097769b4a3a92ed249a) fix: ignore DeadlineExceeded error correctly on bootstrap
* [`77fabace`](https://github.com/talos-systems/talos/commit/77fabaceca242f89949d4bf231e9754b4d04eb5e) chore: ignore future pkg/machinery/vX.Y.Z tags
* [`6b661114`](https://github.com/talos-systems/talos/commit/6b661114d03a7cd1ddd8939ea323d4fe2ce9976c) fix: make COSI runtime history depth smaller
* [`9bf899bd`](https://github.com/talos-systems/talos/commit/9bf899bdd852befbb4aa5ac4f3ceecb3c33502c8) fix: make forfeit leadership connect to the right node
* [`4708beae`](https://github.com/talos-systems/talos/commit/4708beaee53e3aacbeec07c38cdd2c7316d16a4c) feat: implement `talosctl config info` command
* [`6d13d2cf`](https://github.com/talos-systems/talos/commit/6d13d2cf9243adce739673f1982cbc1f12252ef1) fix: close Kubernetes API client
* [`aaa36f3b`](https://github.com/talos-systems/talos/commit/aaa36f3b4fb250d2921f35c09bcb01b6c31ad423) fix: ignore 'not a leader' error on forfeit leadership
* [`22a41936`](https://github.com/talos-systems/talos/commit/22a4193678d2245b4c24b7e173d4cfd5fa876e95) fix: workaround 'Unauthorized' errors when accessing Kubernetes API
* [`71c6f700`](https://github.com/talos-systems/talos/commit/71c6f7004e28c8a72410652d7d38f770bcf8a5f8) chore: bump go.mod dependencies
* [`915cd8fe`](https://github.com/talos-systems/talos/commit/915cd8fe20c55112cc1fa7776c115ac85c7f3da9) docs: add guide for RBAC
* [`f5721050`](https://github.com/talos-systems/talos/commit/f5721050deffe61f892a9fca2d20b3fccb5021a6) fix: controlplane keyusage
* [`3d772661`](https://github.com/talos-systems/talos/commit/3d7726613ca5c5e6b14b4854564d71ee3644d32e) fix: fill uuid argument correctly in the config download URL
* [`d8602025`](https://github.com/talos-systems/talos/commit/d8602025c828189fa15350a15bf3ccefe39bd0ce) chore: update containerd config version 2
* [`5949ec4e`](https://github.com/talos-systems/talos/commit/5949ec4e6e05ada904d69a24c9d21e20cc7dea85) docs: describe the new network configuration subsystem
* [`444d72b4`](https://github.com/talos-systems/talos/commit/444d72b4d7cff7b38c8e3a483bbe10c74251448a) feat: update pkgs version
* [`e883c12b`](https://github.com/talos-systems/talos/commit/e883c12b31e2ddc3860abc04e7c0867701f46026) fix: make output of `upgrade-k8s` command less scary
* [`7f8e50de`](https://github.com/talos-systems/talos/commit/7f8e50de4d9a36dae9de7783d71a981fb6a72854) fix: restart the merge controllers on conflict
* [`60d73609`](https://github.com/talos-systems/talos/commit/60d7360944ff6fc1e75f98e37a754f3bb2962144) fix: ignore deadline exceeded errors on bootstrap
* [`ee06dd69`](https://github.com/talos-systems/talos/commit/ee06dd69fc39d5df720a88991caaf3646c6fa349) fix: don't print git sha of the release twice in the dashboard
* [`07fb61e5`](https://github.com/talos-systems/talos/commit/07fb61e5d22da86b434d30f12b84b845ac1a4df7) fix: issue worker apid certs properly on renewal
* [`84817f73`](https://github.com/talos-systems/talos/commit/84817f733458cbd35549eebc72df6a5df202b299) chore: bump Talos version in upgrade tests
* [`2fa54107`](https://github.com/talos-systems/talos/commit/2fa54107b2c84cabe948ace5d70836dd4be95799) chore: fix tests for disabled RBAC
* [`78583ba9`](https://github.com/talos-systems/talos/commit/78583ba985fa2b90ec610d148b2cbeb0b92d646b) fix: don't set bond delay options if miimon is not enabled
* [`bbf1c091`](https://github.com/talos-systems/talos/commit/bbf1c091d4cea0b4610bce7165a98c7572423b01) feat: add RBAC to `talosctl version` output
* [`5f6ec3ef`](https://github.com/talos-systems/talos/commit/5f6ec3ef66c8bf2cb334e02b5aa9869330c985d8) fix: handle cases when merged resource re-appears before being destroyed
* [`1e9a0e74`](https://github.com/talos-systems/talos/commit/1e9a0e745db73bd45ec0881aa19e43d7badb5914) fix: documentation typos
* [`f228af40`](https://github.com/talos-systems/talos/commit/f228af4061e2025531c953fdb7f8bf83de4bf8b0) chore: bump go.mod dependencies
* [`2060ceaa`](https://github.com/talos-systems/talos/commit/2060ceaa0b16be04a61a00e0085e25889ffe613a) chore: add CAPI version to CI setup
* [`ad047a7d`](https://github.com/talos-systems/talos/commit/ad047a7dee4c0ac26c01862bdaa923fab93cc2e1) chore: small RBAC improvements
</p>
</details>

### Changes from talos-systems/crypto
<details><summary>1 commit</summary>
<p>

* [`deec8d4`](https://github.com/talos-systems/crypto/commit/deec8d47700e10e3ea813bdce01377bd93c83367) chore: implement DeepCopy methods for PEMEncoded* types
</p>
</details>

### Changes from talos-systems/extras
<details><summary>2 commits</summary>
<p>

* [`4957f3c`](https://github.com/talos-systems/extras/commit/4957f3c64bc5fd1574fe3d3f251f52e914e78e41) chore: update pkgs to use CNI plugins v0.9.1
* [`233716a`](https://github.com/talos-systems/extras/commit/233716a04f1e4e1762101b279308630caa46d17d) feat: update Go to 1.16.6
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>4 commits</summary>
<p>

* [`fe24303`](https://github.com/talos-systems/go-blockdevice/commit/fe2430349e9d734ce6dbf4e7b2e0f8a37bb22679) fix: perform correct PMBR partition calculations
* [`2ec0c3c`](https://github.com/talos-systems/go-blockdevice/commit/2ec0c3cc0ff5ff705ed5c910ca1bcd5d93c7b102) fix: preserve the PMBR bootable flag when opening GPT partition
* [`87816a8`](https://github.com/talos-systems/go-blockdevice/commit/87816a81cefc728cfe3cb221b476d8ed4b609fd8) feat: align partition to minimum I/O size
* [`c34b59f`](https://github.com/talos-systems/go-blockdevice/commit/c34b59fb33a7ad8be18bb19bc8c8d8294b4b3a78) feat: expose more encryption options in the LUKS module
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>14 commits</summary>
<p>

* [`12856ce`](https://github.com/talos-systems/pkgs/commit/12856ce15d6d72814a2f40bbaf3f8ab6efb849f9) feat: increase number of CPUs supported by the kernel to 512
* [`cbfabac`](https://github.com/talos-systems/pkgs/commit/cbfabaca6a3faf20914aae5c535e44a393a4f422) chore: update ca-certificates to 2021-07-05
* [`0c011c0`](https://github.com/talos-systems/pkgs/commit/0c011c088068e5fdb55066008b526ca3ef69f218) feat: update GRUB to 2.06
* [`5090d14`](https://github.com/talos-systems/pkgs/commit/5090d149a669f7eb3cc922196b7e82869c152dae) chore: update containerd to v1.5.5
* [`6653902`](https://github.com/talos-systems/pkgs/commit/66539021daf1037782b1c4009dd96544057628d3) feat: add kernel drivers for fusion and scsi-isci
* [`9b4041f`](https://github.com/talos-systems/pkgs/commit/9b4041fb79d9c5d8e18391f1e2f4843a88d26c19) chore: update containerd to v1.5.4
* [`7b6cc05`](https://github.com/talos-systems/pkgs/commit/7b6cc05ceee8c24e746afa7ed105f9f55fef589b) feat: update kernel to latest 5.10.52
* [`65159fb`](https://github.com/talos-systems/pkgs/commit/65159fb19c3138ec612cdca507e5cc795b657a7d) chore: update runc and CNI plugins
* [`514ba34`](https://github.com/talos-systems/pkgs/commit/514ba3420a0773ac7305d00e8b582858f9685953) feat: disable aufs, devmapper, zfs
* [`6bc118f`](https://github.com/talos-systems/pkgs/commit/6bc118f37cfd018183952b9feb009c54f1a3c215) chore: update runc and containerd
* [`b6fca88`](https://github.com/talos-systems/pkgs/commit/b6fca88d22436a0fb78b8a4e06792b7af1a22ef5) feat: update Go to 1.16.6
* [`fd56852`](https://github.com/talos-systems/pkgs/commit/fd568520e8c77bd8d96f96efb47dd2bdd2f36c1a) chore: update `open-isns` and `open-iscsi`
* [`d779204`](https://github.com/talos-systems/pkgs/commit/d779204c0d9e9c8e90f32b1f68eb9ff4b030b83c) chore: update dosfstools to v4.2
* [`bc7c0d7`](https://github.com/talos-systems/pkgs/commit/bc7c0d7c6afaec8226c2a52299981ac519b5e595) feat: add support for hotplug of PCIE devices
</p>
</details>

### Changes from talos-systems/tools
<details><summary>4 commits</summary>
<p>

* [`7172a5d`](https://github.com/talos-systems/tools/commit/7172a5db9d361527aa7bd9c7af407b9d578e2e02) feat: update Go to 1.16.6
* [`1de34d7`](https://github.com/talos-systems/tools/commit/1de34d7961c7ac86f369217dea4ce69cdde04122) chore: update musl
* [`76979a1`](https://github.com/talos-systems/tools/commit/76979a1c194c74c25db22c9ec90ec36f97179e3f) chore: update protobuf deps
* [`0846c64`](https://github.com/talos-systems/tools/commit/0846c6493316b5d00ecc241b7051ced1bac1cf7e) chore: update expat
</p>
</details>

### Dependency Changes

* **github.com/BurntSushi/toml**               v0.3.1 -> v0.4.1
* **github.com/aws/aws-sdk-go**                v1.38.66 -> v1.40.2
* **github.com/containerd/containerd**         v1.5.2 -> v1.5.5
* **github.com/cosi-project/runtime**          93ead370bf57 -> 25f235cd0682
* **github.com/docker/docker**                 v20.10.7 -> v20.10.8
* **github.com/google/uuid**                   v1.2.0 -> v1.3.0
* **github.com/hashicorp/go-getter**           v1.5.4 -> v1.5.6
* **github.com/opencontainers/runtime-spec**   e6143ca7d51d -> 1c3f411f0417
* **github.com/prometheus/procfs**             v0.6.0 -> v0.7.2
* **github.com/rivo/tview**                    d4fb0348227b -> 29d673af0ce2
* **github.com/spf13/cobra**                   v1.1.3 -> v1.2.1
* **github.com/talos-systems/crypto**          v0.3.1 -> deec8d47700e
* **github.com/talos-systems/extras**          v0.4.0 -> v0.5.0-alpha.0-1-g4957f3c
* **github.com/talos-systems/go-blockdevice**  v0.2.1 -> v0.2.3
* **github.com/talos-systems/pkgs**            v0.6.0-1-g7b2e126 -> v0.7.0-alpha.0-13-g12856ce
* **github.com/talos-systems/tools**           v0.6.0 -> v0.7.0-alpha.0-2-g7172a5d
* **github.com/vmware-tanzu/sonobuoy**         v0.52.0 -> v0.53.0
* **go.uber.org/zap**                          v1.17.0 -> v1.18.1
* **golang.org/x/net**                         04defd469f4e -> 853a461950ff
* **golang.org/x/sys**                         59db8d763f22 -> 0f9fa26af87c
* **golang.org/x/time**                        38a9dc6acbc6 -> 1f47c861a9ac
* **google.golang.org/grpc**                   v1.38.0 -> v1.39.1
* **google.golang.org/protobuf**               v1.26.0 -> v1.27.1
* **inet.af/netaddr**                          bf05d8b52dda -> ce7a8ad02cc1
* **k8s.io/api**                               v0.21.2 -> v0.22.0
* **k8s.io/apimachinery**                      v0.21.2 -> v0.22.0
* **k8s.io/apiserver**                         v0.21.2 -> v0.22.0
* **k8s.io/client-go**                         v0.21.2 -> v0.22.0
* **k8s.io/cri-api**                           v0.21.2 -> v0.22.0
* **k8s.io/kubectl**                           v0.21.2 -> v0.22.0
* **k8s.io/kubelet**                           v0.21.2 -> v0.22.0

Previous release can be found at [v0.11.0](https://github.com/talos-systems/talos/releases/tag/v0.11.0)

## [Talos 0.11.0-alpha.2](https://github.com/talos-systems/talos/releases/tag/v0.11.0-alpha.2) (2021-06-23)

Welcome to the v0.11.0-alpha.2 release of Talos!
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Default to Bootstrap workflow

The `init.yaml` is no longer an output of `talosctl gen config`.
We now encourage using the bootstrap API, instead of `init` node types, as we
intend on deprecating this machine type in the future.
The `init.yaml` and `controlplane.yaml` machine configs are identical with the
exception of the machine type.
Users can use a modified `controlplane.yaml` with the machine type set to
`init` if they would like to avoid using the bootstrap API.


### Component Updates

* containerd was updated to 1.5.2
* Linux kernel was updated to 5.10.45
* Kubernetes was updated to 1.21.2
* etcd was updated to 3.4.16


### CoreDNS

Added the flag `cluster.coreDNS.disabled` to coreDNS deployment during the cluster bootstrap.


### Legacy BIOS Support

Added an option to the `machine.install` section of the machine config that can enable marking MBR partition bootable
for the machines that have legacy BIOS which does not support GPT partitioning scheme.


### Multi-arch Installer

Talos installer image (for any arch) now contains artifacts for both `amd64` and `arm64` architecture.
This means that e.g. images for arm64 SBCs can be generated on amd64 host.


### Networking Configuration

Talos networking configuration was completely rewritten to be based on controllers
and resources.
There are no changes to the machine configuration, but any update to `.machine.network` can now
be applied in immediate mode (without a reboot).
Talos should be setting up network configuration much faster on boot now, not blocking on DHCP for unconfigured
interfaces and skipping the reset network step.


### Talos API RBAC

Limited RBAC support in Talos API is now enabled by default for Talos 0.11.
Default `talosconfig` has `os:admin` role embedded in the certificate so that all the APIs are available.
Certificates with reduced set of roles can be created with `talosctl config new` command.

When upgrading from Talos 0.10, RBAC is not enabled by default. Before enabling RBAC, generate `talosconfig` with
`os:admin` role first to make sure that administrator still has access to the cluster when RBAC is enabled.

List of available roles:

* `os:admin` role enables every Talos API
* `os:reader` role limits access to read-only APIs which do not return sensitive data
* `os:etcd:backup` role only allows `talosctl etcd snapshot` API call (for etcd backup automation)


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Serge Logvinov
* Jorik Jonker
* Spencer Smith
* Andrew Rynhard
* Andrew LeCody
* Kevin Hellemun
* Seán C McCord
* Boran Car
* Brandon Nason
* Gabor Nyiri
* Gabor Nyiri
* Joost Coelingh
* Lance R. Vick
* Lennard Klein
* Sébastien Bernard
* Sébastien Bernard

### Changes
<details><summary>162 commits</summary>
<p>

* [`0731be90`](https://github.com/talos-systems/talos/commit/0731be908bfe130b37db3d5f54b96f3981b1c860) feat: add cloud images to releases
* [`b52b2066`](https://github.com/talos-systems/talos/commit/b52b206665ba963ceec0b7a4ff41bcee77aa8a67) feat: split etcd certificates to peer/client
* [`33119d2b`](https://github.com/talos-systems/talos/commit/33119d2b8e4b48367421ed8e66aa4b11e639b2ac) chore: add an option to launch cluster with bad RTC state
* [`d8c2bca1`](https://github.com/talos-systems/talos/commit/d8c2bca1b53dc9d0e7bb627fe43c629a52489dec) feat: reimplement apid certificate generation on top of COSI
* [`3c1b3219`](https://github.com/talos-systems/talos/commit/3c1b32199d294bd52983c4dd57738cad29aa8738) chore: refactor CLI tests
* [`0fd9ea2d`](https://github.com/talos-systems/talos/commit/0fd9ea2d63af00f7d2423c2daba2988c38cdae78) feat: enable MACVTAP support
* [`898673e8`](https://github.com/talos-systems/talos/commit/898673e8d3e53a0022f2564ee26a29991c145aa8) chore: update e2e tests to use latest capi releases
* [`e26c5583`](https://github.com/talos-systems/talos/commit/e26c5583c2dbe771bd50a7f9efe958cdc9c60d54) docs: add AMI IDs for Talos 0.10.4
* [`72ef48f0`](https://github.com/talos-systems/talos/commit/72ef48f0ea1898e80977f56724e931c73d7aff94) fix: assign source address to the DHCP default gateway routes
* [`004885a3`](https://github.com/talos-systems/talos/commit/004885a379a8617a874bd97062eb7af00fe7dc3b) feat: update Linux kernel to 5.10.45, etcd to 3.4.16
* [`821f469a`](https://github.com/talos-systems/talos/commit/821f469a1d82e180528dc07afffde05f08a57dd1) feat: skip overlay mount checks with docker
* [`b6e02311`](https://github.com/talos-systems/talos/commit/b6e02311a36a7eeb5bfb22037989f49483b9e3f0) feat: use COSI RD's sensitivity for RBAC
* [`46751c1a`](https://github.com/talos-systems/talos/commit/46751c1ad2b2102ea6b8e151bdbe854d041250cb) feat: improve security of Kubernetes control plane components
* [`0f659622`](https://github.com/talos-systems/talos/commit/0f659622d02260731a30d4862da99697adc7ab5c) fix: build with custom kernel/rootfs
* [`5b5089ab`](https://github.com/talos-systems/talos/commit/5b5089ab95e2a7a345e18232520d9071180d9f10) fix: mark kube-proxy as system critical priority
* [`42c16f67`](https://github.com/talos-systems/talos/commit/42c16f67f4476b8b57c39fea2bd3ec8168cb8193) chore: bump dependencies
* [`60f78419`](https://github.com/talos-systems/talos/commit/60f78419e490f47dc1424008f33cc1baa05097b4) chore: bump etcd client libraries to final 3.5.0 release
* [`2b0de9ed`](https://github.com/talos-systems/talos/commit/2b0de9edb2b0f158f12cd320ac672c3c3a5a339b) feat: improve security of Kubernetes control plane components
* [`48a5c460`](https://github.com/talos-systems/talos/commit/48a5c460a140b50026210576a46a691393511461) docs: provide more storage details
* [`e13d905c`](https://github.com/talos-systems/talos/commit/e13d905c2e682b8470e62fd1ee9cd4f07a6c6c65) release(v0.11.0-alpha.1): prepare release
* [`70ac771e`](https://github.com/talos-systems/talos/commit/70ac771e0846247dbebf484aca20ef950d8b99c7) fix: use localhost API server endpoint for internal communication
* [`a941eb7d`](https://github.com/talos-systems/talos/commit/a941eb7da06246d59cec1b63883f2d7e3f91ce73) feat: improve security of Kubernetes control plane components
* [`3aae94e5`](https://github.com/talos-systems/talos/commit/3aae94e5306c0d6e31df4aee127ee3562709edd3) feat: provide Kubernetes nodename as a COSI resource
* [`06209bba`](https://github.com/talos-systems/talos/commit/06209bba2867829561a60f0e7cd9847fa9a8edd3) chore: update RBAC rules, remove old APIs
* [`9f24b519`](https://github.com/talos-systems/talos/commit/9f24b519dce07ce05099b242ba95e8a1e319630e) chore: remove bootkube check from cluster health check
* [`4ac9bea2`](https://github.com/talos-systems/talos/commit/4ac9bea27dc098ebdfdc0958f3000d960fad50de) fix: stop etcd client logs from going to the server console
* [`f63ab9dd`](https://github.com/talos-systems/talos/commit/f63ab9dd9bb6c734873dc8073892f5f10a2ed2e1) feat: implement `talosctl config new` command
* [`fa15a668`](https://github.com/talos-systems/talos/commit/fa15a6687fc56820fbc5566d494bedbc1a5f600f) fix: don't enable RBAC feature in the config for Talos < 0.11
* [`2dc27d99`](https://github.com/talos-systems/talos/commit/2dc27d9964fa3df08a6ec11c0b045d7325ea0d2b) fix: do not format state partition in the initialize sequence
* [`b609f33c`](https://github.com/talos-systems/talos/commit/b609f33cdebb0659738d4fa3802035b2b344b9b9) fix: update networking stack after Equnix Metal testing
* [`243a3b53`](https://github.com/talos-systems/talos/commit/243a3b53e0e7591d5958a3b8373ab963990c40d6) fix: separate healthy and unknown flags in the service resource
* [`1a1378be`](https://github.com/talos-systems/talos/commit/1a1378be16fdce45273bdc81fb72715c4766ee4b) fix: update retry package with a fix for errors.Is
* [`cb83edd7`](https://github.com/talos-systems/talos/commit/cb83edd7fcf14bd199950a04e366fc573bcf4270) fix: wait for the network to be ready in mainteancne mode
* [`96f89071`](https://github.com/talos-systems/talos/commit/96f89071c3ecd809d912762e40cb9d98ce052018) feat: update controller-runtime logs to console level on config.debug
* [`973069b6`](https://github.com/talos-systems/talos/commit/973069b611456f758037c9ca4dc50a4a84e7a59c) feat: support NFS 4.1
* [`654dcad4`](https://github.com/talos-systems/talos/commit/654dcad4753211599d12655ec0f0466f27f49589) chore: bump dependencies via dependabot
* [`d7394457`](https://github.com/talos-systems/talos/commit/d7394457d978d073690bec589ea78d957539e333) fix: don't treat ethtool errors as fatal
* [`f2ae9cd0`](https://github.com/talos-systems/talos/commit/f2ae9cd0c1b7d27b5b9971f4820e5feae7934124) feat: replace networkd with new network implementation
* [`caec3063`](https://github.com/talos-systems/talos/commit/caec3063c82777f82599632ca4914a58515cb9a9) fix: do not complain about empty roles
* [`11918a11`](https://github.com/talos-systems/talos/commit/11918a110a628d7e0b8749fce92ef572aca47874) docs: update community meeting time
* [`aeddb9c0`](https://github.com/talos-systems/talos/commit/aeddb9c0977a51e7aca72f69edda8b69d917db13) feat: implement platform config controller (hostnames)
* [`1ece334d`](https://github.com/talos-systems/talos/commit/1ece334da9d7bb247c385dba08202345b83c1a0f) feat: implement controller which runs network operators
* [`744ea8a5`](https://github.com/talos-systems/talos/commit/744ea8a5d4b4cb4ff69c2c2fc636e499af892fee) fix: do not add bootstrap contents option if tail events is not 0
* [`5029edfb`](https://github.com/talos-systems/talos/commit/5029edfb71990581515cabe9634d0519a9988316) fix: overwrite nodes in the gRPC metadata
* [`6a35c8f1`](https://github.com/talos-systems/talos/commit/6a35c8f110abaf0017530650c55a34f1caae6288) feat: implement virtual IP (shared IP) network operator
* [`0f3b8380`](https://github.com/talos-systems/talos/commit/0f3b83803d812a30e1418666fa5758734c20e5c2) chore: expose WatchRequest in the resources client
* [`11e258b1`](https://github.com/talos-systems/talos/commit/11e258b15097493d2b4efd596b2fde2d52579455) feat: implement operator configuration controller
* [`ce3815e7`](https://github.com/talos-systems/talos/commit/ce3815e75e889de32d9473a23e75863f56b893da) feat: implement DHCP6 operator
* [`f010d99a`](https://github.com/talos-systems/talos/commit/f010d99afbc6095ad8fe218187fda306c59d3e1e) feat: implement operator framework with DHCP4 as the first example
* [`f93c9c8f`](https://github.com/talos-systems/talos/commit/f93c9c8fa607a5116274d7e090f49568d01814e7) feat: bring unconfigured links with link carrier up by default
* [`02bd657b`](https://github.com/talos-systems/talos/commit/02bd657b252ae64ea054b2dc338e55ce9352b420) feat: implement network.Status resource and controller
* [`da329f00`](https://github.com/talos-systems/talos/commit/da329f00ab0af9f670207da1e13541aef36c4ca6) feat: enable RBAC by default
* [`0f168a88`](https://github.com/talos-systems/talos/commit/0f168a880143141d8637d21aa9da403383dcf025) feat: add configuration for enabling RBAC
* [`e74f789b`](https://github.com/talos-systems/talos/commit/e74f789b01b9910f8193415dcefb4b32abcb5f5c) feat: implement EtcFileController to render files in `/etc`
* [`5aede1a8`](https://github.com/talos-systems/talos/commit/5aede1a83313152bd83891d0cae4b388a54bd9c2) fix: prefer extraConfig over OVF env, skip empty config
* [`5ad314fe`](https://github.com/talos-systems/talos/commit/5ad314fe7e7cfca8196770071d52b93aa4f767f6) feat: implement basic RBAC interceptors
* [`c031be81`](https://github.com/talos-systems/talos/commit/c031be8139dbe1f803b70fc9941cfe438b9ddeb9) chore: use Go 1.16.5
* [`8b0763f6`](https://github.com/talos-systems/talos/commit/8b0763f6a20691d36d2c82f2a756171c55450a8a) chore: bump dependencies via dependabot
* [`8b8de11d`](https://github.com/talos-systems/talos/commit/8b8de11d9f4d1b1fde43b7fdd56b96d5e3eb5413) feat: implement new controllers for hostname, resolvers and time servers
* [`24859b14`](https://github.com/talos-systems/talos/commit/24859b14108df7c5895022043d02d4d5ca7660a4) docs: update Rpi4 firmware guide
* [`62c702c4`](https://github.com/talos-systems/talos/commit/62c702c4fd6e7a11654f542bbe31d1adfc896731) fix: remove conflicting etcd member on rejoin with empty data directory
* [`ff62a599`](https://github.com/talos-systems/talos/commit/ff62a59984ef0c61dcf549ab38d39584e3630724) fix: drop into maintenance mode if config URL is `none` (metal)
* [`14e696d0`](https://github.com/talos-systems/talos/commit/14e696d068b5d895b4fefc06bc6d26b4ac2bc450) feat: update COSI runtime and add support for tail in the Talos gRPC
* [`a71053fc`](https://github.com/talos-systems/talos/commit/a71053fcd88d7651e536ce29b574e18f84678f3e) feat: default to bootstrap workflow
* [`76aac4bb`](https://github.com/talos-systems/talos/commit/76aac4bb25d8bc6a86458b8ac5be10ca67f236be) feat: implement CPU and Memory stats controller
* [`8f90c6a8`](https://github.com/talos-systems/talos/commit/8f90c6a8e1d76a3ddecc99be4e4b9f0ce0235daa) feat: parse Talos-specific cmdline params
* [`ed10e139`](https://github.com/talos-systems/talos/commit/ed10e139c161b0a6e0f3460e21e4e1752b26cb46) feat: implement NodeAddress controller
* [`33db8857`](https://github.com/talos-systems/talos/commit/33db8857aaf6e411464d08c51560473455e8e156) fix: use COSI runtime DestroyReady input type
* [`6e775363`](https://github.com/talos-systems/talos/commit/6e775363920b7869b83775d1b674807163039eb1) refactor: rename *.Status() to *.TypedSpec() in the resources
* [`97627061`](https://github.com/talos-systems/talos/commit/97627061d7e8de90e2f2745efa7497137447d116) docs: set static IP on ISO install mode
* [`5811f4dd`](https://github.com/talos-systems/talos/commit/5811f4dda1b62848eefae9be56e8b91d443f4d34) feat: implement link (interface) controllers
* [`046b229b`](https://github.com/talos-systems/talos/commit/046b229b13708c3ffe1d77b8884242fc100097d0) chore: skip building multi-arch installer for race-enabled build
* [`73fbb4b5`](https://github.com/talos-systems/talos/commit/73fbb4b523b41d266840eced306242d57a332b4d) fix: only fetch machine uuid if it's not set
* [`f112a540`](https://github.com/talos-systems/talos/commit/f112a540b0e776f06820ee900d6ce9f4f2de02ec) fix: clean up stale snapshots on container start
* [`c036b949`](https://github.com/talos-systems/talos/commit/c036b949486d94cbbce54c7511633d398f75797c) chore: bump dependencies
* [`a4d67a01`](https://github.com/talos-systems/talos/commit/a4d67a01820894d3ebf8c65a06345232fae4f93b) feat: add the ability to disable CoreDNS
* [`76dbfb36`](https://github.com/talos-systems/talos/commit/76dbfb3699df0725a8acf29bff39c43e4aa34f9d) feat: add ability to mark MBR partition bootable
* [`e0f5b1e2`](https://github.com/talos-systems/talos/commit/e0f5b1e20aa0d22898274ddc0f9026c0d813cee2) chore: split mgmt/gen.go into several files
* [`fad1b4f1`](https://github.com/talos-systems/talos/commit/fad1b4f1fdce962b779ceb960f81d572ee5033af) chore: fix go generate for the machinery
* [`1117294a`](https://github.com/talos-systems/talos/commit/1117294ad21945d24b0954d223cc4996df01dd81) release(v0.11.0-alpha.0): prepare release
* [`c0962946`](https://github.com/talos-systems/talos/commit/c09629466321f4d220454164784edf41fd3d5813) chore: prepare for 0.11 release series
* [`72359765`](https://github.com/talos-systems/talos/commit/723597657ad78e9766190ea2e110208c62d0093b) feat: enable GORACE=halt_on_panic=1 in machined binary
* [`0acb04ad`](https://github.com/talos-systems/talos/commit/0acb04ad7a2a0a7b75471f0251b0e04eccd927cd) feat: implement route network controllers
* [`f5bf88a4`](https://github.com/talos-systems/talos/commit/f5bf88a4c2ab8f48fd93bc7ac13543c613bf9bd1) feat: create certificates with os:admin role
* [`1db301ed`](https://github.com/talos-systems/talos/commit/1db301edf6a4057814a6d5b8f87fbfe1e020caeb) feat: switch controller-runtime to zap.Logger
* [`f7cf64d4`](https://github.com/talos-systems/talos/commit/f7cf64d42ec77ca68408ecb0f437ab5f86bc787a) fix: add talos.config to the vApp Properties in VMware OVA
* [`209527ec`](https://github.com/talos-systems/talos/commit/209527eccc6c93edad33a01a3f3d24fb978f2f07) docs: add AMIs for Talos 0.10.3
* [`59cfd312`](https://github.com/talos-systems/talos/commit/59cfd312c1ac531528c4ceb2adeb3f85829cc4e1) chore: bump dependencies via dependabot
* [`1edb20cf`](https://github.com/talos-systems/talos/commit/1edb20cf98fe2e641cefc658d17206e09acabc26) feat: extract config generation
* [`af77c295`](https://github.com/talos-systems/talos/commit/af77c29565b65766d135884ec7740f67b56626e3) docs: update wirguard guide
* [`4fe69121`](https://github.com/talos-systems/talos/commit/4fe691214366c08ea846bdc6233dd592da0d4769) test: better `talosctl ls` tests
* [`04ddda96`](https://github.com/talos-systems/talos/commit/04ddda962fbcfdeaae59d232e7bb7f9c5bb63bc7) feat: update containerd to 1.5.2, runc to 1.0.0-rc95
* [`49c7276b`](https://github.com/talos-systems/talos/commit/49c7276b16a82b7da8c83f8bd930361768f0e249) chore: fix markdown linting
* [`7270495a`](https://github.com/talos-systems/talos/commit/7270495ace9faf48a73829bbed0e4eb2c939eecb) docs: add mayastor quickstart
* [`d3d9112f`](https://github.com/talos-systems/talos/commit/d3d9112f288d3b0f3ebe1c8b28b1c4e2fc8512b2) docs: fix spelling/grammar in What's New for Talos 0.9
* [`82804414`](https://github.com/talos-systems/talos/commit/82804414fc2fcb21da77edc2fbbefe92a14fc30d) test: provide a way to force different boot order in provision library
* [`a1c0e99a`](https://github.com/talos-systems/talos/commit/a1c0e99a1729c704a633dcc557dc46466b828e11) docs: add guide for deploying metrics-server
* [`6bc6658b`](https://github.com/talos-systems/talos/commit/6bc6658b518379d418baafcf9b1045a3b84f48ec) feat: update containerd to 1.5.1
* [`c6567fae`](https://github.com/talos-systems/talos/commit/c6567fae9c59da5148c9876289a4bf248240b99d) chore: dependabot updates
* [`61ccbb3f`](https://github.com/talos-systems/talos/commit/61ccbb3f5a2564376af13ea9bbfe51e364fcb3a1) chore: keep debug symbols in debug builds
* [`1ce362e0`](https://github.com/talos-systems/talos/commit/1ce362e05e41cd76cdda17a6fc971767e036df37) docs: update customizing kernel build steps
* [`a26174b5`](https://github.com/talos-systems/talos/commit/a26174b54846bdfa0b66d2f9147bfe1dc8f2eb52) fix: properly compose pattern and header in etcd members output
* [`0825cf11`](https://github.com/talos-systems/talos/commit/0825cf11f412eef930db269b6cae02d059058101) fix: stop networkd and pods before leaving etcd on upgrade
* [`bed6b15d`](https://github.com/talos-systems/talos/commit/bed6b15d6fcf0634a887b79797d639e221fe9387) fix: properly populate AllowSchedulingOnMasters option in gen config RPC
* [`071f0445`](https://github.com/talos-systems/talos/commit/071f044562dd247dd54584d7b9fa0bb24d6f7599) feat: implement AddressSpec handling
* [`76e38b7b`](https://github.com/talos-systems/talos/commit/76e38b7b8251548292ae15ecda2bfa1c8ddc5cf3) feat: update Kubernetes to 1.21.1
* [`9b1338d9`](https://github.com/talos-systems/talos/commit/9b1338d989e6cdf7e0b6d5fe1ba3c32d27fc2251) chore: parse "boolean" variables
* [`c81cfb21`](https://github.com/talos-systems/talos/commit/c81cfb21670b82e518cf4c32230e8fbbce6be8ff) chore: allow building with debug handlers
* [`c9651673`](https://github.com/talos-systems/talos/commit/c9651673b9eaf811ae4acfed313debbf78bd80e8) feat: update go-smbios library
* [`95c656fb`](https://github.com/talos-systems/talos/commit/95c656fb72b6b858b55dae37020cb59ba26115f8) feat: update containerd to 1.5.0, runc to 1.0.0-rc94
* [`db9c35b5`](https://github.com/talos-systems/talos/commit/db9c35b570b39f4423f4636f9e9f1d14cac5d7c1) feat: implement AddressStatusController
* [`1cf011a8`](https://github.com/talos-systems/talos/commit/1cf011a809b924fc8f2083566d169704c6e07cd5) chore: bump dependencies via dependabot
* [`e3f407a1`](https://github.com/talos-systems/talos/commit/e3f407a1dff3f4ee7e024bbfb64f17b5cb5d625d) fix: properly pass disk type selector from config to matcher
* [`66b2b450`](https://github.com/talos-systems/talos/commit/66b2b450582593e93598fac80c8b3c29e8c8a944) feat: add resources and use HTTPS checks in control plane pods
* [`4ffd7c0a`](https://github.com/talos-systems/talos/commit/4ffd7c0adf281033ac02d37ca434e7f9ad71e692) fix: stop networkd before leaving etcd on 'reset' path
* [`610d38d3`](https://github.com/talos-systems/talos/commit/610d38d309dabaa623494ade12234f1ccf018a9e) docs: add AMIs for 0.10.1, collapse list of AMIs by default
* [`807497ec`](https://github.com/talos-systems/talos/commit/807497ec20dee15953186bda0fe7a45ffec0307c) chore: make conformance pipeline depend on cron-default
* [`3c121359`](https://github.com/talos-systems/talos/commit/3c1213596cdf03daf09050103f57b29e756439b1) feat: implement LinkStatusController
* [`0e8de046`](https://github.com/talos-systems/talos/commit/0e8de04698aac95062f3037da0a9af8b6ee916b0) fix: update go-blockdevice to fix disk type detection
* [`4d50a4ed`](https://github.com/talos-systems/talos/commit/4d50a4edd0eb413c16e899536ccdc2642e37aeaa) fix: update the way NTP sync uses `adjtimex` syscall
* [`1a85c14a`](https://github.com/talos-systems/talos/commit/1a85c14a51fdab43ae84274563bf89b30e4e6d92) fix: avoid data race on CRI pod stop
* [`5de8dbc0`](https://github.com/talos-systems/talos/commit/5de8dbc06c7ed36c8f3af9adea8b1abedeb372b6) fix: repair pine64 support
* [`38239097`](https://github.com/talos-systems/talos/commit/3823909735859f2ac5d95bc39c051fc9c2c07685) fix: properly parse matcher expressions
* [`e54b6b7a`](https://github.com/talos-systems/talos/commit/e54b6b7a3d7412ddce1467dfbd35efe3cfd76f3f) chore: update dependencies via dependabot
* [`f2caed0d`](https://github.com/talos-systems/talos/commit/f2caed0df5b76c4a719f968191081a6e5e2e95c7) chore: use extracted talos-systems/go-kmsg library
* [`79d804c5`](https://github.com/talos-systems/talos/commit/79d804c5b4af50a0fd73db17d2522d6a6b45c9ca) docs: fix typos
* [`a2bb390e`](https://github.com/talos-systems/talos/commit/a2bb390e1d56106d6d3c1526f3f76b34846b0274) feat: deterministic builds
* [`e480fedf`](https://github.com/talos-systems/talos/commit/e480fedff047233e78ad2c22e7b84cbbb22798d5) feat: add USB serial drivers
* [`79299d76`](https://github.com/talos-systems/talos/commit/79299d761c50aff386ab7b3c12f39c1797585632) docs: add Matrix room links
* [`1b3e8b09`](https://github.com/talos-systems/talos/commit/1b3e8b09edcd51cf3df2d43d14c8fbf1e912a465) docs: add survey to README
* [`8d51c9bb`](https://github.com/talos-systems/talos/commit/8d51c9bb190c2c60fa9be6a00572d2eaf4221e94) docs: update redirects to Talos 0.10
* [`1092c3a5`](https://github.com/talos-systems/talos/commit/1092c3a5069a3add439860d90c3615111fa03c98) feat: add Pine64 SBC support
* [`63e01754`](https://github.com/talos-systems/talos/commit/63e0175437e45c8f7e5296841337a640c600982c) feat: pull kernel with VMware balloon module enabled
* [`aeec99d8`](https://github.com/talos-systems/talos/commit/aeec99d8247f4eb534e0db1ed639f95cd726fe08) chore: remove temporary fork
* [`0f49722d`](https://github.com/talos-systems/talos/commit/0f49722d0ff4e731f17a55d1ca50472714334748) feat: add `--config-patch` flag by node type
* [`a01b1d22`](https://github.com/talos-systems/talos/commit/a01b1d22d9f3fa94355817217fefd80fe34628f3) chore: dump dependencies via dependabot
* [`d540a4a4`](https://github.com/talos-systems/talos/commit/d540a4a4711367a0ada203f668382e39876ba081) fix: bump crypto library for the CSR verification fix
* [`c3a4173e`](https://github.com/talos-systems/talos/commit/c3a4173e11a92c2bc51ea4f284ad38c9750105d2) chore: remove security API ReadFile/WriteFile
* [`38037131`](https://github.com/talos-systems/talos/commit/38037131cddc2aefbae0f48fb7e355ec76247b67) chore: update wgctrl dependecy
* [`d9ba0fd0`](https://github.com/talos-systems/talos/commit/d9ba0fd0164b2bfb2bc4ffe7a2d9d6c665a38e4d) docs: create v0.11 docs, promote v0.10 docs, add v0.10 AMIs
* [`2261d7ed`](https://github.com/talos-systems/talos/commit/2261d7ed0212c287273eac647647e4390c530a6e) fix: use both self-signed and Kubernetes CA to verify Kubelet cert
* [`a3537a69`](https://github.com/talos-systems/talos/commit/a3537a691320430eeb7149abe73419ee242312fc) docs: update cloud images for Talos v0.9.3
* [`5b9ee861`](https://github.com/talos-systems/talos/commit/5b9ee86179fb92989b02533d6d6745a5b0f37566) docs: add what's new for Talos 0.10
* [`f1107fa3`](https://github.com/talos-systems/talos/commit/f1107fa3a33955f3aa57a49991c87f9ee47b6e67) docs: add survey
* [`93623d47`](https://github.com/talos-systems/talos/commit/93623d47f24fef0d149fa006678b61e3182ef771) docs: update AWS instructions
* [`a739d1b8`](https://github.com/talos-systems/talos/commit/a739d1b8adbc026796d1c55f7319677f9010f727) feat: add support of custom registry CA certificate usage
* [`7f468d35`](https://github.com/talos-systems/talos/commit/7f468d350a6f80d2815149376fa24f7d7629402c) fix: update osType in OVA other3xLinux64Guest"
* [`4a184b67`](https://github.com/talos-systems/talos/commit/4a184b67d6ae25b21b35373e7dd6eab41b042c96) docs: add etcd backup and restore guide
* [`5fb38d3e`](https://github.com/talos-systems/talos/commit/5fb38d3e5f201934d64bae186c5300e7de7af3d4) chore: refactor Dockerfile for cross-compilation
* [`a8f1e526`](https://github.com/talos-systems/talos/commit/a8f1e526bfc00107c915572df2be08b3f154f4e6) chore: build talosctl for Darwin / Apple Silicon
* [`eb0b64d3`](https://github.com/talos-systems/talos/commit/eb0b64d3138228a6c751387c720ca81c338b834d) chore: list specifically for enabled regions
* [`669a0cbd`](https://github.com/talos-systems/talos/commit/669a0cbdc4756f0ad8f0dacc56a20f71e96fe4cd) fix: check if OVF env is empty
* [`da92049c`](https://github.com/talos-systems/talos/commit/da92049c0b4beae32af80205f50849443cd6dad3) chore: use codecov from the build container
* [`9996d4b0`](https://github.com/talos-systems/talos/commit/9996d4b028f3845071850def75f2b534e4d2b190) chore: use REGISTRY_MIRROR_FLAGS if defined
* [`05cbe250`](https://github.com/talos-systems/talos/commit/05cbe250c87339e097d435d6b10b9d8a5f2eb49e) chore: bump dependencies via dependabot
* [`9a91142a`](https://github.com/talos-systems/talos/commit/9a91142a38b3b1f210773acf8df01ed6a45599c2) feat: print complete member info in etcd members
* [`bb40d6dd`](https://github.com/talos-systems/talos/commit/bb40d6dd06a967464c24ab33744bbf460aa84038) feat: update pkgs version
* [`e7a9164b`](https://github.com/talos-systems/talos/commit/e7a9164b1e1630f953a420d99c865aef6e652d15) test: implement `talosctl conformance` command to run e2e tests
* [`6cb266e7`](https://github.com/talos-systems/talos/commit/6cb266e74e60d9d5423feaad550a7861dc73f11d) fix: update etcd client errors, print etcd join failures
* [`0bd8b0e8`](https://github.com/talos-systems/talos/commit/0bd8b0e8008c12e4914c6e9b5faf06dda6c744f7) feat: provide an option to recover etcd from data directory copy
* [`f9818540`](https://github.com/talos-systems/talos/commit/f98185408d618ebcc780247ea2c42239df27a74e) chore: fix conform with scopes
* [`21018f28`](https://github.com/talos-systems/talos/commit/21018f28c732719535c30c8e1abdbb346f1dc4bf) chore: bump website node.js dependencies
</p>
</details>

### Changes since v0.11.0-alpha.1
<details><summary>19 commits</summary>
<p>

* [`0731be90`](https://github.com/talos-systems/talos/commit/0731be908bfe130b37db3d5f54b96f3981b1c860) feat: add cloud images to releases
* [`b52b2066`](https://github.com/talos-systems/talos/commit/b52b206665ba963ceec0b7a4ff41bcee77aa8a67) feat: split etcd certificates to peer/client
* [`33119d2b`](https://github.com/talos-systems/talos/commit/33119d2b8e4b48367421ed8e66aa4b11e639b2ac) chore: add an option to launch cluster with bad RTC state
* [`d8c2bca1`](https://github.com/talos-systems/talos/commit/d8c2bca1b53dc9d0e7bb627fe43c629a52489dec) feat: reimplement apid certificate generation on top of COSI
* [`3c1b3219`](https://github.com/talos-systems/talos/commit/3c1b32199d294bd52983c4dd57738cad29aa8738) chore: refactor CLI tests
* [`0fd9ea2d`](https://github.com/talos-systems/talos/commit/0fd9ea2d63af00f7d2423c2daba2988c38cdae78) feat: enable MACVTAP support
* [`898673e8`](https://github.com/talos-systems/talos/commit/898673e8d3e53a0022f2564ee26a29991c145aa8) chore: update e2e tests to use latest capi releases
* [`e26c5583`](https://github.com/talos-systems/talos/commit/e26c5583c2dbe771bd50a7f9efe958cdc9c60d54) docs: add AMI IDs for Talos 0.10.4
* [`72ef48f0`](https://github.com/talos-systems/talos/commit/72ef48f0ea1898e80977f56724e931c73d7aff94) fix: assign source address to the DHCP default gateway routes
* [`004885a3`](https://github.com/talos-systems/talos/commit/004885a379a8617a874bd97062eb7af00fe7dc3b) feat: update Linux kernel to 5.10.45, etcd to 3.4.16
* [`821f469a`](https://github.com/talos-systems/talos/commit/821f469a1d82e180528dc07afffde05f08a57dd1) feat: skip overlay mount checks with docker
* [`b6e02311`](https://github.com/talos-systems/talos/commit/b6e02311a36a7eeb5bfb22037989f49483b9e3f0) feat: use COSI RD's sensitivity for RBAC
* [`46751c1a`](https://github.com/talos-systems/talos/commit/46751c1ad2b2102ea6b8e151bdbe854d041250cb) feat: improve security of Kubernetes control plane components
* [`0f659622`](https://github.com/talos-systems/talos/commit/0f659622d02260731a30d4862da99697adc7ab5c) fix: build with custom kernel/rootfs
* [`5b5089ab`](https://github.com/talos-systems/talos/commit/5b5089ab95e2a7a345e18232520d9071180d9f10) fix: mark kube-proxy as system critical priority
* [`42c16f67`](https://github.com/talos-systems/talos/commit/42c16f67f4476b8b57c39fea2bd3ec8168cb8193) chore: bump dependencies
* [`60f78419`](https://github.com/talos-systems/talos/commit/60f78419e490f47dc1424008f33cc1baa05097b4) chore: bump etcd client libraries to final 3.5.0 release
* [`2b0de9ed`](https://github.com/talos-systems/talos/commit/2b0de9edb2b0f158f12cd320ac672c3c3a5a339b) feat: improve security of Kubernetes control plane components
* [`48a5c460`](https://github.com/talos-systems/talos/commit/48a5c460a140b50026210576a46a691393511461) docs: provide more storage details
</p>
</details>

### Changes from talos-systems/crypto
<details><summary>8 commits</summary>
<p>

* [`d3cb772`](https://github.com/talos-systems/crypto/commit/d3cb77220384b3a3119a6f3ddb1340bbc811f1d1) feat: make possible to change KeyUsage
* [`6bc5bb5`](https://github.com/talos-systems/crypto/commit/6bc5bb50c52767296a1b1cab6580e3fcf1358f34) chore: remove unused argument
* [`cd18ef6`](https://github.com/talos-systems/crypto/commit/cd18ef62eb9f65d8b6730a2eb73e47e629949e1b) feat: add support for several organizations
* [`97c888b`](https://github.com/talos-systems/crypto/commit/97c888b3924dd5ac70b8d30dd66b4370b5ab1edc) chore: add options to CSR
* [`7776057`](https://github.com/talos-systems/crypto/commit/7776057f5086157873f62f6a21ec23fa9fd86e05) chore: fix typos
* [`80df078`](https://github.com/talos-systems/crypto/commit/80df078327030af7e822668405bb4853c512bd7c) chore: remove named result parameters
* [`15bdd28`](https://github.com/talos-systems/crypto/commit/15bdd282b74ac406ab243853c1b50338a1bc29d0) chore: minor updates
* [`4f80b97`](https://github.com/talos-systems/crypto/commit/4f80b976b640d773fb025d981bf85bcc8190815b) fix: verify CSR signature before issuing a certificate
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`4fe2706`](https://github.com/talos-systems/extras/commit/4fe27060347c861b716392eec3dfee698becb5f3) feat: build with Go 1.16.5
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>3 commits</summary>
<p>

* [`30c2bc3`](https://github.com/talos-systems/go-blockdevice/commit/30c2bc3cb62af52f0aea9ce347923b0649fb7928) feat: mark MBR bootable
* [`1292574`](https://github.com/talos-systems/go-blockdevice/commit/1292574643e06512255fb0f45107e0c296eb5a3b) fix: make disk type matcher parser case insensitive
* [`b77400e`](https://github.com/talos-systems/go-blockdevice/commit/b77400e0a7261bf25da77c1f28c2f393f367bfa9) fix: properly detect nvme and sd card disk types
</p>
</details>

### Changes from talos-systems/go-debug
<details><summary>5 commits</summary>
<p>

* [`3d0a6e1`](https://github.com/talos-systems/go-debug/commit/3d0a6e1bf5e3c521e83ead2c8b7faad3638b8c5d) feat: race build tag flag detector
* [`5b292e5`](https://github.com/talos-systems/go-debug/commit/5b292e50198b8ed91c434f00e2772db394dbf0b9) feat: disable memory profiling by default
* [`c6d0ae2`](https://github.com/talos-systems/go-debug/commit/c6d0ae2c0ee099fa0940405401e6a02716a15bd8) fix: linters and CI
* [`d969f95`](https://github.com/talos-systems/go-debug/commit/d969f952af9e02feea59963671298fc236ca4399) feat: initial implementation
* [`b2044b7`](https://github.com/talos-systems/go-debug/commit/b2044b70379c84f9706de74044bd2fd6a8e891cf) Initial commit
</p>
</details>

### Changes from talos-systems/go-kmsg
<details><summary>2 commits</summary>
<p>

* [`2edcd3a`](https://github.com/talos-systems/go-kmsg/commit/2edcd3a913508e2d922776f729bfc4bcab031a8b) feat: add initial version
* [`53cdd8d`](https://github.com/talos-systems/go-kmsg/commit/53cdd8d67b9dbab692471a2d5161e7e0b3d04cca) chore: initial commit
</p>
</details>

### Changes from talos-systems/go-loadbalancer
<details><summary>3 commits</summary>
<p>

* [`a445702`](https://github.com/talos-systems/go-loadbalancer/commit/a4457024d5189d754b2da4a30b14072a0e3f5f05) feat: allow dial timeout and keep alive period to be configurable
* [`3c8f347`](https://github.com/talos-systems/go-loadbalancer/commit/3c8f3471d14e37866c65f73170ef83c038ae5a8c) feat: provide a way to configure logger for the loadbalancer
* [`da8e987`](https://github.com/talos-systems/go-loadbalancer/commit/da8e987434c3d407679a40e213b12a8e1c98abb8) feat: implement Reconcile - ability to change upstream list on the fly
</p>
</details>

### Changes from talos-systems/go-retry
<details><summary>3 commits</summary>
<p>

* [`c78cc95`](https://github.com/talos-systems/go-retry/commit/c78cc953d9e95992575305b4e8648392c6c9b9e6) fix: implement `errors.Is` for all errors in the set
* [`7885e16`](https://github.com/talos-systems/go-retry/commit/7885e16b2cb0267bcc8b07cdd0eced14e8005864) feat: add ExpectedErrorf
* [`3d83f61`](https://github.com/talos-systems/go-retry/commit/3d83f6126c1a3a238d1d1d59bfb6273e4087bdac) feat: deprecate UnexpectedError
</p>
</details>

### Changes from talos-systems/go-smbios
<details><summary>1 commit</summary>
<p>

* [`d3a32be`](https://github.com/talos-systems/go-smbios/commit/d3a32bea731a0c2a60ce7f5eae60253300ef27e1) fix: return UUID in middle endian only on SMBIOS >= 2.6
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>22 commits</summary>
<p>

* [`41d6ccc`](https://github.com/talos-systems/pkgs/commit/41d6ccc8d40259e77da6cc46b047f265e6ebc58b) feat: enable MACVTAP support
* [`96072f8`](https://github.com/talos-systems/pkgs/commit/96072f89ac6b6b7dccd25e54ebbb33eef312c8e4) feat: enable adiantum block encryption (both amd64 arm64)
* [`f5eac03`](https://github.com/talos-systems/pkgs/commit/f5eac033223b1116de70c51204af3a096d9130a5) feat: update Linux to 5.10.45
* [`d756119`](https://github.com/talos-systems/pkgs/commit/d756119b2b0dfabda50a945ee16ee4fd62109cb0) feat: enable HP ILO kernel module (both amd64 arm64)
* [`2d51360`](https://github.com/talos-systems/pkgs/commit/2d51360a254b237943e92cd445e42912d39fce7a) feat: support NFS 4.1
* [`e63e4e9`](https://github.com/talos-systems/pkgs/commit/e63e4e97b4c398e090028eaf7b967cc9eafadf96) feat: bump tools for Go 1.16.5
* [`1f8af29`](https://github.com/talos-systems/pkgs/commit/1f8af290e5d242f7dfc784fd2fc7fcfd714500bd) feat: update Linux to 5.10.38
* [`a3a6650`](https://github.com/talos-systems/pkgs/commit/a3a66505f36b9e9f92f4980df3708a872d56caec) feat: update containerd to 1.5.2
* [`c70ea44`](https://github.com/talos-systems/pkgs/commit/c70ea44ba4bc1ffabdb1422deda107a94e1fe94c) feat: update runc to 1.0.0-rc95
* [`db60235`](https://github.com/talos-systems/pkgs/commit/db602359cc594b35291911b4220dc5b331b323bb) feat: add support for netxen card
* [`f934187`](https://github.com/talos-systems/pkgs/commit/f934187ebdc455f18cc6d2da847be3d48a6e3d8f) feat: update containerd to 1.5.1
* [`e8ed5bc`](https://github.com/talos-systems/pkgs/commit/e8ed5bcb848954ca30967de8d7c81afecdea4825) feat: add geneve encapsulation support for openvswitch
* [`9f7903c`](https://github.com/talos-systems/pkgs/commit/9f7903cb5c110f77db8093347b69ec141325d47c) feat: update containerd to 1.5.0, runc to -rc94
* [`d7c0f70`](https://github.com/talos-systems/pkgs/commit/d7c0f70e41bb7bf542092f2882b062ff52f5ae44) feat: add AES-NI support for amd64
* [`b0d9cd2`](https://github.com/talos-systems/pkgs/commit/b0d9cd2c36e37190c5ce7b85acea6a51a853faaf) fix: build `zbin` utility for both amd64 and arm64
* [`bb39b97`](https://github.com/talos-systems/pkgs/commit/bb39b9744c0c4a29ccfa190a0d2cce0f8547676b) feat: add IPMI support in kernel
* [`1148f9a`](https://github.com/talos-systems/pkgs/commit/1148f9a897d9a52b6013396151e1eab264709037) feat: add DS1307 RTC support for arm64
* [`350aa6f`](https://github.com/talos-systems/pkgs/commit/350aa6f200d441d7dbbf60ec8ebb39a6761d6a8b) feat: add USB serial support
* [`de9c582`](https://github.com/talos-systems/pkgs/commit/de9c58238483219a574fb697ddb1126f36a02da3) feat: add Pine64 SBC support
* [`b56f36b`](https://github.com/talos-systems/pkgs/commit/b56f36bedbe9270ae5cf969f8078a10345457e83) feat: enable VMware baloon kernel module
* [`f87c194`](https://github.com/talos-systems/pkgs/commit/f87c19425352eb9b68d20dec987d0c484987dea9) feat: add iPXE build with embedded placeholder script
* [`a8b9e71`](https://github.com/talos-systems/pkgs/commit/a8b9e71e6538d7554b7a48d1361709d5495bb4de) feat: add cpu scaling for rpi
</p>
</details>

### Changes from talos-systems/tools
<details><summary>1 commit</summary>
<p>

* [`c8c2a18`](https://github.com/talos-systems/tools/commit/c8c2a18b7e587e0b8464574e375a680c5a09a028) feat: update Go to 1.16.5
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go**                     v1.27.0 **_new_**
* **github.com/containerd/cgroups**                 4cbc285b3327 -> v1.0.1
* **github.com/containerd/containerd**              v1.4.4 -> v1.5.2
* **github.com/containerd/go-cni**                  v1.0.1 -> v1.0.2
* **github.com/containerd/typeurl**                 v1.0.1 -> v1.0.2
* **github.com/coreos/go-iptables**                 v0.5.0 -> v0.6.0
* **github.com/cosi-project/runtime**               10d6103c19ab -> f1649aff7641
* **github.com/docker/docker**                      v20.10.4 -> v20.10.7
* **github.com/emicklei/dot**                       v0.15.0 -> v0.16.0
* **github.com/evanphx/json-patch**                 v4.9.0 -> v4.11.0
* **github.com/fatih/color**                        v1.10.0 -> v1.12.0
* **github.com/google/go-cmp**                      v0.5.5 -> v0.5.6
* **github.com/google/gofuzz**                      v1.2.0 **_new_**
* **github.com/googleapis/gnostic**                 v0.5.5 **_new_**
* **github.com/grpc-ecosystem/go-grpc-middleware**  v1.2.2 -> v1.3.0
* **github.com/hashicorp/go-getter**                v1.5.2 -> v1.5.4
* **github.com/imdario/mergo**                      v0.3.12 **_new_**
* **github.com/insomniacslk/dhcp**                  cc9239ac6294 -> 465dd6c35f6c
* **github.com/jsimonetti/rtnetlink**               1b79e63a70a0 -> 9c52e516c709
* **github.com/magiconair/properties**              v1.8.5 **_new_**
* **github.com/mattn/go-isatty**                    v0.0.12 -> v0.0.13
* **github.com/mdlayher/arp**                       f72070a231fc **_new_**
* **github.com/mdlayher/ethtool**                   2b88debcdd43 **_new_**
* **github.com/mdlayher/netlink**                   v1.4.0 -> v1.4.1
* **github.com/mdlayher/raw**                       51b895745faf **_new_**
* **github.com/mitchellh/mapstructure**             v1.4.1 **_new_**
* **github.com/opencontainers/runtime-spec**        4d89ac9fbff6 -> e6143ca7d51d
* **github.com/pelletier/go-toml**                  v1.9.0 **_new_**
* **github.com/rivo/tview**                         8a8f78a6dd01 -> d4fb0348227b
* **github.com/rs/xid**                             v1.2.1 -> v1.3.0
* **github.com/sirupsen/logrus**                    v1.8.1 **_new_**
* **github.com/spf13/afero**                        v1.6.0 **_new_**
* **github.com/spf13/cast**                         v1.3.1 **_new_**
* **github.com/spf13/viper**                        v1.7.1 **_new_**
* **github.com/talos-systems/crypto**               39584f1b6e54 -> d3cb77220384
* **github.com/talos-systems/extras**               v0.3.0 -> v0.3.0-1-g4fe2706
* **github.com/talos-systems/go-blockdevice**       1d830a25f64f -> v0.2.1
* **github.com/talos-systems/go-debug**             3d0a6e1bf5e3 **_new_**
* **github.com/talos-systems/go-kmsg**              v0.1.0 **_new_**
* **github.com/talos-systems/go-loadbalancer**      v0.1.0 -> v0.1.1
* **github.com/talos-systems/go-retry**             b9dc1a990133 -> c78cc953d9e9
* **github.com/talos-systems/go-smbios**            fb425d4727e6 -> d3a32bea731a
* **github.com/talos-systems/pkgs**                 v0.5.0-1-g5dd650b -> v0.6.0-alpha.0-12-g41d6ccc
* **github.com/talos-systems/talos/pkg/machinery**  8ffb55943c71 -> 000000000000
* **github.com/talos-systems/tools**                v0.5.0 -> v0.5.0-1-gc8c2a18
* **github.com/vishvananda/netns**                  2eb08e3e575f **_new_**
* **github.com/vmware-tanzu/sonobuoy**              v0.20.0 -> v0.51.0
* **github.com/vmware/govmomi**                     v0.24.0 -> v0.26.0
* **go.etcd.io/etcd/api/v3**                        v3.5.0-alpha.0 -> v3.5.0
* **go.etcd.io/etcd/client/pkg/v3**                 v3.5.0 **_new_**
* **go.etcd.io/etcd/client/v3**                     v3.5.0-alpha.0 -> v3.5.0
* **go.etcd.io/etcd/etcdutl/v3**                    v3.5.0 **_new_**
* **go.uber.org/zap**                               v1.17.0 **_new_**
* **golang.org/x/net**                              e18ecbb05110 -> 04defd469f4e
* **golang.org/x/oauth2**                           81ed05c6b58c **_new_**
* **golang.org/x/sys**                              77cc2087c03b -> 59db8d763f22
* **golang.org/x/term**                             6a3ed077a48d -> 6886f2dfbf5b
* **golang.org/x/time**                             f8bda1e9f3ba -> 38a9dc6acbc6
* **golang.zx2c4.com/wireguard/wgctrl**             bd2cb7843e1b -> 92e472f520a5
* **google.golang.org/appengine**                   v1.6.7 **_new_**
* **google.golang.org/grpc**                        v1.37.0 -> v1.38.0
* **gopkg.in/ini.v1**                               v1.62.0 **_new_**
* **inet.af/netaddr**                               1d252cf8125e **_new_**
* **k8s.io/api**                                    v0.21.0 -> v0.21.2
* **k8s.io/apimachinery**                           v0.21.0 -> v0.21.2
* **k8s.io/apiserver**                              v0.21.0 -> v0.21.2
* **k8s.io/client-go**                              v0.21.0 -> v0.21.2
* **k8s.io/cri-api**                                v0.21.0 -> v0.21.2
* **k8s.io/kubectl**                                v0.21.0 -> v0.21.2
* **k8s.io/kubelet**                                v0.21.0 -> v0.21.2
* **k8s.io/utils**                                  2afb4311ab10 **_new_**
* **sigs.k8s.io/structured-merge-diff/v4**          v4.1.1 **_new_**

Previous release can be found at [v0.10.0](https://github.com/talos-systems/talos/releases/tag/v0.10.0)

## [Talos 0.11.0-alpha.1](https://github.com/talos-systems/talos/releases/tag/v0.11.0-alpha.1) (2021-06-18)

Welcome to the v0.11.0-alpha.1 release of Talos!
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Default to Bootstrap workflow

The `init.yaml` is no longer an output of `talosctl gen config`.
We now encourage using the bootstrap API, instead of `init` node types, as we
intend on deprecating this machine type in the future.
The `init.yaml` and `controlplane.yaml` machine configs are identical with the
exception of the machine type.
Users can use a modified `controlplane.yaml` with the machine type set to
`init` if they would like to avoid using the bootstrap API.


### Component Updates

* containerd was updated to 1.5.2
* Linux kernel was updated to 5.10.38


### CoreDNS

Added the flag `cluster.coreDNS.disabled` to coreDNS deployment during the cluster bootstrap.


### Legacy BIOS Support

Added an option to the `machine.install` section of the machine config that can enable marking MBR partition bootable
for the machines that have legacy BIOS which does not support GPT partitioning scheme.


### Multi-arch Installer

Talos installer image (for any arch) now contains artifacts for both `amd64` and `arm64` architecture.
This means that e.g. images for arm64 SBCs can be generated on amd64 host.


### Networking Configuration

Talos networking configuration was completely rewritten to be based on controllers
and resources.
There are no changes to the machine configuration, but any update to `.machine.network` can now
be applied in immediate mode (without a reboot).
Talos should be setting up network configuration much faster on boot now, not blocking on DHCP for unconfigured
interfaces and skipping the reset network step.


### Talos API RBAC

Limited RBAC support in Talos API is now enabled by default for Talos 0.11.
Default `talosconfig` has `os:admin` role embedded in the certificate so that all the APIs are available.
Certificates with reduced set of roles can be created with `talosctl config new` command.

When upgrading from Talos 0.10, RBAC is not enabled by default. Before enabling RBAC, generate `talosconfig` with
`os:admin` role first to make sure that administrator still have access to the cluster when RBAC is enabled.

List of available roles:

* `os:admin` role enables every Talos API
* `os:reader` role limits access to read-only APIs which do not return sensitive informtation
* `os:etcd:backup` role only allows `talosctl etcd snapshot` API call (for etcd backup automation)


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Jorik Jonker
* Spencer Smith
* Andrew Rynhard
* Serge Logvinov
* Andrew LeCody
* Kevin Hellemun
* Boran Car
* Brandon Nason
* Gabor Nyiri
* Joost Coelingh
* Lance R. Vick
* Lennard Klein
* Seán C McCord
* Sébastien Bernard
* Sébastien Bernard

### Changes
<details><summary>143 commits</summary>
<p>

* [`f8e1cf09`](https://github.com/talos-systems/talos/commit/f8e1cf09d09c5a3d8c8ed0bdcae3d564a97e6446) release(v0.11.0-alpha.1): prepare release
* [`70ac771e`](https://github.com/talos-systems/talos/commit/70ac771e0846247dbebf484aca20ef950d8b99c7) fix: use localhost API server endpoint for internal communication
* [`a941eb7d`](https://github.com/talos-systems/talos/commit/a941eb7da06246d59cec1b63883f2d7e3f91ce73) feat: improve security of Kubernetes control plane components
* [`3aae94e5`](https://github.com/talos-systems/talos/commit/3aae94e5306c0d6e31df4aee127ee3562709edd3) feat: provide Kubernetes nodename as a COSI resource
* [`06209bba`](https://github.com/talos-systems/talos/commit/06209bba2867829561a60f0e7cd9847fa9a8edd3) chore: update RBAC rules, remove old APIs
* [`9f24b519`](https://github.com/talos-systems/talos/commit/9f24b519dce07ce05099b242ba95e8a1e319630e) chore: remove bootkube check from cluster health check
* [`4ac9bea2`](https://github.com/talos-systems/talos/commit/4ac9bea27dc098ebdfdc0958f3000d960fad50de) fix: stop etcd client logs from going to the server console
* [`f63ab9dd`](https://github.com/talos-systems/talos/commit/f63ab9dd9bb6c734873dc8073892f5f10a2ed2e1) feat: implement `talosctl config new` command
* [`fa15a668`](https://github.com/talos-systems/talos/commit/fa15a6687fc56820fbc5566d494bedbc1a5f600f) fix: don't enable RBAC feature in the config for Talos < 0.11
* [`2dc27d99`](https://github.com/talos-systems/talos/commit/2dc27d9964fa3df08a6ec11c0b045d7325ea0d2b) fix: do not format state partition in the initialize sequence
* [`b609f33c`](https://github.com/talos-systems/talos/commit/b609f33cdebb0659738d4fa3802035b2b344b9b9) fix: update networking stack after Equnix Metal testing
* [`243a3b53`](https://github.com/talos-systems/talos/commit/243a3b53e0e7591d5958a3b8373ab963990c40d6) fix: separate healthy and unknown flags in the service resource
* [`1a1378be`](https://github.com/talos-systems/talos/commit/1a1378be16fdce45273bdc81fb72715c4766ee4b) fix: update retry package with a fix for errors.Is
* [`cb83edd7`](https://github.com/talos-systems/talos/commit/cb83edd7fcf14bd199950a04e366fc573bcf4270) fix: wait for the network to be ready in mainteancne mode
* [`96f89071`](https://github.com/talos-systems/talos/commit/96f89071c3ecd809d912762e40cb9d98ce052018) feat: update controller-runtime logs to console level on config.debug
* [`973069b6`](https://github.com/talos-systems/talos/commit/973069b611456f758037c9ca4dc50a4a84e7a59c) feat: support NFS 4.1
* [`654dcad4`](https://github.com/talos-systems/talos/commit/654dcad4753211599d12655ec0f0466f27f49589) chore: bump dependencies via dependabot
* [`d7394457`](https://github.com/talos-systems/talos/commit/d7394457d978d073690bec589ea78d957539e333) fix: don't treat ethtool errors as fatal
* [`f2ae9cd0`](https://github.com/talos-systems/talos/commit/f2ae9cd0c1b7d27b5b9971f4820e5feae7934124) feat: replace networkd with new network implementation
* [`caec3063`](https://github.com/talos-systems/talos/commit/caec3063c82777f82599632ca4914a58515cb9a9) fix: do not complain about empty roles
* [`11918a11`](https://github.com/talos-systems/talos/commit/11918a110a628d7e0b8749fce92ef572aca47874) docs: update community meeting time
* [`aeddb9c0`](https://github.com/talos-systems/talos/commit/aeddb9c0977a51e7aca72f69edda8b69d917db13) feat: implement platform config controller (hostnames)
* [`1ece334d`](https://github.com/talos-systems/talos/commit/1ece334da9d7bb247c385dba08202345b83c1a0f) feat: implement controller which runs network operators
* [`744ea8a5`](https://github.com/talos-systems/talos/commit/744ea8a5d4b4cb4ff69c2c2fc636e499af892fee) fix: do not add bootstrap contents option if tail events is not 0
* [`5029edfb`](https://github.com/talos-systems/talos/commit/5029edfb71990581515cabe9634d0519a9988316) fix: overwrite nodes in the gRPC metadata
* [`6a35c8f1`](https://github.com/talos-systems/talos/commit/6a35c8f110abaf0017530650c55a34f1caae6288) feat: implement virtual IP (shared IP) network operator
* [`0f3b8380`](https://github.com/talos-systems/talos/commit/0f3b83803d812a30e1418666fa5758734c20e5c2) chore: expose WatchRequest in the resources client
* [`11e258b1`](https://github.com/talos-systems/talos/commit/11e258b15097493d2b4efd596b2fde2d52579455) feat: implement operator configuration controller
* [`ce3815e7`](https://github.com/talos-systems/talos/commit/ce3815e75e889de32d9473a23e75863f56b893da) feat: implement DHCP6 operator
* [`f010d99a`](https://github.com/talos-systems/talos/commit/f010d99afbc6095ad8fe218187fda306c59d3e1e) feat: implement operator framework with DHCP4 as the first example
* [`f93c9c8f`](https://github.com/talos-systems/talos/commit/f93c9c8fa607a5116274d7e090f49568d01814e7) feat: bring unconfigured links with link carrier up by default
* [`02bd657b`](https://github.com/talos-systems/talos/commit/02bd657b252ae64ea054b2dc338e55ce9352b420) feat: implement network.Status resource and controller
* [`da329f00`](https://github.com/talos-systems/talos/commit/da329f00ab0af9f670207da1e13541aef36c4ca6) feat: enable RBAC by default
* [`0f168a88`](https://github.com/talos-systems/talos/commit/0f168a880143141d8637d21aa9da403383dcf025) feat: add configuration for enabling RBAC
* [`e74f789b`](https://github.com/talos-systems/talos/commit/e74f789b01b9910f8193415dcefb4b32abcb5f5c) feat: implement EtcFileController to render files in `/etc`
* [`5aede1a8`](https://github.com/talos-systems/talos/commit/5aede1a83313152bd83891d0cae4b388a54bd9c2) fix: prefer extraConfig over OVF env, skip empty config
* [`5ad314fe`](https://github.com/talos-systems/talos/commit/5ad314fe7e7cfca8196770071d52b93aa4f767f6) feat: implement basic RBAC interceptors
* [`c031be81`](https://github.com/talos-systems/talos/commit/c031be8139dbe1f803b70fc9941cfe438b9ddeb9) chore: use Go 1.16.5
* [`8b0763f6`](https://github.com/talos-systems/talos/commit/8b0763f6a20691d36d2c82f2a756171c55450a8a) chore: bump dependencies via dependabot
* [`8b8de11d`](https://github.com/talos-systems/talos/commit/8b8de11d9f4d1b1fde43b7fdd56b96d5e3eb5413) feat: implement new controllers for hostname, resolvers and time servers
* [`24859b14`](https://github.com/talos-systems/talos/commit/24859b14108df7c5895022043d02d4d5ca7660a4) docs: update Rpi4 firmware guide
* [`62c702c4`](https://github.com/talos-systems/talos/commit/62c702c4fd6e7a11654f542bbe31d1adfc896731) fix: remove conflicting etcd member on rejoin with empty data directory
* [`ff62a599`](https://github.com/talos-systems/talos/commit/ff62a59984ef0c61dcf549ab38d39584e3630724) fix: drop into maintenance mode if config URL is `none` (metal)
* [`14e696d0`](https://github.com/talos-systems/talos/commit/14e696d068b5d895b4fefc06bc6d26b4ac2bc450) feat: update COSI runtime and add support for tail in the Talos gRPC
* [`a71053fc`](https://github.com/talos-systems/talos/commit/a71053fcd88d7651e536ce29b574e18f84678f3e) feat: default to bootstrap workflow
* [`76aac4bb`](https://github.com/talos-systems/talos/commit/76aac4bb25d8bc6a86458b8ac5be10ca67f236be) feat: implement CPU and Memory stats controller
* [`8f90c6a8`](https://github.com/talos-systems/talos/commit/8f90c6a8e1d76a3ddecc99be4e4b9f0ce0235daa) feat: parse Talos-specific cmdline params
* [`ed10e139`](https://github.com/talos-systems/talos/commit/ed10e139c161b0a6e0f3460e21e4e1752b26cb46) feat: implement NodeAddress controller
* [`33db8857`](https://github.com/talos-systems/talos/commit/33db8857aaf6e411464d08c51560473455e8e156) fix: use COSI runtime DestroyReady input type
* [`6e775363`](https://github.com/talos-systems/talos/commit/6e775363920b7869b83775d1b674807163039eb1) refactor: rename *.Status() to *.TypedSpec() in the resources
* [`97627061`](https://github.com/talos-systems/talos/commit/97627061d7e8de90e2f2745efa7497137447d116) docs: set static IP on ISO install mode
* [`5811f4dd`](https://github.com/talos-systems/talos/commit/5811f4dda1b62848eefae9be56e8b91d443f4d34) feat: implement link (interface) controllers
* [`046b229b`](https://github.com/talos-systems/talos/commit/046b229b13708c3ffe1d77b8884242fc100097d0) chore: skip building multi-arch installer for race-enabled build
* [`73fbb4b5`](https://github.com/talos-systems/talos/commit/73fbb4b523b41d266840eced306242d57a332b4d) fix: only fetch machine uuid if it's not set
* [`f112a540`](https://github.com/talos-systems/talos/commit/f112a540b0e776f06820ee900d6ce9f4f2de02ec) fix: clean up stale snapshots on container start
* [`c036b949`](https://github.com/talos-systems/talos/commit/c036b949486d94cbbce54c7511633d398f75797c) chore: bump dependencies
* [`a4d67a01`](https://github.com/talos-systems/talos/commit/a4d67a01820894d3ebf8c65a06345232fae4f93b) feat: add the ability to disable CoreDNS
* [`76dbfb36`](https://github.com/talos-systems/talos/commit/76dbfb3699df0725a8acf29bff39c43e4aa34f9d) feat: add ability to mark MBR partition bootable
* [`e0f5b1e2`](https://github.com/talos-systems/talos/commit/e0f5b1e20aa0d22898274ddc0f9026c0d813cee2) chore: split mgmt/gen.go into several files
* [`fad1b4f1`](https://github.com/talos-systems/talos/commit/fad1b4f1fdce962b779ceb960f81d572ee5033af) chore: fix go generate for the machinery
* [`1117294a`](https://github.com/talos-systems/talos/commit/1117294ad21945d24b0954d223cc4996df01dd81) release(v0.11.0-alpha.0): prepare release
* [`c0962946`](https://github.com/talos-systems/talos/commit/c09629466321f4d220454164784edf41fd3d5813) chore: prepare for 0.11 release series
* [`72359765`](https://github.com/talos-systems/talos/commit/723597657ad78e9766190ea2e110208c62d0093b) feat: enable GORACE=halt_on_panic=1 in machined binary
* [`0acb04ad`](https://github.com/talos-systems/talos/commit/0acb04ad7a2a0a7b75471f0251b0e04eccd927cd) feat: implement route network controllers
* [`f5bf88a4`](https://github.com/talos-systems/talos/commit/f5bf88a4c2ab8f48fd93bc7ac13543c613bf9bd1) feat: create certificates with os:admin role
* [`1db301ed`](https://github.com/talos-systems/talos/commit/1db301edf6a4057814a6d5b8f87fbfe1e020caeb) feat: switch controller-runtime to zap.Logger
* [`f7cf64d4`](https://github.com/talos-systems/talos/commit/f7cf64d42ec77ca68408ecb0f437ab5f86bc787a) fix: add talos.config to the vApp Properties in VMware OVA
* [`209527ec`](https://github.com/talos-systems/talos/commit/209527eccc6c93edad33a01a3f3d24fb978f2f07) docs: add AMIs for Talos 0.10.3
* [`59cfd312`](https://github.com/talos-systems/talos/commit/59cfd312c1ac531528c4ceb2adeb3f85829cc4e1) chore: bump dependencies via dependabot
* [`1edb20cf`](https://github.com/talos-systems/talos/commit/1edb20cf98fe2e641cefc658d17206e09acabc26) feat: extract config generation
* [`af77c295`](https://github.com/talos-systems/talos/commit/af77c29565b65766d135884ec7740f67b56626e3) docs: update wirguard guide
* [`4fe69121`](https://github.com/talos-systems/talos/commit/4fe691214366c08ea846bdc6233dd592da0d4769) test: better `talosctl ls` tests
* [`04ddda96`](https://github.com/talos-systems/talos/commit/04ddda962fbcfdeaae59d232e7bb7f9c5bb63bc7) feat: update containerd to 1.5.2, runc to 1.0.0-rc95
* [`49c7276b`](https://github.com/talos-systems/talos/commit/49c7276b16a82b7da8c83f8bd930361768f0e249) chore: fix markdown linting
* [`7270495a`](https://github.com/talos-systems/talos/commit/7270495ace9faf48a73829bbed0e4eb2c939eecb) docs: add mayastor quickstart
* [`d3d9112f`](https://github.com/talos-systems/talos/commit/d3d9112f288d3b0f3ebe1c8b28b1c4e2fc8512b2) docs: fix spelling/grammar in What's New for Talos 0.9
* [`82804414`](https://github.com/talos-systems/talos/commit/82804414fc2fcb21da77edc2fbbefe92a14fc30d) test: provide a way to force different boot order in provision library
* [`a1c0e99a`](https://github.com/talos-systems/talos/commit/a1c0e99a1729c704a633dcc557dc46466b828e11) docs: add guide for deploying metrics-server
* [`6bc6658b`](https://github.com/talos-systems/talos/commit/6bc6658b518379d418baafcf9b1045a3b84f48ec) feat: update containerd to 1.5.1
* [`c6567fae`](https://github.com/talos-systems/talos/commit/c6567fae9c59da5148c9876289a4bf248240b99d) chore: dependabot updates
* [`61ccbb3f`](https://github.com/talos-systems/talos/commit/61ccbb3f5a2564376af13ea9bbfe51e364fcb3a1) chore: keep debug symbols in debug builds
* [`1ce362e0`](https://github.com/talos-systems/talos/commit/1ce362e05e41cd76cdda17a6fc971767e036df37) docs: update customizing kernel build steps
* [`a26174b5`](https://github.com/talos-systems/talos/commit/a26174b54846bdfa0b66d2f9147bfe1dc8f2eb52) fix: properly compose pattern and header in etcd members output
* [`0825cf11`](https://github.com/talos-systems/talos/commit/0825cf11f412eef930db269b6cae02d059058101) fix: stop networkd and pods before leaving etcd on upgrade
* [`bed6b15d`](https://github.com/talos-systems/talos/commit/bed6b15d6fcf0634a887b79797d639e221fe9387) fix: properly populate AllowSchedulingOnMasters option in gen config RPC
* [`071f0445`](https://github.com/talos-systems/talos/commit/071f044562dd247dd54584d7b9fa0bb24d6f7599) feat: implement AddressSpec handling
* [`76e38b7b`](https://github.com/talos-systems/talos/commit/76e38b7b8251548292ae15ecda2bfa1c8ddc5cf3) feat: update Kubernetes to 1.21.1
* [`9b1338d9`](https://github.com/talos-systems/talos/commit/9b1338d989e6cdf7e0b6d5fe1ba3c32d27fc2251) chore: parse "boolean" variables
* [`c81cfb21`](https://github.com/talos-systems/talos/commit/c81cfb21670b82e518cf4c32230e8fbbce6be8ff) chore: allow building with debug handlers
* [`c9651673`](https://github.com/talos-systems/talos/commit/c9651673b9eaf811ae4acfed313debbf78bd80e8) feat: update go-smbios library
* [`95c656fb`](https://github.com/talos-systems/talos/commit/95c656fb72b6b858b55dae37020cb59ba26115f8) feat: update containerd to 1.5.0, runc to 1.0.0-rc94
* [`db9c35b5`](https://github.com/talos-systems/talos/commit/db9c35b570b39f4423f4636f9e9f1d14cac5d7c1) feat: implement AddressStatusController
* [`1cf011a8`](https://github.com/talos-systems/talos/commit/1cf011a809b924fc8f2083566d169704c6e07cd5) chore: bump dependencies via dependabot
* [`e3f407a1`](https://github.com/talos-systems/talos/commit/e3f407a1dff3f4ee7e024bbfb64f17b5cb5d625d) fix: properly pass disk type selector from config to matcher
* [`66b2b450`](https://github.com/talos-systems/talos/commit/66b2b450582593e93598fac80c8b3c29e8c8a944) feat: add resources and use HTTPS checks in control plane pods
* [`4ffd7c0a`](https://github.com/talos-systems/talos/commit/4ffd7c0adf281033ac02d37ca434e7f9ad71e692) fix: stop networkd before leaving etcd on 'reset' path
* [`610d38d3`](https://github.com/talos-systems/talos/commit/610d38d309dabaa623494ade12234f1ccf018a9e) docs: add AMIs for 0.10.1, collapse list of AMIs by default
* [`807497ec`](https://github.com/talos-systems/talos/commit/807497ec20dee15953186bda0fe7a45ffec0307c) chore: make conformance pipeline depend on cron-default
* [`3c121359`](https://github.com/talos-systems/talos/commit/3c1213596cdf03daf09050103f57b29e756439b1) feat: implement LinkStatusController
* [`0e8de046`](https://github.com/talos-systems/talos/commit/0e8de04698aac95062f3037da0a9af8b6ee916b0) fix: update go-blockdevice to fix disk type detection
* [`4d50a4ed`](https://github.com/talos-systems/talos/commit/4d50a4edd0eb413c16e899536ccdc2642e37aeaa) fix: update the way NTP sync uses `adjtimex` syscall
* [`1a85c14a`](https://github.com/talos-systems/talos/commit/1a85c14a51fdab43ae84274563bf89b30e4e6d92) fix: avoid data race on CRI pod stop
* [`5de8dbc0`](https://github.com/talos-systems/talos/commit/5de8dbc06c7ed36c8f3af9adea8b1abedeb372b6) fix: repair pine64 support
* [`38239097`](https://github.com/talos-systems/talos/commit/3823909735859f2ac5d95bc39c051fc9c2c07685) fix: properly parse matcher expressions
* [`e54b6b7a`](https://github.com/talos-systems/talos/commit/e54b6b7a3d7412ddce1467dfbd35efe3cfd76f3f) chore: update dependencies via dependabot
* [`f2caed0d`](https://github.com/talos-systems/talos/commit/f2caed0df5b76c4a719f968191081a6e5e2e95c7) chore: use extracted talos-systems/go-kmsg library
* [`79d804c5`](https://github.com/talos-systems/talos/commit/79d804c5b4af50a0fd73db17d2522d6a6b45c9ca) docs: fix typos
* [`a2bb390e`](https://github.com/talos-systems/talos/commit/a2bb390e1d56106d6d3c1526f3f76b34846b0274) feat: deterministic builds
* [`e480fedf`](https://github.com/talos-systems/talos/commit/e480fedff047233e78ad2c22e7b84cbbb22798d5) feat: add USB serial drivers
* [`79299d76`](https://github.com/talos-systems/talos/commit/79299d761c50aff386ab7b3c12f39c1797585632) docs: add Matrix room links
* [`1b3e8b09`](https://github.com/talos-systems/talos/commit/1b3e8b09edcd51cf3df2d43d14c8fbf1e912a465) docs: add survey to README
* [`8d51c9bb`](https://github.com/talos-systems/talos/commit/8d51c9bb190c2c60fa9be6a00572d2eaf4221e94) docs: update redirects to Talos 0.10
* [`1092c3a5`](https://github.com/talos-systems/talos/commit/1092c3a5069a3add439860d90c3615111fa03c98) feat: add Pine64 SBC support
* [`63e01754`](https://github.com/talos-systems/talos/commit/63e0175437e45c8f7e5296841337a640c600982c) feat: pull kernel with VMware balloon module enabled
* [`aeec99d8`](https://github.com/talos-systems/talos/commit/aeec99d8247f4eb534e0db1ed639f95cd726fe08) chore: remove temporary fork
* [`0f49722d`](https://github.com/talos-systems/talos/commit/0f49722d0ff4e731f17a55d1ca50472714334748) feat: add `--config-patch` flag by node type
* [`a01b1d22`](https://github.com/talos-systems/talos/commit/a01b1d22d9f3fa94355817217fefd80fe34628f3) chore: dump dependencies via dependabot
* [`d540a4a4`](https://github.com/talos-systems/talos/commit/d540a4a4711367a0ada203f668382e39876ba081) fix: bump crypto library for the CSR verification fix
* [`c3a4173e`](https://github.com/talos-systems/talos/commit/c3a4173e11a92c2bc51ea4f284ad38c9750105d2) chore: remove security API ReadFile/WriteFile
* [`38037131`](https://github.com/talos-systems/talos/commit/38037131cddc2aefbae0f48fb7e355ec76247b67) chore: update wgctrl dependecy
* [`d9ba0fd0`](https://github.com/talos-systems/talos/commit/d9ba0fd0164b2bfb2bc4ffe7a2d9d6c665a38e4d) docs: create v0.11 docs, promote v0.10 docs, add v0.10 AMIs
* [`2261d7ed`](https://github.com/talos-systems/talos/commit/2261d7ed0212c287273eac647647e4390c530a6e) fix: use both self-signed and Kubernetes CA to verify Kubelet cert
* [`a3537a69`](https://github.com/talos-systems/talos/commit/a3537a691320430eeb7149abe73419ee242312fc) docs: update cloud images for Talos v0.9.3
* [`5b9ee861`](https://github.com/talos-systems/talos/commit/5b9ee86179fb92989b02533d6d6745a5b0f37566) docs: add what's new for Talos 0.10
* [`f1107fa3`](https://github.com/talos-systems/talos/commit/f1107fa3a33955f3aa57a49991c87f9ee47b6e67) docs: add survey
* [`93623d47`](https://github.com/talos-systems/talos/commit/93623d47f24fef0d149fa006678b61e3182ef771) docs: update AWS instructions
* [`a739d1b8`](https://github.com/talos-systems/talos/commit/a739d1b8adbc026796d1c55f7319677f9010f727) feat: add support of custom registry CA certificate usage
* [`7f468d35`](https://github.com/talos-systems/talos/commit/7f468d350a6f80d2815149376fa24f7d7629402c) fix: update osType in OVA other3xLinux64Guest"
* [`4a184b67`](https://github.com/talos-systems/talos/commit/4a184b67d6ae25b21b35373e7dd6eab41b042c96) docs: add etcd backup and restore guide
* [`5fb38d3e`](https://github.com/talos-systems/talos/commit/5fb38d3e5f201934d64bae186c5300e7de7af3d4) chore: refactor Dockerfile for cross-compilation
* [`a8f1e526`](https://github.com/talos-systems/talos/commit/a8f1e526bfc00107c915572df2be08b3f154f4e6) chore: build talosctl for Darwin / Apple Silicon
* [`eb0b64d3`](https://github.com/talos-systems/talos/commit/eb0b64d3138228a6c751387c720ca81c338b834d) chore: list specifically for enabled regions
* [`669a0cbd`](https://github.com/talos-systems/talos/commit/669a0cbdc4756f0ad8f0dacc56a20f71e96fe4cd) fix: check if OVF env is empty
* [`da92049c`](https://github.com/talos-systems/talos/commit/da92049c0b4beae32af80205f50849443cd6dad3) chore: use codecov from the build container
* [`9996d4b0`](https://github.com/talos-systems/talos/commit/9996d4b028f3845071850def75f2b534e4d2b190) chore: use REGISTRY_MIRROR_FLAGS if defined
* [`05cbe250`](https://github.com/talos-systems/talos/commit/05cbe250c87339e097d435d6b10b9d8a5f2eb49e) chore: bump dependencies via dependabot
* [`9a91142a`](https://github.com/talos-systems/talos/commit/9a91142a38b3b1f210773acf8df01ed6a45599c2) feat: print complete member info in etcd members
* [`bb40d6dd`](https://github.com/talos-systems/talos/commit/bb40d6dd06a967464c24ab33744bbf460aa84038) feat: update pkgs version
* [`e7a9164b`](https://github.com/talos-systems/talos/commit/e7a9164b1e1630f953a420d99c865aef6e652d15) test: implement `talosctl conformance` command to run e2e tests
* [`6cb266e7`](https://github.com/talos-systems/talos/commit/6cb266e74e60d9d5423feaad550a7861dc73f11d) fix: update etcd client errors, print etcd join failures
* [`0bd8b0e8`](https://github.com/talos-systems/talos/commit/0bd8b0e8008c12e4914c6e9b5faf06dda6c744f7) feat: provide an option to recover etcd from data directory copy
* [`f9818540`](https://github.com/talos-systems/talos/commit/f98185408d618ebcc780247ea2c42239df27a74e) chore: fix conform with scopes
* [`21018f28`](https://github.com/talos-systems/talos/commit/21018f28c732719535c30c8e1abdbb346f1dc4bf) chore: bump website node.js dependencies
</p>
</details>

### Changes since v0.11.0-alpha.0
<details><summary>60 commits</summary>
<p>

* [`f8e1cf09`](https://github.com/talos-systems/talos/commit/f8e1cf09d09c5a3d8c8ed0bdcae3d564a97e6446) release(v0.11.0-alpha.1): prepare release
* [`70ac771e`](https://github.com/talos-systems/talos/commit/70ac771e0846247dbebf484aca20ef950d8b99c7) fix: use localhost API server endpoint for internal communication
* [`a941eb7d`](https://github.com/talos-systems/talos/commit/a941eb7da06246d59cec1b63883f2d7e3f91ce73) feat: improve security of Kubernetes control plane components
* [`3aae94e5`](https://github.com/talos-systems/talos/commit/3aae94e5306c0d6e31df4aee127ee3562709edd3) feat: provide Kubernetes nodename as a COSI resource
* [`06209bba`](https://github.com/talos-systems/talos/commit/06209bba2867829561a60f0e7cd9847fa9a8edd3) chore: update RBAC rules, remove old APIs
* [`9f24b519`](https://github.com/talos-systems/talos/commit/9f24b519dce07ce05099b242ba95e8a1e319630e) chore: remove bootkube check from cluster health check
* [`4ac9bea2`](https://github.com/talos-systems/talos/commit/4ac9bea27dc098ebdfdc0958f3000d960fad50de) fix: stop etcd client logs from going to the server console
* [`f63ab9dd`](https://github.com/talos-systems/talos/commit/f63ab9dd9bb6c734873dc8073892f5f10a2ed2e1) feat: implement `talosctl config new` command
* [`fa15a668`](https://github.com/talos-systems/talos/commit/fa15a6687fc56820fbc5566d494bedbc1a5f600f) fix: don't enable RBAC feature in the config for Talos < 0.11
* [`2dc27d99`](https://github.com/talos-systems/talos/commit/2dc27d9964fa3df08a6ec11c0b045d7325ea0d2b) fix: do not format state partition in the initialize sequence
* [`b609f33c`](https://github.com/talos-systems/talos/commit/b609f33cdebb0659738d4fa3802035b2b344b9b9) fix: update networking stack after Equnix Metal testing
* [`243a3b53`](https://github.com/talos-systems/talos/commit/243a3b53e0e7591d5958a3b8373ab963990c40d6) fix: separate healthy and unknown flags in the service resource
* [`1a1378be`](https://github.com/talos-systems/talos/commit/1a1378be16fdce45273bdc81fb72715c4766ee4b) fix: update retry package with a fix for errors.Is
* [`cb83edd7`](https://github.com/talos-systems/talos/commit/cb83edd7fcf14bd199950a04e366fc573bcf4270) fix: wait for the network to be ready in mainteancne mode
* [`96f89071`](https://github.com/talos-systems/talos/commit/96f89071c3ecd809d912762e40cb9d98ce052018) feat: update controller-runtime logs to console level on config.debug
* [`973069b6`](https://github.com/talos-systems/talos/commit/973069b611456f758037c9ca4dc50a4a84e7a59c) feat: support NFS 4.1
* [`654dcad4`](https://github.com/talos-systems/talos/commit/654dcad4753211599d12655ec0f0466f27f49589) chore: bump dependencies via dependabot
* [`d7394457`](https://github.com/talos-systems/talos/commit/d7394457d978d073690bec589ea78d957539e333) fix: don't treat ethtool errors as fatal
* [`f2ae9cd0`](https://github.com/talos-systems/talos/commit/f2ae9cd0c1b7d27b5b9971f4820e5feae7934124) feat: replace networkd with new network implementation
* [`caec3063`](https://github.com/talos-systems/talos/commit/caec3063c82777f82599632ca4914a58515cb9a9) fix: do not complain about empty roles
* [`11918a11`](https://github.com/talos-systems/talos/commit/11918a110a628d7e0b8749fce92ef572aca47874) docs: update community meeting time
* [`aeddb9c0`](https://github.com/talos-systems/talos/commit/aeddb9c0977a51e7aca72f69edda8b69d917db13) feat: implement platform config controller (hostnames)
* [`1ece334d`](https://github.com/talos-systems/talos/commit/1ece334da9d7bb247c385dba08202345b83c1a0f) feat: implement controller which runs network operators
* [`744ea8a5`](https://github.com/talos-systems/talos/commit/744ea8a5d4b4cb4ff69c2c2fc636e499af892fee) fix: do not add bootstrap contents option if tail events is not 0
* [`5029edfb`](https://github.com/talos-systems/talos/commit/5029edfb71990581515cabe9634d0519a9988316) fix: overwrite nodes in the gRPC metadata
* [`6a35c8f1`](https://github.com/talos-systems/talos/commit/6a35c8f110abaf0017530650c55a34f1caae6288) feat: implement virtual IP (shared IP) network operator
* [`0f3b8380`](https://github.com/talos-systems/talos/commit/0f3b83803d812a30e1418666fa5758734c20e5c2) chore: expose WatchRequest in the resources client
* [`11e258b1`](https://github.com/talos-systems/talos/commit/11e258b15097493d2b4efd596b2fde2d52579455) feat: implement operator configuration controller
* [`ce3815e7`](https://github.com/talos-systems/talos/commit/ce3815e75e889de32d9473a23e75863f56b893da) feat: implement DHCP6 operator
* [`f010d99a`](https://github.com/talos-systems/talos/commit/f010d99afbc6095ad8fe218187fda306c59d3e1e) feat: implement operator framework with DHCP4 as the first example
* [`f93c9c8f`](https://github.com/talos-systems/talos/commit/f93c9c8fa607a5116274d7e090f49568d01814e7) feat: bring unconfigured links with link carrier up by default
* [`02bd657b`](https://github.com/talos-systems/talos/commit/02bd657b252ae64ea054b2dc338e55ce9352b420) feat: implement network.Status resource and controller
* [`da329f00`](https://github.com/talos-systems/talos/commit/da329f00ab0af9f670207da1e13541aef36c4ca6) feat: enable RBAC by default
* [`0f168a88`](https://github.com/talos-systems/talos/commit/0f168a880143141d8637d21aa9da403383dcf025) feat: add configuration for enabling RBAC
* [`e74f789b`](https://github.com/talos-systems/talos/commit/e74f789b01b9910f8193415dcefb4b32abcb5f5c) feat: implement EtcFileController to render files in `/etc`
* [`5aede1a8`](https://github.com/talos-systems/talos/commit/5aede1a83313152bd83891d0cae4b388a54bd9c2) fix: prefer extraConfig over OVF env, skip empty config
* [`5ad314fe`](https://github.com/talos-systems/talos/commit/5ad314fe7e7cfca8196770071d52b93aa4f767f6) feat: implement basic RBAC interceptors
* [`c031be81`](https://github.com/talos-systems/talos/commit/c031be8139dbe1f803b70fc9941cfe438b9ddeb9) chore: use Go 1.16.5
* [`8b0763f6`](https://github.com/talos-systems/talos/commit/8b0763f6a20691d36d2c82f2a756171c55450a8a) chore: bump dependencies via dependabot
* [`8b8de11d`](https://github.com/talos-systems/talos/commit/8b8de11d9f4d1b1fde43b7fdd56b96d5e3eb5413) feat: implement new controllers for hostname, resolvers and time servers
* [`24859b14`](https://github.com/talos-systems/talos/commit/24859b14108df7c5895022043d02d4d5ca7660a4) docs: update Rpi4 firmware guide
* [`62c702c4`](https://github.com/talos-systems/talos/commit/62c702c4fd6e7a11654f542bbe31d1adfc896731) fix: remove conflicting etcd member on rejoin with empty data directory
* [`ff62a599`](https://github.com/talos-systems/talos/commit/ff62a59984ef0c61dcf549ab38d39584e3630724) fix: drop into maintenance mode if config URL is `none` (metal)
* [`14e696d0`](https://github.com/talos-systems/talos/commit/14e696d068b5d895b4fefc06bc6d26b4ac2bc450) feat: update COSI runtime and add support for tail in the Talos gRPC
* [`a71053fc`](https://github.com/talos-systems/talos/commit/a71053fcd88d7651e536ce29b574e18f84678f3e) feat: default to bootstrap workflow
* [`76aac4bb`](https://github.com/talos-systems/talos/commit/76aac4bb25d8bc6a86458b8ac5be10ca67f236be) feat: implement CPU and Memory stats controller
* [`8f90c6a8`](https://github.com/talos-systems/talos/commit/8f90c6a8e1d76a3ddecc99be4e4b9f0ce0235daa) feat: parse Talos-specific cmdline params
* [`ed10e139`](https://github.com/talos-systems/talos/commit/ed10e139c161b0a6e0f3460e21e4e1752b26cb46) feat: implement NodeAddress controller
* [`33db8857`](https://github.com/talos-systems/talos/commit/33db8857aaf6e411464d08c51560473455e8e156) fix: use COSI runtime DestroyReady input type
* [`6e775363`](https://github.com/talos-systems/talos/commit/6e775363920b7869b83775d1b674807163039eb1) refactor: rename *.Status() to *.TypedSpec() in the resources
* [`97627061`](https://github.com/talos-systems/talos/commit/97627061d7e8de90e2f2745efa7497137447d116) docs: set static IP on ISO install mode
* [`5811f4dd`](https://github.com/talos-systems/talos/commit/5811f4dda1b62848eefae9be56e8b91d443f4d34) feat: implement link (interface) controllers
* [`046b229b`](https://github.com/talos-systems/talos/commit/046b229b13708c3ffe1d77b8884242fc100097d0) chore: skip building multi-arch installer for race-enabled build
* [`73fbb4b5`](https://github.com/talos-systems/talos/commit/73fbb4b523b41d266840eced306242d57a332b4d) fix: only fetch machine uuid if it's not set
* [`f112a540`](https://github.com/talos-systems/talos/commit/f112a540b0e776f06820ee900d6ce9f4f2de02ec) fix: clean up stale snapshots on container start
* [`c036b949`](https://github.com/talos-systems/talos/commit/c036b949486d94cbbce54c7511633d398f75797c) chore: bump dependencies
* [`a4d67a01`](https://github.com/talos-systems/talos/commit/a4d67a01820894d3ebf8c65a06345232fae4f93b) feat: add the ability to disable CoreDNS
* [`76dbfb36`](https://github.com/talos-systems/talos/commit/76dbfb3699df0725a8acf29bff39c43e4aa34f9d) feat: add ability to mark MBR partition bootable
* [`e0f5b1e2`](https://github.com/talos-systems/talos/commit/e0f5b1e20aa0d22898274ddc0f9026c0d813cee2) chore: split mgmt/gen.go into several files
* [`fad1b4f1`](https://github.com/talos-systems/talos/commit/fad1b4f1fdce962b779ceb960f81d572ee5033af) chore: fix go generate for the machinery
</p>
</details>

### Changes from talos-systems/crypto
<details><summary>7 commits</summary>
<p>

* [`6bc5bb5`](https://github.com/talos-systems/crypto/commit/6bc5bb50c52767296a1b1cab6580e3fcf1358f34) chore: remove unused argument
* [`cd18ef6`](https://github.com/talos-systems/crypto/commit/cd18ef62eb9f65d8b6730a2eb73e47e629949e1b) feat: add support for several organizations
* [`97c888b`](https://github.com/talos-systems/crypto/commit/97c888b3924dd5ac70b8d30dd66b4370b5ab1edc) chore: add options to CSR
* [`7776057`](https://github.com/talos-systems/crypto/commit/7776057f5086157873f62f6a21ec23fa9fd86e05) chore: fix typos
* [`80df078`](https://github.com/talos-systems/crypto/commit/80df078327030af7e822668405bb4853c512bd7c) chore: remove named result parameters
* [`15bdd28`](https://github.com/talos-systems/crypto/commit/15bdd282b74ac406ab243853c1b50338a1bc29d0) chore: minor updates
* [`4f80b97`](https://github.com/talos-systems/crypto/commit/4f80b976b640d773fb025d981bf85bcc8190815b) fix: verify CSR signature before issuing a certificate
</p>
</details>

### Changes from talos-systems/extras
<details><summary>1 commit</summary>
<p>

* [`4fe2706`](https://github.com/talos-systems/extras/commit/4fe27060347c861b716392eec3dfee698becb5f3) feat: build with Go 1.16.5
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>3 commits</summary>
<p>

* [`30c2bc3`](https://github.com/talos-systems/go-blockdevice/commit/30c2bc3cb62af52f0aea9ce347923b0649fb7928) feat: mark MBR bootable
* [`1292574`](https://github.com/talos-systems/go-blockdevice/commit/1292574643e06512255fb0f45107e0c296eb5a3b) fix: make disk type matcher parser case insensitive
* [`b77400e`](https://github.com/talos-systems/go-blockdevice/commit/b77400e0a7261bf25da77c1f28c2f393f367bfa9) fix: properly detect nvme and sd card disk types
</p>
</details>

### Changes from talos-systems/go-debug
<details><summary>5 commits</summary>
<p>

* [`3d0a6e1`](https://github.com/talos-systems/go-debug/commit/3d0a6e1bf5e3c521e83ead2c8b7faad3638b8c5d) feat: race build tag flag detector
* [`5b292e5`](https://github.com/talos-systems/go-debug/commit/5b292e50198b8ed91c434f00e2772db394dbf0b9) feat: disable memory profiling by default
* [`c6d0ae2`](https://github.com/talos-systems/go-debug/commit/c6d0ae2c0ee099fa0940405401e6a02716a15bd8) fix: linters and CI
* [`d969f95`](https://github.com/talos-systems/go-debug/commit/d969f952af9e02feea59963671298fc236ca4399) feat: initial implementation
* [`b2044b7`](https://github.com/talos-systems/go-debug/commit/b2044b70379c84f9706de74044bd2fd6a8e891cf) Initial commit
</p>
</details>

### Changes from talos-systems/go-kmsg
<details><summary>2 commits</summary>
<p>

* [`2edcd3a`](https://github.com/talos-systems/go-kmsg/commit/2edcd3a913508e2d922776f729bfc4bcab031a8b) feat: add initial version
* [`53cdd8d`](https://github.com/talos-systems/go-kmsg/commit/53cdd8d67b9dbab692471a2d5161e7e0b3d04cca) chore: initial commit
</p>
</details>

### Changes from talos-systems/go-loadbalancer
<details><summary>3 commits</summary>
<p>

* [`a445702`](https://github.com/talos-systems/go-loadbalancer/commit/a4457024d5189d754b2da4a30b14072a0e3f5f05) feat: allow dial timeout and keep alive period to be configurable
* [`3c8f347`](https://github.com/talos-systems/go-loadbalancer/commit/3c8f3471d14e37866c65f73170ef83c038ae5a8c) feat: provide a way to configure logger for the loadbalancer
* [`da8e987`](https://github.com/talos-systems/go-loadbalancer/commit/da8e987434c3d407679a40e213b12a8e1c98abb8) feat: implement Reconcile - ability to change upstream list on the fly
</p>
</details>

### Changes from talos-systems/go-retry
<details><summary>3 commits</summary>
<p>

* [`c78cc95`](https://github.com/talos-systems/go-retry/commit/c78cc953d9e95992575305b4e8648392c6c9b9e6) fix: implement `errors.Is` for all errors in the set
* [`7885e16`](https://github.com/talos-systems/go-retry/commit/7885e16b2cb0267bcc8b07cdd0eced14e8005864) feat: add ExpectedErrorf
* [`3d83f61`](https://github.com/talos-systems/go-retry/commit/3d83f6126c1a3a238d1d1d59bfb6273e4087bdac) feat: deprecate UnexpectedError
</p>
</details>

### Changes from talos-systems/go-smbios
<details><summary>1 commit</summary>
<p>

* [`d3a32be`](https://github.com/talos-systems/go-smbios/commit/d3a32bea731a0c2a60ce7f5eae60253300ef27e1) fix: return UUID in middle endian only on SMBIOS >= 2.6
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>18 commits</summary>
<p>

* [`2d51360`](https://github.com/talos-systems/pkgs/commit/2d51360a254b237943e92cd445e42912d39fce7a) feat: support NFS 4.1
* [`e63e4e9`](https://github.com/talos-systems/pkgs/commit/e63e4e97b4c398e090028eaf7b967cc9eafadf96) feat: bump tools for Go 1.16.5
* [`1f8af29`](https://github.com/talos-systems/pkgs/commit/1f8af290e5d242f7dfc784fd2fc7fcfd714500bd) feat: update Linux to 5.10.38
* [`a3a6650`](https://github.com/talos-systems/pkgs/commit/a3a66505f36b9e9f92f4980df3708a872d56caec) feat: update containerd to 1.5.2
* [`c70ea44`](https://github.com/talos-systems/pkgs/commit/c70ea44ba4bc1ffabdb1422deda107a94e1fe94c) feat: update runc to 1.0.0-rc95
* [`db60235`](https://github.com/talos-systems/pkgs/commit/db602359cc594b35291911b4220dc5b331b323bb) feat: add support for netxen card
* [`f934187`](https://github.com/talos-systems/pkgs/commit/f934187ebdc455f18cc6d2da847be3d48a6e3d8f) feat: update containerd to 1.5.1
* [`e8ed5bc`](https://github.com/talos-systems/pkgs/commit/e8ed5bcb848954ca30967de8d7c81afecdea4825) feat: add geneve encapsulation support for openvswitch
* [`9f7903c`](https://github.com/talos-systems/pkgs/commit/9f7903cb5c110f77db8093347b69ec141325d47c) feat: update containerd to 1.5.0, runc to -rc94
* [`d7c0f70`](https://github.com/talos-systems/pkgs/commit/d7c0f70e41bb7bf542092f2882b062ff52f5ae44) feat: add AES-NI support for amd64
* [`b0d9cd2`](https://github.com/talos-systems/pkgs/commit/b0d9cd2c36e37190c5ce7b85acea6a51a853faaf) fix: build `zbin` utility for both amd64 and arm64
* [`bb39b97`](https://github.com/talos-systems/pkgs/commit/bb39b9744c0c4a29ccfa190a0d2cce0f8547676b) feat: add IPMI support in kernel
* [`1148f9a`](https://github.com/talos-systems/pkgs/commit/1148f9a897d9a52b6013396151e1eab264709037) feat: add DS1307 RTC support for arm64
* [`350aa6f`](https://github.com/talos-systems/pkgs/commit/350aa6f200d441d7dbbf60ec8ebb39a6761d6a8b) feat: add USB serial support
* [`de9c582`](https://github.com/talos-systems/pkgs/commit/de9c58238483219a574fb697ddb1126f36a02da3) feat: add Pine64 SBC support
* [`b56f36b`](https://github.com/talos-systems/pkgs/commit/b56f36bedbe9270ae5cf969f8078a10345457e83) feat: enable VMware baloon kernel module
* [`f87c194`](https://github.com/talos-systems/pkgs/commit/f87c19425352eb9b68d20dec987d0c484987dea9) feat: add iPXE build with embedded placeholder script
* [`a8b9e71`](https://github.com/talos-systems/pkgs/commit/a8b9e71e6538d7554b7a48d1361709d5495bb4de) feat: add cpu scaling for rpi
</p>
</details>

### Changes from talos-systems/tools
<details><summary>1 commit</summary>
<p>

* [`c8c2a18`](https://github.com/talos-systems/tools/commit/c8c2a18b7e587e0b8464574e375a680c5a09a028) feat: update Go to 1.16.5
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go**                     v1.27.0 **_new_**
* **github.com/containerd/cgroups**                 4cbc285b3327 -> v1.0.1
* **github.com/containerd/containerd**              v1.4.4 -> v1.5.2
* **github.com/containerd/go-cni**                  v1.0.1 -> v1.0.2
* **github.com/containerd/typeurl**                 v1.0.1 -> v1.0.2
* **github.com/coreos/go-iptables**                 v0.5.0 -> v0.6.0
* **github.com/cosi-project/runtime**               10d6103c19ab -> ca95c7538d17
* **github.com/docker/docker**                      v20.10.4 -> v20.10.7
* **github.com/emicklei/dot**                       v0.15.0 -> v0.16.0
* **github.com/fatih/color**                        v1.10.0 -> v1.12.0
* **github.com/google/go-cmp**                      v0.5.5 -> v0.5.6
* **github.com/google/gofuzz**                      v1.2.0 **_new_**
* **github.com/googleapis/gnostic**                 v0.5.5 **_new_**
* **github.com/grpc-ecosystem/go-grpc-middleware**  v1.2.2 -> v1.3.0
* **github.com/hashicorp/go-getter**                v1.5.2 -> v1.5.3
* **github.com/imdario/mergo**                      v0.3.12 **_new_**
* **github.com/insomniacslk/dhcp**                  cc9239ac6294 -> fb4eaaa00ad2
* **github.com/jsimonetti/rtnetlink**               1b79e63a70a0 -> b34cb89a106b
* **github.com/magiconair/properties**              v1.8.5 **_new_**
* **github.com/mattn/go-isatty**                    v0.0.12 -> v0.0.13
* **github.com/mdlayher/arp**                       f72070a231fc **_new_**
* **github.com/mdlayher/ethtool**                   2b88debcdd43 **_new_**
* **github.com/mdlayher/netlink**                   v1.4.0 -> v1.4.1
* **github.com/mdlayher/raw**                       51b895745faf **_new_**
* **github.com/mitchellh/mapstructure**             v1.4.1 **_new_**
* **github.com/opencontainers/runtime-spec**        4d89ac9fbff6 -> e6143ca7d51d
* **github.com/pelletier/go-toml**                  v1.9.0 **_new_**
* **github.com/rivo/tview**                         8a8f78a6dd01 -> 807e706f86d1
* **github.com/rs/xid**                             v1.2.1 -> v1.3.0
* **github.com/sirupsen/logrus**                    v1.8.1 **_new_**
* **github.com/spf13/afero**                        v1.6.0 **_new_**
* **github.com/spf13/cast**                         v1.3.1 **_new_**
* **github.com/spf13/viper**                        v1.7.1 **_new_**
* **github.com/talos-systems/crypto**               39584f1b6e54 -> 6bc5bb50c527
* **github.com/talos-systems/extras**               v0.3.0 -> v0.3.0-1-g4fe2706
* **github.com/talos-systems/go-blockdevice**       1d830a25f64f -> 30c2bc3cb62a
* **github.com/talos-systems/go-debug**             3d0a6e1bf5e3 **_new_**
* **github.com/talos-systems/go-kmsg**              v0.1.0 **_new_**
* **github.com/talos-systems/go-loadbalancer**      v0.1.0 -> v0.1.1
* **github.com/talos-systems/go-retry**             b9dc1a990133 -> c78cc953d9e9
* **github.com/talos-systems/go-smbios**            fb425d4727e6 -> d3a32bea731a
* **github.com/talos-systems/pkgs**                 v0.5.0-1-g5dd650b -> v0.6.0-alpha.0-8-g2d51360
* **github.com/talos-systems/talos/pkg/machinery**  8ffb55943c71 -> 000000000000
* **github.com/talos-systems/tools**                v0.5.0 -> v0.5.0-1-gc8c2a18
* **github.com/vishvananda/netns**                  2eb08e3e575f **_new_**
* **github.com/vmware-tanzu/sonobuoy**              v0.20.0 -> v0.51.0
* **github.com/vmware/govmomi**                     v0.24.0 -> v0.26.0
* **go.etcd.io/etcd/api/v3**                        v3.5.0-alpha.0 -> v3.5.0-rc.1
* **go.etcd.io/etcd/client/pkg/v3**                 v3.5.0-rc.1 **_new_**
* **go.etcd.io/etcd/client/v3**                     v3.5.0-alpha.0 -> v3.5.0-rc.1
* **go.etcd.io/etcd/etcdutl/v3**                    v3.5.0-rc.1 **_new_**
* **go.uber.org/zap**                               v1.17.0 **_new_**
* **golang.org/x/net**                              e18ecbb05110 -> abc453219eb5
* **golang.org/x/oauth2**                           81ed05c6b58c **_new_**
* **golang.org/x/sys**                              77cc2087c03b -> ebe580a85c40
* **golang.org/x/term**                             6a3ed077a48d -> a79de5458b56
* **golang.zx2c4.com/wireguard/wgctrl**             bd2cb7843e1b -> 92e472f520a5
* **google.golang.org/appengine**                   v1.6.7 **_new_**
* **google.golang.org/grpc**                        v1.37.0 -> v1.38.0
* **gopkg.in/ini.v1**                               v1.62.0 **_new_**
* **inet.af/netaddr**                               1d252cf8125e **_new_**
* **k8s.io/api**                                    v0.21.0 -> v0.21.1
* **k8s.io/apimachinery**                           v0.21.0 -> v0.21.1
* **k8s.io/apiserver**                              v0.21.0 -> v0.21.1
* **k8s.io/client-go**                              v0.21.0 -> v0.21.1
* **k8s.io/kubectl**                                v0.21.0 -> v0.21.1
* **k8s.io/kubelet**                                v0.21.0 -> v0.21.1
* **k8s.io/utils**                                  2afb4311ab10 **_new_**
* **sigs.k8s.io/structured-merge-diff/v4**          v4.1.1 **_new_**

Previous release can be found at [v0.10.0](https://github.com/talos-systems/talos/releases/tag/v0.10.0)

## [Talos 0.11.0-alpha.0](https://github.com/talos-systems/talos/releases/tag/v0.11.0-alpha.0) (2021-05-26)

Welcome to the v0.11.0-alpha.0 release of Talos!
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Component Updates

* containerd was updated to 1.5.2
* Linux kernel was updated to 5.10.29


### Multi-arch Installer

Talos installer image (for any arch) now contains artifacts for both `amd64` and `arm64` architecture.
This means that e.g. images for arm64 SBCs can be generated on amd64 host.


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Jorik Jonker
* Spencer Smith
* Serge Logvinov
* Andrew LeCody
* Andrew Rynhard
* Boran Car
* Brandon Nason
* Gabor Nyiri
* Joost Coelingh
* Kevin Hellemun
* Lance R. Vick
* Lennard Klein
* Seán C McCord
* Sébastien Bernard
* Sébastien Bernard

### Changes
<details><summary>82 commits</summary>
<p>

* [`c0962946`](https://github.com/talos-systems/talos/commit/c09629466321f4d220454164784edf41fd3d5813) chore: prepare for 0.11 release series
* [`72359765`](https://github.com/talos-systems/talos/commit/723597657ad78e9766190ea2e110208c62d0093b) feat: enable GORACE=halt_on_panic=1 in machined binary
* [`0acb04ad`](https://github.com/talos-systems/talos/commit/0acb04ad7a2a0a7b75471f0251b0e04eccd927cd) feat: implement route network controllers
* [`f5bf88a4`](https://github.com/talos-systems/talos/commit/f5bf88a4c2ab8f48fd93bc7ac13543c613bf9bd1) feat: create certificates with os:admin role
* [`1db301ed`](https://github.com/talos-systems/talos/commit/1db301edf6a4057814a6d5b8f87fbfe1e020caeb) feat: switch controller-runtime to zap.Logger
* [`f7cf64d4`](https://github.com/talos-systems/talos/commit/f7cf64d42ec77ca68408ecb0f437ab5f86bc787a) fix: add talos.config to the vApp Properties in VMware OVA
* [`209527ec`](https://github.com/talos-systems/talos/commit/209527eccc6c93edad33a01a3f3d24fb978f2f07) docs: add AMIs for Talos 0.10.3
* [`59cfd312`](https://github.com/talos-systems/talos/commit/59cfd312c1ac531528c4ceb2adeb3f85829cc4e1) chore: bump dependencies via dependabot
* [`1edb20cf`](https://github.com/talos-systems/talos/commit/1edb20cf98fe2e641cefc658d17206e09acabc26) feat: extract config generation
* [`af77c295`](https://github.com/talos-systems/talos/commit/af77c29565b65766d135884ec7740f67b56626e3) docs: update wirguard guide
* [`4fe69121`](https://github.com/talos-systems/talos/commit/4fe691214366c08ea846bdc6233dd592da0d4769) test: better `talosctl ls` tests
* [`04ddda96`](https://github.com/talos-systems/talos/commit/04ddda962fbcfdeaae59d232e7bb7f9c5bb63bc7) feat: update containerd to 1.5.2, runc to 1.0.0-rc95
* [`49c7276b`](https://github.com/talos-systems/talos/commit/49c7276b16a82b7da8c83f8bd930361768f0e249) chore: fix markdown linting
* [`7270495a`](https://github.com/talos-systems/talos/commit/7270495ace9faf48a73829bbed0e4eb2c939eecb) docs: add mayastor quickstart
* [`d3d9112f`](https://github.com/talos-systems/talos/commit/d3d9112f288d3b0f3ebe1c8b28b1c4e2fc8512b2) docs: fix spelling/grammar in What's New for Talos 0.9
* [`82804414`](https://github.com/talos-systems/talos/commit/82804414fc2fcb21da77edc2fbbefe92a14fc30d) test: provide a way to force different boot order in provision library
* [`a1c0e99a`](https://github.com/talos-systems/talos/commit/a1c0e99a1729c704a633dcc557dc46466b828e11) docs: add guide for deploying metrics-server
* [`6bc6658b`](https://github.com/talos-systems/talos/commit/6bc6658b518379d418baafcf9b1045a3b84f48ec) feat: update containerd to 1.5.1
* [`c6567fae`](https://github.com/talos-systems/talos/commit/c6567fae9c59da5148c9876289a4bf248240b99d) chore: dependabot updates
* [`61ccbb3f`](https://github.com/talos-systems/talos/commit/61ccbb3f5a2564376af13ea9bbfe51e364fcb3a1) chore: keep debug symbols in debug builds
* [`1ce362e0`](https://github.com/talos-systems/talos/commit/1ce362e05e41cd76cdda17a6fc971767e036df37) docs: update customizing kernel build steps
* [`a26174b5`](https://github.com/talos-systems/talos/commit/a26174b54846bdfa0b66d2f9147bfe1dc8f2eb52) fix: properly compose pattern and header in etcd members output
* [`0825cf11`](https://github.com/talos-systems/talos/commit/0825cf11f412eef930db269b6cae02d059058101) fix: stop networkd and pods before leaving etcd on upgrade
* [`bed6b15d`](https://github.com/talos-systems/talos/commit/bed6b15d6fcf0634a887b79797d639e221fe9387) fix: properly populate AllowSchedulingOnMasters option in gen config RPC
* [`071f0445`](https://github.com/talos-systems/talos/commit/071f044562dd247dd54584d7b9fa0bb24d6f7599) feat: implement AddressSpec handling
* [`76e38b7b`](https://github.com/talos-systems/talos/commit/76e38b7b8251548292ae15ecda2bfa1c8ddc5cf3) feat: update Kubernetes to 1.21.1
* [`9b1338d9`](https://github.com/talos-systems/talos/commit/9b1338d989e6cdf7e0b6d5fe1ba3c32d27fc2251) chore: parse "boolean" variables
* [`c81cfb21`](https://github.com/talos-systems/talos/commit/c81cfb21670b82e518cf4c32230e8fbbce6be8ff) chore: allow building with debug handlers
* [`c9651673`](https://github.com/talos-systems/talos/commit/c9651673b9eaf811ae4acfed313debbf78bd80e8) feat: update go-smbios library
* [`95c656fb`](https://github.com/talos-systems/talos/commit/95c656fb72b6b858b55dae37020cb59ba26115f8) feat: update containerd to 1.5.0, runc to 1.0.0-rc94
* [`db9c35b5`](https://github.com/talos-systems/talos/commit/db9c35b570b39f4423f4636f9e9f1d14cac5d7c1) feat: implement AddressStatusController
* [`1cf011a8`](https://github.com/talos-systems/talos/commit/1cf011a809b924fc8f2083566d169704c6e07cd5) chore: bump dependencies via dependabot
* [`e3f407a1`](https://github.com/talos-systems/talos/commit/e3f407a1dff3f4ee7e024bbfb64f17b5cb5d625d) fix: properly pass disk type selector from config to matcher
* [`66b2b450`](https://github.com/talos-systems/talos/commit/66b2b450582593e93598fac80c8b3c29e8c8a944) feat: add resources and use HTTPS checks in control plane pods
* [`4ffd7c0a`](https://github.com/talos-systems/talos/commit/4ffd7c0adf281033ac02d37ca434e7f9ad71e692) fix: stop networkd before leaving etcd on 'reset' path
* [`610d38d3`](https://github.com/talos-systems/talos/commit/610d38d309dabaa623494ade12234f1ccf018a9e) docs: add AMIs for 0.10.1, collapse list of AMIs by default
* [`807497ec`](https://github.com/talos-systems/talos/commit/807497ec20dee15953186bda0fe7a45ffec0307c) chore: make conformance pipeline depend on cron-default
* [`3c121359`](https://github.com/talos-systems/talos/commit/3c1213596cdf03daf09050103f57b29e756439b1) feat: implement LinkStatusController
* [`0e8de046`](https://github.com/talos-systems/talos/commit/0e8de04698aac95062f3037da0a9af8b6ee916b0) fix: update go-blockdevice to fix disk type detection
* [`4d50a4ed`](https://github.com/talos-systems/talos/commit/4d50a4edd0eb413c16e899536ccdc2642e37aeaa) fix: update the way NTP sync uses `adjtimex` syscall
* [`1a85c14a`](https://github.com/talos-systems/talos/commit/1a85c14a51fdab43ae84274563bf89b30e4e6d92) fix: avoid data race on CRI pod stop
* [`5de8dbc0`](https://github.com/talos-systems/talos/commit/5de8dbc06c7ed36c8f3af9adea8b1abedeb372b6) fix: repair pine64 support
* [`38239097`](https://github.com/talos-systems/talos/commit/3823909735859f2ac5d95bc39c051fc9c2c07685) fix: properly parse matcher expressions
* [`e54b6b7a`](https://github.com/talos-systems/talos/commit/e54b6b7a3d7412ddce1467dfbd35efe3cfd76f3f) chore: update dependencies via dependabot
* [`f2caed0d`](https://github.com/talos-systems/talos/commit/f2caed0df5b76c4a719f968191081a6e5e2e95c7) chore: use extracted talos-systems/go-kmsg library
* [`79d804c5`](https://github.com/talos-systems/talos/commit/79d804c5b4af50a0fd73db17d2522d6a6b45c9ca) docs: fix typos
* [`a2bb390e`](https://github.com/talos-systems/talos/commit/a2bb390e1d56106d6d3c1526f3f76b34846b0274) feat: deterministic builds
* [`e480fedf`](https://github.com/talos-systems/talos/commit/e480fedff047233e78ad2c22e7b84cbbb22798d5) feat: add USB serial drivers
* [`79299d76`](https://github.com/talos-systems/talos/commit/79299d761c50aff386ab7b3c12f39c1797585632) docs: add Matrix room links
* [`1b3e8b09`](https://github.com/talos-systems/talos/commit/1b3e8b09edcd51cf3df2d43d14c8fbf1e912a465) docs: add survey to README
* [`8d51c9bb`](https://github.com/talos-systems/talos/commit/8d51c9bb190c2c60fa9be6a00572d2eaf4221e94) docs: update redirects to Talos 0.10
* [`1092c3a5`](https://github.com/talos-systems/talos/commit/1092c3a5069a3add439860d90c3615111fa03c98) feat: add Pine64 SBC support
* [`63e01754`](https://github.com/talos-systems/talos/commit/63e0175437e45c8f7e5296841337a640c600982c) feat: pull kernel with VMware balloon module enabled
* [`aeec99d8`](https://github.com/talos-systems/talos/commit/aeec99d8247f4eb534e0db1ed639f95cd726fe08) chore: remove temporary fork
* [`0f49722d`](https://github.com/talos-systems/talos/commit/0f49722d0ff4e731f17a55d1ca50472714334748) feat: add `--config-patch` flag by node type
* [`a01b1d22`](https://github.com/talos-systems/talos/commit/a01b1d22d9f3fa94355817217fefd80fe34628f3) chore: dump dependencies via dependabot
* [`d540a4a4`](https://github.com/talos-systems/talos/commit/d540a4a4711367a0ada203f668382e39876ba081) fix: bump crypto library for the CSR verification fix
* [`c3a4173e`](https://github.com/talos-systems/talos/commit/c3a4173e11a92c2bc51ea4f284ad38c9750105d2) chore: remove security API ReadFile/WriteFile
* [`38037131`](https://github.com/talos-systems/talos/commit/38037131cddc2aefbae0f48fb7e355ec76247b67) chore: update wgctrl dependecy
* [`d9ba0fd0`](https://github.com/talos-systems/talos/commit/d9ba0fd0164b2bfb2bc4ffe7a2d9d6c665a38e4d) docs: create v0.11 docs, promote v0.10 docs, add v0.10 AMIs
* [`2261d7ed`](https://github.com/talos-systems/talos/commit/2261d7ed0212c287273eac647647e4390c530a6e) fix: use both self-signed and Kubernetes CA to verify Kubelet cert
* [`a3537a69`](https://github.com/talos-systems/talos/commit/a3537a691320430eeb7149abe73419ee242312fc) docs: update cloud images for Talos v0.9.3
* [`5b9ee861`](https://github.com/talos-systems/talos/commit/5b9ee86179fb92989b02533d6d6745a5b0f37566) docs: add what's new for Talos 0.10
* [`f1107fa3`](https://github.com/talos-systems/talos/commit/f1107fa3a33955f3aa57a49991c87f9ee47b6e67) docs: add survey
* [`93623d47`](https://github.com/talos-systems/talos/commit/93623d47f24fef0d149fa006678b61e3182ef771) docs: update AWS instructions
* [`a739d1b8`](https://github.com/talos-systems/talos/commit/a739d1b8adbc026796d1c55f7319677f9010f727) feat: add support of custom registry CA certificate usage
* [`7f468d35`](https://github.com/talos-systems/talos/commit/7f468d350a6f80d2815149376fa24f7d7629402c) fix: update osType in OVA other3xLinux64Guest"
* [`4a184b67`](https://github.com/talos-systems/talos/commit/4a184b67d6ae25b21b35373e7dd6eab41b042c96) docs: add etcd backup and restore guide
* [`5fb38d3e`](https://github.com/talos-systems/talos/commit/5fb38d3e5f201934d64bae186c5300e7de7af3d4) chore: refactor Dockerfile for cross-compilation
* [`a8f1e526`](https://github.com/talos-systems/talos/commit/a8f1e526bfc00107c915572df2be08b3f154f4e6) chore: build talosctl for Darwin / Apple Silicon
* [`eb0b64d3`](https://github.com/talos-systems/talos/commit/eb0b64d3138228a6c751387c720ca81c338b834d) chore: list specifically for enabled regions
* [`669a0cbd`](https://github.com/talos-systems/talos/commit/669a0cbdc4756f0ad8f0dacc56a20f71e96fe4cd) fix: check if OVF env is empty
* [`da92049c`](https://github.com/talos-systems/talos/commit/da92049c0b4beae32af80205f50849443cd6dad3) chore: use codecov from the build container
* [`9996d4b0`](https://github.com/talos-systems/talos/commit/9996d4b028f3845071850def75f2b534e4d2b190) chore: use REGISTRY_MIRROR_FLAGS if defined
* [`05cbe250`](https://github.com/talos-systems/talos/commit/05cbe250c87339e097d435d6b10b9d8a5f2eb49e) chore: bump dependencies via dependabot
* [`9a91142a`](https://github.com/talos-systems/talos/commit/9a91142a38b3b1f210773acf8df01ed6a45599c2) feat: print complete member info in etcd members
* [`bb40d6dd`](https://github.com/talos-systems/talos/commit/bb40d6dd06a967464c24ab33744bbf460aa84038) feat: update pkgs version
* [`e7a9164b`](https://github.com/talos-systems/talos/commit/e7a9164b1e1630f953a420d99c865aef6e652d15) test: implement `talosctl conformance` command to run e2e tests
* [`6cb266e7`](https://github.com/talos-systems/talos/commit/6cb266e74e60d9d5423feaad550a7861dc73f11d) fix: update etcd client errors, print etcd join failures
* [`0bd8b0e8`](https://github.com/talos-systems/talos/commit/0bd8b0e8008c12e4914c6e9b5faf06dda6c744f7) feat: provide an option to recover etcd from data directory copy
* [`f9818540`](https://github.com/talos-systems/talos/commit/f98185408d618ebcc780247ea2c42239df27a74e) chore: fix conform with scopes
* [`21018f28`](https://github.com/talos-systems/talos/commit/21018f28c732719535c30c8e1abdbb346f1dc4bf) chore: bump website node.js dependencies
</p>
</details>

### Changes from talos-systems/crypto
<details><summary>1 commit</summary>
<p>

* [`4f80b97`](https://github.com/talos-systems/crypto/commit/4f80b976b640d773fb025d981bf85bcc8190815b) fix: verify CSR signature before issuing a certificate
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>2 commits</summary>
<p>

* [`1292574`](https://github.com/talos-systems/go-blockdevice/commit/1292574643e06512255fb0f45107e0c296eb5a3b) fix: make disk type matcher parser case insensitive
* [`b77400e`](https://github.com/talos-systems/go-blockdevice/commit/b77400e0a7261bf25da77c1f28c2f393f367bfa9) fix: properly detect nvme and sd card disk types
</p>
</details>

### Changes from talos-systems/go-debug
<details><summary>5 commits</summary>
<p>

* [`3d0a6e1`](https://github.com/talos-systems/go-debug/commit/3d0a6e1bf5e3c521e83ead2c8b7faad3638b8c5d) feat: race build tag flag detector
* [`5b292e5`](https://github.com/talos-systems/go-debug/commit/5b292e50198b8ed91c434f00e2772db394dbf0b9) feat: disable memory profiling by default
* [`c6d0ae2`](https://github.com/talos-systems/go-debug/commit/c6d0ae2c0ee099fa0940405401e6a02716a15bd8) fix: linters and CI
* [`d969f95`](https://github.com/talos-systems/go-debug/commit/d969f952af9e02feea59963671298fc236ca4399) feat: initial implementation
* [`b2044b7`](https://github.com/talos-systems/go-debug/commit/b2044b70379c84f9706de74044bd2fd6a8e891cf) Initial commit
</p>
</details>

### Changes from talos-systems/go-kmsg
<details><summary>2 commits</summary>
<p>

* [`2edcd3a`](https://github.com/talos-systems/go-kmsg/commit/2edcd3a913508e2d922776f729bfc4bcab031a8b) feat: add initial version
* [`53cdd8d`](https://github.com/talos-systems/go-kmsg/commit/53cdd8d67b9dbab692471a2d5161e7e0b3d04cca) chore: initial commit
</p>
</details>

### Changes from talos-systems/go-loadbalancer
<details><summary>3 commits</summary>
<p>

* [`a445702`](https://github.com/talos-systems/go-loadbalancer/commit/a4457024d5189d754b2da4a30b14072a0e3f5f05) feat: allow dial timeout and keep alive period to be configurable
* [`3c8f347`](https://github.com/talos-systems/go-loadbalancer/commit/3c8f3471d14e37866c65f73170ef83c038ae5a8c) feat: provide a way to configure logger for the loadbalancer
* [`da8e987`](https://github.com/talos-systems/go-loadbalancer/commit/da8e987434c3d407679a40e213b12a8e1c98abb8) feat: implement Reconcile - ability to change upstream list on the fly
</p>
</details>

### Changes from talos-systems/go-smbios
<details><summary>1 commit</summary>
<p>

* [`d3a32be`](https://github.com/talos-systems/go-smbios/commit/d3a32bea731a0c2a60ce7f5eae60253300ef27e1) fix: return UUID in middle endian only on SMBIOS >= 2.6
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>15 commits</summary>
<p>

* [`a3a6650`](https://github.com/talos-systems/pkgs/commit/a3a66505f36b9e9f92f4980df3708a872d56caec) feat: update containerd to 1.5.2
* [`c70ea44`](https://github.com/talos-systems/pkgs/commit/c70ea44ba4bc1ffabdb1422deda107a94e1fe94c) feat: update runc to 1.0.0-rc95
* [`db60235`](https://github.com/talos-systems/pkgs/commit/db602359cc594b35291911b4220dc5b331b323bb) feat: add support for netxen card
* [`f934187`](https://github.com/talos-systems/pkgs/commit/f934187ebdc455f18cc6d2da847be3d48a6e3d8f) feat: update containerd to 1.5.1
* [`e8ed5bc`](https://github.com/talos-systems/pkgs/commit/e8ed5bcb848954ca30967de8d7c81afecdea4825) feat: add geneve encapsulation support for openvswitch
* [`9f7903c`](https://github.com/talos-systems/pkgs/commit/9f7903cb5c110f77db8093347b69ec141325d47c) feat: update containerd to 1.5.0, runc to -rc94
* [`d7c0f70`](https://github.com/talos-systems/pkgs/commit/d7c0f70e41bb7bf542092f2882b062ff52f5ae44) feat: add AES-NI support for amd64
* [`b0d9cd2`](https://github.com/talos-systems/pkgs/commit/b0d9cd2c36e37190c5ce7b85acea6a51a853faaf) fix: build `zbin` utility for both amd64 and arm64
* [`bb39b97`](https://github.com/talos-systems/pkgs/commit/bb39b9744c0c4a29ccfa190a0d2cce0f8547676b) feat: add IPMI support in kernel
* [`1148f9a`](https://github.com/talos-systems/pkgs/commit/1148f9a897d9a52b6013396151e1eab264709037) feat: add DS1307 RTC support for arm64
* [`350aa6f`](https://github.com/talos-systems/pkgs/commit/350aa6f200d441d7dbbf60ec8ebb39a6761d6a8b) feat: add USB serial support
* [`de9c582`](https://github.com/talos-systems/pkgs/commit/de9c58238483219a574fb697ddb1126f36a02da3) feat: add Pine64 SBC support
* [`b56f36b`](https://github.com/talos-systems/pkgs/commit/b56f36bedbe9270ae5cf969f8078a10345457e83) feat: enable VMware baloon kernel module
* [`f87c194`](https://github.com/talos-systems/pkgs/commit/f87c19425352eb9b68d20dec987d0c484987dea9) feat: add iPXE build with embedded placeholder script
* [`a8b9e71`](https://github.com/talos-systems/pkgs/commit/a8b9e71e6538d7554b7a48d1361709d5495bb4de) feat: add cpu scaling for rpi
</p>
</details>

### Dependency Changes

* **github.com/containerd/cgroups**                 4cbc285b3327 -> v1.0.1
* **github.com/containerd/containerd**              v1.4.4 -> v1.5.2
* **github.com/containerd/go-cni**                  v1.0.1 -> v1.0.2
* **github.com/containerd/typeurl**                 v1.0.1 -> v1.0.2
* **github.com/coreos/go-iptables**                 v0.5.0 -> v0.6.0
* **github.com/cosi-project/runtime**               10d6103c19ab -> 8a4533ce68e2
* **github.com/docker/docker**                      v20.10.4 -> v20.10.6
* **github.com/emicklei/dot**                       v0.15.0 -> v0.16.0
* **github.com/fatih/color**                        v1.10.0 -> v1.11.0
* **github.com/grpc-ecosystem/go-grpc-middleware**  v1.2.2 -> v1.3.0
* **github.com/hashicorp/go-getter**                v1.5.2 -> v1.5.3
* **github.com/mdlayher/ethtool**                   2b88debcdd43 **_new_**
* **github.com/opencontainers/runtime-spec**        4d89ac9fbff6 -> e6143ca7d51d
* **github.com/plunder-app/kube-vip**               v0.3.2 -> v0.3.4
* **github.com/rs/xid**                             v1.2.1 -> v1.3.0
* **github.com/talos-systems/crypto**               39584f1b6e54 -> 4f80b976b640
* **github.com/talos-systems/go-blockdevice**       1d830a25f64f -> 1292574643e0
* **github.com/talos-systems/go-debug**             3d0a6e1bf5e3 **_new_**
* **github.com/talos-systems/go-kmsg**              v0.1.0 **_new_**
* **github.com/talos-systems/go-loadbalancer**      v0.1.0 -> v0.1.1
* **github.com/talos-systems/go-smbios**            fb425d4727e6 -> d3a32bea731a
* **github.com/talos-systems/pkgs**                 v0.5.0-1-g5dd650b -> v0.6.0-alpha.0-5-ga3a6650
* **github.com/vmware-tanzu/sonobuoy**              v0.20.0 -> v0.50.0
* **github.com/vmware/govmomi**                     v0.24.0 -> v0.25.0
* **go.etcd.io/etcd/api/v3**                        v3.5.0-alpha.0 -> v3.5.0-beta.3
* **go.etcd.io/etcd/client/pkg/v3**                 v3.5.0-beta.3 **_new_**
* **go.etcd.io/etcd/client/v3**                     v3.5.0-alpha.0 -> v3.5.0-beta.3
* **go.etcd.io/etcd/etcdutl/v3**                    v3.5.0-beta.3 **_new_**
* **go.uber.org/zap**                               c23abee72d19 **_new_**
* **golang.org/x/net**                              e18ecbb05110 -> 0714010a04ed
* **golang.org/x/sys**                              77cc2087c03b -> 0981d6026fa6
* **golang.org/x/term**                             6a3ed077a48d -> a79de5458b56
* **golang.zx2c4.com/wireguard/wgctrl**             bd2cb7843e1b -> f9ad6d392236
* **google.golang.org/grpc**                        v1.37.0 -> v1.38.0
* **inet.af/netaddr**                               1d252cf8125e **_new_**
* **k8s.io/api**                                    v0.21.0 -> v0.21.1
* **k8s.io/apimachinery**                           v0.21.0 -> v0.21.1
* **k8s.io/apiserver**                              v0.21.0 -> v0.21.1
* **k8s.io/client-go**                              v0.21.0 -> v0.21.1
* **k8s.io/kubectl**                                v0.21.0 -> v0.21.1
* **k8s.io/kubelet**                                v0.21.0 -> v0.21.1

Previous release can be found at [v0.10.0](https://github.com/talos-systems/talos/releases/tag/v0.10.0)

## [Talos 0.10.0-alpha.2](https://github.com/talos-systems/talos/releases/tag/v0.10.0-alpha.2) (2021-04-08)

Welcome to the v0.10.0-alpha.2 release of Talos!
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### Disaster Recovery

* support for creating etcd snapshots (backups) with `talosctl etcd snapshot` command.
* etcd cluster can be recovered from a snapshot using `talosctl boostrap --recover-from=` command.


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


### Optimizations

* Talos `system` services now run without container images on initramfs from the single executable; this change reduces RAM usage, initramfs size and boot time..


### SBCs

* u-boot version was updated to fix the boot and USB issues on Raspberry Pi 4 8GiB version.
* added support for Rock Pi 4.


### Time Syncrhonization

* `timed` service was replaced with a time sync controller, no machine configuration changes.
* Talos now prefers last successful time server (by IP address) on each sync attempt (improves sync accuracy).


### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Artem Chernyshev
* Spencer Smith
* Seán C McCord
* Andrew Rynhard
* Branden Cash
* Jorik Jonker
* Matt Zahorik
* bzub

### Changes
<details><summary>104 commits</summary>
<p>

* [`e0650218`](https://github.com/talos-systems/talos/commit/e0650218a6b0a05a8e109262a0d7ed3d7359ea37) feat: support etcd recovery from snapshot on bootstrap
* [`247bd50e`](https://github.com/talos-systems/talos/commit/247bd50e0510f57c969e3bb8fee5b53bfcdbb074) docs: describe steps to install and boot Talos from the SSD on rockpi4
* [`e6b4e524`](https://github.com/talos-systems/talos/commit/e6b4e524ffa33a5c76368f0fe8e9c372e3297cfc) test: update CAPA to 0.6.4
* [`28753f6d`](https://github.com/talos-systems/talos/commit/28753f6dcb85450965e4d4a0fb68f448e1deee23) fix: trim endpoints/nodes from arguments in talosctl config
* [`aca63b88`](https://github.com/talos-systems/talos/commit/aca63b8829ad0eebd449573120bff2d9b90ba828) docs: fix "DigitalOcean" spelling
* [`33035901`](https://github.com/talos-systems/talos/commit/33035901ff7875bdf9eb99fb86b377318f60d74b) fix: revert mark PMBR EFI partition as bootable
* [`fbfd1eb2`](https://github.com/talos-systems/talos/commit/fbfd1eb2b1684fe38caa12b8d46d608c42b5daf6) refactor: pull new version of os-runtime, update code
* [`8737ea71`](https://github.com/talos-systems/talos/commit/8737ea716a5d9adf24959a56a73dd61e1139b808) feat: allow external cloud provides configration
* [`3909e2d0`](https://github.com/talos-systems/talos/commit/3909e2d011b9d11653903687e5a4210daa440ef2) chore: update Go to 1.16.3
* [`690eb20e`](https://github.com/talos-systems/talos/commit/690eb20e9763d8f3036f0a1b4b9447f19c5ec05b) chore: update blockdevice library for PMBR bootable fix
* [`a8761b8e`](https://github.com/talos-systems/talos/commit/a8761b8e1efd07a3bda3d8f706d3d7bf658955bb) fix: require leader on etcd member operations
* [`3dc84625`](https://github.com/talos-systems/talos/commit/3dc84625cb1b323bad1dd93d89a13d3d59ea22d8) fix: make both HDMI ports work on RPi 4
* [`bd5ae1e0`](https://github.com/talos-systems/talos/commit/bd5ae1e0b5dd303a017156ba7af733f79d3c13ef) fix: add a check for overlay mounts in installer pre-flight checks
* [`df8649cb`](https://github.com/talos-systems/talos/commit/df8649cbe6f4fcf04c4b84a444ec2519e37ac171) refactor: download modules before `go generate`
* [`39ae0415`](https://github.com/talos-systems/talos/commit/39ae0415e9d932c01ff33163d97daef375c21a7f) chore: bump dependencies via dependabot
* [`e16d6d34`](https://github.com/talos-systems/talos/commit/e16d6d3468a7a072b41e94fdc352df15b8321376) fix: publish rockpi4 image to release artifacts
* [`39c6dbcc`](https://github.com/talos-systems/talos/commit/39c6dbcc7ae8f07e1ab4c2a82508ebee07f66207) feat: add --config-patch parameter to talosctl gen config
* [`e664362c`](https://github.com/talos-systems/talos/commit/e664362cecb476a41360143a05c0cfad718b2e0f) feat: add API and command to save etcd snapshot (backup)
* [`61b694b9`](https://github.com/talos-systems/talos/commit/61b694b94896da47e2ddf677cbf12b18007268a5) fix: create rootfs for system services via /system tmpfs
* [`abc2e17e`](https://github.com/talos-systems/talos/commit/abc2e17ebb6d440438e407e5a5d1c5c1f7d1eeff) test: update 0.9.x version in upgrade tests to 0.9.1
* [`a1e64154`](https://github.com/talos-systems/talos/commit/a1e6415403df9827fb486492a4b292b9aab3076b) fix: retry Kubernetes API errors on cordon/uncordon/etc
* [`063d1abe`](https://github.com/talos-systems/talos/commit/063d1abe9cf1634f3517893977fc907dd9004c55) fix: print task failure error immediately
* [`e039172e`](https://github.com/talos-systems/talos/commit/e039172edac115afbd5bf36a1f266e5967ca5398) fix: ignore EOF errors from Kubernetes API when converting control plane
* [`7bcb91a4`](https://github.com/talos-systems/talos/commit/7bcb91a433f14a29a0d2bbe9d70eb5a997eb9ab0) docs: fix typo for stage flag
* [`a43acb21`](https://github.com/talos-systems/talos/commit/a43acb2150cadd78da51c41569b7f219b704f089) feat: bring in Linux 5.10.27, support for 32-bit time syscalls
* [`e2bb5973`](https://github.com/talos-systems/talos/commit/e2bb5973da5b2dc15aba2a809e0e31426b6f22b3) release(v0.10.0-alpha.1): prepare release
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

### Changes since v0.10.0-alpha.1
<details><summary>25 commits</summary>
<p>

* [`e0650218`](https://github.com/talos-systems/talos/commit/e0650218a6b0a05a8e109262a0d7ed3d7359ea37) feat: support etcd recovery from snapshot on bootstrap
* [`247bd50e`](https://github.com/talos-systems/talos/commit/247bd50e0510f57c969e3bb8fee5b53bfcdbb074) docs: describe steps to install and boot Talos from the SSD on rockpi4
* [`e6b4e524`](https://github.com/talos-systems/talos/commit/e6b4e524ffa33a5c76368f0fe8e9c372e3297cfc) test: update CAPA to 0.6.4
* [`28753f6d`](https://github.com/talos-systems/talos/commit/28753f6dcb85450965e4d4a0fb68f448e1deee23) fix: trim endpoints/nodes from arguments in talosctl config
* [`aca63b88`](https://github.com/talos-systems/talos/commit/aca63b8829ad0eebd449573120bff2d9b90ba828) docs: fix "DigitalOcean" spelling
* [`33035901`](https://github.com/talos-systems/talos/commit/33035901ff7875bdf9eb99fb86b377318f60d74b) fix: revert mark PMBR EFI partition as bootable
* [`fbfd1eb2`](https://github.com/talos-systems/talos/commit/fbfd1eb2b1684fe38caa12b8d46d608c42b5daf6) refactor: pull new version of os-runtime, update code
* [`8737ea71`](https://github.com/talos-systems/talos/commit/8737ea716a5d9adf24959a56a73dd61e1139b808) feat: allow external cloud provides configration
* [`3909e2d0`](https://github.com/talos-systems/talos/commit/3909e2d011b9d11653903687e5a4210daa440ef2) chore: update Go to 1.16.3
* [`690eb20e`](https://github.com/talos-systems/talos/commit/690eb20e9763d8f3036f0a1b4b9447f19c5ec05b) chore: update blockdevice library for PMBR bootable fix
* [`a8761b8e`](https://github.com/talos-systems/talos/commit/a8761b8e1efd07a3bda3d8f706d3d7bf658955bb) fix: require leader on etcd member operations
* [`3dc84625`](https://github.com/talos-systems/talos/commit/3dc84625cb1b323bad1dd93d89a13d3d59ea22d8) fix: make both HDMI ports work on RPi 4
* [`bd5ae1e0`](https://github.com/talos-systems/talos/commit/bd5ae1e0b5dd303a017156ba7af733f79d3c13ef) fix: add a check for overlay mounts in installer pre-flight checks
* [`df8649cb`](https://github.com/talos-systems/talos/commit/df8649cbe6f4fcf04c4b84a444ec2519e37ac171) refactor: download modules before `go generate`
* [`39ae0415`](https://github.com/talos-systems/talos/commit/39ae0415e9d932c01ff33163d97daef375c21a7f) chore: bump dependencies via dependabot
* [`e16d6d34`](https://github.com/talos-systems/talos/commit/e16d6d3468a7a072b41e94fdc352df15b8321376) fix: publish rockpi4 image to release artifacts
* [`39c6dbcc`](https://github.com/talos-systems/talos/commit/39c6dbcc7ae8f07e1ab4c2a82508ebee07f66207) feat: add --config-patch parameter to talosctl gen config
* [`e664362c`](https://github.com/talos-systems/talos/commit/e664362cecb476a41360143a05c0cfad718b2e0f) feat: add API and command to save etcd snapshot (backup)
* [`61b694b9`](https://github.com/talos-systems/talos/commit/61b694b94896da47e2ddf677cbf12b18007268a5) fix: create rootfs for system services via /system tmpfs
* [`abc2e17e`](https://github.com/talos-systems/talos/commit/abc2e17ebb6d440438e407e5a5d1c5c1f7d1eeff) test: update 0.9.x version in upgrade tests to 0.9.1
* [`a1e64154`](https://github.com/talos-systems/talos/commit/a1e6415403df9827fb486492a4b292b9aab3076b) fix: retry Kubernetes API errors on cordon/uncordon/etc
* [`063d1abe`](https://github.com/talos-systems/talos/commit/063d1abe9cf1634f3517893977fc907dd9004c55) fix: print task failure error immediately
* [`e039172e`](https://github.com/talos-systems/talos/commit/e039172edac115afbd5bf36a1f266e5967ca5398) fix: ignore EOF errors from Kubernetes API when converting control plane
* [`7bcb91a4`](https://github.com/talos-systems/talos/commit/7bcb91a433f14a29a0d2bbe9d70eb5a997eb9ab0) docs: fix typo for stage flag
* [`a43acb21`](https://github.com/talos-systems/talos/commit/a43acb2150cadd78da51c41569b7f219b704f089) feat: bring in Linux 5.10.27, support for 32-bit time syscalls
</p>
</details>

### Changes from talos-systems/extras
<details><summary>3 commits</summary>
<p>

* [`cf3934a`](https://github.com/talos-systems/extras/commit/cf3934ae09b22c396226bed6618b3d03ab298e33) feat: build with Go 1.16.3
* [`c0fa0c0`](https://github.com/talos-systems/extras/commit/c0fa0c04641d8dfc418888c210788a6894e8d40c) feat: bump Go to 1.16.2
* [`5f89d77`](https://github.com/talos-systems/extras/commit/5f89d77a91f44d52146dae9c23b4654d219042b9) feat: bump Go to 1.16.1
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>3 commits</summary>
<p>

* [`1d830a2`](https://github.com/talos-systems/go-blockdevice/commit/1d830a25f64f6fb96a1bedd800c0b40b107dc833) fix: revert mark the EFI partition in PMBR as bootable
* [`bec914f`](https://github.com/talos-systems/go-blockdevice/commit/bec914ffdda42abcfe642bc2cdfc9fcda56a74ee) fix: mark the EFI partition in PMBR as bootable
* [`776b37d`](https://github.com/talos-systems/go-blockdevice/commit/776b37d31de0781f098f5d9d1894fbea3f2dfa1d) feat: add options to probe disk by various sysblock parameters
</p>
</details>

### Changes from talos-systems/os-runtime
<details><summary>5 commits</summary>
<p>

* [`86d9e09`](https://github.com/talos-systems/os-runtime/commit/86d9e090bdc4ebfdc8bba0333a067ce189e839da) chore: bump go.mod dependencies
* [`2de411a`](https://github.com/talos-systems/os-runtime/commit/2de411a4765de15de1d5b1524131d262801eb395) feat: major rewrite of the os-runtime with new features
* [`ded40a7`](https://github.com/talos-systems/os-runtime/commit/ded40a78343f77dfc02ba5e5857a6baea99da682) feat: implement controller runtime gRPC bridge
* [`0d5b5a9`](https://github.com/talos-systems/os-runtime/commit/0d5b5a942c26c8de35e741c078a38ab6529a54b7) feat: implement resource state service and client
* [`d04ec51`](https://github.com/talos-systems/os-runtime/commit/d04ec51da46abf20110d6a4d5acc250fa7810c17) feat: add common COSI resource protobuf, implement bridge with state
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>8 commits</summary>
<p>

* [`9a6cf6b`](https://github.com/talos-systems/pkgs/commit/9a6cf6b99e1b8c0ef49e5dba2ce7e0260212c30d) feat: build with Go 1.16.3
* [`60ce626`](https://github.com/talos-systems/pkgs/commit/60ce6260e3956566d40ef77e2194c31c18c92d10) feat: update Linux to 5.10.27, enable 32-bit time syscalls
* [`fdf4866`](https://github.com/talos-systems/pkgs/commit/fdf48667851b4c80b0ca220c574d2fb57a943f64) feat: bump tools for Go 1.16.2
* [`35f9b6f`](https://github.com/talos-systems/pkgs/commit/35f9b6f22bbe094e93723559132b2a23f0853c2b) feat: update kernel to 5.10.23
* [`dbae83e`](https://github.com/talos-systems/pkgs/commit/dbae83e704da264066ceeca20e0fe66883b542ba) fix: do not use git-lfs for rockpi4 binaries
* [`1c6b9a3`](https://github.com/talos-systems/pkgs/commit/1c6b9a3a6ef91bce4f0cba18c466a9ece7b14750) feat: bump tools for Go 1.16.1
* [`c18073f`](https://github.com/talos-systems/pkgs/commit/c18073fe79b9d7ec36411c6f329fa60c580d4cea) feat: add u-boot for Rock Pi 4
* [`6b85a2b`](https://github.com/talos-systems/pkgs/commit/6b85a2bffbb144f25356eed6ed9dc8bb9a3fd392) feat: upgrade u-boot to 2021.04-rc3
</p>
</details>

### Changes from talos-systems/tools
<details><summary>5 commits</summary>
<p>

* [`1f26def`](https://github.com/talos-systems/tools/commit/1f26def38066c41fdb5c4bfe85559a87aa832c51) feat: update Go to 1.16.3
* [`41b8073`](https://github.com/talos-systems/tools/commit/41b807369779606f54d76e56038bfaf88d4f0f25) feat: bump protobuf-related tools
* [`f7bce92`](https://github.com/talos-systems/tools/commit/f7bce92febdf9f58f2940952d5138494b9232ea8) chore: bump Go to 1.16.2
* [`bcf3380`](https://github.com/talos-systems/tools/commit/bcf3380dd55810e556851acbe20e20cb4ddd5ef0) feat: bump protobuf deps, add protoc-gen-go-grpc
* [`b49c40e`](https://github.com/talos-systems/tools/commit/b49c40e0ad701f13192c1ad85ec616224343dc3f) feat: bump Go to 1.16.1
</p>
</details>

### Dependency Changes

* **github.com/coreos/go-semver**              v0.3.0 **_new_**
* **github.com/golang/protobuf**               v1.4.3 -> v1.5.2
* **github.com/google/go-cmp**                 v0.5.4 -> v0.5.5
* **github.com/hashicorp/go-multierror**       v1.1.0 -> v1.1.1
* **github.com/talos-systems/extras**          v0.2.0-1-g0db3328 -> v0.3.0-alpha.0-2-gcf3934a
* **github.com/talos-systems/go-blockdevice**  bb3ad73f6983 -> 1d830a25f64f
* **github.com/talos-systems/os-runtime**      7b3d14457439 -> 86d9e090bdc4
* **github.com/talos-systems/pkgs**            v0.4.1-2-gd471b60 -> v0.5.0-alpha.0-5-g9a6cf6b
* **github.com/talos-systems/tools**           v0.4.0-1-g3b25a7e -> v0.5.0-alpha.0-4-g1f26def
* **go.etcd.io/etcd/etcdctl/v3**               v3.5.0-alpha.0 **_new_**
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


### Optimizations

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
