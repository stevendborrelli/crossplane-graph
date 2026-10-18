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
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestView(t *testing.T) {
	x := xr(ref("vpc", "NopResource"), ref("subnet", "NopResource", "vpc"), ref("app", "NopResource", "subnet", "vpc"))

	nodes, err := readGraph(x)
	if err != nil {
		t.Fatalf("readGraph(...): %v", err)
	}

	nodes[0].Exists, nodes[0].Ready = true, true // vpc
	nodes[1].Exists = true                       // subnet, not yet ready

	v := view(x, nodes)

	if v.XR.Kind != "XOrdering" || v.XR.Name != "ordered" || v.XR.Namespace != "default" {
		t.Errorf("view(...).XR: got %+v", v.XR)
	}

	if v.Waves != 3 || v.Edges != 3 {
		t.Errorf("view(...): want 3 waves and 3 edges, got %d and %d", v.Waves, v.Edges)
	}

	got := map[string]string{}
	for _, n := range v.Nodes {
		got[n.Name] = n.State
	}

	want := map[string]string{"vpc": "ready", "subnet": "creating", "app": "creating"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("view(...): -want states, +got states:\n%s", diff)
	}

	// Sorted, resources and edges both, so an unchanged graph encodes to the
	// same bytes and the server can tell when something moved.
	if v.Nodes[0].Name != "app" || !cmp.Equal(v.Nodes[0].DependsOn, []string{"subnet", "vpc"}) {
		t.Errorf("view(...): want app first with sorted edges, got %+v", v.Nodes[0])
	}

	a, _ := json.Marshal(view(x, nodes))
	b, _ := json.Marshal(view(x, nodes))

	if string(a) != string(b) {
		t.Error("view(...) of the same graph should encode to the same bytes")
	}
}

func TestHubServeEvents(t *testing.T) {
	h := newHub()
	srv := httptest.NewServer(http.HandlerFunc(h.serveEvents))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: want text/event-stream, got %q", ct)
	}

	events := make(chan string)

	go func() {
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			if e, ok := strings.CutPrefix(s.Text(), "event: "); ok {
				events <- e
			}
		}

		// The stream only ends when the test cancels it, so an error here
		// says nothing about the server.
		_ = s.Err()

		close(events)
	}()

	next := func() string {
		select {
		case e := <-events:
			return e
		case <-ctx.Done():
			t.Fatal("timed out waiting for an event")
			return ""
		}
	}

	// A graph event when there is a graph, then the status it came with.
	h.publish([]byte(`{"n":1}`), []byte(`{"at":"now"}`))

	if got := []string{next(), next()}; !cmp.Equal(got, []string{"graph", "status"}) {
		t.Errorf("first read: want graph then status, got %v", got)
	}

	// The same graph again sends only its status: nothing moved.
	h.publish([]byte(`{"n":1}`), []byte(`{"at":"later"}`))

	if got := next(); got != "status" {
		t.Errorf("unchanged graph: want only status, got %q", got)
	}

	// A failed read, publishing no graph, keeps the last one and says so.
	h.publish(nil, []byte(`{"at":"later","error":"timeout"}`))

	if got := next(); got != "status" {
		t.Errorf("failed read: want only status, got %q", got)
	}

	if g, _, _ := h.latest(); string(g) != `{"n":1}` {
		t.Errorf("failed read: want the last graph kept, got %s", g)
	}
}

func TestEmbeddedWebView(t *testing.T) {
	site, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		t.Fatalf("fs.Sub(...): %v", err)
	}

	page, err := fs.ReadFile(site, "index.html")
	if err != nil {
		t.Fatalf("the web view isn't embedded; run go generate: %v", err)
	}

	if !strings.Contains(string(page), "crossplane-graph") {
		t.Errorf("index.html doesn't look like the web view:\n%s", page)
	}
}
