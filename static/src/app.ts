import { parseResponse, formatResponse } from "./response.js";
import { searchNetwork, type SearchResponse } from "./lookup.js";
import { RequestTrace } from "./trace.js";
import { PeerGraph } from "./graph.js";

declare global {
  interface Window {
    hljs?: {
      highlightElement(element: HTMLElement): void;
    };
  }
}

interface RunningNode {
  id: string;
  port: number;
  url: string;
}

function element<T extends HTMLElement>(id: string): T {
  const found = document.getElementById(id);

  if (!found) {
    throw new Error(`Missing element: ${id}`);
  }

  return found as T;
}

const nodeSelect = element<HTMLSelectElement>("node");
const identitySelect = element<HTMLSelectElement>("identity");
const actionSelect = element<HTMLSelectElement>("action");
const controls = element<HTMLFieldSetElement>("node-controls");
const status = element("status");
const result = element("result");
let nodes: RunningNode[] = [];
let busy = false;
let peerRequest = 0;
const trace = new RequestTrace(
  element<HTMLOListElement>("trace"),
  element("trace-state"),
);

const graph = new PeerGraph(
  element("peers"),
  element("graph-detail"),
  (id) => {
    if (busy) {
      return;
    }

    void expandGraphNode(id);
  },
);

function showResponse(text: string): void {
  result.textContent = text;
  delete result.dataset.highlighted;
  result.className = "";
  element("response-size").textContent = `${new TextEncoder().encode(text).length} B`;

  try {
    parseResponse(text);
    result.className = "language-json";
    element("response-format").textContent = "JSON";
    window.hljs?.highlightElement(result);
  } catch {
    element("response-format").textContent = "TEXT";
  }
}

async function expandGraphNode(id: string, epoch = peerRequest): Promise<void> {
  const source = graph.exploration.nodes.get(id) ?? nodes.find((node) => node.id === id);

  if (!source) {
    return;
  }

  const context = element("peer-context");
  context.textContent = `Loading neighbors of ${source.url}…`;

  try {
    const managed = nodes.some((node) => node.id === id && node.url === source.url);
    const path = managed
      ? `/nodes/${encodeURIComponent(id)}/peers`
      : `/explore-peer?url=${encodeURIComponent(source.url)}`;
    const response = await request(path);

    if (epoch !== peerRequest) {
      return;
    }

    const peers = parseResponse(response) as { id: string | number; value: string }[];
    graph.expand(
      source,
      peers.map((peer) => {
        return { id: String(peer.id), url: peer.value };
      }),
      new Set(nodes.map((node) => node.id)),
    );
    element("peer-count").textContent = String(graph.exploration.nodes.size);
    context.textContent = `Expanded ${source.url}. ${graph.exploration.nodes.size} nodes explored. Click any node to keep exploring.`;
  } catch (error) {
    if (epoch === peerRequest) {
      context.textContent = error instanceof Error ? error.message : String(error);
    }
  }
}

function resetGraph(): void {
  ++peerRequest;
  const source = nodes.find((node) => node.id === nodeSelect.value);

  if (!source) {
    graph.clear("No node selected.");
    element("peer-count").textContent = "0";
    element("peer-context").textContent = "Start a cluster to explore the routing graph.";
    return;
  }

  graph.reset(source, new Set(nodes.map((node) => node.id)));
  element("peer-count").textContent = "1";
  element("peer-context").textContent = `Starting at ${source.url}. Click the source node to discover its neighbors.`;
}

async function refreshPeers(): Promise<void> {
  const source = nodes.find((node) => node.id === nodeSelect.value);

  if (!source) {
    resetGraph();
    return;
  }

  if (graph.exploration.root?.id !== source.id) {
    ++peerRequest;
    graph.select(source, new Set(nodes.map((node) => node.id)));
  }

  await expandGraphNode(source.id);
}

