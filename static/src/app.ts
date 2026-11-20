import { parseResponse, formatResponse } from "./response.js";
import { searchNetwork, type SearchResponse } from "./lookup.js";

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

async function request(path: string, init?: RequestInit): Promise<string> {
  const response = await fetch(path, {
    ...init,
    signal: AbortSignal.timeout(30000),
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
    button.disabled = disabled;
  }
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

  updateIdentities(identity);
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
  result.textContent = "Waiting for a response…";
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
        (node) => {
          status.textContent = `Searching ${node.url} · ${node.id}…`;
        },
      );

      const outcome = search.found ? "Value found" : "Value not found";
      const hopLabel = search.hops === 1 ? "hop" : "hops";
      const queryLabel = search.queries === 1 ? "query" : "queries";
      const limitNotice = search.limitReached ? " Query limit reached." : "";

      element("search-summary").textContent =
        `${outcome}: ${search.hops} ${hopLabel}, ` +
        `${search.queries} ${queryLabel}.${limitNotice}`;
      result.textContent = JSON.stringify(search, null, 2);
    } else {
      const response = await request(path, {
        method,
        headers: body ? { "Content-Type": "application/json" } : undefined,
        body,
      });
      result.textContent = formatResponse(response);
    }
  } catch (error) {
    result.textContent = error instanceof Error ? error.message : String(error);
    throw error;
  }
}

nodeSelect.addEventListener("change", () => {
  updateIdentities();
});

actionSelect.addEventListener("change", updateFields);

element<HTMLFormElement>("services").addEventListener("submit", (event) => {
  event.preventDefault();
  const data = new FormData(event.currentTarget as HTMLFormElement);

  void run(async () => {
    try {
      await request("/start-services", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          from_port: Number(data.get("from_port")),
          amount: Number(data.get("amount")),
        }),
      });
      result.textContent = "Services started. Select a node to view its identity.";
    } finally {
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
