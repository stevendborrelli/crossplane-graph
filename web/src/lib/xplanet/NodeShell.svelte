<script>
  /**
   * The one box every object primitive is built from.
   *
   * It owns three things so that PodNode, ResourceNode and ExternalNode never
   * have to: registering itself as a scene anchor, carrying the status colour
   * on a left border, and staying above the connector layer. Anything visually
   * shared between object types belongs here, so that a change to how "busy"
   * reads lands on every diagram at once.
   */
  import { getScene } from './scene-context.js';

  let {
    id,
    /** Omit for a static diagram: with no simulation there is no state to
     *  report, and an always-grey mark is just noise. */
    status = null,
    title,
    subtitle = null,
    badge = null,
    /** Renders as a button when set, for fault injection and the like. */
    onclick = null,
    ariaLabel = null,
    /** What kind of thing this is. Drives the ornament, never the colour —
     *  colour belongs to status alone. */
    role = 'managed',
    /** Rows for a record box: [{ name, type, note? }]. */
    fields = null,
    children,
    class: className = '',
  } = $props();

  const scene = getScene();
  let el = $state(null);

  $effect(() => (el ? scene.registerAnchor(id, el) : undefined));
</script>

<svelte:element
  this={onclick ? 'button' : 'div'}
  bind:this={el}
  class="node role-{role} {status ? `status-${status}` : ''} {className}"
  class:interactive={onclick}
  type={onclick ? 'button' : undefined}
  {onclick}
  aria-label={ariaLabel}