async function request(
  path: string,
  init?: RequestInit,
  timeoutMs = 30000,
): Promise<string> {
  const response = await fetch(path, {
    ...init,
    signal: AbortSignal.timeout(timeoutMs),
  });
  const text = await response.text();

  if (!response.ok) {
    throw new Error(`${response.status}: ${text}`);
  }

  return text;
}

function disableButtons(disabled: boolean): void {
  const buttons = document.querySelectorAll<HTMLButtonElement>("button");

  for (const button of buttons) {
    if (button.id !== "graph-expand") {
      button.disabled = disabled;
    }
  }

  graph.setBusy(disabled);
}

async function run(task: () => Promise<void>): Promise<void> {
  if (busy) {
    return;
  }

  busy = true;
  disableButtons(true);
  status.textContent = "Processing…";

  try {
    await task();

    if (nodes.length > 0) {
      status.textContent = `${nodes.length} node(s) available.`;
    } else {
      status.textContent = "No nodes available. Start the services to continue.";
    }
  } catch (error) {
    status.textContent = error instanceof Error ? error.message : String(error);
  } finally {
    busy = false;
    disableButtons(false);
    controls.disabled = nodes.length === 0;
    updateFields();
  }
}

async function refresh(): Promise<void> {
  const selected = nodeSelect.value;
  const identity = identitySelect.value;
  nodes = JSON.parse(await request("/all-services")) as RunningNode[];
  nodes.sort((a, b) => a.port - b.port);

  const options = nodes.map((node) => {
    return new Option(`${node.port} · ${node.id} · ${node.url}`, node.id);
  });
  nodeSelect.replaceChildren(...options);

  if (nodes.some((node) => node.id === selected)) {
    nodeSelect.value = selected;
  }

  const sharesExploredNodes = nodes.some((node) => graph.exploration.nodes.has(node.id));

  if (graph.exploration.nodes.size > 0 && !sharesExploredNodes) {
    ++peerRequest;
    graph.clear("Cluster replaced. Select a node to begin exploring.");
  }

  updateIdentities(identity);
  element("node-count").textContent = String(nodes.length);
  await refreshPeers();
}

function updateIdentities(previous = identitySelect.value): void {
  const peers = nodes.filter((node) => node.id !== nodeSelect.value);
  const options = peers.map((node) => {
    return new Option(`${node.port} · ${node.id}`, node.id);
  });
  identitySelect.replaceChildren(...options);

  if (peers.some((node) => node.id === previous)) {
    identitySelect.value = previous;
  }

  updateFields();
}

function updateFields(): void {
  const action = actionSelect.value;
  const hasKey = ["store", "get", "get-local-search"].includes(action);

  element("identity-fields").hidden = action !== "more";
  element("search-help").hidden = action !== "get";
  element("key-field").hidden = !hasKey;
  element("value-field").hidden = action !== "store";
  element("message-field").hidden = action !== "send-message";

  element<HTMLInputElement>("key").required = hasKey;
  element<HTMLTextAreaElement>("value").required = action === "store";
  element<HTMLTextAreaElement>("message").required = action === "send-message";
  identitySelect.required = action === "more";
  element<HTMLButtonElement>("execute").disabled =
    busy || (action === "more" && !identitySelect.value);
}

async function stopAllServices(): Promise<void> {
  await request("/delete-services", { method: "DELETE" });
  await refresh();
}

async function stopSelectedService(): Promise<void> {
  const id = encodeURIComponent(nodeSelect.value);
  await request(`/delete-service?id=${id}`, { method: "DELETE" });
  await refresh();
}

