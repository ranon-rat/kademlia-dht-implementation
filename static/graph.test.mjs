import test from "node:test";
import assert from "node:assert/strict";
import { GraphExploration } from "./graph.js";
import { ForceLayout, minimumNodeDistance, xorLinkStrength } from "./physics.js";
const maxNewNodes = 500;
const source = { id: "18446744073709551615", url: "http://localhost:9000" };

test("expanding a peer preserves the source and previously discovered nodes", () => {
  const graph = new GraphExploration();
  const peer = { id: "2", url: "http://localhost:9002" };
  const next = { id: "3", url: "http://localhost:9003" };
  graph.reset(source);
  graph.expand(source, [source, peer, peer]);
  graph.expand(peer, [source, next]);
  assert.equal(graph.root.id, source.id);
  assert.equal(graph.nodes.size, 3);
  assert.deepEqual(graph.neighbors.get(source.id), ["2"]);
  assert.deepEqual(graph.neighbors.get(peer.id), [source.id, "3"]);
  graph.expand(peer, [source, next]);
  assert.equal(graph.nodes.size, 3);
  graph.reset(source);
  assert.equal(graph.nodes.size, 1);
  assert.equal(graph.nodes.get(source.id), source);
  assert.equal(graph.neighbors.size, 0);
});

function settle(layout) {
  for (let i = 0; i < 2000; i++) {
    if (!layout.step()) return i;
  }
  assert.fail("simulation did not settle");
}

function assertSeparated(layout) {
  const points = [...layout.particles.values()];
  for (let i = 0; i < points.length; i++) {
    for (let j = i + 1; j < points.length; j++) {
      const distance = Math.hypot(
        points[i].x - points[j].x,
        points[i].y - points[j].y,
      );
      assert.ok(distance >= minimumNodeDistance - 0.1, `overlap: ${distance}`);
    }
  }
}

test("dense graph separates 50 nodes, anchors its source, and stops animating", () => {
  const layout = new ForceLayout();
  const ids = Array.from({ length: maxNewNodes }, (_, index) =>
    String(index + 1),
  );
  const neighbors = new Map(
    ids.map((id) => [id, ids.filter((other) => other !== id)]),
  );
  layout.sync("1", ids, neighbors);
  settle(layout);
  assertSeparated(layout);
  assert.equal(layout.particles.get("1").x, 500);
  assert.equal(layout.particles.get("1").y, 260);
  for (const point of layout.particles.values()) {
    assert.ok(Number.isFinite(point.x) && Number.isFinite(point.y));
  }
});

test("expansion keeps existing positions and seeds new nodes near the referring node", () => {
  const layout = new ForceLayout();
  layout.sync("1", ["1", "2"], new Map([["1", ["2"]]]));
  settle(layout);
  const previous = { ...layout.particles.get("2") };
  layout.sync(
    "1",
    ["1", "2", "3"],
    new Map([
      ["1", ["2"]],
      ["2", ["3"]],
    ]),
  );
  assert.deepEqual(layout.particles.get("2"), previous);
  const added = layout.particles.get("3");
  assert.ok(Math.hypot(added.x - previous.x, added.y - previous.y) < 220);
  settle(layout);
  assertSeparated(layout);
});

test("dragged nodes push peers aside without overlapping the fixed source", () => {
  const layout = new ForceLayout();
  layout.sync("1", ["1", "2", "3"], new Map([["1", ["2", "3"]]]));
  settle(layout);
  layout.hold("2");
  layout.move("2", 500, 260);
  layout.step();
  assertSeparated(layout);
  assert.equal(layout.particles.get("1").x, 500);
  assert.equal(layout.particles.get("1").y, 260);
  layout.release();
  settle(layout);
  assertSeparated(layout);
  layout.reset();
  assert.equal(layout.particles.size, 0);
  assert.equal(layout.step(), false);
});

test("changing the selected source keeps peers, edges, and physics positions", () => {
  const graph = new GraphExploration();
  const second = { id: "2", url: "http://localhost:9002" };
  graph.reset(source);
  graph.expand(source, [second]);
  const layout = new ForceLayout();
  layout.sync(source.id, [...graph.nodes.keys()], graph.neighbors);
  settle(layout);
  const positions = [...layout.particles.values()].map((point) => ({
    ...point,
  }));
  graph.select(second);
  layout.sync(second.id, [...graph.nodes.keys()], graph.neighbors);
  assert.equal(graph.root.id, second.id);
  assert.equal(graph.nodes.size, 2);
  assert.deepEqual(graph.neighbors.get(source.id), [second.id]);
  assert.deepEqual([...layout.particles.values()], positions);
  graph.reset(second);
  assert.equal(graph.nodes.size, 1);
  assert.equal(graph.root.id, second.id);
});

test('showing a 200-node cluster does not invent edges or discard exploration', () => {
  const graph = new GraphExploration();
  graph.reset(source);
  const external = { id: '999', url: 'http://localhost:9999' };
  graph.expand(source, [external]);
  const cluster = Array.from({ length: 199 }, (_, index) => ({ id:String(index+1),url:`http://localhost:${9001+index}` }));
  graph.addNodes([source,...cluster]);
  assert.equal(graph.nodes.size,201);
  assert.deepEqual(graph.neighbors.get(source.id),['999']);
  assert.equal(graph.neighbors.size,1);
  const layout = new ForceLayout();
  layout.sync(source.id,[source.id,...cluster.map(node=>node.id)],new Map());
  settle(layout);
  assert.equal(layout.particles.size,200);
  assertSeparated(layout);
});

test("link attraction decreases with log2 XOR distance and stays finite", () => {
  const one = xorLinkStrength("0", "1");
  const two = xorLinkStrength("0", "2");
  const four = xorLinkStrength("0", "4");
  assert.equal(one / two, 2);
  assert.equal(one / four, 3);
  assert.ok(xorLinkStrength("0", "16") > xorLinkStrength("0", "256"));
  assert.equal(xorLinkStrength("5", "5"), one);
  assert.equal(xorLinkStrength("18446744073709551615", "18446744073709551614"), one);
  assert.equal(xorLinkStrength("18446744073709551615", "0"), xorLinkStrength("0", "18446744073709551615"));
  assert.ok(Number.isFinite(xorLinkStrength("0", "18446744073709551615")));
});

test("closer XOR identities pull more strongly at the same physical distance", () => {
  const layout = new ForceLayout();
  const root = "18446744073709551615";
  const near = "18446744073709551614";
  const far = "1";
  layout.sync(root, [root, near, far], new Map([[root, [near, far]]]));
  Object.assign(layout.particles.get(near), {x:720,y:260,vx:0,vy:0});
  Object.assign(layout.particles.get(far), {x:280,y:260,vx:0,vy:0});
  layout.step();
  const nearPull = 720 - layout.particles.get(near).x;
  const farPull = layout.particles.get(far).x - 280;
  assert.ok(nearPull > farPull, `${nearPull} should exceed ${farPull}`);
  settle(layout);
  assertSeparated(layout);
});
