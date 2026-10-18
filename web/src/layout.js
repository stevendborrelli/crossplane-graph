/**
 * How a graph is laid out on the page: one column per wave, left to right in
 * the order Crossplane creates them, with resources on a cycle in a column of
 * their own at the end.
 *
 * Within a column, resources are ordered by where what they depend on sits,
 * the barycentre heuristic, so edges mostly run straight across rather than
 * crossing. A few passes is plenty for graphs of the size compositions have.
 */

/** Columns of resource names: [{ key, label, names }]. */
export function columnsFor(nodes) {
  const waves = new Map();
  const cyclic = [];

  for (const n of nodes) {
    if (n.wave < 0) {
      cyclic.push(n.name);
      continue;
    }
    if (!waves.has(n.wave)) waves.set(n.wave, []);
    waves.get(n.wave).push(n.name);
  }

  const columns = [...waves.keys()]
    .sort((a, b) => a - b)
    .map((w) => ({ key: `wave-${w}`, label: `wave ${w}`, names: waves.get(w).sort() }));

  order(columns, nodes);

  if (cyclic.length > 0) {
    columns.push({ key: 'cycle', label: 'cycle', names: cyclic.sort() });
  }

  return columns;
}

function order(columns, nodes) {
  const deps = new Map(nodes.map((n) => [n.name, n.dependsOn ?? []]));

  for (let pass = 0; pass < 4; pass += 1) {
    const position = new Map();
    for (const col of columns) {
      col.names.forEach((name, i) => position.set(name, i / Math.max(col.names.length - 1, 1)));
    }

    for (const col of columns.slice(1)) {
      const score = new Map(
        col.names.map((name) => {
          const placed = deps.get(name).filter((d) => position.has(d));
          if (placed.length === 0) return [name, position.get(name)];
          return [name, placed.reduce((sum, d) => sum + position.get(d), 0) / placed.length];
        }),
      );
      col.names.sort((a, b) => score.get(a) - score.get(b) || a.localeCompare(b));
      col.names.forEach((name, i) => position.set(name, i / Math.max(col.names.length - 1, 1)));
    }
  }
}

/** The NodeShell status mark each crossplane-graph state draws with. The
 *  shell has four: working, done, stopped, and stopped for good. */
export function markFor(state) {
  switch (state) {
    case 'ready':
      return 'ready';
    case 'creating':
    case 'deleting':
      return 'busy';
    case 'deadlocked':
      return 'error';
    default:
      return 'idle';
  }
}

/** The order states read in a tally: done, working, waiting, going away. */
export const STATES = ['ready', 'creating', 'pending', 'held', 'deleting', 'deleted', 'deadlocked'];
