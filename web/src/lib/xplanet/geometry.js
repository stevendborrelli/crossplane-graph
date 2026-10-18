/**
 * Pure geometry for the connector layer.
 *
 * Everything here works in *scene coordinates*: pixels relative to the top-left
 * of the <Scene> box, which is what Scene hands out after measuring its
 * anchors. No DOM access, so it stays testable and runs the same on the server.
 */

export function centerOf(rect) {
  return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
}

/**
 * Where a connector meets `rect` when heading toward `toward`.
 *
 * The facing side is chosen by whichever axis the centre-to-centre ray would
 * leave through, but the connector attaches at the *midpoint* of that side
 * rather than at the exit point. Using the exit point puts connectors near a
 * corner whenever the other box sits diagonally — and a node with several
 * stacked neighbours ends up with connectors scattered down its edge instead
 * of fanning cleanly from one place.
 */
export function edgePoint(rect, toward, pad = 6) {
  const c = centerOf(rect);
  const dx = toward.x - c.x;
  const dy = toward.y - c.y;
  if (dx === 0 && dy === 0) return c;

  const halfW = rect.width / 2 + pad;
  const halfH = rect.height / 2 + pad;

  // Compare the ray against the box's own proportions, so a wide box still
  // hands off through its short sides when the target is directly above.
  if (Math.abs(dx) / halfW >= Math.abs(dy) / halfH) {
    return { x: c.x + Math.sign(dx) * halfW, y: c.y };
  }
  return { x: c.x, y: c.y + Math.sign(dy) * halfH };
}

/** Smallest rect covering them all, grown by `pad`. */
export function unionRect(rects, pad = 0) {
  const live = rects.filter(Boolean);
  if (live.length === 0) return null;
  const left = Math.min(...live.map((r) => r.x));
  const top = Math.min(...live.map((r) => r.y));
  const right = Math.max(...live.map((r) => r.x + r.width));
  const bottom = Math.max(...live.map((r) => r.y + r.height));
  return {
    x: left - pad,
    y: top - pad,
    width: right - left + pad * 2,
    height: bottom - top + pad * 2,
  };
}

/** Which face of `rect` a connector heading toward `toward` should leave by. */
export function sideFor(rect, toward) {
  const c = centerOf(rect);
  const dx = toward.x - c.x;
  const dy = toward.y - c.y;
  const halfW = rect.width / 2 || 1;
  const halfH = rect.height / 2 || 1;
  // Compared against the box's own proportions, so a wide box still hands off
  // through its short sides when the target is directly above or below.
  if (Math.abs(dx) / halfW >= Math.abs(dy) / halfH) return dx >= 0 ? 'right' : 'left';
  return dy >= 0 ? 'bottom' : 'top';
}

/** A point `fraction` of the way along one face, pushed out by `pad`. */
export function attachPoint(rect, side, fraction, pad = 6) {
  switch (side) {
    case 'left':
      return { x: rect.x - pad, y: rect.y + rect.height * fraction };
    case 'right':
      return { x: rect.x + rect.width + pad, y: rect.y + rect.height * fraction };
    case 'top':
      return { x: rect.x + rect.width * fraction, y: rect.y - pad };
    default:
      return { x: rect.x + rect.width * fraction, y: rect.y + rect.height + pad };
  }
}

/** Where the nth of `count` connectors sharing a face should sit: a lone one
 *  takes the middle, several spread evenly and symmetrically about it. */
export function shareFraction(index, count) {
  return (index + 1) / (count + 1);
}

/**
 * A path is `{ d, at(t), length }`: an SVG path string for the rail, and a
 * sampler so a packet travelling the same route lands exactly on the line.
 * Both come from one definition, so the drawn wire and the moving dot can
 * never disagree.
 */
export function linePath(a, b) {
  return {
    d: `M ${a.x} ${a.y} L ${b.x} ${b.y}`,
    length: Math.hypot(b.x - a.x, b.y - a.y),
    at: (t) => ({ x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t }),
  };
}

/**
 * Quadratic bow, perpendicular to the chord. `bow` is a fraction of the chord
 * length, signed: positive bends one way, negative the other, which is how two
 * opposite-direction edges between the same pair of nodes avoid overlapping.
 */
export function arcPath(a, b, bow = 0.18) {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = Math.hypot(dx, dy) || 1;
  const cx = (a.x + b.x) / 2 - (dy / len) * len * bow;
  const cy = (a.y + b.y) / 2 + (dx / len) * len * bow;

  return {
    d: `M ${a.x} ${a.y} Q ${cx} ${cy} ${b.x} ${b.y}`,
    length: len,
    at: (t) => {
      const u = 1 - t;
      return {
        x: u * u * a.x + 2 * u * t * cx + t * t * b.x,
        y: u * u * a.y + 2 * u * t * cy + t * t * b.y,
      };
    },
  };
}

export function pathBetween(fromRect, toRect, bow = 0) {
  if (!fromRect || !toRect) return null;
  const a = edgePoint(fromRect, centerOf(toRect));
  const b = edgePoint(toRect, centerOf(fromRect));
  return bow ? arcPath(a, b, bow) : linePath(a, b);
}

/** Direction of travel at `t`, in degrees, for orienting a packet or arrowhead. */
export function headingAt(path, t) {
  const step = 0.01;
  const a = path.at(Math.max(0, t - step));
  const b = path.at(Math.min(1, t + step));
  return (Math.atan2(b.y - a.y, b.x - a.x) * 180) / Math.PI;
}
