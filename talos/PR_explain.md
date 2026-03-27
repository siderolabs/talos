# Add `talosctl explain` command

## Summary

Adds a new `talosctl explain <type>[.field[.field...]]` command that works like `kubectl explain` for Talos COSI resources. Shows resource definition metadata and field information with dot notation navigation.

## Usage

```bash
# Show top-level resource information
talosctl explain links

# Navigate into spec fields
talosctl explain links.spec

# Shorthand for spec.bondMaster (spec is implied)
talosctl explain links.bondMaster

# Full path navigation
talosctl explain links.spec.bondMaster.mode

# Navigate metadata
talosctl explain links.metadata
talosctl explain links.metadata.labels
```

## Output format

Matches `kubectl explain`:

- **Top level**: RESOURCE, NAMESPACE, ID, ALIASES, SENSITIVITY, PRINT COLUMNS, FIELDS (metadata + spec)
- **Field navigation**: RESOURCE, NAMESPACE, FIELD, FIELDS (one level only)

```bash
$ talosctl explain links
RESOURCE:     LinkStatus <LinkStatuses.net.talos.dev>
NAMESPACE:    network
ID:           linkstatuses.net.talos.dev
ALIASES:      link, links, linkstatus, ls
SENSITIVITY:  non-sensitive

PRINT COLUMNS:
  NAME         JSON PATH
  Alias        {.alias}
  Type         {.type}
  ...

FIELDS:
  metadata	<object>
    Resource metadata (namespace, type, id, version, phase, owner, labels, annotations, finalizers).

  spec	<object>
    Resource specification.

$ talosctl explain links.spec
RESOURCE:     LinkStatus <LinkStatuses.net.talos.dev>
NAMESPACE:    network

FIELD: spec

FIELDS:
  bondMaster       <Object>
  broadcastAddr    <string>
  driver           <string>
  ...
```

## Implementation details

### File: `cmd/talosctl/cmd/talos/explain.go`

1. **Argument parsing** (`resolveExplainArg`)
   - First tries to resolve full argument as resource type
   - If fails, splits on first dot: `resourceType.fieldPath`
   - Returns `ResourceDefinition` and field path components

2. **Path normalization** (`normalizeFieldPath`)
   - If path starts with neither `metadata` nor `spec`, prepends `spec`
   - Enables `links.bondMaster` shorthand for `links.spec.bondMaster`

3. **Single-node pinning**
   - COSI methods don't support one-to-many proxying
   - Pins all calls to first node like `helpers.ForEachResource`

4. **Field discovery**
   - **Metadata**: Static field list (common to all COSI resources)
   - **Spec**: Fetches one resource from node, marshals to YAML, introspects fields
   - Only shows one level of fields per path (no recursive expansion)

5. **Type inference**
   - Maps Go types to kubectl-like names: `boolean`, `integer`, `string`, `Object`
   - Handles arrays: `[]string`, `[]Object`

## Limitations

- **No field descriptions**: COSI resources don't carry runtime descriptions (unlike Kubernetes OpenAPI)
- **Spec fields require sample**: Only shows fields present in at least one resource on the node
- **Type inference**: Approximate (can't distinguish `uint32` from `int64`)

## Testing

```bash
# Build
go build ./cmd/talosctl/

# Help
./talosctl explain --help

# Against live cluster
./talosctl explain links
./talosctl explain links.spec
./talosctl explain links.bondMaster
```

## Integration

- Registered via `addCommand(explainCmd)` in `init()`
- Appears in "Manage running Talos clusters" command group
- Reuses `completeResourceDefinition()` for tab completion
- Uses existing patterns: `WithClient()`, `helpers.ClientVersionCheck()`
