<script>
  /**
   * The coordinate space connectors live in.
   *
   * Nodes are ordinary HTML laid out however the diagram wants — grid, flex,
   * whatever reads best — and register themselves by name with `use:anchor`.
   * Scene measures them and exposes their boxes in scene coordinates, so an
   * <Edge from="crossplane" to="function"> never needs to know where either
   * end actually sits. That is what makes a new diagram topology cost no new
   * CSS: move the nodes around and the wiring follows.
   */
  import { setScene } from './scene-context.js';
  import {
    pathBetween,
    centerOf,
    sideFor,
    attachPoint,
    shareFraction,
    linePath,
    arcPath,
    unionRect,
  } from './geometry.js';

  let {
    children,
    height = null,
    label = null,
    class: className = '',
    /** Room inside the scene box, so a group field has space to sit outside
     *  the nodes it encloses without being clipped. */
    pad = '0',
  } = $props();

  /* Marker ids are document-global, so each Scene needs its own or the first
     one on the page wins for everybody. */
  const uid = $props.id();

  let sceneEl = $state(null);
  let rects = $state({});
  let edges = $state({});
  let groups = $state({});

  /** Live elements, kept out of $state: we react to their measured boxes, not
   *  to the nodes themselves. */
  const elements = new Map();
  let observer = null;
  let pending = 0;

  function measure() {
    pending = 0;
    if (!sceneEl) return;
    const base = sceneEl.getBoundingClientRect();
    const next = {};
    for (const [id, el] of elements) {
      const box = el.getBoundingClientRect();
      next[id] = {
        x: box.left - base.left,
        y: box.top - base.top,
        width: box.width,
        height: box.height,
      };
    }
    rects = next;
  }

  /** Coalesce the burst of measure requests that a mount or resize produces. */
  function scheduleMeasure() {
    if (pending || typeof requestAnimationFrame === 'undefined') return;
    pending = requestAnimationFrame(measure);
  }

  const scene = {
    get rects() { return rects; },

    registerAnchor(id, el) {
      elements.set(id, el);
      observer?.observe(el);
      scheduleMeasure();
      return () => {
        elements.delete(id);
        observer?.unobserve(el);
        scheduleMeasure();
      };
    },

    setGroup(id, descriptor) {
      groups[id] = { id, ...descriptor };
    },

    removeGroup(id) {
      delete groups[id];
    },

    setEdge(id, descriptor) {
      edges[id] = { id, ...descriptor };
    },

    removeEdge(id) {
      delete edges[id];
    },

    /** The route between two anchors, or null while either is unmeasured.
     *  Looks up the shared routing table so a packet rides exactly the line
     *  its rail is drawn on; the reverse direction reuses the same route,
     *  since a response travels the wire its request came in on. */
    pathFor(fromId, toId, bow = 0) {
      const forward = routes.get(`${fromId}->${toId}`);
      if (forward) return buildPath(forward.a, forward.b, bow || forward.bow);
      const reverse = routes.get(`${toId}->${fromId}`);
      if (reverse) return buildPath(reverse.b, reverse.a, bow || reverse.bow);
      return pathBetween(rects[fromId], rects[toId], bow);
    },

    remeasure: scheduleMeasure,
  };

  setScene(scene);

  $effect(() => {
    observer = new ResizeObserver(scheduleMeasure);
    observer.observe(sceneEl);
    for (const el of elements.values()) observer.observe(el);

    // Fonts landing after first paint change every box; measure again when
    // they do, rather than leaving connectors pointing at stale positions.
    document.fonts?.ready.then(scheduleMeasure);
    window.addEventListener('resize', scheduleMeasure);
    measure();

    return () => {
      observer.disconnect();
      observer = null;
      window.removeEventListener('resize', scheduleMeasure);
      if (pending) cancelAnimationFrame(pending);
    };
  });

  function buildPath(a, b, bow) {
    return bow ? arcPath(a, b, bow) : linePath(a, b);
  }

  /**
   * Where every connector meets every box.
   *
   * Routing is worked out for the whole scene at once rather than per edge,
   * because the right attachment point depends on the connector's siblings: a
   * lone connector takes the middle of the face it leaves by, while several
   * sharing a face spread evenly along it, pushing the outermost toward the
   * corners. Done per edge, each one would independently claim the middle and
   * they would all pile up on the same point.
   */
  const routes = $derived.by(() => {
    const live = Object.values(edges).filter((edge) => rects[edge.from] && rects[edge.to]);

    // One entry per edge *end*, since the two ends are placed independently.
    const ends = live.flatMap((edge) => [
      { edge, node: edge.from, other: edge.to, which: 'a' },
      { edge, node: edge.to, other: edge.from, which: 'b' },
    ]);

    const faces = new Map();
    for (const end of ends) {
      end.side = sideFor(rects[end.node], centerOf(rects[end.other]));
      const key = `${end.node}|${end.side}`;
      if (!faces.has(key)) faces.set(key, []);
      faces.get(key).push(end);
    }

    const placed = new Map();
    for (const group of faces.values()) {
      const horizontal = group[0].side === 'left' || group[0].side === 'right';
      // Order by where the far end sits, so sibling rails never cross.
      group.sort((p, q) => {
        const pc = centerOf(rects[p.other]);
        const qc = centerOf(rects[q.other]);
        return horizontal ? pc.y - qc.y : pc.x - qc.x;
      });
      group.forEach((end, index) => {
        placed.set(
          `${end.edge.id}|${end.which}`,
          attachPoint(rects[end.node], end.side, shareFraction(index, group.length)),
        );
      });
    }

    const table = new Map();
    for (const edge of live) {
      table.set(`${edge.from}->${edge.to}`, {
        edge,
        a: placed.get(`${edge.id}|a`),
        b: placed.get(`${edge.id}|b`),
        bow: edge.bow ?? 0,
      });
    }
    return table;
  });

  const fields = $derived(
    Object.values(groups)
      .map((group) => ({
        ...group,
        box: unionRect(
          group.nodes.map((nodeId) => rects[nodeId]),
          group.pad,
        ),
      }))
      .filter((group) => group.box),
  );

  const wires = $derived(
    [...routes.values()].map((route) => ({
      ...route.edge,
      path: buildPath(route.a, route.b, route.bow),
    })),
  );
