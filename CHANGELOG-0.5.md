# [v0.5.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.8...v0.5.0-alpha.0) (2020-04-08)

### Bug Fixes

- add bnx2 and bnx2x firmware ([3a89d79](https://github.com/talos-systems/talos/commit/3a89d79f842fc52efb8a097f854b486afa2042e6))
- delete tag on revert with empty label ([83d0851](https://github.com/talos-systems/talos/commit/83d08515632c2df9f26258c772eee25668b075aa))
- don't use ARP table for networkd health check ([c144484](https://github.com/talos-systems/talos/commit/c144484a4420b397518b906ff0e1e0c363371ca9))
- ignore EINVAL on unmounting when mount point isn't mounted ([f18b573](https://github.com/talos-systems/talos/commit/f18b5737d8149ab186806c31f8180d8047b29c00))
- make sure Close() is called on every path ([5255883](https://github.com/talos-systems/talos/commit/5255883034a046a03dd45868744571d7ab52647f))
- make upgrades work with UEFI ([6fe5fed](https://github.com/talos-systems/talos/commit/6fe5fed6f933039937aeb9ec715210ca44b68a0a))
- mount TLS certs into bootkube container ([7c03497](https://github.com/talos-systems/talos/commit/7c034972c5d0ab1b128dfcc47a6f94a85d2d28e6))
- move empty label check ([47327ec](https://github.com/talos-systems/talos/commit/47327eca0986ff707ba06d78edd4c442c921dcee))
- wait for `system-containerd` to become healthy before proceeding ([314edf6](https://github.com/talos-systems/talos/commit/314edf63f4cf6922aa5bd1004d3fc1f2c1d1c6db))
- wait for USB storage ([6629fcf](https://github.com/talos-systems/talos/commit/6629fcf74882c0746605a763d710fb54fa5d46e1))

### Features

- add BNX drivers ([675a0ee](https://github.com/talos-systems/talos/commit/675a0eea0e0ed80b39a14323f9bb974079dd50e4))
- allow for exposing ports on docker clusters ([b84d5e2](https://github.com/talos-systems/talos/commit/b84d5e2660ae51623fc15e346e5ed33a4b405842))
- introduce ability to specify extra hosts in /etc/hosts ([38609bf](https://github.com/talos-systems/talos/commit/38609bf58131c86713937943416ec1d96e4f36ac))
- make `--wait` default option to `talosctl cluster create` ([104af43](https://github.com/talos-systems/talos/commit/104af4380e60dac09f701c3788d3d0c22057f748))
- move bootkube out as full service ([2294a65](https://github.com/talos-systems/talos/commit/2294a65972f2a18d32554c6aa871a317401728c7))
- upgrade kubernetes to 1.18 ([3a4eaee](https://github.com/talos-systems/talos/commit/3a4eaeeef06434c698d40ceed39f442106dd6cec))
- upgrade Linux to v5.5.15 ([681b1a8](https://github.com/talos-systems/talos/commit/681b1a8cb24209ea176893358be35ddf650eedef))
