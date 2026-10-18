<script>
  /**
   * The rails between resources, drawn for a layered graph.
   *
   * Scene's own Edge picks each end's face from the angle between the two
   * boxes, which suits a free-form diagram. Here every box sits in a column,
   * and a dependency always runs to a later column, so a steep diagonal would
   * leave through a box's bottom and cut through its neighbours. Instead each
   * rail leaves the right face of what is depended on and enters the left
   * face of what depends on it, as a curve that runs horizontally into both,
   * and its arrowhead touches the box it points at.
   *
   * It draws from the boxes Scene measures, so it must sit inside a Scene,
   * and draws its own layer beneath the nodes.
   */
  import { getScene } from './lib/xplanet/scene-context.js';
  import { centerOf, shareFraction } from './lib/xplanet/geometry.js';

  /** wires: [{ from, to, tone, dashed }]; columns: [[name, ...], ...], left
   *  to right, so a rail that skips a column can be routed around its boxes. */
  let { wires = [], columns = [] } = $props();

  const scene = getScene();
  const uid = $props.id();
  const TONES = ['idle', 'ready', 'active', 'error'];
  /** Length of the straight run into a box, in pixels: longer than the
   *  9px arrowhead, so the head lies along it. */
  const STRAIGHT = 14;
  /** How far below a column's last box (or above a lone box) a rail that
   *  skips the column passes. */
  const GAP_CLEARANCE = 14;
  /** Distance between rails sharing a gap, at most. */
  const LANE = 5;

  /** Where every rail meets every box.
   *
   *  Routes are worked out first, then attachment points: ends sharing a face
   *  are spread along it in the order of the height each rail actually
   *  arrives from or leaves toward - the gap it threads, for a rail that
   *  skips a column, rather than the box at its far end - so neighbouring
   *  rails don't cross. */
  const routed = $derived.by(() => {
    const rects = scene.rects;
    const live = wires.filter((w) => rects[w.from] && rects[w.to]).map((w) => ({ ...w }));
    const columnOf = new Map(columns.flatMap((names, i) => names.map((n) => [n, i])));

    /** Where a rail crosses column i: straight through the gap between two
     *  boxes nearest to height y, from the column's left edge to its right.
     *  Above the top box only when the column has a single box. */
    function passage(i, y) {
      const boxes = columns[i].map((n) => rects[n]).filter(Boolean).sort((p, q) => p.y - q.y);
      if (boxes.length === 0) return null;
      const left = Math.min(...boxes.map((r) => r.x));
      const right = Math.max(...boxes.map((r) => r.x + r.width));
      const gaps = boxes.slice(1).map((r, k) => (boxes[k].y + boxes[k].height + r.y) / 2);
      // Below the last box always works; above the first would cross the
      // column's heading, so it's only a fallback for a lone box.
      gaps.push(boxes.at(-1).y + boxes.at(-1).height + GAP_CLEARANCE);
      if (boxes.length === 1) gaps.push(boxes[0].y - GAP_CLEARANCE);
      const gy = gaps.reduce((best, g) => (Math.abs(g - y) < Math.abs(best - y) ? g : best));
      return { column: i, left, right, y: gy };
    }

    /** A curve between two points that leaves and arrives horizontally. */
    function curve(p, q) {
      const dir = q.x >= p.x ? 1 : -1;
      const reach = Math.max(12, Math.abs(q.x - p.x) / 2);
      return `C ${p.x + dir * reach} ${p.y}, ${q.x - dir * reach} ${q.y}, ${q.x} ${q.y}`;
    }

    // Routes. A rail that skips columns threads each one it crosses, aiming
    // for the gap nearest where a straight line between the two boxes would
    // pass, rather than running through the boxes in between.
    for (const w of live) {
      const a = rects[w.from];
      const b = rects[w.to];
      // Forward, the usual case; backward only on a cycle's column.
      const forward = centerOf(b).x >= centerOf(a).x;
      w.out = forward ? 'right' : 'left';
      w.in = forward ? 'left' : 'right';
      w.via = [];

      const ci = columnOf.get(w.from);
      const cj = columnOf.get(w.to);
      if (!forward || ci === undefined || cj === undefined) continue;

      const from = { x: a.x + a.width, y: centerOf(a).y };
      const to = { x: b.x, y: centerOf(b).y };
      for (let k = ci + 1; k < cj; k += 1) {
        const left = Math.min(...columns[k].map((n) => rects[n]?.x ?? Infinity));
        const t = Math.min(Math.max((left - from.x) / Math.max(to.x - from.x, 1), 0), 1);
        const p = passage(k, from.y + (to.y - from.y) * t);
        if (p) w.via.push(p);
      }
    }

    // Rails threading the same gap get a lane each, a few pixels apart, in
    // the order of where they come from, so two long rails don't draw as one.
    const lanes = new Map();
    for (const w of live) {
      for (const p of w.via) {
        const key = `${p.column}|${p.y}`;
        if (!lanes.has(key)) lanes.set(key, []);
        lanes.get(key).push({ w, p });
      }
    }
    for (const group of lanes.values()) {
      if (group.length < 2) continue;
      group.sort(
        (m, n) =>
          centerOf(rects[m.w.from]).y - centerOf(rects[n.w.from]).y ||
          centerOf(rects[m.w.to]).y - centerOf(rects[n.w.to]).y,
      );
      const spacing = Math.min(LANE, (GAP_CLEARANCE * 1.4) / group.length);
      group.forEach(({ p }, i) => {
        p.y += (i - (group.length - 1) / 2) * spacing;
      });
    }

    // Attachment points, in the order of where each rail heads.
    const ends = new Map();
    function add(key, end) {
      if (!ends.has(key)) ends.set(key, []);
      ends.get(key).push(end);
    }
    for (const w of live) {
      const leaving = w.via.length ? w.via[0].y : centerOf(rects[w.to]).y;
      const arriving = w.via.length ? w.via.at(-1).y : centerOf(rects[w.from]).y;
      add(`${w.from}|${w.out}`, { w, which: 'a', toward: leaving });
      add(`${w.to}|${w.in}`, { w, which: 'b', toward: arriving });
    }

    const points = new Map();
    for (const [key, group] of ends) {
      const [id, side] = key.split('|');
      const r = rects[id];
      group.sort((p, q) => p.toward - q.toward);
      group.forEach((end, i) => {
        // Kept clear of the corners, where a rail would graze the border.
        const f = 0.2 + 0.6 * shareFraction(i, group.length);
        points.set(`${end.w.from}->${end.w.to}|${end.which}`, {
          x: side === 'right' ? r.x + r.width : r.x,
          y: r.y + r.height * f,
        });
      });
    }

    return live.map((w) => {
      const a = points.get(`${w.from}->${w.to}|a`);
      const b = points.get(`${w.from}->${w.to}|b`);
      // The rail ends in a straight run a little longer than the arrowhead,
      // so the head, which points along the rail's last few pixels, sits on
      // a stretch heading the same way rather than on the curve's turn. It
      // stops a hair short so the head's tip sits on the border.
      const into = w.in === 'left' ? 1 : -1;
      const out = w.out === 'right' ? 1 : -1;
      const bx = b.x - into;
      const run = { x: bx - into * STRAIGHT, y: b.y };
      const start = { x: a.x + out * 4, y: a.y };

      let d = `M ${a.x} ${a.y} L ${start.x} ${start.y} `;
      let at = start;
      for (const p of w.via) {
        d += `${curve(at, { x: p.left - 4, y: p.y })} L ${p.right + 4} ${p.y} `;
        at = { x: p.right + 4, y: p.y };
      }

      d += `${curve(at, run)} L ${bx} ${b.y}`;
      return { ...w, d };
    });
  });
