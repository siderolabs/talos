# log-linter

`log-linter` checks Go code for logging policy violations.

It can run in two modes:

- as a standalone CLI
- as a `golangci-lint` module plugin custom linter

Current checks:

- disallowed `log/slog` imports unless allowlisted
- disallowed stdlib `log.*` calls unless allowlisted
- printf-style formatting directives in `*zap.Logger` message strings
- `fmt.Sprintf(...)` used as a zap message
- ad hoc zap root logger constructors not wrapped with `.With(logging.Component(...))` in the same expression

## Standalone usage

The repository keeps its `golangci-lint` configuration inline in `.golangci.yml`, so there is no checked-in standalone `log-linter.yaml` file.

The CLI accepts either a standalone log-linter YAML file or a `golangci-lint`
config file; the format is detected automatically. When given a `golangci-lint`
config, it reads the settings from `linters.settings.custom.loglinter.settings`.

Point the CLI at the repository `.golangci.yml`:

```bash
go run ./tools/loglinter -config .golangci.yml
```

Or at your own standalone YAML file:

```bash
go run ./tools/loglinter -config /path/to/log-linter.yaml
```

Or through the `tools` module:

```bash
cd tools
go tool github.com/siderolabs/talos/tools/loglinter -config /path/to/log-linter.yaml
```

Optional file or directory filters can be passed after `-config`.

```bash
go run ./tools/loglinter -config .golangci.yml internal/app/apid pkg/grpc
```

Exit codes:

- `0` when no issues are found
- `1` when lint issues are found
- `2` on config or package-loading errors

## golangci-lint module plugin usage

This module also exposes a `golangci-lint` custom linter plugin named `loglinter`.

### Build a custom golangci-lint binary

A repo-root `.custom-gcl.yml` is included for the custom binary build.

From the repo root:

```bash
make golangci-lint-custom
```

That builds the binary through the Docker-based flow and outputs it to `_out/custom-gcl`.

Run it directly:

```bash
_out/custom-gcl run --config .golangci.yml
```

### Enable the plugin in `.golangci.yml`

Add a custom linter entry like this:

```yaml
version: "2"

linters:
  enable:
    - loglinter
  settings:
    custom:
      loglinter:
        type: module
        description: checks logging conventions
        settings:
          exclude:
            - "**/*_test.go"
            - "_out/**"
            - "vendor/**"
          rules:
            slog_imports:
              allow:
                - "internal/app/machined/pkg/runtime/v1alpha1/platform/vmware/vmware_supported.go"
```

The plugin accepts inline config directly inside `settings:`.
It also supports `settings.config` if you want the plugin and the standalone CLI to share the same external YAML file.

Path resolution differs slightly by mode:

- inline plugin settings use the directory containing the `golangci-lint` config file as the repository root
- `settings.config` paths are resolved from the current working directory and then searched upward through parent directories
- standalone or external YAML files use the directory containing that YAML file as the repository root

## Config

The standalone CLI reads a YAML config.
The `golangci-lint` plugin can either use the same structure inline under `linters.settings.custom.loglinter.settings` or load an external YAML file through `settings.config`.

All rules are enabled by default. An empty config applies every rule to every analyzed Go file under the inferred repository root.

Use config mainly to carve out exceptions.

- the repository root is inferred from the YAML file location for standalone/external config, or from `.golangci.yml` for inline plugin config.
- `allow` is the recommended way to exempt files from a specific rule.
- `exclude` is a broader file skip for a rule or for the whole run.
- `include` is still supported when you intentionally want to narrow a rule, but the default is repo-wide enforcement.
- `enabled: false` explicitly disables a rule.
- globs are repo-relative, slash-separated, and support `**`.

Example standalone YAML:

```yaml
exclude:
  - "**/*_test.go"
  - "_out/**"
  - "vendor/**"

rules:
  stdlib_log_calls:
    allow:
      - "internal/app/init/**/*.go"
      - "internal/app/machined/pkg/runtime/v1alpha1/**/*.go"
      - "internal/app/poweroff/**/*.go"
      - "tools/**/*.go"

  slog_imports:
    allow:
      - "internal/app/machined/pkg/runtime/v1alpha1/platform/vmware/vmware_supported.go"
      - "pkg/provision/providers/vm/dnsd.go"

  zap_root_component:
    allow:
      - "pkg/logging/zap.go"
```

## Inline exceptions

A single issue can be ignored with a same-line or immediately preceding comment:

```go
// loglint:ignore stdlib_log_calls compatibility adapter
log.Printf("allowed here")
```

Multiple rules can be listed as a comma-separated token:

```go
// loglint:ignore stdlib_log_calls,zap_root_component reason
```

Use `all` to suppress every rule for the next line or same line.

## Notes

- The standalone CLI loads packages with `go/packages`.
- The `golangci-lint` plugin runs as a normal analyzer inside the custom binary.
- Once `loglinter` is enabled in `.golangci.yml`, use the custom binary (`./custom-gcl`) or `make golangci-lint-custom`; a stock `golangci-lint` binary will not have the plugin compiled in.
- The root-component check is intentionally conservative: it only accepts `.With(logging.Component(...))` when that wrapper appears in the same expression as the configured constructor call.
- Constructor names and component helper calls are configurable.
