# [v0.2.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.0...v0.2.0-alpha.1) (2019-07-05)


### Bug Fixes

* **init:** secret data at rest encryption key should be truly random ([#797](https://github.com/talos-systems/talos/issues/797)) ([6b0a66b](https://github.com/talos-systems/talos/commit/6b0a66b))
* append probed block devices ([2c6bf9b](https://github.com/talos-systems/talos/commit/2c6bf9b))
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794)) ([18f59d8](https://github.com/talos-systems/talos/commit/18f59d8))
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818)) ([cca60ed](https://github.com/talos-systems/talos/commit/cca60ed))
* use existing logic to perform reset ([5d8ee0a](https://github.com/talos-systems/talos/commit/5d8ee0a))


### Features

* **initramfs:** Add kernel arg for default interface ([c194621](https://github.com/talos-systems/talos/commit/c194621))
* **osd:** implement container metrics for CRI inspector ([#824](https://github.com/talos-systems/talos/issues/824)) ([5d91d76](https://github.com/talos-systems/talos/commit/5d91d76))
* **osd:** implement CRI inspector for containers ([#817](https://github.com/talos-systems/talos/issues/817)) ([237e903](https://github.com/talos-systems/talos/commit/237e903))



# [0.2.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.28...v0.2.0-alpha.0) (2019-06-27)


### Bug Fixes

* Add gitmeta as dependency for push ([#718](https://github.com/talos-systems/talos/issues/718)) ([8a5acff](https://github.com/talos-systems/talos/commit/8a5acff))
* containers test by locking image to specific tag ([#734](https://github.com/talos-systems/talos/issues/734)) ([89b876c](https://github.com/talos-systems/talos/commit/89b876c))
* ensure index remains in bounds for ud gen ([#710](https://github.com/talos-systems/talos/issues/710)) ([921114d](https://github.com/talos-systems/talos/commit/921114d))
* **init:** Add modules mountpoint for kube services ([#767](https://github.com/talos-systems/talos/issues/767)) ([d935ee0](https://github.com/talos-systems/talos/commit/d935ee0))
* **init:** fix leaky ticker ([#784](https://github.com/talos-systems/talos/issues/784)) ([4aaa7f6](https://github.com/talos-systems/talos/commit/4aaa7f6))
* **init:** use 127.0.0.1 IP in healthchecks to avoid resolver weirdness ([#715](https://github.com/talos-systems/talos/issues/715)) ([7a4a677](https://github.com/talos-systems/talos/commit/7a4a677))
* **osctl:** allow '-target' flag for `osctl restart` ([#732](https://github.com/talos-systems/talos/issues/732)) ([0c0a034](https://github.com/talos-systems/talos/commit/0c0a034))
* **osctl:** avoid panic on empty 'talosconfig' ([#725](https://github.com/talos-systems/talos/issues/725)) ([f5969d2](https://github.com/talos-systems/talos/commit/f5969d2))
* **osctl:** display non-fatal errors from ps/stats in osctl ([#724](https://github.com/talos-systems/talos/issues/724)) ([f200eb7](https://github.com/talos-systems/talos/commit/f200eb7))
* **osctl:** Revert "display non-fatal errors from ps/stats in osctl ([#724](https://github.com/talos-systems/talos/issues/724))" ([#727](https://github.com/talos-systems/talos/issues/727)) ([fb320a8](https://github.com/talos-systems/talos/commit/fb320a8))
* **proxyd:** Add support for dropping broken backends ([#790](https://github.com/talos-systems/talos/issues/790)) ([6a0684a](https://github.com/talos-systems/talos/commit/6a0684a))
* run basic-integration on nightly cron ([#735](https://github.com/talos-systems/talos/issues/735)) ([1178896](https://github.com/talos-systems/talos/commit/1178896))
* top-level docs now appear properly with sidebar ([#785](https://github.com/talos-systems/talos/issues/785)) ([19594b3](https://github.com/talos-systems/talos/commit/19594b3))
* update hack/dev for new userdata location ([#777](https://github.com/talos-systems/talos/issues/777)) ([0131f83](https://github.com/talos-systems/talos/commit/0131f83))
* we don't need no stinkin' localapiendpoint ([#741](https://github.com/talos-systems/talos/issues/741)) ([8a89ecd](https://github.com/talos-systems/talos/commit/8a89ecd))
* **proxyd:** Fix backend deletion ([#729](https://github.com/talos-systems/talos/issues/729)) ([c88b6fc](https://github.com/talos-systems/talos/commit/c88b6fc))
* **proxyd:** remove self-hosted label in listwatch ([#782](https://github.com/talos-systems/talos/issues/782)) ([007290a](https://github.com/talos-systems/talos/commit/007290a))
* **proxyd:** Use local apiserver endpoint ([#776](https://github.com/talos-systems/talos/issues/776)) ([acf975b](https://github.com/talos-systems/talos/commit/acf975b))


### Features

* **ci:** enable nightly e2e tests ([#716](https://github.com/talos-systems/talos/issues/716)) ([4ba12fe](https://github.com/talos-systems/talos/commit/4ba12fe))
* **init:** Add service stop api ([#708](https://github.com/talos-systems/talos/issues/708)) ([d68e303](https://github.com/talos-systems/talos/commit/d68e303))
* **init:** Add support for kubeadm reset during upgrade ([#714](https://github.com/talos-systems/talos/issues/714)) ([0d5f521](https://github.com/talos-systems/talos/commit/0d5f521))
* **init:** Add support for stopping individual services ([#706](https://github.com/talos-systems/talos/issues/706)) ([1a01440](https://github.com/talos-systems/talos/commit/1a01440))
* **init:** Implement 'ls' command ([#721](https://github.com/talos-systems/talos/issues/721)) ([532a53b](https://github.com/talos-systems/talos/commit/532a53b)), closes [#719](https://github.com/talos-systems/talos/issues/719)
* **init:** move 'ls' API to init from osd ([#755](https://github.com/talos-systems/talos/issues/755)) ([76071ab](https://github.com/talos-systems/talos/commit/76071ab)), closes [#752](https://github.com/talos-systems/talos/issues/752)
* **init:** unify filesystem walkers for `ls`/`cp` APIs ([#779](https://github.com/talos-systems/talos/issues/779)) ([6d5ee0c](https://github.com/talos-systems/talos/commit/6d5ee0c))
* add support for upgrading init nodes ([#761](https://github.com/talos-systems/talos/issues/761)) ([ebc725a](https://github.com/talos-systems/talos/commit/ebc725a))
* **osctl:** implement 'cp' to copy files out of the Talos node ([#740](https://github.com/talos-systems/talos/issues/740)) ([9ed45f7](https://github.com/talos-systems/talos/commit/9ed45f7))
* **osctl:** improve output of `stats` and `ps` commands ([#788](https://github.com/talos-systems/talos/issues/788)) ([17f28d3](https://github.com/talos-systems/talos/commit/17f28d3))
* **osd:** extend Routes API ([#756](https://github.com/talos-systems/talos/issues/756)) ([81163ce](https://github.com/talos-systems/talos/commit/81163ce))
* enable debug in udevd service ([#783](https://github.com/talos-systems/talos/issues/783)) ([fde6b4b](https://github.com/talos-systems/talos/commit/fde6b4b))
* use eudev for udevd ([#780](https://github.com/talos-systems/talos/issues/780)) ([85afe4f](https://github.com/talos-systems/talos/commit/85afe4f))


### Performance Improvements

* **proxyd:** filter listwatch and remove backend on non-running pod ([#781](https://github.com/talos-systems/talos/issues/781)) ([5f26992](https://github.com/talos-systems/talos/commit/5f26992))
