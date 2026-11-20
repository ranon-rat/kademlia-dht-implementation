import type { SearchNode, SearchStep } from "./lookup.js";

interface TraceEntry {
  row: HTMLLIElement;
  detail: HTMLElement;
  timing: HTMLElement;
  started: number;
}

export class RequestTrace {
  private entries = new Map<string, TraceEntry>();

  constructor(
    private list: HTMLOListElement,
    private state: HTMLElement,
  ) {}

  reset(): void {
    this.entries.clear();
    this.list.replaceChildren();
    this.state.textContent = "RUNNING";
  }

  visit(node: SearchNode, route: string, hops: number, parent?: SearchNode): void {
    const row = document.createElement("li");
    row.className = "trace-step";
    row.dataset.state = "pending";

    const title = document.createElement("div");
    title.className = "step-title";
    const label = document.createElement("span");
    label.textContent = `HOP ${hops.toString().padStart(2, "0")} / ${route}`;
    const timing = document.createElement("span");
    timing.textContent = "PENDING";
    title.append(label, timing);

    const address = document.createElement("p");
    address.className = "step-route";
    address.textContent = node.url;

    const detail = document.createElement("p");
    detail.className = "step-detail";
    const origin = parent ? `From ${parent.url}` : "Starting node";
    detail.textContent = `${origin}\nID ${node.id}\nWaiting for response…`;
    row.append(title, address, detail);
    this.list.append(row);
    this.entries.set(node.id, { row, detail, timing, started: performance.now() });
    this.list.scrollTop = this.list.scrollHeight;
  }

  complete(step: SearchStep): void {
    const entry = this.entries.get(step.node.id);

    if (!entry) {
      return;
    }

    let state = "done";

    if (step.status.startsWith("Error:")) {
      state = "error";
    } else if (step.status === "Value found") {
      state = "found";
    }

    entry.row.dataset.state = state;
    entry.timing.textContent = `${Math.round(performance.now() - entry.started)} ms`;
    const origin = step.parent ? `From ${step.parent.url}` : "Starting node";
    const lines = [origin, `ID ${step.node.id}`, step.status];

    if (step.candidates) {
      lines.push(`${step.candidates.length} candidate(s) returned`);

      for (const candidate of step.candidates) {
        lines.push(`  → ${candidate.url}`);
      }
    }

    entry.detail.textContent = lines.join("\n");
    this.list.scrollTop = this.list.scrollHeight;
  }

  finish(): void {
    const hasErrors = [...this.entries.values()].some((entry) => {
      return entry.row.dataset.state === "error";
    });
    this.state.textContent = hasErrors ? "WITH ERRORS" : "COMPLETE";
  }
}