async function executeOperation(): Promise<void> {
  const action = actionSelect.value;
  const route = action === "send-message" ? "messages" : action;
  const query = new URLSearchParams();

  if (["store", "get", "get-local-search"].includes(action)) {
    query.set("key", element<HTMLInputElement>("key").value);
  }

  if (action === "more") {
    const identity = nodes.find((node) => node.id === identitySelect.value);

    if (!identity) {
      throw new Error("Select another node as the identity.");
    }

    query.set("id", identity.id);
    query.set("url", identity.url);
  }

  let body: string | undefined;

  if (action === "store") {
    body = JSON.stringify({
      key: element<HTMLInputElement>("key").value,
      value: element<HTMLTextAreaElement>("value").value,
    });
  }

  if (action === "send-message") {
    body = JSON.stringify({
      id: crypto.randomUUID(),
      content: element<HTMLTextAreaElement>("message").value,
    });
  }

  const path = `/nodes/${nodeSelect.value}/${route}?${query}`;
  const selectedNode = nodes.find((node) => node.id === nodeSelect.value);
  const method = body ? "POST" : "GET";

  element("request").textContent =
    `${method} ${selectedNode?.url}/${route}?${query}`;
  showResponse("Waiting for a response…");
  trace.reset();
  element("hop-count").textContent = "0";
  element("query-count").textContent = "0";
  element("search-summary").textContent = "";

  try {
    if (action === "get") {
      if (!selectedNode) {
        throw new Error("Select a node.");
      }

      const search = await searchNetwork(
        selectedNode,
        async (candidate) => {
          const available = nodes.some((node) => {
            return node.id === candidate.id && node.url === candidate.url;
          });

          if (!available) {
            throw new Error("This node is not available in management.");
          }

          const id = encodeURIComponent(candidate.id);
          const response = await request(`/nodes/${id}/get?${query}`);
          return parseResponse(response) as SearchResponse;
        },
        (node, hops, parent) => {
          status.textContent = `Searching ${node.url} · ${node.id}…`;
          trace.visit(node, "GET /get", hops, parent);
          element("hop-count").textContent = String(hops);
        },
        100,
        (step) => {
          trace.complete(step);
          const queries = Number(element("query-count").textContent) + 1;
          element("query-count").textContent = String(queries);
        },
      );

      const outcome = search.found ? "Value found" : "Value not found";
      const hopLabel = search.hops === 1 ? "hop" : "hops";
      const queryLabel = search.queries === 1 ? "query" : "queries";
      const limitNotice = search.limitReached ? " Query limit reached." : "";

      element("search-summary").textContent =
        `${outcome}: ${search.hops} ${hopLabel}, ` +
        `${search.queries} ${queryLabel}.${limitNotice}`;
      element("hop-count").textContent = String(search.hops);
      element("query-count").textContent = String(search.queries);
      showResponse(JSON.stringify(search, null, 2));
    } else {
      if (selectedNode) {
        trace.visit(selectedNode, `${method} /${route}`, 0);
      }

      const response = await request(path, {
        method,
        headers: body ? { "Content-Type": "application/json" } : undefined,
        body,
      });
      showResponse(formatResponse(response));
      element("query-count").textContent = "1";

      if (selectedNode) {
        trace.complete({
          node: selectedNode,
          hops: 0,
          status: "Request completed",
        });
      }

      element("search-summary").textContent = "Request completed. 1 query.";
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    showResponse(message);

    if (selectedNode) {
      trace.complete({
        node: selectedNode,
        hops: 0,
        status: `Error: ${message}`,
      });
    }

    element("search-summary").textContent = "Request failed. See the trace for details.";
    throw error;
  } finally {
    trace.finish();
    await refreshPeers();
  }
}

nodeSelect.addEventListener("change", () => {
  updateIdentities();
  void refreshPeers();
});

actionSelect.addEventListener("change", updateFields);

element<HTMLFormElement>("services").addEventListener("submit", (event) => {
  event.preventDefault();
  const data = new FormData(event.currentTarget as HTMLFormElement);

  void run(async () => {
    const amount = Number(data.get("amount"));
    let starting = true;
    let polling = false;
    status.textContent = `Starting cluster: 0 / ${amount} nodes…`;

    const progress = window.setInterval(async () => {
      if (polling) {
        return;
      }

      polling = true;

      try {
        const current = JSON.parse(
          await request("/all-services", undefined, 5000),
        ) as RunningNode[];

        if (starting) {
          status.textContent = `Starting cluster: ${current.length} / ${amount} nodes…`;
          element("node-count").textContent = String(current.length);
        }
      } catch {
        // A progress request must not interrupt the cluster startup request.
      } finally {
        polling = false;
      }
    }, 1000);

    try {
      // Each node may take up to three seconds to become ready on the server.
      const timeoutMs = Math.max(30000, amount * 3500 + 10000);
      await request(
        "/start-services",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            from_port: Number(data.get("from_port")),
            amount,
          }),
        },
        timeoutMs,
      );
      showResponse("Services started. Select a node to view its identity.");
    } finally {
      starting = false;
      window.clearInterval(progress);
      await refresh();
    }
  });
});

