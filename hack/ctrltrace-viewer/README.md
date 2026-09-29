# ctrltrace viewer

A single-page viewer for the controller runtime traces produced by
`internal/pkg/ctrltrace`. It replays a capture as an animated graph: controllers and
resource types as nodes, clustered by namespace, flashing as they reconcile, with
resource nodes growing and shrinking as instances come and go.

## Using it

Open `index.html` in a browser — straight off the filesystem is fine — then drop a
`.jsonl` capture onto the page, or use **Load a trace**. Nothing is uploaded; the file
is parsed in the browser.

To capture one, see [the tracer's README](../../internal/pkg/ctrltrace/README.md):

```sh
make talos WITH_DEBUG=1
# boot with talos.trace=1 talos.trace.specs=1
talosctl -n <node> read /run/talos-trace.jsonl > talos-boot.jsonl
```

The page also tries to fetch `talos-boot.jsonl` from its own directory on load, so
dropping a capture there and serving the directory gives you a viewer that opens
straight onto that boot:

```sh
cp talos-boot.jsonl hack/ctrltrace-viewer/
python3 -m http.server -d hack/ctrltrace-viewer 8000
```

## Themes

The **Theme** control in the footer picks between *Auto* (follows the browser), *Light*,
*Dark* and *Projector*; `t` cycles them, and the choice is remembered per browser.

*Projector* is the one to use on stage: white ground, near-black ink, deep blue and rust
marks instead of the screen palette's lighter steps, thicker strokes and larger type. Lecture
hall projectors crush low contrast, so the greys that read as pleasantly quiet on a laptop
disappear entirely — this theme pushes everything toward black and scales every stroke,
node and label by 1.45x.

## What it shows

* **Squares** are controllers, **circles** are resource types; size follows the number
  of live instances. Position is a force layout clustered by namespace, computed once
  and frozen so the map stays put between replays.
* A node is an empty outline until the trace first touches it, so the boot fills in the
  architecture rather than starting fully lit.
* Selecting a node draws its edges as arrows in the direction data flows — orange out
  of the controller that writes a resource, blue from a resource into the controllers
  that read it. The side panel lists the same thing in text, along with the live
  instances at the current playhead; clicking an instance shows its spec, when the
  capture was taken with `talos.trace.specs=1`.
* The graph is the union of the static dependency graph embedded in the trace (the same
  one `talosctl inspect dependencies` renders) and the traffic actually observed, which
  in practice adds a few hundred edges the controllers never declared.
* The scrub track is state operations per second on a square-root scale — a boot is
  heavy-tailed enough that a linear axis shows one spike and nothing else.

## Notes

* The page loads d3 from cdnjs and its fonts from Google Fonts, so first paint wants a
  network connection. Everything else, including parsing a multi-megabyte trace, is local.
* Large captures are fine: a 6 MB, 29k-event trace parses, lays out and renders in well
  under a second.
