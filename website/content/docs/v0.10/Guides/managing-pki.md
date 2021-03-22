---
title: "Managing PKI"
description: ""
---

## Generating an Administrator Key Pair

In order to create a key pair, you will need the root CA.

Save the the CA public key, and CA private key as `ca.crt`, and `ca.key` respectively.
Now, run the following commands to generate a certificate:

```bash
talosctl gen key --name admin
talosctl gen csr --key admin.key --ip 127.0.0.1
talosctl gen crt --ca ca --csr admin.csr --name admin
```

Now, base64 encode `admin.crt`, and `admin.key`:

```bash
cat admin.crt | base64
cat admin.key | base64
```

You can now set the `crt` and `key` fields in the `talosconfig` to the base64 encoded strings.

## Renewing an Expired Administrator Certificate

In order to renew the certificate, you will need the root CA, and the admin private key.
The base64 encoded key can be found in any one of the control plane node's configuration file.
Where it is exactly will depend on the specific version of the configuration file you are using.

Save the the CA public key, CA private key, and admin private key as `ca.crt`, `ca.key`, and `admin.key` respectively.
Now, run the following commands to generate a certificate:

```bash
talosctl gen csr --key admin.key --ip 127.0.0.1
talosctl gen crt --ca ca --csr admin.csr --name admin
```

You should see `admin.crt` in your current directory.
Now, base64 encode `admin.crt`:

```bash
cat admin.crt | base64
```

You can now set the certificate in the `talosconfig` to the base64 encoded string.