element("refresh").addEventListener("click", () => {
  void run(refresh);
});

element("stop-all").addEventListener("click", () => {
  void run(stopAllServices);
});

element("stop-node").addEventListener("click", () => {
  void run(stopSelectedService);
});

element<HTMLFormElement>("operation").addEventListener("submit", (event) => {
  event.preventDefault();
  void run(executeOperation);
});

void run(refresh);

element("refresh-peers").addEventListener("click", () => {
  void refreshPeers();
});

element("copy-response").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(result.textContent ?? "");
    status.textContent = "Response copied to clipboard.";
  } catch {
    status.textContent = "Could not copy. Select the response text to copy it manually.";
  }
});

element("graph-zoom-in").addEventListener("click", () => {
  graph.zoom(1.25);
});

element("graph-zoom-out").addEventListener("click", () => {
  graph.zoom(0.8);
});

element("graph-reset").addEventListener("click", () => {
  graph.resetView();
});

element("graph-clear").addEventListener("click", resetGraph);

const expandMapButton = element<HTMLButtonElement>("graph-expand");
const topologyPanel = element("topology-panel");
let mapExpanded = false;
const backgroundInert = new Map<HTMLElement, boolean>();

function setMapExpanded(expanded: boolean): void {
  mapExpanded = expanded;
  document.body.classList.toggle("map-expanded", expanded);
  expandMapButton.textContent = expanded ? "Restore view" : "Expand map";
  expandMapButton.setAttribute("aria-expanded", String(expanded));

  if (expanded) {
    topologyPanel.setAttribute("role", "dialog");
    topologyPanel.setAttribute("aria-modal", "true");
    const background = document.querySelectorAll<HTMLElement>(
      ".topbar, .sidebar, .workspace-heading, .inspector-grid, .workspace-footer",
    );

    for (const section of background) {
      backgroundInert.set(section, section.inert);
      section.inert = true;
    }
  } else {
    topologyPanel.removeAttribute("role");
    topologyPanel.removeAttribute("aria-modal");

    for (const [section, inert] of backgroundInert) {
      section.inert = inert;
    }

    backgroundInert.clear();
  }

  graph.resizePreservingView();
  expandMapButton.focus({ preventScroll: true });
}

expandMapButton.addEventListener("click", () => {
  setMapExpanded(!mapExpanded);
});

document.addEventListener("keydown", (event) => {
  if (!mapExpanded) {
    return;
  }

  if (event.key === "Escape") {
    event.preventDefault();
    setMapExpanded(false);
    return;
  }

  if (event.key === "Tab") {
    const controls = [...topologyPanel.querySelectorAll<HTMLElement>(
      'button:not(:disabled), [tabindex="0"]',
    )].filter((control) => control.getClientRects().length > 0);
    const first = controls[0];
    const last = controls[controls.length - 1];

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
});

element("graph-show-all").addEventListener("click", () => {
  void run(async () => {
    await refresh();
    const source = nodes.find((node) => node.id === nodeSelect.value);

    if (!source) {
      return;
    }

    graph.showNodes(source, nodes);
    element("peer-count").textContent = String(graph.exploration.nodes.size);
    element("peer-context").textContent = `${nodes.length} managed nodes loaded from management. Click any node to discover its connections.`;
  });
});
