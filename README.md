# crossplane-graph

`crossplane-graph` prints the ordering graph a Crossplane composite resource
(XR) carries: its composed resources grouped into the waves Crossplane creates
them in, each resource's state, and what the graph is holding back and why.

```text
$ crossplane-graph xordering/ordered -n default
XOrdering default/ordered   Ready=False
10 composed resources · 10 edges · 4 waves

wave 0 ───────────────────────────────────────────────────────────
 ✔ ready       standalone  NopResource
 ✔ ready       vpc         NopResource

wave 1 ───────────────────────────────────────────────────────────
 ◐ creating    gateway     NopResource
 ◐ creating    sg          NopResource
 ✔ ready       subnet-a    NopResource
 ◐ creating    subnet-b    NopResource

wave 2 ───────────────────────────────────────────────────────────
 ✔ ready       database    NopResource
 ⊘ pending     instance    NopResource  waiting for [sg subnet-b] to be ready

wave 3 ───────────────────────────────────────────────────────────
 ⊘ pending     app         NopResource  waiting for [instance] to be ready
 ⊘ pending     backup      NopResource  waiting for [database] to be ready

4 ready · 3 creating · 3 pending
```

It answers the two questions ordering raises most often: did the dependencies
reach the XR, and what is this resource waiting for?

## Status

Experimental. Composed resource ordering is a Crossplane prototype, open as
[crossplane/crossplane#7842][pr] with the design in
[crossplane/crossplane#7841][design]. Neither is merged, and the fields this
tool reads may still change. Against a Crossplane without ordering, an XR has
no graph to print.

To try it, install the prototype's release chart:

```shell
helm install crossplane oci://ghcr.io/stevendborrelli/charts/crossplane \
  --version 2.5.0-ordering.2 -n crossplane-system --create-namespace
```

[pr]: https://github.com/crossplane/crossplane/pull/7842
[design]: https://github.com/crossplane/crossplane/pull/7841

## Install

```shell
go install github.com/stevendborrelli/crossplane-graph@latest
```

It needs Go 1.26 or later, and a kubeconfig that can read the XR and its
composed resources.

## Usage

```shell
crossplane-graph xordering/ordered -n default          # a namespaced XR
crossplane-graph inferencegateway/default              # a cluster-scoped XR
crossplane-graph xordering/ordered -n default --edges  # and what each resource waits on
crossplane-graph servingstack/my-stack -n modelplane-system --dot | dot -Tpng -o graph.png
```

The resource is `kind/name` or `resource.group/name`. Use the second form when
two API groups share a kind.

To watch a graph converge:

```shell
watch -c -n1 crossplane-graph xordering/ordered -n default --color always
```

| Flag | |
| --- | --- |
| `-n`, `--namespace` | Namespace of a namespaced XR. |
| `--kubeconfig` | Path to a kubeconfig. Defaults to `$KUBECONFIG`, then `~/.kube/config`. |
| `--context` | Kubeconfig context to use. Defaults to the current context. |
| `-e`, `--edges` | Print each resource's dependencies beneath it, rather than only the waves. |
| `--dot` | Print Graphviz DOT instead of a tree. |
| `--color` | `auto`, `always` or `never`. |
| `--timeout` | How long to spend talking to the API server. Defaults to 30s. |
| `--serve` | Serve a live web view at this address, such as `localhost:8080`, rather than printing. |
| `--interval` | How often the web view reads the graph again. Defaults to 2s. |

### The web view

```shell
crossplane-graph xordering/ordered -n default --serve localhost:8080
```

`--serve` draws the same graph in a browser and keeps it current: a column per
wave, a box per composed resource, and a line for each dependency, running from
what a resource depends on to the resource. A line is green once its dependency
is ready, and dashed while the resource it leads to is still pending. Click a
resource for its object name, its reason, and what depends on it. Deadlocks and
cycles are called out at the top.

The graph is read every `--interval` by one poller, however many pages are
open, and pushed to them as server-sent events. A read that fails keeps the
last graph on screen, marked stale. If the composite is deleted, the page says
so, and fills in again when it's recreated.

It listens only where you tell it to. `localhost:8080` stays on your machine;
`:8080` serves anyone who can reach it, with your kubeconfig's access to the
cluster behind it.

The page is a Svelte app in [`web/`](web), built into `web/dist` and embedded in
the binary, so `go install` gets it without Node. Its boxes and connectors come
from Steven Borrelli's XPlanet primitives, copied into
[`web/src/lib/xplanet`](web/src/lib/xplanet). After changing anything under
`web/src`, rebuild it and commit the result:

```shell
(cd web && npm ci)
go generate ./...
```

`npm --prefix web run dev` serves the page with hot reload, proxying the API to
a `crossplane-graph --serve localhost:8080` running alongside.

## What it reads

Everything comes from the API server. It never runs a function.

- **The graph** comes from the XR's composed resource references,
  `spec.crossplane.resourceRefs`, where each entry records its composition
  resource name and what it depends on. Crossplane orders teardown from this
  field, so it is also the right thing to look at when teardown orders itself
  oddly. Legacy v1 XRs keep their references at `spec.resourceRefs`, which
  is read when the modern field is absent.
- **What is held back** comes from `status.crossplane.pendingResources`. A
  resource held back from creation has no reference yet, so without this it
  would be missing from the graph entirely. Resources held back from deletion
  appear here too, and deadlocks, which waiting won't fix, get a block of
  their own.
- **Each resource's state** comes from fetching the composed resource itself.

The states:

| State | Meaning |
|---|---|
| `ready` | The resource reports Ready, so what depends on it can be created. |
| `creating` | The resource exists but isn't ready, or Crossplane has written its reference and is about to apply it. |
| `pending` | The graph is holding back its creation until what it depends on is ready. |
| `held` | The graph is holding back its deletion until what depends on it is gone. |
| `deleting` | The resource is being deleted. |
| `deleted` | Teardown has removed it, and its reference hasn't been dropped yet. |
| `deadlocked` | Waiting won't resolve it, and someone has to act. |

`pending` and `held` both come from `status.crossplane.pendingResources`.

## Caveats

- **Readiness is a best-effort read.** Crossplane decides readiness from the
  function pipeline's verdict, which isn't stored anywhere this tool can read.
  So it judges each resource by its own conditions instead, which for most
  resources agrees. The two can briefly disagree, for example showing a
  dependency ready while its dependent is still pending on it, until
  Crossplane's next reconcile.

## License

Apache 2.0. See [LICENSE](LICENSE).
