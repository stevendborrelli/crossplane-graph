<script>
  /**
   * The graph as a plate: a column per wave, a box per composed resource, and
   * a rail for each dependency, running from what is depended on to what
   * depends on it - left to right, the order Crossplane creates them in, and
   * right to left the order it deletes them in.
   */
  import Scene from './lib/xplanet/Scene.svelte';
  import NodeShell from './lib/xplanet/NodeShell.svelte';
  import Wires from './Wires.svelte';
  import { columnsFor, markFor } from './layout.js';

  let { graph, selected = null, onselect } = $props();

  const byName = $derived(new Map(graph.nodes.map((n) => [n.name, n])));
  const columns = $derived(columnsFor(graph.nodes));

  /** Only edges between resources the XR references can be drawn; an edge
   *  to anything else has no box to land on. */
  const wires = $derived(
    graph.nodes.flatMap((n) =>
      n.dependsOn.filter((d) => byName.has(d)).map((d) => ({ from: d, to: n.name })),
    ),
  );

  /** Lit when it touches the selected resource; red when either end is
   *  deadlocked; green once what is depended on is ready; dashed while the
   *  dependent is still waiting to be created. */
  function tone(wire) {
    if (selected && (wire.from === selected || wire.to === selected)) return 'active';
    const from = byName.get(wire.from);
    const to = byName.get(wire.to);
    if (from.state === 'deadlocked' || to.state === 'deadlocked') return 'error';
    if (from.state === 'ready') return 'ready';
    return 'idle';
  }

  function related(name) {
    if (!selected) return true;
    if (name === selected) return true;
    return wires.some(
      (w) => (w.from === selected && w.to === name) || (w.to === selected && w.from === name),
    );
  }
</script>

<div class="frame">
  <Scene label="Ordering graph of {graph.xr.kind} {graph.xr.name}" pad="1.5rem 1rem" class="graph-scene">
    <Wires
      wires={wires.map((w) => ({ ...w, tone: tone(w), dashed: byName.get(w.to).state === 'pending' }))}
      columns={columns.map((c) => c.names)}
    />

    <div class="columns">
      {#each columns as column (column.key)}
        <section class="column" class:cycle={column.key === 'cycle'}>
          <h2>{column.label}</h2>
          {#each column.names as name (name)}
            {@const n = byName.get(name)}
            <div class="slot" class:faded={!related(name)}>
              <NodeShell
                id={name}
                role="plain"
                status={markFor(n.state)}
                title={name}
                subtitle={n.kind}
                badge={n.state === 'deadlocked' ? 'deadlocked' : null}
                ariaLabel="{name}, {n.kind}, {n.state}"
                onclick={() => onselect(selected === name ? null : name)}
                class={selected === name ? 'selected' : ''}
              >
                <span class="state state-{n.state}">{n.state}</span>
                {#if n.reason}<span class="reason" title={n.reason}>{n.reason}</span>{/if}
              </NodeShell>
            </div>
          {/each}
        </section>
      {/each}
    </div>
  </Scene>
</div>

<style>
  /* Wide graphs scroll sideways inside the frame rather than squeezing every
     box down to an unreadable width. */
  /* The scrollbar is styled so it's drawn all the time: an overlay
     scrollbar only appears while scrolling, and nothing else says the graph
     carries on to the right. */
  .frame {
    overflow-x: auto;
    padding-bottom: 1rem;
    scrollbar-width: thin;
    scrollbar-color: oklch(70% 0.05 254 / 0.6) oklch(22% 0.085 264 / 0.6);
  }

  /* Scene contains its layout, so columns overflowing it wouldn't count
     toward the frame's scrollable area and the frame would never scroll.
     Sizing the scene to its columns lets the overflow happen at the frame. */
  .frame :global(.graph-scene) {
    width: max-content;
    min-width: 100%;
  }

  .columns {
    display: flex;
    gap: 5rem;
    min-width: max-content;
  }

  .column {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    width: 14rem;
  }

  h2 {
    margin: 0 0 0.25rem;
    font-family: var(--font-mono);
    font-size: 0.625rem;
    font-weight: 400;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-base-500);
  }

  .cycle h2 { color: var(--ink-error); }

  .slot { transition: opacity 160ms ease-out; }
  .slot.faded { opacity: 0.35; }

  .slot :global(.node) { width: 100%; }
  .slot :global(.node.selected) {
    border-color: var(--color-accent-300);
    box-shadow: 0 0 0 1px var(--color-accent-300);
  }

  .state {
    font-family: var(--font-mono);
    font-size: 0.5625rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .state-ready { color: var(--ink-ready); }
  .state-creating, .state-deleting { color: var(--ink-busy); }
  .state-pending, .state-held, .state-deleted { color: var(--ink-idle); }
  .state-deadlocked { color: var(--ink-error); }

  /* One line, the rest on hover: a reason is often a whole sentence, and a
     column of wrapped sentences pushes the graph off the screen. */
  .reason {
    min-width: 0;
    overflow: hidden;
    font-size: 0.625rem;
    color: var(--color-base-400);
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
