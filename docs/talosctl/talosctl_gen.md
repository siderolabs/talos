<!-- markdownlint-disable -->
## talosctl gen

Generate CAs, certificates, and private keys

### Options

```
  -h, --help   help for gen
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl](talosctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl gen ca](talosctl_gen_ca.md)	 - Generates a self-signed X.509 certificate authority
* [talosctl gen config](talosctl_gen_config.md)	 - Generates a set of configuration files for Talos cluster
* [talosctl gen crt](talosctl_gen_crt.md)	 - Generates an X.509 Ed25519 certificate
* [talosctl gen csr](talosctl_gen_csr.md)	 - Generates a CSR using an Ed25519 private key
* [talosctl gen key](talosctl_gen_key.md)	 - Generates an Ed25519 private key
* [talosctl gen keypair](talosctl_gen_keypair.md)	 - Generates an X.509 Ed25519 key pair

