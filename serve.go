/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// The web view is a Svelte app built into web/dist, and committed, so that
// go install gets a working --serve without Node. Rebuild it with go generate
// after changing anything under web/src.
//
//go:generate npm --prefix web run build
//go:embed all:web/dist
var webDist embed.FS

// A graphView is what the web view draws: the same reading the tree is
// rendered from, with each resource's state already decided, so the page
// never has to know how a state is derived.
type graphView struct {
	XR     xrView         `json:"xr"`
	Gone   bool           `json:"gone,omitempty"` // The composite doesn't exist.
	Nodes  []nodeView     `json:"nodes"`
	Waves  int            `json:"waves"`
	Edges  int            `json:"edges"`
	Cyclic []string       `json:"cyclic"`
	Counts map[string]int `json:"counts"`
}

type xrView struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Ready      string `json:"ready,omitempty"`
	Deleting   bool   `json:"deleting,omitempty"`
}

type nodeView struct {
	Name       string   `json:"name"`
	APIVersion string   `json:"apiVersion,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Object     string   `json:"object,omitempty"`
	Namespace  string   `json:"namespace,omitempty"`
	State      string   `json:"state"`
	Reason     string   `json:"reason,omitempty"`
	Wave       int      `json:"wave"` // -1 for a resource on a cycle.
	DependsOn  []string `json:"dependsOn"`
	Requires   []string `json:"requires"`
}

// view decides everything the page shows. Resources are sorted by name, and
// their edges too, so that an unchanged graph marshals to the same bytes and
// the server can tell when something actually moved.
func view(xr *unstructured.Unstructured, nodes []*node) graphView {
	cyclic := waves(nodes)
	byWave, _, counts := group(nodes)

	v := graphView{
		XR: xrView{
			APIVersion: xr.GetAPIVersion(),
			Kind:       xr.GetKind(),
			Name:       xr.GetName(),
			Namespace:  xr.GetNamespace(),
			Ready:      xrReady(xr),
			Deleting:   xr.GetDeletionTimestamp() != nil,
		},
		Nodes:  make([]nodeView, 0, len(nodes)),
		Waves:  len(byWave),
		Cyclic: cyclic,
		Counts: counts,
	}

	for _, n := range nodes {
		deps := append([]string{}, n.DependsOn...)
		sort.Strings(deps)

		req := append([]string{}, n.Requires...)
		sort.Strings(req)

		v.Edges += len(deps)
		v.Nodes = append(v.Nodes, nodeView{
			Name:       n.Name,
			APIVersion: n.APIVersion,
			Kind:       n.Kind,
			Object:     n.Object,
			Namespace:  n.Namespace,
			State:      n.state(),
			Reason:     n.Reason,
			Wave:       n.wave,
			DependsOn:  deps,
			Requires:   req,
		})
	}

	sort.Slice(v.Nodes, func(a, b int) bool { return v.Nodes[a].Name < v.Nodes[b].Name })

	return v
}

// A hub holds the latest reading and wakes every open page when it changes.
//
// One poller serves every page, rather than one per page, so a dashboard
// left open in three tabs costs the API server what one does.
type hub struct {
	mu      sync.Mutex
	graph   []byte        // The latest graph, as JSON. Nil until the first read.
	status  []byte        // When it was read, and the error if the last read failed.
	changed chan struct{} // Closed, and replaced, whenever graph or status changes.
}

func newHub() *hub {
	return &hub{changed: make(chan struct{})}
}

// publish records a reading. A graph is only replaced when it differs, so a
// page redraws when something moved rather than on every tick; the status
// is always replaced, so a page can show how fresh its graph is.
func (h *hub) publish(graph, status []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if graph != nil && !bytes.Equal(graph, h.graph) {
		h.graph = graph
	}

	h.status = status

	close(h.changed)
	h.changed = make(chan struct{})
}

func (h *hub) latest() (graph, status []byte, changed <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.graph, h.status, h.changed
}

type statusView struct {
	At    time.Time `json:"at"`
	Error string    `json:"error,omitempty"`
}

// poll reads the graph every interval until ctx is done.
//
// A composite that doesn't exist is reported as gone, with no resources, so
// the page clears rather than showing the last of a teardown as if it were
// still happening; it fills in again if the composite is created. Any other
// failed read keeps the last good graph, and says why in the status, because
// a graph that is a few seconds stale is more useful than an empty page.
func (c *cli) poll(ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, h *hub) {
	t := time.NewTicker(c.Interval)
	defer t.Stop()

	// Who the composite is, for the page to name once it's gone. Until it
	// has been read, all there is to go on is how it was asked for.
	kind, name, _ := strings.Cut(c.Resource, "/")
	last := xrView{Kind: kind, Name: name, Namespace: c.Namespace}

	for {
		rctx, cancel := context.WithTimeout(ctx, c.Timeout)
		xr, nodes, err := c.read(rctx, dyn, mapper)

		cancel()

		st := statusView{At: time.Now().UTC()}

		var graph []byte

		switch {
		case kerrors.IsNotFound(err):
			graph, _ = json.Marshal(graphView{XR: last, Gone: true, Nodes: []nodeView{}, Cyclic: []string{}, Counts: map[string]int{}})
		case err != nil:
			st.Error = err.Error()
		default:
			v := view(xr, nodes)
			last = xrView{APIVersion: v.XR.APIVersion, Kind: v.XR.Kind, Name: v.XR.Name, Namespace: v.XR.Namespace}

			if graph, err = json.Marshal(v); err != nil {
				st.Error = fmt.Sprintf("cannot encode the graph: %v", err)
			}
		}

		status, _ := json.Marshal(st)
		h.publish(graph, status)

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// serve runs the web view until interrupted.
func (c *cli) serve(dyn dynamic.Interface, mapper meta.RESTMapper) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	site, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		return fmt.Errorf("cannot read the embedded web view: %w", err)
	}

	h := newHub()
	go c.poll(ctx, dyn, mapper, h)

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(site))
	mux.HandleFunc("GET /api/graph", h.serveGraph)
	mux.HandleFunc("GET /api/events", h.serveEvents)

	l, err := net.Listen("tcp", c.Serve)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", c.Serve, err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()

		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(sctx)
	}()

	fmt.Fprintf(os.Stderr, "Serving the graph of %s at http://%s\n", c.Resource, l.Addr())

	if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("cannot serve: %w", err)
	}

	return nil
}

// serveGraph answers with the latest graph, for anything that wants one
// reading rather than a stream.
func (h *hub) serveGraph(w http.ResponseWriter, _ *http.Request) {
	graph, _, _ := h.latest()
	if graph == nil {
		http.Error(w, "the graph hasn't been read yet", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(graph)
}

// serveEvents streams server-sent events: a graph event whenever the graph
// changes, and a status event on every read, so a page knows both what to
// draw and how stale it is.
func (h *hub) serveEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	var sent []byte

	for {
		graph, status, changed := h.latest()

		if graph != nil && !bytes.Equal(graph, sent) {
			fmt.Fprintf(w, "event: graph\ndata: %s\n\n", graph)
			sent = graph
		}

		if status != nil {
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", status)
		}

		if err := rc.Flush(); err != nil {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-changed:
		}
	}
}