>
  <span class="head">
    {#if status}<span class="mark" aria-hidden="true"></span>{/if}
    <span class="title">{title}</span>
    {#if badge}<span class="badge">{badge}</span>{/if}
  </span>
  {#if subtitle}<span class="subtitle">{subtitle}</span>{/if}

  {#if fields}
    <span class="fields">
      {#each fields as field (field.name)}
        <span class="field">
          <span class="fname">{field.name}</span>
          <span class="ftype">{field.type}{#if field.note}<span class="fnote">{field.note}</span>{/if}</span>
        </span>
      {/each}
    </span>
  {/if}

  {#if children}<span class="foot">{@render children()}</span>{/if}
</svelte:element>

<style>
  /* Engraved, to match the Mermaid plates: no fill, so the graticule and
     starfield read straight through the node, and one calm hairline border.

     Object class is drawn as an ornament on the box — a stack, a double rule,
     a concentric ring — rather than as a colour, because colour already means
     status here and an engraved chart distinguishes classes by mark, not hue. */
  .node {
    position: relative;
    /* Above the wire layer, so rails pass behind the box rather than over it. */
    z-index: 1;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.6875rem;
    text-align: left;
    font: inherit;
    color: inherit;
    background: none;
    border: 1px solid oklch(80% 0.110 232 / 0.8);
    border-radius: 2px;
    transition: border-color 220ms ease-out, background-color 220ms ease-out;
  }

  /* A pod is a running process: a seam ruled across the top, the way a
     container is drawn on a plate. */
  .role-pod::before,
  .role-core::before,
  .role-function::before,
  .role-provider::before {
    content: '';
    position: absolute;
    inset: 0.1875rem 0.1875rem auto;
    height: 1px;
    background: oklch(80% 0.110 232 / 0.62);
  }

  /* A composite is made of other resources, so it is drawn as a stack. */
  .role-composite::after {
    content: '';
    position: absolute;
    inset: -0.25rem -0.25rem 0.25rem 0.25rem;
    z-index: -1;
    border: 1px solid oklch(80% 0.110 232 / 0.45);
    border-radius: 2px;
    pointer-events: none;
  }

  /* A managed resource stands for something outside the cluster: a second
     rule inside its leading edge, the way a chart doubles a boundary line. */
  .role-managed::after {
    content: '';
    position: absolute;
    top: 0.1875rem;
    bottom: 0.1875rem;
    left: 0.1875rem;
    width: 1px;
    background: oklch(80% 0.110 232 / 0.6);
    pointer-events: none;
  }

  /* The core pod runs the loop everything else hangs off: concentric rule. */
  .role-core::after {
    content: '';
    position: absolute;
    inset: -0.25rem;
    border: 1px solid oklch(80% 0.110 232 / 0.45);
    border-radius: 3px;
    pointer-events: none;
  }

  /* role-plain carries no ornament: for anything that is not a pod, a
     resource or an external system — a payload, a stage, a label. */

  /* A message is a record: its name sits above a rule, like a table head. */
  .role-message .subtitle,
  .role-message .head {
    padding-bottom: 0.3rem;
  }
  .role-message .fields {
    display: flex;
    flex-direction: column;
    margin-top: 0.1rem;
    border-top: 1px solid oklch(80% 0.110 232 / 0.35);
  }
  .field {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.15rem 0;
    font-family: var(--font-mono);
    font-size: 0.5625rem;
    line-height: 1.35;
  }
  .field + .field { border-top: 1px solid oklch(80% 0.110 232 / 0.14); }
  .fname { color: var(--color-base-100); }
  .ftype { color: var(--color-base-500); text-align: right; }
  /* Margin rather than a space character: JSX/Svelte collapse the leading
     whitespace in a span, which glued the note to the type name. */
  .fnote { margin-left: 0.4em; color: oklch(76% 0.13 75 / 0.8); }

  /* Outside the cluster: dashed, like a Mermaid subgraph field. */
  .role-external { border-style: dashed; }

  /* Status is a small filled mark beside the name, the way a chart marks a
     variable star. Tinting whole borders turns a dense diagram into a bank of
     traffic lights and buries the one state that wants attention, so only an
     error restates itself on the box. */
  /* The mark says whether this node is doing work right now; its colour says
     what state it is in. A caret for running, pause bars for stopped — the
     transport idiom, which readers already know — rather than a coloured dot,
     which is the house vocabulary of every dashboard and relates to nothing
     else on the plate. The caret also matches the arrowheads on the rails. */
  .mark {
    flex: none;
    width: 0.375rem;
    height: 0.4375rem;
    background: var(--node-ink);
    clip-path: polygon(0 0, 100% 50%, 0 100%);
    transition: background-color 220ms ease-out;
  }

  /* Finished: a filled block. Completing the transport set — play, pause,
     stop — so "done" is distinguishable from "not started yet" by shape and
     not by colour alone, which also survives a colour-blind reader. */
  .status-ready .mark {
    clip-path: none;
    height: 0.375rem;
    background: var(--node-ink);
  }

  /* Stopped but not finished: two bars. Drawn as a gradient because clip-path
     cannot cut one element into two separate shapes. */
  .status-idle .mark,
  .status-error .mark {
    clip-path: none;
    background: linear-gradient(
      90deg,
      var(--node-ink) 0 34%,
      transparent 34% 66%,
      var(--node-ink) 66% 100%
    );
  }

  .status-idle { --node-ink: oklch(70% 0.05 254 / 0.5); }
  .status-busy { --node-ink: oklch(80% 0.13 75); }
  .status-ready { --node-ink: oklch(72% 0.15 155); }
  .status-error { --node-ink: oklch(70% 0.19 22); }

  .status-error { border-color: oklch(70% 0.19 22 / 0.7); }

  .interactive { cursor: pointer; }
  .interactive:hover { background-color: oklch(80% 0.110 232 / 0.06); }
  .interactive:focus-visible { outline: 1px solid var(--color-accent-300); outline-offset: 3px; }

  .head {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .title {
    margin-right: auto;
    font-family: var(--font-display);
    font-size: 0.8125rem;
    color: var(--color-base-50);
  }

  /* Names in the display face, technical identifiers in mono — the same
     hierarchy the diagrams use for node names versus wire labels. */
  .subtitle {
    font-family: var(--font-mono);
    font-size: 0.625rem;
    color: var(--color-base-400);
  }

  .badge {
    flex: none;
    padding: 0 0.3rem;
    font-family: var(--font-mono);
    font-size: 0.5625rem;
    letter-spacing: 0.06em;
    border: 1px solid oklch(70% 0.19 22 / 0.7);
    border-radius: 1px;
    color: oklch(80% 0.14 22);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.3rem;
    min-height: 1.125rem;
  }

  @media (prefers-reduced-motion: reduce) {
    .node, .mark { transition-duration: 0ms; }
  }
</style>
