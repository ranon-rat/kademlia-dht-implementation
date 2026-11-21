export interface Particle {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
}

export const minimumNodeDistance = 145;
const linkLength = 215;
const centerX = 500;
const centerY = 260;
const baseLinkStrength = 0.02;
const identityBits = 64;

export function xorLinkStrength(source: string, target: string): number {
  // Compute XOR before converting to a floating-point value for the simulation.
  const distance = BigInt(source) ^ BigInt(target);
  const logarithmicDistance = distance > 1n ? Math.log2(Number(distance)) : 0;

  // Keep the original stiffness for distant uint64 identities, and make
  // nearby identities more strongly connected. The +1 handles XOR 0 and 1.
  return baseLinkStrength * identityBits / (1 + logarithmicDistance);
}

interface ForceLink {
  source: string;
  target: string;
  strength: number;
}

export class ForceLayout {
  readonly particles = new Map<string, Particle>();
  private links: ForceLink[] = [];
  private root = "";
  private held?: string;
  private heat = 0;

  reset(): void {
    this.particles.clear();
    this.links = [];
    this.root = "";
    this.held = undefined;
    this.heat = 0;
  }

  sync(root: string, ids: string[], neighbors: Map<string, string[]>): void {
    this.root = root;
    const remaining = new Set(ids);

    for (const id of this.particles.keys()) {
      if (!remaining.has(id)) {
        this.particles.delete(id);
      }
    }

    for (const id of ids) {
      if (this.particles.has(id)) {
        continue;
      }

      let parent = this.particles.get(root);
      let linked = false;

      for (const [source, peers] of neighbors) {
        if (peers.includes(id) && this.particles.has(source)) {
          parent = this.particles.get(source);
          linked = true;
          break;
        }
      }

      const count = this.particles.size;
      const angle = linked
        ? Number(BigInt(id) % 360n) * Math.PI / 180
        : count * 2.399963;
      const radius = linked ? linkLength : minimumNodeDistance * Math.sqrt(count) * 0.8;
      this.particles.set(id, {
        id,
        x: id === root ? centerX : (parent?.x ?? centerX) + Math.cos(angle) * radius,
        y: id === root ? centerY : (parent?.y ?? centerY) + Math.sin(angle) * radius,
        vx: 0,
        vy: 0,
      });
    }

    this.links = [];
    const seen = new Set<string>();

    for (const [source, peers] of neighbors) {
      for (const target of peers) {
        const key = [source, target].sort().join(":");

        if (source !== target && !seen.has(key)) {
          seen.add(key);
          this.links.push({
            source,
            target,
            strength: xorLinkStrength(source, target),
          });
        }
      }
    }

    this.reheat();
  }

  hold(id: string): void {
    this.held = id;
    this.reheat();
  }

  move(id: string, x: number, y: number): void {
    const particle = this.particles.get(id);

    if (particle) {
      particle.x = x;
      particle.y = y;
      particle.vx = 0;
      particle.vy = 0;
      this.reheat();
    }
  }

  release(): void {
    this.held = undefined;
    this.reheat();
  }

  step(): boolean {
    const particles = [...this.particles.values()];

    for (let i = 0; i < particles.length; i++) {
      for (let j = i + 1; j < particles.length; j++) {
        const first = particles[i];
        const second = particles[j];
        const direction = this.direction(first, second, i + j);
        const strength = Math.min(3, 9000 / (direction.distance ** 2)) * this.heat;
        this.push(first, -direction.x * strength, -direction.y * strength);
        this.push(second, direction.x * strength, direction.y * strength);
      }
    }

    for (const { source, target, strength: linkStrength } of this.links) {
      const first = this.particles.get(source);
      const second = this.particles.get(target);

      if (!first || !second) {
        continue;
      }

      const direction = this.direction(first, second);
      const extension = direction.distance - linkLength;
      const force = Math.max(-3, Math.min(3, extension * linkStrength));
      const strength = force * this.heat;
      this.push(first, direction.x * strength, direction.y * strength);
      this.push(second, -direction.x * strength, -direction.y * strength);
    }

    let speed = 0;

    for (const particle of particles) {
      if (this.fixed(particle)) {
        particle.vx = 0;
        particle.vy = 0;
        continue;
      }

      particle.vx += (centerX - particle.x) * 0.0006 * this.heat;
      particle.vy += (centerY - particle.y) * 0.0006 * this.heat;
      particle.vx = Math.max(-10, Math.min(10, particle.vx * 0.78));
      particle.vy = Math.max(-10, Math.min(10, particle.vy * 0.78));
      particle.x += particle.vx;
      particle.y += particle.vy;
      speed = Math.max(speed, Math.hypot(particle.vx, particle.vy));
    }

    // Enforce spacing after integration, including while a node is held.
    let overlap = 0;

    for (let pass = 0; pass < 6; pass++) {
      overlap = this.separate(particles);
    }

    this.heat *= 0.97;
    return this.heat > 0.002 || speed > 0.02 || overlap > 0.05;
  }

  private reheat(): void {
    this.heat = 1;
  }

  private fixed(particle: Particle): boolean {
    return particle.id === this.root || particle.id === this.held;
  }

  private push(particle: Particle, x: number, y: number): void {
    if (!this.fixed(particle)) {
      particle.vx += x;
      particle.vy += y;
    }
  }

  private direction(first: Particle, second: Particle, seed = 1): {
    x: number;
    y: number;
    distance: number;
  } {
    const dx = second.x - first.x;
    const dy = second.y - first.y;
    const distance = Math.hypot(dx, dy);

    if (distance < 0.001) {
      const angle = seed * 2.399963;
      return { x: Math.cos(angle), y: Math.sin(angle), distance: 0.001 };
    }

    return { x: dx / distance, y: dy / distance, distance };
  }

  private separate(particles: Particle[]): number {
    let overlap = 0;

    for (let i = 0; i < particles.length; i++) {
      for (let j = i + 1; j < particles.length; j++) {
        const first = particles[i];
        const second = particles[j];
        const direction = this.direction(first, second, i + j);
        const correction = minimumNodeDistance - direction.distance;

        if (correction <= 0) {
          continue;
        }

        overlap = Math.max(overlap, correction);
        const firstFixed = this.fixed(first);
        const secondFixed = this.fixed(second);
        let firstShare = secondFixed ? 1 : 0.5;
        let secondShare = firstFixed ? 1 : 0.5;

        if (firstFixed) {
          firstShare = 0;
        }

        if (secondFixed) {
          secondShare = 0;
        }

        // A dragged peer can never overlap the anchored source.
        if (firstFixed && secondFixed) {
          firstShare = first.id === this.held ? 1 : 0;
          secondShare = second.id === this.held ? 1 : 0;
        }

        first.x -= direction.x * correction * firstShare;
        first.y -= direction.y * correction * firstShare;
        second.x += direction.x * correction * secondShare;
        second.y += direction.y * correction * secondShare;
      }
    }

    return overlap;
  }
}