</script>

<svg class="rails" aria-hidden="true">
  <defs>
    <!-- One marker per tone, filled to match, rather than context-stroke,
         which not every engine draws. Sized in pixels, not stroke widths, so
         a hairline rail still gets a head you can see. -->
    {#each TONES as tone (tone)}
      <marker
        id="{uid}-head-{tone}"
        viewBox="0 0 10 10"
        refX="10"
        refY="5"
        markerUnits="userSpaceOnUse"
        markerWidth="9"
        markerHeight="9"
        orient="auto"
      >
        <path class="head tone-{tone}" d="M 0 1 L 10 5 L 0 9 z" />
      </marker>
    {/each}
  </defs>

  {#each routed as rail (`${rail.from}->${rail.to}`)}
    <path
      class="rail tone-{rail.tone}"
      class:dashed={rail.dashed}
      d={rail.d}
      marker-end="url(#{uid}-head-{rail.tone})"
    />
  {/each}
</svg>

<style>
  .rails {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
    pointer-events: none;
  }

  .rail {
    fill: none;
    stroke-width: 1.25px;
    stroke-linecap: round;
    transition: stroke 200ms ease-out;
  }
  .rail.dashed { stroke-dasharray: 3 4; }

  /* The inks of XPlanet's Scene connectors, so the rails read as its own. */
  .rail.tone-idle { stroke: oklch(72% 0.135 238 / 0.7); }
  .rail.tone-active { stroke: oklch(84% 0.15 235); stroke-width: 1.75px; }
  .rail.tone-ready { stroke: oklch(74% 0.15 155 / 0.75); }
  .rail.tone-error { stroke: oklch(72% 0.19 22 / 0.85); }

  .head.tone-idle { fill: oklch(72% 0.135 238 / 0.85); }
  .head.tone-active { fill: oklch(84% 0.15 235); }
  .head.tone-ready { fill: oklch(74% 0.15 155 / 0.9); }
  .head.tone-error { fill: oklch(72% 0.19 22 / 0.9); }
</style>
