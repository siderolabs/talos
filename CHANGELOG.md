## [Talos 0.9.0-beta.1](https://github.com/talos-systems/talos/releases/tag/v0.9.0-beta.1) (2021-03-17)

Welcome to the v0.9.0-beta.1 release of Talos!  
*This is a pre-release of Talos*



Please try out the release binaries and report any issues at
https://github.com/talos-systems/talos/issues.

### New Features

* Control Plane as Static Pods
* ECDSA Keys for Kubernetes PKI
* Disk Encryption
* Virtual Shared IP for Control Plane Endpoint

More in the [docs](https://www.talos.dev/docs/v0.9/introduction/what-is-new/).


### Upgrading

Please read [upgrade notes](https://www.talos.dev/docs/v0.9/guides/upgrading-talos/#upgrading-from-talos-08) before upgrading from Talos 0.8.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Alexey Palazhchenko
* Andrew Rynhard
* Spencer Smith
* Seán C McCord
* Andrew Rynhard
* Brandon McNama
* Guilhem Lettron
* Willem Monsuwe
* vlad doster

### Changes
<details><summary>182 commits</summary>
<p>

* [`9d360536`](https://github.com/talos-systems/talos/commit/9d36053616a9976f82d90158cb834aa66ae9545b) fix: talosctl health should not check kube-proxy when it is disabled
* [`3844103d`](https://github.com/talos-systems/talos/commit/3844103d110f1a50028fee3264c308e8145e7b64) test: update aws cloud provider version
* [`5bf28b8c`](https://github.com/talos-systems/talos/commit/5bf28b8c811135241713c4d4da9f52eefb13904f) fix: properly format spec comments in the resources
* [`6d7b0efc`](https://github.com/talos-systems/talos/commit/6d7b0efc6083cc458f406b9d3a62e42aef70a2f0) fix: don't touch any partitions on upgrade with --preserve
* [`aaa19e1e`](https://github.com/talos-systems/talos/commit/aaa19e1edd1090cfa889e27f4986117891e221a8) chore: update Linux to 5.10.23
* [`96477d24`](https://github.com/talos-systems/talos/commit/96477d24920e35d27b076d1874729a4faf5b4737) chore: fix provision tests after changes to build-container
* [`67e0317b`](https://github.com/talos-systems/talos/commit/67e0317b9d0ad5cbe167e150b9ea61e97f47bf15) fix: update output of `convert-k8s` command
* [`51f59f43`](https://github.com/talos-systems/talos/commit/51f59f435192e9d1ec976121a0ddd3cc8a1a4416) fix: move containerd CRI config files under `/var/`
* [`96521a18`](https://github.com/talos-systems/talos/commit/96521a186d90a055f92b61d5866d5b574174d254) chore: update Go to 1.15.9
* [`dbcb643e`](https://github.com/talos-systems/talos/commit/dbcb643e8101e4616d8b9cee755af9975e0cd8fe) release(v0.9.0-beta.0): prepare release
* [`3863be9c`](https://github.com/talos-systems/talos/commit/3863be9ce390b7ca860845af59f64b7795e73ff1) chore: bump release scope to v0.9
* [`d3798cd7`](https://github.com/talos-systems/talos/commit/d3798cd7a87c236ed70a8b0d1543a9237eee933e) docs: document controller runtime, resources and talosctl get
* [`c2e353d6`](https://github.com/talos-systems/talos/commit/c2e353d6afa38d8a3af533b10c9b4bdf0ef3412d) fix: do not print out help string if the parameters are correct
* [`56c95eac`](https://github.com/talos-systems/talos/commit/56c95eace33774bf475b54ee32a8327b618edfce) chore: bump dependencies via dependabot
* [`49853fc2`](https://github.com/talos-systems/talos/commit/49853fc2ecb846898d66d90fc76e6d875b775901) fix: mkdir source of the extra mounts for the kubelet
* [`e8e91d64`](https://github.com/talos-systems/talos/commit/e8e91d6434968bbcc52e832a4eb4ee87de09e228) fix: properly propagate nameservers to provisioned docker clusters
* [`f4ca6e9a`](https://github.com/talos-systems/talos/commit/f4ca6e9a6e134c8069b230fcefd274aba4e04a1e) feat: update containerd to version 1.4.4
* [`3084a3f3`](https://github.com/talos-systems/talos/commit/3084a3f35b6838a644659a5a7f0bcacb2f6c94f1) chore: update tools/pkgs/extras tags
* [`81acadf3`](https://github.com/talos-systems/talos/commit/81acadf345d00a30f26cdc979dd06e7dd0086c7c) fix: ignore connection refused errors when updating/converting cp
* [`db3785b9`](https://github.com/talos-systems/talos/commit/db3785b9301b1ce7772ea90eace093f13ae45db7) fix: align partition start to the physical sector size
* [`df52c135`](https://github.com/talos-systems/talos/commit/df52c135817639b9408ac34b81781ce8a6dcb1b5) chore: fix //nolint directives
* [`f3a32fff`](https://github.com/talos-systems/talos/commit/f3a32fff996a22790637d1288d9c5aa0ebd10ae9) chore: expire objects in CI S3 bucket
* [`7e8f1365`](https://github.com/talos-systems/talos/commit/7e8f13652ce57797252804891952049c66c43f6e) chore: fix upgrade tests by bumping 0.9 to alpha.5
* [`044fb770`](https://github.com/talos-systems/talos/commit/044fb7708cc7786e8620403e14f680b47a8e6907) fix: chmod etcd PKI path to fix virtual IP for upgrades with persistence
* [`ec72ae89`](https://github.com/talos-systems/talos/commit/ec72ae892b9b49d55b400f776c8208dd0592dfda) release(v0.9.0-alpha.5): prepare release
* [`4e47f676`](https://github.com/talos-systems/talos/commit/4e47f6766ed3703a33a721d35f09ada73b8fa715) feat: bypass lock if ACPI reboot/shutdown issued
* [`60b7f79f`](https://github.com/talos-systems/talos/commit/60b7f79fd81b19e023266b46961dbac8cb6ce4a1) feat: add `--on-reboot` flag to talosctl edit/patch machineConfig
* [`49a23bbd`](https://github.com/talos-systems/talos/commit/49a23bbde8111e2b17e03133d4bc97cfe1244560) chore: bump Go module dependencies
* [`40a2e4d4`](https://github.com/talos-systems/talos/commit/40a2e4d4fa140b53c315ae6c51864acd34836b2c) feat: support JSON output in `talosctl get`, event types
* [`638af35d`](https://github.com/talos-systems/talos/commit/638af35db0433c52c8325f5c2fcfadc0462109d6) chore: properly propagate context object in the controller
* [`60aa011c`](https://github.com/talos-systems/talos/commit/60aa011c7abf98a7f3883397ee53b92863feb10e) feat: rename namespaces, resources, types etc
* [`3a2caca7`](https://github.com/talos-systems/talos/commit/3a2caca7817e62088f538ea571598ad972c255da) release(v0.9.0-alpha.4): prepare release
* [`8ffb5594`](https://github.com/talos-systems/talos/commit/8ffb55943c71a100c0b1fd53c5520b2cf3ec72b8) fix: ignore 'ENOENT' (no such file directory) on mount
* [`a241e9ee`](https://github.com/talos-systems/talos/commit/a241e9ee473a741d045960954f4b8007917eeba6) feat: update linux kernel to 5.10.19
* [`561f8aa1`](https://github.com/talos-systems/talos/commit/561f8aa15eb47f5a7f329ede2190748ca4ee8ee3) fix: move etcd to `cri` containerd runner
* [`1d8ed9b5`](https://github.com/talos-systems/talos/commit/1d8ed9b5cd54625b34cb345c0dc8fe134bfbb35c) chore: update provision/upgrade tests to 0.9.0-alpha.3
* [`02c0c25b`](https://github.com/talos-systems/talos/commit/02c0c25bad07c682e92520a4ec3197be1a8c7973) docs: bump v0.8 release version in the SBCs guides
* [`9333e2a6`](https://github.com/talos-systems/talos/commit/9333e2a600d300d0c6981fea87eba18141d8c321) docs: add disk encryption guide
* [`a12a5dd2`](https://github.com/talos-systems/talos/commit/a12a5dd2553fe4a8c0b092c2ec90835bf1001d39) release(v0.9.0-alpha.3): prepare release
* [`31e56e63`](https://github.com/talos-systems/talos/commit/31e56e63db24efba88a10d4b0c4190aeebbb125b) fix: update in-cluster kubeconfig validity to match other certs
* [`c2f7a4b6`](https://github.com/talos-systems/talos/commit/c2f7a4b6f883870d1c94621a4f88520916f7647f) fix: add ApplyDynamicConfig call in the apply-config --immediate mode
* [`376fdcf6`](https://github.com/talos-systems/talos/commit/376fdcf6cb7a1260b79849b0fbbb1e0bf8c2f73e) feat: implement etcd remove-member cli command
* [`c8ae0093`](https://github.com/talos-systems/talos/commit/c8ae00937e819199d3713fd5fe0fe5f25db6ea39) chore: bump dependencies via dependabot
* [`d173fd4c`](https://github.com/talos-systems/talos/commit/d173fd4c0194cf421ef715ac0fcf09e4870f6e80) feat: update etcd to 3.4.15
* [`5ae315f4`](https://github.com/talos-systems/talos/commit/5ae315f493f6585b24cf2e55ef8ef009170c07ee) fix: set hdmi_safe=1 on Raspberry Pi for maximum HDMI compatibility
* [`61cb2fb2`](https://github.com/talos-systems/talos/commit/61cb2fb25c42a5e8f260adf3df4a989da9045ddd) feat: talosctl: allow v-prefixed k8s versions
* [`c7ee2390`](https://github.com/talos-systems/talos/commit/c7ee2390877ef40883384ec6540bacc2dd9bd709) fix: show stopped/exited containers via CRI inspector
* [`d7cdc8cc`](https://github.com/talos-systems/talos/commit/d7cdc8cc15e3c38557c10ae1179681c1e7596897) feat: implement simple layer 2 shared IP for CP
* [`63160277`](https://github.com/talos-systems/talos/commit/63160277d6fbcd5a262239e99d6f4512fd4941b8) fix: make ApplyDynamicConfig idempotent
* [`041620c8`](https://github.com/talos-systems/talos/commit/041620c8520ca9f9603b95717bc9a073eaf954ca) feat: implement talosctl edit and patch config commands
* [`c29cfaa0`](https://github.com/talos-systems/talos/commit/c29cfaa09b6c25a3a80b3e226dbcff265d6c8934) chore: build both Darwin and Linux versions of talosctl
* [`953ce643`](https://github.com/talos-systems/talos/commit/953ce643ab935aeb1de449e2a7607ac8a6d6ffe3) feat: bump etcd client library to 3.5.0-alpha.0
* [`24b4c0bc`](https://github.com/talos-systems/talos/commit/24b4c0bcb3d1865dd0eb5669ac592f3957f51b43) refactor: add context to the networkd
* [`9464c4cb`](https://github.com/talos-systems/talos/commit/9464c4cbcd47892cdb764754011996d7d4106981) refactor: split WithNetworkConfig into sub-options
* [`779ac74a`](https://github.com/talos-systems/talos/commit/779ac74a08ae1384875e1db0e98ff346ba24fd03) fix: improve the drain function
* [`f24c8153`](https://github.com/talos-systems/talos/commit/f24c815373c0e249c80186939574e62ccc8c82e7) fix: correctly set service state in the resource
* [`4e19b597`](https://github.com/talos-systems/talos/commit/4e19b597ab65b1c6df80eba5ebabde1642ae2777) test: add integration test with Canal CNI and reset API
* [`589d0189`](https://github.com/talos-systems/talos/commit/589d01892cb3e80dda92364495513eafe4b4f0fa) fix: update the layout of the Disks API to match proxying requirements
* [`7587af95`](https://github.com/talos-systems/talos/commit/7587af9585c14d9dec040afbdcacc6c1283f28a4) docs: update AMI images for 0.8.4
* [`7108bb3f`](https://github.com/talos-systems/talos/commit/7108bb3f5b402d4a82b263aa9a053bcbe8402e24) test: upgrade master to master tests
* [`09369fed`](https://github.com/talos-systems/talos/commit/09369fedba9535cd7105bc2e2b934063a807f47f) fix: stop and clean up installer container correctly
* [`85d1669f`](https://github.com/talos-systems/talos/commit/85d1669fb009ab906eb5ea883b66abd06537c59b) chore: bump dependencies via dependabot
* [`84ad6cbb`](https://github.com/talos-systems/talos/commit/84ad6cbb1a1b247381a5d6a272e76ecbcec88992) chore: switch CI to stop embedding local registry into the builds
* [`1a491ee8`](https://github.com/talos-systems/talos/commit/1a491ee85e20469fefa42a0b29cdb29b2a03c1df) fix: sanitize volume name better in static pod extra volumes
* [`5aa75e02`](https://github.com/talos-systems/talos/commit/5aa75e020e5ade2c2ae77222fdbf4f344b98381b) release(v0.9.0-alpha.2): prepare release
* [`3b672d34`](https://github.com/talos-systems/talos/commit/3b672d342dad60cdc4e9c85addc452792f391841) feat: u-boot 2021.01, ca-certificates update, Linux file ACLs
* [`e355d4fa`](https://github.com/talos-systems/talos/commit/e355d4faedeaa3248c37e57de28754a93e50dd55) fix: redirect warnings in manifest apply k8s client
* [`c37f2c6d`](https://github.com/talos-systems/talos/commit/c37f2c6d367813d9fe571813174dcffd7a477940) docs: add link to GitHub Discussions as a support forum
* [`e2f1fbcf`](https://github.com/talos-systems/talos/commit/e2f1fbcfdbb0b0a076fd07df13488df085362263) feat: support control plane upgrades with Talos managed control plane
* [`8789849c`](https://github.com/talos-systems/talos/commit/8789849c70844caf5d0fba6d77a9e586112eac3c) feat: add support for extra volume mounts for control plane pods
* [`06b8c094`](https://github.com/talos-systems/talos/commit/06b8c094847d7f70e3bd3e6a9e845a9d9845afe5) test: enable disk encryption key rotation test
* [`41430e72`](https://github.com/talos-systems/talos/commit/41430e72d22f1e9828ad5704b6ef0a6b1be99ce1) fix: handle case when kubelet serving certificates are issued
* [`7a6e0cd3`](https://github.com/talos-systems/talos/commit/7a6e0cd3e51750821d4647de2bedd544dc127dff) fix: correctly escape extra args in kube-proxy manifest
* [`41b9f134`](https://github.com/talos-systems/talos/commit/41b9f134523bb5c17fe84576cd1af27ed5b98c6a) feat: add a warning to boot log if running self-hosted control plane
* [`2b76c489`](https://github.com/talos-systems/talos/commit/2b76c4890f704bea5e9cf9ed7214292daee9cb1e) feat: add an option to disable kube-proxy manifest
* [`d2d5c72b`](https://github.com/talos-systems/talos/commit/d2d5c72bb5454bcb09149e0ffe7e3d844aa98a2d) fix: skip empty manifest YAML sub-documents
* [`e9fc54f6`](https://github.com/talos-systems/talos/commit/e9fc54f6e316ec850fedbe60b8440e059deab755) feat: update Kubernetes to 1.20.3
* [`b9143981`](https://github.com/talos-systems/talos/commit/b914398154e21d41bf301abde0272e49cde331f4) refactor: split kubernetes/etcd resource generation into subresources
* [`c2d10963`](https://github.com/talos-systems/talos/commit/c2d109637ba9e64b490e95f86f9aca83ee6a8c9e) chore: add default cron pipeline to the list of pipelines
* [`ce6bfbdb`](https://github.com/talos-systems/talos/commit/ce6bfbdbb7cbf7a12f9f185ad3925c16d20adc97) chore: run default pipeline as part of the `cron` pipeline
* [`32d25885`](https://github.com/talos-systems/talos/commit/32d25885288f0a5acea5fcc9a6f1afc4fea973f7) test: update integration tests to use wrapped client for etcd APIs
* [`54d6a452`](https://github.com/talos-systems/talos/commit/54d6a452178fc4acaecf4764d6130f632d75e447) feat: add state encryption support
* [`8e35560b`](https://github.com/talos-systems/talos/commit/8e35560baae0b281ca4082e9fd89aee4528e93d9) release(v0.9.0-alpha.1): prepare release
* [`7751920d`](https://github.com/talos-systems/talos/commit/7751920dbacee1a0d84fb721f001ae79c8d50587) feat: add a tool and package to convert self-hosted CP to static pods
* [`3a78bfce`](https://github.com/talos-systems/talos/commit/3a78bfcecdb4deff7bcc2738eb0152ff06fd5ee2) test: trigger e2e on thrice daily
* [`58ff2c98`](https://github.com/talos-systems/talos/commit/58ff2c9808e49c99503e977295da152ac7889430) feat: implement ephemeral partition encryption
* [`e5bd35ae`](https://github.com/talos-systems/talos/commit/e5bd35ae3c86c491cc1548587ed9adaedc900dd2) feat: add resource watch API + CLI
* [`6207fa51`](https://github.com/talos-systems/talos/commit/6207fa517bd4985857bda9ce1f52548f32edb692) test: update aws templates
* [`cc83b838`](https://github.com/talos-systems/talos/commit/cc83b8380825d709c9f1ceeba1ff39eb78f14e89) feat: rename apply-config --no-reboot to --on-reboot
* [`254e0e91`](https://github.com/talos-systems/talos/commit/254e0e91e1b05c35878c39fc2eddde8002088609) fix: correctly unwrap responses for etcd commands
* [`292bc396`](https://github.com/talos-systems/talos/commit/292bc396817328d6212e190e39e13f9c814c42b9) chore(ci): fix schedules in Drone pipelines
* [`02b3719d`](https://github.com/talos-systems/talos/commit/02b3719df9a499e49a66f8bf30d85cbd3cca4e81) feat: skip filesystem for state and ephemeral partitions in the installer
* [`edbaa0bc`](https://github.com/talos-systems/talos/commit/edbaa0bc728ddedc95f063b75da5a2497b9dabaf) chore: update artifacts bucket name in Drone
* [`f1d1f72b`](https://github.com/talos-systems/talos/commit/f1d1f72b5833bbefe0e0452348a1ecc467254921) chore(ci): update gcp templates
* [`162d8b6b`](https://github.com/talos-systems/talos/commit/162d8b6bef5fc155a7f337371ca1358c36c4ab89) fix: drop cri dependency on etcd
* [`b315a7e1`](https://github.com/talos-systems/talos/commit/b315a7e1f8c313b1d0069c94b8fc573e347676fa) chore: rework Drone pipelines
* [`9205870e`](https://github.com/talos-systems/talos/commit/9205870ee6949196d4043912be2b1c8a0efe3246) fix: move versions to annotations in control plane static pods
* [`ecd0921d`](https://github.com/talos-systems/talos/commit/ecd0921d7d357244609ef11b7724137919e1aef0) feat: stop all pods before unmounting ephemeral partition
* [`aa9bef27`](https://github.com/talos-systems/talos/commit/aa9bef2785716b927242e78a062a3be69fd32338) feat: bump Go to 1.15.8
* [`f96548e1`](https://github.com/talos-systems/talos/commit/f96548e165e12ce4c9d53749be0e5485b2621593) refactor: extract go-cmd into a separate library
* [`8d7a36cc`](https://github.com/talos-systems/talos/commit/8d7a36cc0cc22cb26cb3bbbe656a3ec5e33b87fb) fix: find master node IPs correctly in health checks
* [`6791036c`](https://github.com/talos-systems/talos/commit/6791036cfa94566f0f947f95effa8a43ddfd0f92) fix: add 3 seconds grub boot timeout
* [`ffe34ec1`](https://github.com/talos-systems/talos/commit/ffe34ec100b1a2e1969f15c0dc3c39e5e75ace2e) fix: don't use filename from URL when downloading manifest
* [`1111edfc`](https://github.com/talos-systems/talos/commit/1111edfc7681f2634d43c061a0f9f5bcfe56db4e) fix: pass attributes when adding routes
* [`d99a016a`](https://github.com/talos-systems/talos/commit/d99a016af2382e6ba22877c2dcc87af610c0c1f3) fix: correct response structure for GenerateConfig API
* [`df009903`](https://github.com/talos-systems/talos/commit/df0099036c4f47ef262d846dbe7db9ecdd16ead3) fix: correctly extract wrapped error messages
* [`1a32d55e`](https://github.com/talos-systems/talos/commit/1a32d55e4053045b70922b40dd6f0c54770118df) fix: prevent crash in machined on apid service stop
* [`daea9d38`](https://github.com/talos-systems/talos/commit/daea9d3811d9267d9531621581846b20af1ce84c) feat: support version contract for Talos config generation
* [`f9896777`](https://github.com/talos-systems/talos/commit/f9896777fcede79f8e4db6cc7f299caa543e9d3c) feat: update Linux to 5.10.14
* [`1908ba79`](https://github.com/talos-systems/talos/commit/1908ba79d3e474c134ee4fca80092a4047a584af) docs: update AMI list for 0.8.2
* [`7f3dca8e`](https://github.com/talos-systems/talos/commit/7f3dca8e4cedd8d584f3a057fac739bea45cfb01) test: add support for IPv6 in talosctl cluster create
* [`3aaa888f`](https://github.com/talos-systems/talos/commit/3aaa888f9a91b84446db3b1fc2f57cfeae67968e) docs: fix typos
* [`edf57772`](https://github.com/talos-systems/talos/commit/edf57772224c25f1f2b07d8c1e0e12d688151ebc) feat: add an option to force upgrade without checks
* [`85ae9f75`](https://github.com/talos-systems/talos/commit/85ae9f75e91f7ac557ad1cef1ae9e49919decd8f) fix: wait for time sync before generating Kubernetes certificates
* [`b526c2cc`](https://github.com/talos-systems/talos/commit/b526c2cc33bc5cf9adfcbe6ad994e6391d0a1869) fix: set proper hostname on docker nodes
* [`a07cfbd5`](https://github.com/talos-systems/talos/commit/a07cfbd5a42318be189fd7a6c0fb1ab1707528dd) fix: mount kubelet secrets from system instead of ephemeral
* [`4734fe7d`](https://github.com/talos-systems/talos/commit/4734fe7dd3f918acce1f138ced5241ec171519cd) feat: upgrade CoreDNS to 1.8.0
* [`d29a56b0`](https://github.com/talos-systems/talos/commit/d29a56b0c098b09ea8cb143db302e09eddac34be) chore: update dependencies via dependabot
* [`33de89ef`](https://github.com/talos-systems/talos/commit/33de89ef90bd2c26014dfeea999eaf49b4c99733) fix: allow loading of empty config files
* [`757cc204`](https://github.com/talos-systems/talos/commit/757cc204ecc434736d584441f45d2571f2f342ef) fix: prefer configured nameservers, fix DHCP6 in container
* [`6cf98a73`](https://github.com/talos-systems/talos/commit/6cf98a7322bf71c1a6aab139cce37a17d9e56fb9) feat: implement IPv6 DHCP client in networkd
* [`5855b8d5`](https://github.com/talos-systems/talos/commit/5855b8d532def16b5bc49fa0c692d5c2fc8cc3f4) fix: refresh control plane endpoints on worker apids on schedule
* [`47c260e3`](https://github.com/talos-systems/talos/commit/47c260e365a3da294761eabc2a4611670228f2f3) fix: update DHCP client to use Request-Ack sequence after an Offer
* [`42cadf5c`](https://github.com/talos-systems/talos/commit/42cadf5c514e6c7523acb087df557c6c3a03187f) release(v0.9.0-alpha.0): prepare release
* [`2277ce8a`](https://github.com/talos-systems/talos/commit/2277ce8abe234678ccceb33ced4b711966bab7ae) feat: move to ECDSA keys for all Kubernetes/etcd certs and keys
* [`9947ec84`](https://github.com/talos-systems/talos/commit/9947ec84d70b477e9173447bad59fce029f22fa4) fix: use hugetlbfs instead of none
* [`389349c0`](https://github.com/talos-systems/talos/commit/389349c02bf38ca4d8eca9a30aaf703d707db9d6) fix: use grpc load-balancing when connecting to trustd
* [`6eafca03`](https://github.com/talos-systems/talos/commit/6eafca037de61ed600a52b0c1b54f733645dd839) feat: update kernel
* [`b441915c`](https://github.com/talos-systems/talos/commit/b441915c0c8d35ec6147e919e71cd623f7101786) feat: mount hugetlbfs
* [`e4e6da38`](https://github.com/talos-systems/talos/commit/e4e6da38818dd6dd110c8b95321506aad06f8d0d) feat: allow fqdn to be used when registering k8s node
* [`87ccf0eb`](https://github.com/talos-systems/talos/commit/87ccf0eb21f310800de77de871c9e506ca12a885) test: clear connection refused errors after reset
* [`c36e4a93`](https://github.com/talos-systems/talos/commit/c36e4a935536b37a50fe4bdcaa03371beae90023) feat: copy cryptsetup executable from pkgs
* [`8974b529`](https://github.com/talos-systems/talos/commit/8974b529af89b87299b3a0ebf7ef96cac54dd850) chore: bump dependencies (via dependabot)
* [`512c79e8`](https://github.com/talos-systems/talos/commit/512c79e8d646f38699f7cb69e99d7a1643d86f8a) fix: lower memory usage a bit by disabling memory profiling
* [`1cded4d3`](https://github.com/talos-systems/talos/commit/1cded4d33ee5506ce7241ba828dcbbb550c88190) chore: fix import path for fsnotify
* [`698fdd9d`](https://github.com/talos-systems/talos/commit/698fdd9d610d89650867713685eb827dd1236fb5) chore: add dependabot config
* [`064d3322`](https://github.com/talos-systems/talos/commit/064d33229879165a73656dcf59d50e385c814bfb) fix: don't probe disks in container mode
* [`1051d2ab`](https://github.com/talos-systems/talos/commit/1051d2ab654c70a66c1370dd093e3a53a6a1128a) fix: prefix rendered Talos-owned static pod manifests
* [`7be3a860`](https://github.com/talos-systems/talos/commit/7be3a860917323d7f5986c04d61c9b8681731186) fix: bump timeout for worker apid waiting for kubelet client config
* [`76a67944`](https://github.com/talos-systems/talos/commit/76a6794436c072150f27b7ab0a45ae738a1e8bd8) fix: kill all processes and umount all disk on reboot/shutdown
* [`18db20db`](https://github.com/talos-systems/talos/commit/18db20dbc2318647da2639b548a0101a85268420) fix: open blockdevices with exclusive flock for partitioning
* [`e0a0f588`](https://github.com/talos-systems/talos/commit/e0a0f58801ce8a7c71e167431f66d1be792a50cc) feat: use multi-arch images for k8s and Flannel CNI
* [`a83af037`](https://github.com/talos-systems/talos/commit/a83af037305337b37afc59977575a0e66e757793) refactor: update go-blockdevice and restructure disk interaction code
* [`0aaf8fa9`](https://github.com/talos-systems/talos/commit/0aaf8fa968691d26c8b78af5ec3eb77d040d211d) feat: replace bootkube with Talos-managed control plane
* [`a2b6939c`](https://github.com/talos-systems/talos/commit/a2b6939c218346daf4d7c8487d2557e99f1515c6) docs: update components.md
* [`11863dd7`](https://github.com/talos-systems/talos/commit/11863dd74d5a2c86b39266276c5b3dd495dddf07) feat: implement resource API in Talos
* [`e9aa4947`](https://github.com/talos-systems/talos/commit/e9aa494775a5704212e3576978444f9a944c5e2c) feat: update Linux to 5.10.7, musl-libc to 1.2.2
* [`78eecc05`](https://github.com/talos-systems/talos/commit/78eecc0574b19b359e8381e66210a43b12c74e30) chore: enable virtio-balloon and monitor in QEMU provisioner
* [`d71ac4c4`](https://github.com/talos-systems/talos/commit/d71ac4c4ffaf4575a37358644cc8be14d8d1d4a5) feat: update Kubernetes to 1.20.2
* [`d515613b`](https://github.com/talos-systems/talos/commit/d515613bb7862f15ef68da57cdde7130624dbc03) fix: list command unlimited recursion default behavior
* [`9883d0af`](https://github.com/talos-systems/talos/commit/9883d0af1972c470fc0984e94db12066914463b7) feat: support Wireguard networking
* [`00d345fd`](https://github.com/talos-systems/talos/commit/00d345fd3afa08c9eb9b31843d5a8bd102b607da) docs: add v0.9 docs
* [`af5c34b3`](https://github.com/talos-systems/talos/commit/af5c34b340f1143abc848388eadad96834f16df5) fix: pick first interface valid hostname (vs. last one)
* [`275ca76c`](https://github.com/talos-systems/talos/commit/275ca76c5bbf8071a52899a59ca9452888c480f3) chore: update protobuf, grpc-go, prototool
* [`d19486af`](https://github.com/talos-systems/talos/commit/d19486afaa79cd8dbc223246ed8bb86a7dff9e12) fix: allow 'console' argument in kernel args to be always overridden
* [`47fb5720`](https://github.com/talos-systems/talos/commit/47fb5720cf125a2fb65b46d3d40047b8b81731fd) test: skip etcd tests on non-HA clusters
* [`529c0358`](https://github.com/talos-systems/talos/commit/529c03587f5b7ecc4f13765af2e8f409afcb690c) docs: add modes to validate command
* [`d455f917`](https://github.com/talos-systems/talos/commit/d455f917fb7b66bdb802ab8d50b23bc4f95280a8) docs: document omitting DiskPartition size
* [`5325a66e`](https://github.com/talos-systems/talos/commit/5325a66e3e3bc4ae648e38ff60bfbf9a261caeea) fix: bring up bonded interfaces correctly on packet
* [`a8dd2ff3`](https://github.com/talos-systems/talos/commit/a8dd2ff30d36b248eb5789b9c9dc67df62970ee8) fix: checkpoint controller-manager and scheduler
* [`f9ff4848`](https://github.com/talos-systems/talos/commit/f9ff4848e05e35ab7613f6e410b137a2ca139ce5) feat: bump pkgs for kernel with CONFIG_IPV6_MULTIPLE_TABLES
* [`f2c029a0`](https://github.com/talos-systems/talos/commit/f2c029a07df508ab16f0785ae738cdebb53106ed) chore: update upgrade test version used
* [`7b6c4bcb`](https://github.com/talos-systems/talos/commit/7b6c4bcb1f74618d331006fe4b31b0bc2b24aa92) refactor: define default kernel flags in machinery instead of procfs
* [`f3465b8e`](https://github.com/talos-systems/talos/commit/f3465b8e3e639b1df17713c0387ffe8e29f3ad50) feat: support type filter in list API and CLI
* [`5590fe19`](https://github.com/talos-systems/talos/commit/5590fe19ebae67c1ede0b1407a4b792cbdb7a8c2) docs: update references to 0.8.0, add 0.8.0 AWS AMIs
* [`11229a01`](https://github.com/talos-systems/talos/commit/11229a0180c58655a8ab6f63fd12bd2b3405da9b) docs: fix latest docs
* [`ff0749c4`](https://github.com/talos-systems/talos/commit/ff0749c4a709e389f5c9ddbfa91769310a86bc67) docs: set latest docs to v0.8
* [`6a0e652f`](https://github.com/talos-systems/talos/commit/6a0e652f0c0ca7ccee58b8e7eaf5c0553963a201) fix: correctly transport gRPC errors from apid
* [`47fb7d26`](https://github.com/talos-systems/talos/commit/47fb7d26e0a2887a524802788523ec5ff9ad3af6) fix: use SetAll instead of AppendAll when building kernel args
* [`b4ddfbfe`](https://github.com/talos-systems/talos/commit/b4ddfbfe9bec8d4fc7e8e68a112b62e583860f23) fix: add more dependencies for bootstrap services
* [`73c81c50`](https://github.com/talos-systems/talos/commit/73c81c501e239f06df23731df21d53125f570fa7) fix: pass disk image flags to e2e-qemu cluster create command
* [`5e3b8ee0`](https://github.com/talos-systems/talos/commit/5e3b8ee099f8b3d2d2d37e507b7eb3a064894cd7) fix: ignore pods spun up from checkpoints in health checks
* [`a83e8758`](https://github.com/talos-systems/talos/commit/a83e8758db66e1b45e69b888d9240ff94fb85c7f) feat: add commands to manage/query etcd cluster
* [`e75bb27c`](https://github.com/talos-systems/talos/commit/e75bb27cf4c3a828ac8a6e70603257dc075a49be) fix: leave etcd for staged upgrades
* [`f1964aab`](https://github.com/talos-systems/talos/commit/f1964aab5314f9d8e1106f68611aec666463dd30) fix: ignore errors on stopping/removing pod sandboxes
* [`6540e9bf`](https://github.com/talos-systems/talos/commit/6540e9bf70d5682ebe67682cd865b30a156f0665) feat: support disk image in talosctl cluster create
* [`b1d48143`](https://github.com/talos-systems/talos/commit/b1d4814308dd35d75b5c3a93f73f6faa0a673d77) feat: update Kubernetes to 1.20.1
* [`4f74b11d`](https://github.com/talos-systems/talos/commit/4f74b11db48031b3cc0502d30398d157550e6760) docs: provide AMIs for 0.8.0-beta.0
* [`14b43068`](https://github.com/talos-systems/talos/commit/14b43068d04673a143783fa0143d907b0d900c9b) docs: fix SBC docs to point to beta.0 instead of beta.1
* [`941556cf`](https://github.com/talos-systems/talos/commit/941556cffbcadcb4c3334d0da30c4e9077e91ab5) fix: use the correct console on Banana Pi M64
* [`e791e7dc`](https://github.com/talos-systems/talos/commit/e791e7dca95175671e058c9cc72e280b681b7bc1) fix: don't run LabelNodeAsMaster in two sequences
* [`a4f864d4`](https://github.com/talos-systems/talos/commit/a4f864d4694353300946d3f041e6195e37632917) docs: update Talos release for SBCs
</p>
</details>

### Changes since v0.9.0-beta.1
<details><summary>0 commit</summary>
<p>

</p>
</details>

### Changes from talos-systems/crypto
<details><summary>5 commits</summary>
<p>

* [`39584f1`](https://github.com/talos-systems/crypto/commit/39584f1b6e54e9966db1f16369092b2215707134) feat: support for key/certificate types RSA, Ed25519, ECDSA
* [`cf75519`](https://github.com/talos-systems/crypto/commit/cf75519cab82bd1b128ae9b45107c6bb422bd96a) fix: function NewKeyPair should create certificate with proper subject
* [`751c95a`](https://github.com/talos-systems/crypto/commit/751c95aa9434832a74deb6884cff7c5fd785db0b) feat: add 'PEMEncodedKey' which allows to transport keys in YAML
* [`562c3b6`](https://github.com/talos-systems/crypto/commit/562c3b66f89866746c0ba47927c55f41afed0f7f) feat: add support for public RSA key in RSAKey
* [`bda0e9c`](https://github.com/talos-systems/crypto/commit/bda0e9c24e80c658333822e2002e0bc671ac53a3) feat: enable more conversions between encoded and raw versions
</p>
</details>

### Changes from talos-systems/extras
<details><summary>5 commits</summary>
<p>

* [`0db3328`](https://github.com/talos-systems/extras/commit/0db33285dc672bf0f595ac37ac7e08b076345cd3) feat: bump Go to 1.15.9
* [`b852b69`](https://github.com/talos-systems/extras/commit/b852b69e24062a3450fbfba1753467b3c098f8d7) chore: bump tools and pkgs to 0.4.0
* [`302cc61`](https://github.com/talos-systems/extras/commit/302cc6176f94a6f32154f53d2dc931120a6f3603) feat: bump Go to 1.15.8
* [`3cb9fc9`](https://github.com/talos-systems/extras/commit/3cb9fc994162cd3a38d75fa9111b9d99ba807c4b) feat: build tc-redirect-tap from our fork
* [`cc8f5b9`](https://github.com/talos-systems/extras/commit/cc8f5b92dd3a51f9ebffcc3f2ad60a86a298b9e4) chore: bump tools for Go 1.15.7 update
</p>
</details>

### Changes from talos-systems/go-blockdevice
<details><summary>6 commits</summary>
<p>

* [`bb3ad73`](https://github.com/talos-systems/go-blockdevice/commit/bb3ad73f69836acc2785ec659435e24a531359e7) fix: align partition start to physical sector size
* [`8f976c2`](https://github.com/talos-systems/go-blockdevice/commit/8f976c2031108651738ebd4db69fb09758754a28) feat: replace exec.Command with go-cmd module
* [`1cf7f25`](https://github.com/talos-systems/go-blockdevice/commit/1cf7f252c38cf11ef07723de2debc27d1da6b520) fix: properly handle no child processes error from cmd.Wait
* [`04a9851`](https://github.com/talos-systems/go-blockdevice/commit/04a98510c07fe8477f598befbfe6eaec4f4b73a2) feat: implement luks encryption provider
* [`b0375e4`](https://github.com/talos-systems/go-blockdevice/commit/b0375e4267fdc6108bd9ff7a5dc97b80cd924b1d) feat: add an option to open block device with exclusive flock
* [`5a1c7f7`](https://github.com/talos-systems/go-blockdevice/commit/5a1c7f768e016c93f6c0be130ffeaf34109b5b4d) refactor: add devname into gpt.Partition, refactor probe package
</p>
</details>

### Changes from talos-systems/go-cmd
<details><summary>4 commits</summary>
<p>

* [`68eb006`](https://github.com/talos-systems/go-cmd/commit/68eb0067e0f0fa18db1eb91257764d5a7b69ab30) feat: return typed error for exit error
* [`333ccf1`](https://github.com/talos-systems/go-cmd/commit/333ccf125e0e8f36e4d67d05ea0f0e0f09827c73) feat: add stdin support into the Run methods
* [`c5c8f1c`](https://github.com/talos-systems/go-cmd/commit/c5c8f1c4f9d549b11fda70358ff21c9956c5f295) feat: extract cmd module from Talos into a separate module
* [`77685fc`](https://github.com/talos-systems/go-cmd/commit/77685fc53eb44020f11e2fc5451a86235231903b) Initial commit
</p>
</details>

### Changes from talos-systems/go-procfs
<details><summary>2 commits</summary>
<p>

* [`8cbc42d`](https://github.com/talos-systems/go-procfs/commit/8cbc42d3dc246a693d9b307c5358f6f7f3cb60bc) feat: provide an option to overwrite some args in AppendAll
* [`24d06a9`](https://github.com/talos-systems/go-procfs/commit/24d06a955782ed7d468f5117e986ec632f316310) refactor: remove talos kernel default args
</p>
</details>

### Changes from talos-systems/go-retry
<details><summary>1 commit</summary>
<p>

* [`b9dc1a9`](https://github.com/talos-systems/go-retry/commit/b9dc1a990133dd3399549b4ea199759bdfe58bb8) feat: add support for `context.Context` in Retry
</p>
</details>

### Changes from talos-systems/go-smbios
<details><summary>2 commits</summary>
<p>

* [`fb425d4`](https://github.com/talos-systems/go-smbios/commit/fb425d4727e620b6a2b6ba49e405a2c6f0e46304) feat: add memory device
* [`0bb4f96`](https://github.com/talos-systems/go-smbios/commit/0bb4f96a6679e8fc958903c4f451ca068f8e3c41) feat: add physical memory array
</p>
</details>

### Changes from talos-systems/net
<details><summary>3 commits</summary>
<p>

* [`0519054`](https://github.com/talos-systems/net/commit/05190541b0fafc44fc6f3a2f8ba98d9b4a7b527a) feat: add ParseCIDR
* [`52c7509`](https://github.com/talos-systems/net/commit/52c75099437634e312f54dd0941a44c626da9b66) feat: add a function to format IPs in CIDR notation
* [`005a94f`](https://github.com/talos-systems/net/commit/005a94f8b36b5dfd56873cb168af9efceb072eeb) feat: add methods to manage CIDR list, check for non-local IPv6
</p>
</details>

### Changes from talos-systems/os-runtime
<details><summary>13 commits</summary>
<p>

* [`7b3d144`](https://github.com/talos-systems/os-runtime/commit/7b3d14457439d4fc10928cd6332c867b4acbae45) feat: use go-yaml fork and serialize spec as RawYAML objects
* [`84c3c87`](https://github.com/talos-systems/os-runtime/commit/84c3c875eb2bf241465b8d2fe3a30abcc8a74807) chore: provide fmt.Stringer for EventType
* [`8b3f192`](https://github.com/talos-systems/os-runtime/commit/8b3f192ecca24b85ec8131081d246a3d3e7db6bf) feat: update naming conventions for resources and types
* [`28dd9aa`](https://github.com/talos-systems/os-runtime/commit/28dd9aaf98d60d57c2a25c6f2614ae762de60ead) feat: add an option to bootstrap WatchKind with initial list of resources
* [`734f1e1`](https://github.com/talos-systems/os-runtime/commit/734f1e1cee9e7424721e715eff28e2f0df7a6c4a) feat: add support for exporting dependency graph
* [`eb6e3df`](https://github.com/talos-systems/os-runtime/commit/eb6e3dfd68f82a32f1f51bf85d161bf2f1bfbc59) feat: sort resources returned from the List() API
* [`b8955a5`](https://github.com/talos-systems/os-runtime/commit/b8955a5475fe7b6c436757477c887ed7ef82eee7) fix: attach stack trace to panic error message
* [`b64f477`](https://github.com/talos-systems/os-runtime/commit/b64f4771a41ca92cd02246ec69f77e9a4d6ca673) feat: restart failing controllers automatically with exp backoff
* [`98acf0d`](https://github.com/talos-systems/os-runtime/commit/98acf0d2d3321a088e05f2d12c4c0ca00cbe3de0) fix: preserve original YAML formatting in resource.Any
* [`53fb919`](https://github.com/talos-systems/os-runtime/commit/53fb919b39e67c9980cc81591711b7dc4c9499ae) feat: controller runtime implementation
* [`f450ab7`](https://github.com/talos-systems/os-runtime/commit/f450ab759f4800f2ee8c65319f3bd323c41ec196) feat: implement namespaces, clean up context use
* [`81bf414`](https://github.com/talos-systems/os-runtime/commit/81bf4142e713ddb553cac5673ee55461d3b113f2) feat: initial version of the runtime based on the state
* [`657fda9`](https://github.com/talos-systems/os-runtime/commit/657fda9265f8b378528bfa4dc23b466ba9eb14c4) Initial commit
</p>
</details>

### Changes from talos-systems/pkgs
<details><summary>23 commits</summary>
<p>

* [`d471b60`](https://github.com/talos-systems/pkgs/commit/d471b608132843d0f3cd4a3e7b0d9dbd569c1db9) feat: update kernel to 5.10.23
* [`8e2a376`](https://github.com/talos-systems/pkgs/commit/8e2a376dd6c693f0075a592a882d58e7d0409a31) feat: bump tools for Go 1.15.9
* [`af19871`](https://github.com/talos-systems/pkgs/commit/af198710d5417c129e5ef8c90182332ec55d367a) feat: update containerd to 1.4.4
* [`a053811`](https://github.com/talos-systems/pkgs/commit/a0538119c96823840cb9d03a7337244cbe77b2a7) chore: bump tools to the tag 0.4.0
* [`04e6d12`](https://github.com/talos-systems/pkgs/commit/04e6d12f409bbe9c6c0563b6f96007e8f740f816) feat: update kernel to 5.10.19
* [`bf4b778`](https://github.com/talos-systems/pkgs/commit/bf4b7784ef87ef8003705a37b7bdf09484287910) feat: update u-boot to 2021.01
* [`c02be5f`](https://github.com/talos-systems/pkgs/commit/c02be5f30f9edc21386227cd94129f2331270d3e) feat: update ca-certificates to 2021-01-19
* [`be6d186`](https://github.com/talos-systems/pkgs/commit/be6d1863c3521938b7c3a505165fbd5345e31a55) feat: enable POSIX file ACLs on XFS
* [`6748819`](https://github.com/talos-systems/pkgs/commit/674881902e2bde5c4b44999b8a8099659df43b50) feat: update Linux to 5.10.17, disable init_on_free=1 by default
* [`c623457`](https://github.com/talos-systems/pkgs/commit/c623457305d0c12ff7ab91b66798937cbaeaa99f) feat: bump raspberrypi-firmware
* [`a0bb6ab`](https://github.com/talos-systems/pkgs/commit/a0bb6ab1da6b5ab80710ca85727e5e9c423a8d3f) feat: update Go to 1.15.8
* [`0368166`](https://github.com/talos-systems/pkgs/commit/0368166dae3f5093dee38eb857bfbef2bcf462c1) feat: update Linux to 5.10.14
* [`2a04697`](https://github.com/talos-systems/pkgs/commit/2a04697df5e29eeac70164b92d139c5cf767103e) chore: add conform configuration
* [`f9d9690`](https://github.com/talos-systems/pkgs/commit/f9d969027a119d34dda8dba28ca4517c45c913aa) feat: build CNI plugins, bump version to current master
* [`72c4450`](https://github.com/talos-systems/pkgs/commit/72c44501e30da127554954433f29cc79eb6fe355) chore: bump tools for Go 1.15.7 update
* [`4ce1f2c`](https://github.com/talos-systems/pkgs/commit/4ce1f2c8b95778f97ccbd02c6c9db6a410700985) feat: add cryptsetup dependencies to all targets
* [`3c35918`](https://github.com/talos-systems/pkgs/commit/3c35918b701528dd6baba4c50336041c8934af96) feat: enable NVME-over-TCP
* [`1380273`](https://github.com/talos-systems/pkgs/commit/138027300c8789faf29270a8f8edb2b38118173d) feat: enable hyperv_utils in Linux kernel
* [`0386ef5`](https://github.com/talos-systems/pkgs/commit/0386ef5ebb1a9e67f887976e2e6431172f226be9) feat: update libmusl to 1.2.2
* [`d02d119`](https://github.com/talos-systems/pkgs/commit/d02d119930cf79a15088f4960429048066c6bfa5) feat: update Linux kernel to 5.10.7
* [`db10362`](https://github.com/talos-systems/pkgs/commit/db10362c76c5d2680493ad85edefee986d4c0f72) feat: enable more VIRTIO options
* [`8e68598`](https://github.com/talos-systems/pkgs/commit/8e6859852d4efc4d01144eaf2383ffe67ab62b73) feat: enable CONFIG_WIREGUARD kernel option
* [`2409ba7`](https://github.com/talos-systems/pkgs/commit/2409ba7ff544df0c60dc6e9787aefd2b91ead9b6) feat: enable CONFIG_IPV6_MULTIPLE_TABLES option
</p>
</details>

### Changes from talos-systems/tools
<details><summary>11 commits</summary>
<p>

* [`3b25a7e`](https://github.com/talos-systems/tools/commit/3b25a7ec127f05e56eb6e62c1cceecbde0fc1b93) feat: bump Go to 1.15.9
* [`017d570`](https://github.com/talos-systems/tools/commit/017d570e51c7a7c27ce18cbf18e203e6b10172f1) chore: bump tools to 0.2.0
* [`4b418f3`](https://github.com/talos-systems/tools/commit/4b418f3f1b837f3807cc599c73382f13986b21dc) feat: upgrade Python 3.9.2, enable pip
* [`0026740`](https://github.com/talos-systems/tools/commit/0026740a711b2c5c22d39e257c21e079fbf1c5e8) feat: update Go to version 1.15.8
* [`ca12352`](https://github.com/talos-systems/tools/commit/ca1235203e51bb4d72c2106286e8d10a54268bc2) chore: make it easier to update deps.png
* [`e54841a`](https://github.com/talos-systems/tools/commit/e54841a2400d3cfac59dbdfdbee3ea186c0b55eb) feat: bump Go to 1.15.7
* [`5fa9459`](https://github.com/talos-systems/tools/commit/5fa9459a7423fe5f3d7f1d136c19d09763bae3c3) feat: bump rhash to 1.4.1
* [`24a6dac`](https://github.com/talos-systems/tools/commit/24a6dac5988e8f0c2b33626cfcf576529a975442) feat: bump toolchain for libmusl CVE-2020-28928 fix
* [`0fe682e`](https://github.com/talos-systems/tools/commit/0fe682e1f1ab03635cc7dfa6c62512663aec1b21) feat: switch to older protoc-gen-go with gRPC
* [`2fd95a7`](https://github.com/talos-systems/tools/commit/2fd95a7fa1d6f244cba3a89b9c0f401873230b51) feat: add protoc-gen-go-grpc
* [`4689294`](https://github.com/talos-systems/tools/commit/4689294a949dfd1112229b6320c57ae702aa5b63) feat: upgrade proto libraries
</p>
</details>

### Dependency Changes

* **github.com/AlekSi/pointer**                     v1.1.0 **_new_**
* **github.com/containerd/containerd**              v1.4.3 -> v1.4.4
* **github.com/containernetworking/cni**            v0.8.0 -> v0.8.1
* **github.com/containernetworking/plugins**        v0.8.7 -> v0.9.1
* **github.com/coreos/go-iptables**                 v0.4.5 -> v0.5.0
* **github.com/docker/docker**                      v1.13.1 -> v20.10.4
* **github.com/elazarl/goproxy**                    a92cc753f88e **_new_**
* **github.com/elazarl/goproxy/ext**                a92cc753f88e **_new_**
* **github.com/emicklei/dot**                       v0.15.0 **_new_**
* **github.com/emicklei/go-restful**                v2.15.0 **_new_**
* **github.com/evanphx/json-patch**                 v4.9.0 **_new_**
* **github.com/fsnotify/fsnotify**                  v1.4.9 **_new_**
* **github.com/gdamore/tcell/v2**                   acf90d56d591 -> v2.2.0
* **github.com/google/go-cmp**                      v0.5.4 **_new_**
* **github.com/google/uuid**                        v1.1.2 -> v1.2.0
* **github.com/hashicorp/go-getter**                v1.5.1 -> v1.5.2
* **github.com/insomniacslk/dhcp**                  4de412bc85d8 -> cc9239ac6294
* **github.com/jsimonetti/rtnetlink**               8bebea019a6c -> 1b79e63a70a0
* **github.com/mdlayher/netlink**                   v1.1.1 -> v1.4.0
* **github.com/morikuni/aec**                       v1.0.0 **_new_**
* **github.com/plunder-app/kube-vip**               v0.3.2 **_new_**
* **github.com/prometheus/procfs**                  v0.2.0 -> v0.6.0
* **github.com/rivo/tview**                         f007e9ad3893 -> 8a8f78a6dd01
* **github.com/spf13/cobra**                        v1.1.1 -> v1.1.3
* **github.com/stretchr/testify**                   v1.6.1 -> v1.7.0
* **github.com/talos-systems/crypto**               e0dd56ac4745 -> 39584f1b6e54
* **github.com/talos-systems/extras**               v0.1.0-6-gdc32cc8 -> v0.2.0-1-g0db3328
* **github.com/talos-systems/go-blockdevice**       f2728a581972 -> bb3ad73f6983
* **github.com/talos-systems/go-cmd**               68eb0067e0f0 **_new_**
* **github.com/talos-systems/go-procfs**            a82654edcec1 -> 8cbc42d3dc24
* **github.com/talos-systems/go-retry**             8c63d290a688 -> b9dc1a990133
* **github.com/talos-systems/go-smbios**            80196199691e -> fb425d4727e6
* **github.com/talos-systems/net**                  v0.2.0 -> 05190541b0fa
* **github.com/talos-systems/os-runtime**           7b3d14457439 **_new_**
* **github.com/talos-systems/pkgs**                 v0.3.0-59-g3f7a335 -> v0.4.1-2-gd471b60
* **github.com/talos-systems/talos/pkg/machinery**  6a7cc0264819 -> 8ffb55943c71
* **github.com/talos-systems/tools**                v0.3.0-13-g05b7372 -> v0.4.0-1-g3b25a7e
* **github.com/vmware-tanzu/sonobuoy**              v0.19.0 -> v0.20.0
* **go.etcd.io/etcd/api/v3**                        v3.5.0-alpha.0 **_new_**
* **go.etcd.io/etcd/client/v3**                     v3.5.0-alpha.0 **_new_**
* **go.etcd.io/etcd/pkg/v3**                        v3.5.0-alpha.0 **_new_**
* **golang.org/x/crypto**                           c8d3bf9c5392 -> 5ea612d1eb83
* **golang.org/x/net**                              69a78807bb2b -> e18ecbb05110
* **golang.org/x/sync**                             67f06af15bc9 -> 036812b2e83c
* **golang.org/x/sys**                              760e229fe7c5 -> 77cc2087c03b
* **golang.org/x/term**                             7de9c90e9dd1 -> 6a3ed077a48d
* **golang.org/x/time**                             3af7569d3a1e -> f8bda1e9f3ba
* **golang.zx2c4.com/wireguard/wgctrl**             bd2cb7843e1b **_new_**
* **google.golang.org/grpc**                        v1.29.1 -> v1.36.0
* **gopkg.in/yaml.v3**                              eeeca48fe776 -> 496545a6307b
* **honnef.co/go/tools**                            v0.1.2 **_new_**
* **k8s.io/api**                                    v0.20.1 -> v0.20.4
* **k8s.io/apiserver**                              v0.20.1 -> v0.20.4
* **k8s.io/client-go**                              v0.20.1 -> v0.20.4
* **k8s.io/kubectl**                                v0.20.4 **_new_**
* **k8s.io/kubelet**                                v0.20.1 -> v0.20.4

Previous release can be found at [v0.8.0](https://github.com/talos-systems/talos/releases/tag/v0.8.0)

<a name="v0.9.0-beta.0"></a>
## [v0.9.0-beta.0](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.5...v0.9.0-beta.0) (2021-03-09)

### Chore

* bump release scope to v0.9
* bump dependencies via dependabot
* update tools/pkgs/extras tags
* fix //nolint directives
* expire objects in CI S3 bucket
* fix upgrade tests by bumping 0.9 to alpha.5

### Docs

* document controller runtime, resources and talosctl get

### Feat

* update containerd to version 1.4.4

### Fix

* do not print out help string if the parameters are correct
* mkdir source of the extra mounts for the kubelet
* properly propagate nameservers to provisioned docker clusters
* ignore connection refused errors when updating/converting cp
* align partition start to the physical sector size
* chmod etcd PKI path to fix virtual IP for upgrades with persistence

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

### Release

* **v0.9.0-alpha.5:** prepare release

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

### Release

* **v0.9.0-alpha.4:** prepare release

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

### Release

* **v0.9.0-alpha.3:** prepare release

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

### Release

* **v0.9.0-alpha.2:** prepare release

### Test

* enable disk encryption key rotation test
* update integration tests to use wrapped client for etcd APIs

<a name="v0.9.0-alpha.1"></a>
## [v0.9.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.0...v0.9.0-alpha.1) (2021-02-18)

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

### Release

* **v0.9.0-alpha.1:** prepare release

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

### Release

* **v0.9.0-alpha.0:** prepare release

### Test

* clear connection refused errors after reset
* skip etcd tests on non-HA clusters
