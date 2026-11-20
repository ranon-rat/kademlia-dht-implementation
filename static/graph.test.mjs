import test from 'node:test';
import assert from 'node:assert/strict';
import { GraphExploration } from './graph.js';
import { ForceLayout, minimumNodeDistance } from './physics.js';

const source = { id: '18446744073709551615', url: 'http://localhost:9000' };

test('expanding a peer preserves the source and previously discovered nodes', () => {
  const graph = new GraphExploration();
  const peer = { id: '2', url: 'http://localhost:9002' };
  const next = { id: '3', url: 'http://localhost:9003' };
  graph.reset(source);
  graph.expand(source, [source, peer, peer]);
  graph.expand(peer, [source, next]);
  assert.equal(graph.root.id, source.id);
  assert.equal(graph.nodes.size, 3);
  assert.deepEqual(graph.neighbors.get(source.id), ['2']);
  assert.deepEqual(graph.neighbors.get(peer.id), [source.id, '3']);
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
  assert.fail('simulation did not settle');
}

function assertSeparated(layout) {
  const points = [...layout.particles.values()];
  for (let i = 0; i < points.length; i++) {
    for (let j = i + 1; j < points.length; j++) {
      const distance = Math.hypot(points[i].x - points[j].x, points[i].y - points[j].y);
      assert.ok(distance >= minimumNodeDistance - 0.1, `overlap: ${distance}`);
    }
  }
}

test('dense graph separates 50 nodes, anchors its source, and stops animating', () => {
  const layout = new ForceLayout();
  const ids = Array.from({ length: 50 }, (_, index) => String(index + 1));
  const neighbors = new Map(ids.map((id) => [id, ids.filter(other => other !== id)]));
  layout.sync('1', ids, neighbors);
  settle(layout);
  assertSeparated(layout);
  assert.equal(layout.particles.get('1').x, 500);
  assert.equal(layout.particles.get('1').y, 260);
  for (const point of layout.particles.values()) {
    assert.ok(Number.isFinite(point.x) && Number.isFinite(point.y));
  }
});

test('expansion keeps existing positions and seeds new nodes near the referring node', () => {
  const layout = new ForceLayout();
  layout.sync('1', ['1', '2'], new Map([['1', ['2']]]));
  settle(layout);
  const previous = { ...layout.particles.get('2') };
  layout.sync('1', ['1', '2', '3'], new Map([['1', ['2']], ['2', ['3']]]));
  assert.deepEqual(layout.particles.get('2'), previous);
  const added = layout.particles.get('3');
  assert.ok(Math.hypot(added.x - previous.x, added.y - previous.y) < 220);
  settle(layout);
  assertSeparated(layout);
});

test('dragged nodes push peers aside without overlapping the fixed source', () => {
  const layout = new ForceLayout();
  layout.sync('1', ['1', '2', '3'], new Map([['1', ['2', '3']]]));
  settle(layout);
  layout.hold('2');
  layout.move('2', 500, 260);
  layout.step();
  assertSeparated(layout);
  assert.equal(layout.particles.get('1').x, 500);
  assert.equal(layout.particles.get('1').y, 260);
  layout.release();
  settle(layout);
  assertSeparated(layout);
  layout.reset();
  assert.equal(layout.particles.size, 0);
  assert.equal(layout.step(), false);
});

test('changing the selected source keeps peers, edges, and physics positions', () => {
  const graph = new GraphExploration();
  const second = { id: '2', url: 'http://localhost:9002' };
  graph.reset(source);
  graph.expand(source, [second]);
  const layout = new ForceLayout();
  layout.sync(source.id, [...graph.nodes.keys()], graph.neighbors);
  settle(layout);
  const positions = [...layout.particles.values()].map(point => ({ ...point }));
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
