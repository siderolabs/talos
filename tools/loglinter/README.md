# log-linter

`log-linter` checks Go code for logging policy violations.

It runs as a `golangci-lint` module plugin custom linter named `loglinter`.

Current checks:

- disallowed `log/slog` imports unless allowlisted
- disallowed stdlib `log.*` calls unless allowlisted
- printf-style formatting directives in `*zap.Logger` message strings
- `fmt.Sprintf(...)` used as a zap message
- ad hoc zap root logger constructors not wrapped with `.With(logging.Component(...))` in the same expression

## Usage

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

The repository root is the directory containing the `golangci-lint` config file.

## Config

All rules are enabled by default. An empty settings block applies every rule to every analyzed Go file under the repository root.

Use config mainly to carve out exceptions.

- `allow` is the recommended way to exempt files from a specific rule.
- `exclude` is a broader file skip for a rule or for the whole run.
- `include` is still supported when you intentionally want to narrow a rule, but the default is repo-wide enforcement.
- `enabled: false` explicitly disables a rule.
- globs are repo-relative, slash-separated, and support `**`.

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

- Once `loglinter` is enabled in `.golangci.yml`, use the custom binary (`./custom-gcl`) or `make golangci-lint-custom`; a stock `golangci-lint` binary will not have the plugin compiled in.
- The root-component check is intentionally conservative: it only accepts `.With(logging.Component(...))` when that wrapper appears in the same expression as the configured constructor call.
- Constructor names and component helper calls are configurable.
