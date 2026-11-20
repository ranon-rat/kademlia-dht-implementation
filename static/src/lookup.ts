export interface SearchNode {
  id: string;
  url: string;
}

interface Candidate extends SearchNode {
  parent?: SearchNode;
  distance: bigint;
  hops: number;
}

export interface SearchResponse {
  value?: string | null;
  node?: {
    id: string | number;
    value: string;
    XorDistance: string | number;
  }[] | null;
}

export interface SearchStep {
  parent?: SearchNode;
  node: SearchNode;
  status: string;
  hops: number;
  candidates?: SearchNode[];
}

export interface SearchResult {
  found: boolean;
  hops: number;
  queries: number;
  value?: string;
  node?: SearchNode;
  trace: SearchStep[];
  limitReached: boolean;
}

// /get remains a single-hop operation. Follow its returned peers from the client.
export async function searchNetwork(
  start: SearchNode,
  fetchNode: (node: SearchNode) => Promise<SearchResponse>,
  onVisit: (node: SearchNode, hops: number, parent?: SearchNode) => void = () => {},
  maxVisits = 100,
  onStep: (step: SearchStep) => void = () => {},
): Promise<SearchResult> {
  const pending: Candidate[] = [
    {
      ...start,
      distance: 0n,
      hops: 0,
    },
  ];
  const queued = new Set([start.id]);
  const trace: SearchStep[] = [];

  function recordStep(step: SearchStep): void {
    trace.push(step);
    onStep(step);
  }

  while (pending.length && trace.length < maxVisits) {
    pending.sort((a, b) => {
      if (a.distance < b.distance) {
        return -1;
      }

      if (a.distance > b.distance) {
        return 1;
      }

      return 0;
    });

    const current = pending.shift()!;
    const node: SearchNode = {
      id: current.id,
      url: current.url,
    };
    onVisit(node, current.hops, current.parent);

    try {
      const response = await fetchNode(node);

      if (typeof response.value === "string") {
        recordStep({
          parent: current.parent,
          node,
          hops: current.hops,
          status: "Value found",
        });

        return {
          found: true,
          hops: current.hops,
          queries: trace.length,
          value: response.value,
          node,
          trace,
          limitReached: false,
        };
      }

      const candidates = (response.node ?? []).map((peer) => {
        return {
          id: String(peer.id),
          url: peer.value,
          distance: BigInt(peer.XorDistance),
          hops: current.hops + 1,
          parent: node,
        };
      });

      recordStep({
        parent: current.parent,
        node,
        hops: current.hops,
        status: "No local value; checking candidates",
        candidates: candidates.map(({ id, url }) => ({ id, url })),
      });

      for (const candidate of candidates) {
        if (queued.has(candidate.id)) {
          continue;
        }

        queued.add(candidate.id);
        pending.push(candidate);
      }
    } catch (error) {
      recordStep({
        parent: current.parent,
        node,
        hops: current.hops,
        status: `Error: ${error instanceof Error ? error.message : String(error)}`,
      });
    }
  }

  return {
    found: false,
    hops: Math.max(0, ...trace.map((step) => step.hops)),
    queries: trace.length,
    trace,
    limitReached: pending.length > 0,
  };
}
