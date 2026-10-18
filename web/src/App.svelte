<script>
  /**
   * The live page: one composite's ordering graph, redrawn whenever
   * crossplane-graph reads it and something has moved.
   *
   * The server streams two kinds of event. A graph event carries the whole
   * graph and arrives only when it changes; a status event arrives on every
   * read and says when it was taken and whether it failed. A failed read
   * keeps the last good graph on screen, marked stale, rather than blanking
   * it - mid-teardown the API server is often the busiest thing there is.
   */
  import GraphView from './GraphView.svelte';
  import { STATES } from './layout.js';

  let graph = $state(null);
  let status = $state(null);
  let connected = $state(false);
  let selected = $state(null);
  let now = $state(Date.now());

  $effect(() => {
    const events = new EventSource('api/events');
    events.addEventListener('graph', (e) => {
      graph = JSON.parse(e.data);
      if (selected && !graph.nodes.some((n) => n.name === selected)) selected = null;
    });
    events.addEventListener('status', (e) => (status = JSON.parse(e.data)));
    events.onopen = () => (connected = true);
    // EventSource reconnects by itself; this only says so meanwhile.
    events.onerror = () => (connected = false);

    const tick = setInterval(() => (now = Date.now()), 1000);
    return () => {
      events.close();
      clearInterval(tick);
    };
  });

  const age = $derived(status ? Math.max(0, Math.round((now - Date.parse(status.at)) / 1000)) : null);
  const stale = $derived(!connected || Boolean(status?.error) || (age !== null && age > 15));

  const where = $derived(
    graph ? (graph.xr.namespace ? `${graph.xr.namespace}/${graph.xr.name}` : graph.xr.name) : '',
  );

  const detail = $derived(graph && selected ? graph.nodes.find((n) => n.name === selected) : null);
  const dependents = $derived(
    graph && selected ? graph.nodes.filter((n) => n.dependsOn.includes(selected)).map((n) => n.name) : [],
  );
  const deadlocked = $derived(graph ? graph.nodes.filter((n) => n.state === 'deadlocked') : []);

  $effect(() => {
    document.title = graph ? `${graph.xr.kind} ${where} · crossplane-graph` : 'crossplane-graph';
  });
</script>

