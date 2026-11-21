import { ForceLayout } from "./physics.js";

export interface GraphNode {
  id: string;
  url: string;
}

const svgNamespace = "http://www.w3.org/2000/svg";
const width = 1000;
const height = 520;

function svgElement<K extends keyof SVGElementTagNameMap>(
  tag: K,
  attributes: Record<string, string> = {},
): SVGElementTagNameMap[K] {
  const element = document.createElementNS(svgNamespace, tag);

  for (const [name, value] of Object.entries(attributes)) {
    element.setAttribute(name, value);
  }

  return element;
}

function nodeLabel(node: GraphNode): string {
  try {
    const address = new URL(node.url);
    return address.port || address.hostname;
  } catch {
    return node.id.slice(0, 6);
  }
}

export class GraphExploration {
  root?: GraphNode;
  nodes = new Map<string, GraphNode>();
  neighbors = new Map<string, string[]>();

  reset(root?: GraphNode): void {
    this.root = root;
    this.nodes.clear();
    this.neighbors.clear();

    if (root) {
      this.nodes.set(root.id, root);
    }
  }

  select(source: GraphNode): void {
    this.root = source;
    this.nodes.set(source.id, source);
  }

  addNodes(nodes: GraphNode[]): void {
    for (const node of nodes) {
      this.nodes.set(node.id, node);
    }
  }

  expand(source: GraphNode, peers: GraphNode[]): void {
    if (!this.root) {
      this.reset(source);
    }

    this.nodes.set(source.id, source);
    const ids = new Set<string>();

    for (const peer of peers) {
      if (peer.id !== source.id) {
        this.nodes.set(peer.id, peer);
        ids.add(peer.id);
      }
    }

    this.neighbors.set(source.id, [...ids]);
  }
}

export class PeerGraph {
  private svg: SVGSVGElement;
  private scale = 1;
  private centerX = width / 2;
  private centerY = height / 2;
  private locked = false;
  private compact?: boolean;
  private layout = new ForceLayout();
  private frame?: number;
  private fitWhenSettled = false;
  private nodeElements = new Map<string, SVGGElement>();
  private edgeElements: { source: string; target: string; line: SVGLineElement }[] = [];
  private suppressedClick?: string;
  private nodeDrag?: {
    id: string;
    startX: number;
    startY: number;
    offsetX: number;
    offsetY: number;
    moved: boolean;
  };
  readonly exploration = new GraphExploration();
  private managed = new Set<string>();
  private drag?: { x: number; y: number; centerX: number; centerY: number };

