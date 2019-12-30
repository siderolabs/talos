<!-- markdownlint-disable -->
## osctl gen keypair

Generates an X.509 Ed25519 key pair

### Synopsis

Generates an X.509 Ed25519 key pair

```
osctl gen keypair [flags]
```

### Options

```
  -h, --help                  help for keypair
      --ip string             generate the certificate for this IP address
      --organization string   X.509 distinguished name for the Organization
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [osctl gen](osctl_gen.md)	 - Generate CAs, certificates, and private keys