<main>
  <header>
    {#if graph}
      <div class="title">
        <h1><span class="kind">{graph.xr.kind}</span> {where}</h1>
        {#if graph.xr.ready}
          <span class="pill" class:ok={graph.xr.ready === 'True'}>Ready={graph.xr.ready}</span>
        {/if}
        {#if graph.xr.deleting}<span class="pill warn">deleting</span>{/if}
        {#if graph.gone}<span class="pill">not found</span>{/if}
      </div>
      {#if !graph.gone}
        <p class="summary">
          {graph.nodes.length} composed resources · {graph.edges} edges · {graph.waves} waves
        </p>
        <p class="tally">
          {#each STATES as s (s)}
            {#if graph.counts[s]}<span class="state-{s}">{graph.counts[s]} {s}</span>{/if}
          {/each}
        </p>
      {/if}
    {:else}
      <h1>crossplane-graph</h1>
    {/if}

    <p class="freshness" class:stale>
      {#if !connected}
        reconnecting to crossplane-graph…
      {:else if status?.error}
        last read failed: {status.error}
      {:else if age !== null}
        read {age}s ago
      {:else}
        reading…
      {/if}
    </p>
  </header>

  {#if graph?.gone}
    <p class="gone">
      This composite doesn't exist: it has been deleted, or hasn't been created yet. The graph will
      appear here when it is.
    </p>
  {:else if graph}
    {#if graph.edges === 0}
      <p class="note">
        No ordering constraints. Either the pipeline declares none, or this Crossplane doesn't
        support them (<code>--enable-composed-resource-ordering</code>).
      </p>
    {/if}

    {#if deadlocked.length > 0}
      <section class="callout error">
        <h2>Deadlocked, so waiting will not resolve these</h2>
        <ul>
          {#each deadlocked as n (n.name)}<li><strong>{n.name}</strong> {n.reason}</li>{/each}
        </ul>
      </section>
    {/if}

    {#if graph.cyclic?.length > 0}
      <section class="callout error">
        <h2>Cycle, so these can never be created or deleted in order</h2>
        <p>{graph.cyclic.join(', ')}</p>
      </section>
    {/if}

    <div class="body" class:stale>
      <GraphView {graph} {selected} onselect={(name) => (selected = name)} />
    </div>

    {#if detail}
      <aside class="detail">
        <button class="close" onclick={() => (selected = null)} aria-label="Close">×</button>
        <h2>{detail.name}</h2>
        <dl>
          <dt>state</dt><dd class="state-{detail.state}">{detail.state}</dd>
          {#if detail.reason}<dt>reason</dt><dd>{detail.reason}</dd>{/if}
          <dt>kind</dt><dd>{detail.apiVersion} {detail.kind}</dd>
          {#if detail.object}
            <dt>object</dt><dd>{detail.namespace ? `${detail.namespace}/` : ''}{detail.object}</dd>
          {/if}
          <dt>wave</dt><dd>{detail.wave < 0 ? 'on a cycle' : detail.wave}</dd>
          <dt>depends on</dt><dd>{detail.dependsOn.join(', ') || '—'}</dd>
          <dt>depended on by</dt><dd>{dependents.join(', ') || '—'}</dd>
          {#if detail.requires.length > 0}
            <dt>requires</dt><dd>{detail.requires.join(', ')}</dd>
          {/if}
        </dl>
      </aside>
    {/if}

    <p class="legend">
      Rails run from what a resource depends on to the resource: left to right is the order
      Crossplane creates them, right to left the order it deletes them. Green once the dependency is
      ready; dashed while the dependent waits to be created. Click a resource for its details.
    </p>
  {/if}
</main>

<style>
  main {
    padding: 1.5rem clamp(1rem, 3vw, 2.5rem) 3rem;
  }

  header { margin-bottom: 1.5rem; }

  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.75rem;
  }

  h1 {
    margin: 0;
    font-family: var(--font-display);
    font-size: 1.5rem;
    font-weight: 500;
    color: var(--color-base-50);
  }
  .kind { color: var(--color-accent-300); }

  .pill {
    padding: 0 0.4rem;
    font-family: var(--font-mono);
    font-size: 0.6875rem;
    border: 1px solid oklch(70% 0.05 254 / 0.5);
    border-radius: 1px;
    color: var(--color-base-300);
  }
  .pill.ok { border-color: var(--ink-ready); color: var(--ink-ready); }
  .pill.warn { border-color: var(--ink-busy); color: var(--ink-busy); }

  .summary, .tally, .freshness, .legend, .note {
    margin: 0.4rem 0 0;
    font-family: var(--font-mono);
    font-size: 0.75rem;
  }
  .summary { color: var(--color-base-400); }
  .tally { display: flex; flex-wrap: wrap; gap: 0 0.9rem; }
  .freshness { color: var(--color-base-500); }
  .freshness.stale { color: var(--ink-busy); }

  .state-ready { color: var(--ink-ready); }
  .state-creating, .state-deleting { color: var(--ink-busy); }
  .state-pending, .state-held, .state-deleted { color: var(--ink-idle); }
  .state-deadlocked { color: var(--ink-error); }

  .note { margin-bottom: 1rem; color: var(--ink-busy); }
  code { font-family: var(--font-mono); }

  .callout {
    margin: 0 0 1.25rem;
    padding: 0.6rem 0.9rem;
    border: 1px solid oklch(70% 0.19 22 / 0.7);
    border-radius: 2px;
    font-size: 0.8125rem;
  }
  .callout h2 {
    margin: 0 0 0.3rem;
    font-family: var(--font-display);
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--ink-error);
  }
  .callout ul, .callout p { margin: 0; padding-left: 1rem; }

  .body { transition: opacity 200ms ease-out; }
  .body.stale { opacity: 0.6; }

  .detail {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    z-index: 10;
    width: min(24rem, calc(100vw - 2rem));
    padding: 0.9rem 1rem;
    background: oklch(18% 0.075 265 / 0.95);
    border: 1px solid oklch(80% 0.110 232 / 0.6);
    border-radius: 2px;
    font-size: 0.8125rem;
  }
  .detail h2 {
    margin: 0 1.5rem 0.6rem 0;
    font-family: var(--font-display);
    font-size: 1rem;
    font-weight: 500;
    color: var(--color-base-50);
    overflow-wrap: anywhere;
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 0.3rem 0.9rem;
    margin: 0;
  }
  dt { font-family: var(--font-mono); font-size: 0.6875rem; color: var(--color-base-500); }
  dd { margin: 0; overflow-wrap: anywhere; }
  .close {
    position: absolute;
    top: 0.4rem;
    right: 0.6rem;
    font-size: 1.1rem;
    color: var(--color-base-400);
    background: none;
    border: none;
    cursor: pointer;
  }
  .close:hover { color: var(--color-base-50); }

  .gone {
    margin: 2rem 0;
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    color: var(--color-base-400);
  }

  .legend { max-width: 52rem; color: var(--color-base-500); line-height: 1.5; }
</style>
