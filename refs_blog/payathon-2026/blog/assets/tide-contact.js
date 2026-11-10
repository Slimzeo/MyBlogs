// Both renderers use CSS pixels in the hero. The creature renderer supplies its
// painted glyphs, including pose and pointer displacement, before each tide frame.
const surfaces = new WeakMap();
export const pointerRadius = 60;

export function contactsFor(surface) {
  if (!surfaces.has(surface)) surfaces.set(surface, new TideContacts());
  return surfaces.get(surface);
}

function insideBox(x, y, box) {
  return x >= box.left && x <= box.right && y >= box.top && y <= box.bottom;
}

// A procedural jellyfish has no source path for its bell. Its rim and crown
// form a convex outline. The tentacles are separate ordered strokes.
function bellOutline(points) {
  const sorted = points.slice().sort((a, b) => a.x - b.x || a.y - b.y);
  const side = list => {
    const result = [];
    for (const point of list) {
      while (result.length >= 2) {
        const a = result[result.length - 2], b = result[result.length - 1];
        if ((b.x - a.x) * (point.y - a.y) - (b.y - a.y) * (point.x - a.x) > 0) break;
        result.pop();
      }
      result.push(point);
    }
    result.pop();
    return result;
  };
  return sorted.length < 3 ? sorted : [...side(sorted), ...side(sorted.reverse())];
}

function makeOutline(part) {
  const points = part.id === 'bell' ? bellOutline(part.points) : part.points.sort((a, b) => a.order - b.order);
  const closed = !part.stroke && points.length >= 3;
  // Clearance includes the widths of the creature's ink and one tide glyph.
  const margin = part.stroke ? 10 : 16;
  const edges = [];
  let left = Infinity, top = Infinity, right = -Infinity, bottom = -Infinity, area = 0;
  for (let i = 0; i < points.length; i++) {
    const a = points[i], b = points[(i + 1) % points.length];
    left = Math.min(left, a.x);
    right = Math.max(right, a.x);
    top = Math.min(top, a.y);
    bottom = Math.max(bottom, a.y);
    if (!closed && i + 1 === points.length && i > 0) break;
    const dx = b.x - a.x, dy = b.y - a.y;
    const length2 = dx * dx + dy * dy;
    edges.push({x: a.x, y: a.y, endY: b.y, dx, dy,
      invLength2: length2 ? 1 / length2 : 0, slope: dy ? dx / dy : 0});
    area += a.x * b.y - a.y * b.x;
  }
  return {left: left - margin, right: right + margin, top: top - margin, bottom: bottom + margin,
    closed, edges, margin, winding: area >= 0 ? 1 : -1};
}

// Signed distance to the sampled outline. A solid body pushes interior points
// out as well. Open legs and tentacles only displace water along the stroke.
function contact(x, y, part) {
  if (!insideBox(x, y, part)) return null;
  let inside = false, best = Infinity, nearest, nx = 0, ny = 0;
  for (const edge of part.edges) {
    const rx = x - edge.x, ry = y - edge.y;
    if (part.closed && (edge.y > y) !== (edge.endY > y) && rx < ry * edge.slope) inside = !inside;
    const t = Math.max(0, Math.min(1, (rx * edge.dx + ry * edge.dy) * edge.invLength2));
    const dx = rx - t * edge.dx, dy = ry - t * edge.dy;
    const distance2 = dx * dx + dy * dy;
    if (distance2 < best) {
      best = distance2;
      nx = dx;
      ny = dy;
      nearest = edge;
    }
  }
  if (!inside && best >= part.margin * part.margin) return null;
  const distance = Math.sqrt(best);
  if (distance > .001) {
    const sign = inside ? -1 / distance : 1 / distance;
    nx *= sign;
    ny *= sign;
  } else {
    const length = Math.hypot(nearest.dx, nearest.dy);
    nx = length ? part.winding * nearest.dy / length : 1;
    ny = length ? -part.winding * nearest.dx / length : 0;
  }
  return {nx, ny, depth: part.margin + (inside ? distance : -distance)};
}

class TideContacts {
  constructor() {
    this.animals = [];
    this.groups = new Map();
    this.stillFrame = null;
  }
  begin(view) {
    this.view = view;
    this.groups.clear();
  }
  add(glyph) {
    const edge = glyph.tideEdge;
    if (!edge || glyph.alpha < .20 || glyph.char === ' ') return;
    let parts = this.groups.get(glyph.subject);
    if (!parts) this.groups.set(glyph.subject, parts = new Map());
    let part = parts.get(edge.id);
    if (!part) parts.set(edge.id, part = {id: edge.id, stroke: edge.stroke, points: []});
    part.points.push({x: glyph.x - this.view.left, y: glyph.y - this.view.top, order: edge.order});
  }
  finish() {
    this.animals = [];
    for (const parts of this.groups.values()) {
      const animal = {left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity, parts: []};
      for (const part of parts.values()) {
        const outline = makeOutline(part);
        if (outline.right < 0 || outline.bottom < 0 || outline.left > this.view.width || outline.top > this.view.height) continue;
        animal.left = Math.min(animal.left, outline.left);
        animal.right = Math.max(animal.right, outline.right);
        animal.top = Math.min(animal.top, outline.top);
        animal.bottom = Math.max(animal.bottom, outline.bottom);
        animal.parts.push(outline);
      }
      if (animal.parts.length) this.animals.push(animal);
    }
    this.stillFrame?.();
  }
  clear() {
    this.groups.clear();
    this.animals = [];
    this.stillFrame?.();
  }
  nearestExit(x, y) {
    let hit = null;
    for (const animal of this.animals) {
      if (!insideBox(x, y, animal)) continue;
      for (const part of animal.parts) {
        const candidate = contact(x, y, part);
        if (candidate && (!hit || candidate.depth > hit.depth)) hit = candidate;
      }
    }
    return hit;
  }
  repel(particle, moving, pointer) {
    const x = particle.x, y = particle.y;
    // Query the glyph center, not the top-left corner of its texture cell.
    let hit = this.nearestExit(x + 8, y + 8);
    particle.tideHidden = false;
    if (!hit) return;
    for (let pass = 0; hit && pass < 3; pass++) {
      particle.x += hit.nx * (hit.depth + .3);
      particle.y += hit.ny * (hit.depth + .3);
      // A moving limb can push water back into the pointer's cleared circle.
      if (moving && pointer) {
        const dx = particle.x - pointer.x, dy = particle.y - pointer.y;
        const distance = Math.hypot(dx, dy);
        if (distance < pointerRadius) {
          particle.x = pointer.x + (distance ? dx / distance : 1) * pointerRadius;
          particle.y = pointer.y + (distance ? dy / distance : 0) * pointerRadius;
        }
      }
      hit = this.nearestExit(particle.x + 8, particle.y + 8);
    }
    // Where limbs overlap, do not draw a point trapped between their edges.
    particle.tideHidden = Boolean(hit);
    if (!moving) return;
    const dx = particle.x - x, dy = particle.y - y;
    const distance = Math.hypot(dx, dy);
    const impulse = 3 * Math.min(1, 24 / Math.max(.01, distance));
    particle.vx += dx * impulse;
    particle.vy += dy * impulse;
    if (particle.armed && distance > 1) {
      particle.energy = Math.max(particle.energy, .74);
      particle.armed = false;
    }
    particle.quiet = 0;
  }
}
