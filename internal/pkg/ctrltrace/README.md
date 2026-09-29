# ctrltrace — controller runtime tracing

`ctrltrace` records what the COSI controller runtime actually does while Talos boots:
which controller woke up, what it read, what it wrote, and when every resource appeared,
changed and went away.

It is a development facility, and it is compiled in **only** under `WITH_DEBUG=1` (build tag
`sidero.debug`). A production image gets `stub.go` instead: no tracer, no state wrapping, no
way to turn it on. Even in a debug build nothing is wrapped or allocated until the trace is
explicitly enabled, so it costs nothing on a normal boot.

## Capturing a trace

Build a debug image:

```sh
make talos WITH_DEBUG=1
```

Then boot the node with the trace kernel arguments:

```text
talos.trace=1
```

Optional:

| parameter | default | meaning |
| --- | --- | --- |
| `talos.trace.path` | `/run/talos-trace.jsonl` | output file |
| `talos.trace.specs` | `0` | also record resource specs (YAML) on create/update |
| `talos.trace.duration` | unlimited | stop tracing after e.g. `90s`, keeping the file small |

In a container-based cluster (`talosctl cluster create --provisioner docker`) set the
`TALOS_TRACE=1` environment variable on the container instead.

For a QEMU cluster:

```sh
talosctl cluster create \
  --extra-boot-kernel-args "talos.trace=1 talos.trace.specs=1 talos.trace.duration=3m"
```

Pull the trace off the node once it is up:

```sh
talosctl -n <node> read /run/talos-trace.jsonl > talos-boot.jsonl
```

## Handle the file with care

`talos.trace.specs=1` records the YAML of every resource created or updated, and unlike the
API the tracer does **not** redact the types Talos marks `Sensitivity: sensitive` — a secret
you cannot read is not much use when you are debugging how it came to exist. So a capture with
specs contains root CA private keys, machine tokens and the rest, in the clear. This is why the
tracer is a `WITH_DEBUG=1` facility aimed at throwaway clusters.

The file is written `0600`, but once it is off the node it is just a file. Even without specs
it records resource IDs — node names, cluster membership, affiliate IDs, certificate SANs.
Don't hand a capture around, and don't take one from a cluster you care about.

## What is in the file

Newline-delimited JSON, one record per line, in chronological order.

* `{"k":"meta","op":"start"}` — header: format version, Talos version, wall clock of `t=0`.
* `{"k":"graph","op":"dependencies"}` — the static controller/resource graph, the same one
  `talosctl inspect dependencies` renders. Embedding it makes the trace self-contained.
* `{"k":"ctrl",...}` — controller lifecycle: `start`, `wake` (the controller picked up a
  reconcile event), `queue`, `inputs`, `stop`, `crash`.
* `{"k":"op",...}` — a state operation: `get`, `list`, `create`, `update`, `destroy`,
  `watch`, `watchkind`, with the resource coordinates, owner, version, phase, finalizers,
  duration and error.
* `{"k":"meta","op":"stop"}` — footer: events written and dropped.

`t` is microseconds since the tracer started. Field names are short on purpose — a boot
produces a lot of these.

## How the attribution works

Writes are attributed by the resource owner, but reads are not owned by anybody. To attribute
them, `WrapController` tags the context it hands to each controller's `Run` with the controller
name; that tag rides along into every `Get`/`List`/`Modify` the controller makes and is read
back by the state wrapper. Operations with no tag (the API server, the v1alpha1 sequencer) are
recorded for writes only — reads from those would drown out the controllers.

The runtime restarts a failed controller by calling `Run` again with the *same* context and
runtime values, and controllers may rely on that identity — `internal/pkg/dns.Manager`
panics outright if `ServeBackground` is handed a different context on a restart. So the
wrapper builds the tagged context and the runtime wrapper once per controller and reuses
them; re-tagging per `Run` breaks that contract, and would also leave a second event relay
behind on every restart, competing with the first for the same upstream channel.

Controller *wake-ups* are exact: the wrapper relays the reconcile event channel, and an
unbuffered hand-off completes precisely when the controller enters a new reconcile cycle. Where
that cycle *ends* is not observable from outside the controller, so it is reconstructed offline
from the controller's last state operation before its next wake-up.

Run the package tests with the tag; without it only the stub test runs:

```sh
go test -tags sidero.debug ./internal/pkg/ctrltrace/...
```

## Caveats

* A spec longer than 8 KiB is cut off, and the tail is marked `# ... truncated`.
* Events are dropped rather than blocking a controller when the queue (64k) overflows; the
  footer reports how many.
* Tracing starts before `/run` is mounted, so early events sit in the queue until the file
  can be opened.
* With the tracer on, the state wrapper does not implement the optional `Teardowner` /
  `TeardownAndDestroyer` fast paths, so teardowns decompose into `get` + `update`. This is
  visible in the trace and is the only behavioral difference.

## Viewing a trace

[`hack/ctrltrace-viewer`](../../../hack/ctrltrace-viewer/) is a single-page viewer for
these files: open its `index.html` and drop the capture onto it. It draws the controllers
and resource types as a graph clustered by namespace and replays the capture — controllers
flash as they reconcile, resource nodes grow with their live instance count, read and write
pulses travel the edges, and selecting a node shows what reads and writes it.
