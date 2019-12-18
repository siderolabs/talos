<!-- markdownlint-disable -->
## osctl gen csr

Generates a CSR using an Ed25519 private key

### Synopsis

Generates a CSR using an Ed25519 private key

```
osctl gen csr [flags]
```

### Options

```
  -h, --help         help for csr
      --ip string    generate the certificate for this IP address
      --key string   path to the PEM encoded EC or RSA PRIVATE KEY
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/root/.talos/config")
```

### SEE ALSO

* [osctl gen](osctl_gen.md)	 - Generate CAs, certificates, and private keys

