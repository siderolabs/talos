# kubeimportlinter

`kubeimportlinter` checks that versioned Kubernetes imports are aliased to a descriptive name.

Imports whose final path segment is a bare API version (e.g. `v1`, `v1beta1`, `v2`) from a Kubernetes host (`k8s.io`, `sigs.k8s.io`, ...) must be renamed:

```go
import "k8s.io/api/core/v1"            // invalid: package name is bare v1
import v1 "k8s.io/api/core/v1"         // invalid: alias equals the version
import corev1 "k8s.io/api/core/v1"     // valid
```

It runs as a `golangci-lint` module plugin custom linter named `kubeimportlinter`.

## Usage

The repo-root `.custom-gcl.yml` lists this module for the custom binary build:

```bash
make golangci-lint-custom
_out/custom-gcl run --config .golangci.yml
```

Enable it in `.golangci.yml`:

```yaml
linters:
  enable:
    - kubeimportlinter
  settings:
    custom:
      kubeimportlinter:
        type: module
        settings:
          exclude:
            - "**/*_test.go"
            - "vendor/**"
          rules:
            versioned_imports:
              # override the default host list if needed
              hosts:
                - k8s.io
                - sigs.k8s.io
              allow:
                - "path/to/exempt.go"
```

## Config

All rules are enabled by default. An empty settings block applies the rule to
every analyzed Go file under the repository root.

- the repository root is the directory containing the `golangci-lint` config file.
- `versioned_imports.hosts` lists the import path prefixes the rule applies to
  (defaults to `k8s.io`, `sigs.k8s.io`).
- `allow` exempts files from the rule; `exclude` is a broader file skip;
  `include` narrows the rule; `enabled: false` disables it.
- globs are repo-relative, slash-separated, and support `**`.

## Inline exceptions

```go
// kubeimportlint:ignore versioned_imports intentional bare alias
v1 "k8s.io/api/core/v1"
```