  constructor(
    private container: HTMLElement,
    private detail: HTMLElement,
    private onExpand: (id: string) => void,
  ) {
    this.svg = svgElement("svg", {
      viewBox: `0 0 ${width} ${height}`,
      role: "group",
      "aria-label": "Routing graph. Select a neighboring node to explore its peers.",
    });
    this.container.append(this.svg);
    const observer = new ResizeObserver(() => {
      const compact = this.container.clientWidth < 500;

      if (this.compact !== compact) {
        this.compact = compact;
        this.resetView();
      }
    });
    observer.observe(this.container);
    this.svg.addEventListener("wheel", (event) => {
      event.preventDefault();
      let pixels = event.deltaY;

      if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
        pixels *= 16;
      } else if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
        pixels *= this.container.clientHeight;
      }

      const factor = Math.exp(-Math.max(-200, Math.min(200, pixels)) * 0.002);
      this.zoom(factor, this.worldPoint(event));
    }, { passive: false });
    this.svg.addEventListener("pointerdown", (event) => {
      if (event.button !== 0) {
        return;
      }

      this.suppressedClick = undefined;
      this.fitWhenSettled = false;
      const group = (event.target as Element).closest<SVGGElement>(".graph-node");
      const id = group?.dataset.nodeId;
      const particle = id ? this.layout.particles.get(id) : undefined;

      if (id && group && particle) {
        const point = this.worldPoint(event);
        this.nodeDrag = {
          id,
          startX: event.clientX,
          startY: event.clientY,
          offsetX: particle.x - point.x,
          offsetY: particle.y - point.y,
          moved: false,
        };
        this.layout.hold(id);
        group.setPointerCapture(event.pointerId);
        group.focus({ preventScroll: true });
        event.preventDefault();
        return;
      }

      this.drag = {
        x: event.clientX,
        y: event.clientY,
        centerX: this.centerX,
        centerY: this.centerY,
      };
      this.svg.setPointerCapture(event.pointerId);
    });
    this.svg.addEventListener("pointermove", (event) => {
      if (this.nodeDrag) {
        const movement = Math.hypot(
          event.clientX - this.nodeDrag.startX,
          event.clientY - this.nodeDrag.startY,
        );
        this.nodeDrag.moved ||= movement > 4;

        if (this.nodeDrag.moved) {
          const point = this.worldPoint(event);
          this.layout.move(
            this.nodeDrag.id,
            point.x + this.nodeDrag.offsetX,
            point.y + this.nodeDrag.offsetY,
          );
          this.wake();
        }

        return;
      }

      if (!this.drag) {
        return;
      }

      const bounds = this.svg.getBoundingClientRect();
      const unitsPerPixel = Math.max(width / bounds.width, height / bounds.height) / this.scale;
      this.centerX = this.drag.centerX - (event.clientX - this.drag.x) * unitsPerPixel;
      this.centerY = this.drag.centerY - (event.clientY - this.drag.y) * unitsPerPixel;
      this.updateView();
    });
    this.svg.addEventListener("pointerup", () => {
      this.endDrag();
    });
    this.svg.addEventListener("pointercancel", () => {
      this.endDrag();
    });
    this.svg.addEventListener("lostpointercapture", () => {
      this.endDrag();
    });
  }

  setBusy(busy: boolean): void {
    this.locked = busy;
    this.svg.setAttribute("aria-busy", String(busy));
  }

  zoom(factor: number, anchor?: DOMPoint): void {
    this.fitWhenSettled = false;
    const previousScale = this.scale;
    this.scale = Math.min(5, Math.max(0.2, this.scale * factor));

    if (anchor) {
      const ratio = previousScale / this.scale;
      this.centerX = anchor.x + (this.centerX - anchor.x) * ratio;
      this.centerY = anchor.y + (this.centerY - anchor.y) * ratio;
    }

    this.updateView();
  }

  resizePreservingView(): void {
    // Synchronize the breakpoint before ResizeObserver handles the layout change.
    this.compact = this.container.clientWidth < 500;
    this.updateView();
  }

  resetView(): void {
    this.scale = this.container.clientWidth < 500 ? 2.4 : 1;
    this.centerX = width / 2;
    this.centerY = height / 2;
    this.updateView();
  }

  clear(message: string): void {
    this.stop();
    this.layout.reset();
    this.exploration.reset();
    this.svg.replaceChildren();
    this.nodeElements.clear();
    this.edgeElements = [];
    this.detail.textContent = message;
  }

  reset(source: GraphNode, managed: Set<string>): void {
    this.stop();
    this.layout.reset();
    this.exploration.reset(source);
    this.managed = managed;
    this.resetView();
    this.draw();
  }

  select(source: GraphNode, managed: Set<string>): void {
    const initial = !this.exploration.root;
    this.exploration.select(source);
    this.managed = managed;

    if (initial) {
      this.resetView();
    }

    this.draw();
  }

  showNodes(source: GraphNode, nodes: GraphNode[]): void {
    if (!this.exploration.root) {
      this.exploration.select(source);
    }

    this.managed = new Set(nodes.map((node) => node.id));

    this.exploration.addNodes(nodes);

    // Adding a cluster does not invent routing edges between its nodes.
    this.draw();
    this.fitView();
    this.fitWhenSettled = true;
  }

  expand(source: GraphNode, peers: GraphNode[], managed: Set<string>): void {
    this.managed = managed;
    this.exploration.expand(source, peers);

    this.draw();
    const root = this.exploration.root;

    if (root) {
      const distance = BigInt(source.id) ^ BigInt(root.id);
      this.showDetail(source, distance, source.id === root.id, managed.has(source.id));
    }
  }

  private draw(): void {
    const source = this.exploration.root;

    if (!source) {
      return;
    }

    const focusedID = document.activeElement?.getAttribute("data-node-id");
    this.svg.replaceChildren();

    this.nodeElements.clear();
    this.edgeElements = [];
    this.layout.sync(
      source.id,
      [...this.exploration.nodes.keys()],
      this.exploration.neighbors,
    );
    const drawn = new Set<string>();

    for (const [id, neighbors] of this.exploration.neighbors) {
      const start = this.layout.particles.get(id);

      if (!start) {
        continue;
      }

      for (const neighbor of neighbors) {
        const end = this.layout.particles.get(neighbor);
        const key = [id, neighbor].sort().join(":");

        if (!end || drawn.has(key)) {
          continue;
        }

        drawn.add(key);
        const line = svgElement("line", {
          x1: String(start.x),
          y1: String(start.y),
          x2: String(end.x),
          y2: String(end.y),
          class: "graph-edge",
          "data-source-id": id,
          "data-peer-id": neighbor,
        });
        this.svg.append(line);
        this.edgeElements.push({ source: id, target: neighbor, line });
      }
    }

    for (const node of this.exploration.nodes.values()) {
      const position = this.layout.particles.get(node.id);

      if (!position) {
        continue;
      }

      this.drawNode(
        node,
        position.x,
        position.y,
        node.id === source.id,
        this.managed.has(node.id),
        BigInt(node.id) ^ BigInt(source.id),
      );
    }

    this.wake();
    this.showDetail(source, 0n, true, true);
    this.setBusy(this.locked);

    if (focusedID) {
      const target = this.svg.querySelector<SVGGElement>(`[data-node-id="${focusedID}"]`);
      target?.focus({ preventScroll: true });
    }
  }

  private drawNode(
    node: GraphNode,
    x: number,
    y: number,
    selected: boolean,
    managed: boolean,
    distance: bigint,
  ): void {
    const group = svgElement("g", {
      transform: `translate(${x} ${y})`,
      class: `graph-node${selected ? " selected" : ""}${this.exploration.neighbors.has(node.id) ? " expanded" : ""}${managed ? "" : " external"}`,
      "data-node-id": node.id,
      tabindex: "0",
      role: "button",
      "aria-label": `${selected ? "Selected node" : "Peer"} ${node.url}. ID ${node.id}. XOR distance ${distance}. Select to expand its neighbors.`,
    });
    const title = svgElement("title");
    title.textContent = `${node.url}\nID ${node.id}\nXOR ${distance}`;
    group.append(title);
    group.append(svgElement("circle", {
      r: selected ? "40" : "27",
      class: "graph-node-circle",
    }));
    const label = svgElement("text", { "text-anchor": "middle", y: "5" });
    label.textContent = nodeLabel(node);
    const identity = svgElement("text", {
      "text-anchor": "middle",
      y: selected ? "62" : "48",
      class: "graph-node-id",
    });
    identity.textContent = `${node.id.slice(0, 8)}…`;
    group.append(label, identity);

    if (selected) {
      const caption = svgElement("text", {
        "text-anchor": "middle",
        y: "-57",
        class: "graph-selected-label",
      });
      caption.textContent = "SELECTED NODE";
      group.append(caption);
    }

    const inspect = () => {
      this.showDetail(node, distance, selected, managed);

      for (const edge of this.svg.querySelectorAll(".graph-edge")) {
        const linked = edge.getAttribute("data-peer-id") === node.id ||
          edge.getAttribute("data-source-id") === node.id;
        edge.classList.toggle("highlighted", linked);
      }
    };
    const explore = () => {
      if (this.locked) {
        return;
      }

      this.onExpand(node.id);
    };
    group.addEventListener("pointerenter", inspect);
    group.addEventListener("focus", inspect);
    group.addEventListener("click", () => {
      if (this.suppressedClick === node.id) {
        this.suppressedClick = undefined;
        return;
      }

      inspect();
      explore();
    });
    group.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        explore();
      }
    });
    this.svg.append(group);
    this.nodeElements.set(node.id, group);
  }

  private worldPoint(event: { clientX: number; clientY: number }): DOMPoint {
    const point = new DOMPoint(event.clientX, event.clientY);
    const matrix = this.svg.getScreenCTM();
    return matrix ? point.matrixTransform(matrix.inverse()) : point;
  }

  private endDrag(): void {
    this.drag = undefined;

    if (!this.nodeDrag) {
      return;
    }

    if (this.nodeDrag.moved) {
      this.suppressedClick = this.nodeDrag.id;
    }

    this.nodeDrag = undefined;
    this.layout.release();
    this.wake();
  }

  private wake(): void {
    if (this.frame !== undefined) {
      return;
    }

    this.frame = requestAnimationFrame(() => {
      this.frame = undefined;
      const moving = this.layout.step();
      this.paintPositions();

      if (moving) {
        this.wake();
      } else if (this.fitWhenSettled) {
        this.fitView();
        this.fitWhenSettled = false;
      }
    });
  }

  private fitView(): void {
    const particles = [...this.layout.particles.values()];

    if (particles.length === 0) {
      return;
    }

    const left = Math.min(...particles.map((point) => point.x)) - 90;
    const right = Math.max(...particles.map((point) => point.x)) + 90;
    const top = Math.min(...particles.map((point) => point.y)) - 90;
    const bottom = Math.max(...particles.map((point) => point.y)) + 90;
    const viewHeight = Math.max(bottom - top, (right - left) * height / width);
    this.scale = Math.min(3, height / viewHeight);
    this.centerX = (left + right) / 2;
    this.centerY = (top + bottom) / 2;
    this.updateView();
  }

  private stop(): void {
    this.fitWhenSettled = false;
    if (this.frame !== undefined) {
      cancelAnimationFrame(this.frame);
      this.frame = undefined;
    }

    this.nodeDrag = undefined;
    this.drag = undefined;
  }

  private paintPositions(): void {
    for (const [id, element] of this.nodeElements) {
      const particle = this.layout.particles.get(id);

      if (particle) {
        element.setAttribute("transform", `translate(${particle.x} ${particle.y})`);
      }
    }

    for (const { source, target, line } of this.edgeElements) {
      const start = this.layout.particles.get(source);
      const end = this.layout.particles.get(target);

      if (start && end) {
        line.setAttribute("x1", String(start.x));
        line.setAttribute("y1", String(start.y));
        line.setAttribute("x2", String(end.x));
        line.setAttribute("y2", String(end.y));
      }
    }
  }

  private showDetail(node: GraphNode, distance: bigint, selected: boolean, managed: boolean): void {
    const expanded = this.exploration.neighbors.has(node.id);
    const type = selected ? "SOURCE" : "PEER";
    const state = expanded ? "Expanded" : "Click to expand";
    const hint = managed ? "" : " / Discovered peer";
    this.detail.textContent = `${type} / ${node.url} / ID ${node.id} / XOR ${distance} / ${state}${hint}`;
  }

  private updateView(): void {
    const viewWidth = width / this.scale;
    const viewHeight = height / this.scale;
    this.svg.setAttribute(
      "viewBox",
      `${this.centerX - viewWidth / 2} ${this.centerY - viewHeight / 2} ${viewWidth} ${viewHeight}`,
    );
  }
}
