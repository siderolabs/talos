<!-- markdownlint-disable -->
## osctl gen

Generate CAs, certificates, and private keys

### Synopsis

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

* [osctl](osctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [osctl gen ca](osctl_gen_ca.md)	 - Generates a self-signed X.509 certificate authority
* [osctl gen crt](osctl_gen_crt.md)	 - Generates an X.509 Ed25519 certificate
* [osctl gen csr](osctl_gen_csr.md)	 - Generates a CSR using an Ed25519 private key
* [osctl gen key](osctl_gen_key.md)	 - Generates an Ed25519 private key
* [osctl gen keypair](osctl_gen_keypair.md)	 - Generates an X.509 Ed25519 key pair