</script>

<div
  class="scene {className}"
  style="{height ? `min-height: ${height};` : ''} padding: {pad}"
  bind:this={sceneEl}
  role={label ? 'img' : undefined}
  aria-label={label}
>
  <!-- Rails sit under the nodes so a connector never covers a label. -->
  <svg class="wires" aria-hidden="true">
    <defs>
      <marker
        id="{uid}-arrow"
        viewBox="0 0 8 8"
        refX="7"
        refY="4"
        markerWidth="6"
        markerHeight="6"
        orient="auto-start-reverse"
      >
        <!-- auto-start-reverse lets this single marker serve both ends: at
             marker-start it flips to point back the way it came. -->
        <path class="arrowhead" d="M 0 1 L 7 4 L 0 7 z" />
      </marker>
    </defs>

    {#each fields as field (field.id)}
      <rect
        class="field"
        x={field.box.x}
        y={field.box.y}
        width={field.box.width}
        height={field.box.height}
      />
    {/each}

    {#each wires as wire (wire.id)}
      <path
        class="wire tone-{wire.tone ?? 'idle'}"
        class:dashed={wire.dashed}
        d={wire.path.d}
        marker-start={wire.arrow === 'both' || wire.arrow === 'start'
          ? `url(#${uid}-arrow)`
          : undefined}
        marker-end={wire.arrow === true || wire.arrow === 'end' || wire.arrow === 'both'
          ? `url(#${uid}-arrow)`
          : undefined}
      />
    {/each}
  </svg>

  {@render children?.()}

  {#each fields as field (field.id)}
    {#if field.label}
      <span
        class="field-label"
        style="transform: translate3d({field.box.x + field.box.width / 2}px, {field.box.y}px, 0)"
      >{field.label}</span>
    {/if}
  {/each}
</div>

<style>
  .scene {
    position: relative;
    /* Isolate layout: the rAF loop above should never be able to invalidate
       the article around it. Paint containment is deliberately not used — it
       would clip a group field, which sits outside the nodes it encloses. */
    contain: layout style;
  }

  .wires {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
    pointer-events: none;
  }

  /* Same weight and ink as the Mermaid rails in src/styles/diagrams.css, so a
     hand-built scene and a generated diagram read as one language. */
  .wire {
    fill: none;
    stroke-width: 0.9px;
    stroke-linecap: round;
  }
  .wire.dashed { stroke-dasharray: 2 3; }

  /* Same ruled field as a Mermaid subgraph in src/styles/diagrams.css. */
  .field {
    fill: none;
    stroke: oklch(70% 0.05 254 / 0.45);
    stroke-width: 0.75px;
    stroke-dasharray: 2 3;
  }

  .field-label {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 1;
    translate: -50% -50%;
    padding: 0 0.4rem;
    font-family: var(--font-display);
    font-size: 0.6875rem;
    color: var(--color-accent-300);
    /* Sat on the plate so the dashed rule does not run through the words. */
    background: oklch(15.5% 0.070 265);
    white-space: nowrap;
    pointer-events: none;
  }

  /* context-stroke makes the head take its wire's colour, so an active or
     error edge keeps its arrow in step. Fallback first for older engines. */
  .arrowhead {
    fill: oklch(72% 0.135 238 / 0.7);
    fill: context-stroke;
  }

  /* Idle is the same ink as a Mermaid edge in src/styles/diagrams.css, so a
     hand-built scene and a generated diagram draw their connectors alike.
     It was previously near-greyscale and read as grey next to the boxes. */
  .tone-idle { stroke: oklch(72% 0.135 238 / 0.7); }
  .tone-active { stroke: oklch(82% 0.15 235 / 0.95); }
  .tone-ready { stroke: oklch(74% 0.15 155 / 0.65); }
  .tone-error { stroke: oklch(72% 0.19 22 / 0.8); }
</style>
