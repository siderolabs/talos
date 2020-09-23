// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package v1alpha1 configuration file contains all the options available for configuring a machine.

We can generate the files using `talosctl`.
This configuration is enough to get started in most cases, however it can be customized as needed.

```bash
talosctl config generate --version v1alpha1 <cluster name> <cluster endpoint>
````

This will generate a machine config for each node type, and a talosconfig.
The following is an example of an `init.yaml`:

```yaml
version: v1alpha1
machine:
  type: init
  token: 5dt69c.npg6duv71zwqhzbg
  ca:
    crt: <base64 encoded Ed25519 certificate>
    key: <base64 encoded Ed25519 key>
  certSANs: []
  kubelet: {}
  network: {}
  install:
    disk: /dev/sda
    image: ghcr.io/talos-systems/installer:latest
    bootloader: true
    wipe: false
    force: false
cluster:
  controlPlane:
    endpoint: https://1.2.3.4
  clusterName: example
  network:
    cni: ""
    dnsDomain: cluster.local
    podSubnets:
    - 10.244.0.0/16
    serviceSubnets:
    - 10.96.0.0/12
  token: wlzjyw.bei2zfylhs2by0wd
  certificateKey: 20d9aafb46d6db4c0958db5b3fc481c8c14fc9b1abd8ac43194f4246b77131be
  aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
  ca:
    crt: <base64 encoded RSA certificate>
    key: <base64 encoded RSA key>
  apiServer: {}
  controllerManager: {}
  scheduler: {}
  etcd:
    ca:
      crt: <base64 encoded RSA certificate>
      key: <base64 encoded RSA key>
```
*/
package v1alpha1
