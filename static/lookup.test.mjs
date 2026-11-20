import test from 'node:test';
import assert from 'node:assert/strict';
import { searchNetwork } from './lookup.js';
import { parseResponse, formatResponse } from './response.js';

const peer = (id, distance = '1') => ({ id, value: `http://localhost:${id}`, XorDistance: distance });
const start = { id: '1', url: 'http://localhost:1' };

test('follows candidates across hops, skips cycles, and continues after failed peers', async () => {
  const visited = [];
  const result = await searchNetwork(start, async node => {
    visited.push(node.id);
    switch (node.id) {
      case '1': return { value: null, node: [peer('1'), peer('3', '9'), peer('2', '2')] };
      case '2': throw new Error('offline');
      case '3': return { node: [peer('1'), peer('4')] };
      case '4': return { value: 'found' };
      default: throw new Error('unexpected node');
    }
  });
  assert.deepEqual(visited, ['1', '2', '3', '4']);
  assert.equal(result.value, 'found');
  assert.equal(result.hops, 2);
  assert.equal(result.queries, 4);
  assert.deepEqual(result.trace.map(step => step.hops), [0, 1, 1, 2]);
  assert.equal(result.node.id, '4');
  assert.equal(result.trace[3].parent.id, '3');
  assert.equal(result.trace[2].parent.id, '1');
  assert.match(result.trace[1].status, /offline/);
});

test('empty string is a found value', async () => {
  const result = await searchNetwork(start, async () => ({ value: '' }));
  assert.equal(result.found, true);
  assert.equal(result.value, '');
  assert.equal(result.hops, 0);
  assert.equal(result.queries, 1);
});

test('missing value exhausts peers once', async () => {
  let visits = 0;
  const result = await searchNetwork(start, async () => { visits++; return { value: null, node: [peer('1')] }; });
  assert.equal(result.found, false);
  assert.equal(result.limitReached, false);
  assert.equal(visits, 1);
});

test('lookup stops at visit limit', async () => {
  const result = await searchNetwork(start, async node => ({ node: [peer(String(Number(node.id) + 1))] }), undefined, 3);
  assert.equal(result.trace.length, 3);
  assert.equal(result.hops, 2);
  assert.equal(result.queries, 3);
  assert.equal(result.limitReached, true);
});

test('response formatting preserves uint64 identifiers and message contents', () => {
  const raw = '{"id":18446744073709551615,"content":"number 18446744073709551615 and \\\"quotes\\\"","node":[{"XorDistance":18446744073709551614}]}';
  const parsed = parseResponse(raw);
  assert.equal(parsed.id, '18446744073709551615');
  assert.equal(parsed.node[0].XorDistance, '18446744073709551614');
  assert.equal(parsed.content, 'number 18446744073709551615 and "quotes"');
  assert.match(formatResponse(raw), /\n  "id": "18446744073709551615"/);
  assert.equal(formatResponse('not found'), 'not found');
});

test('reports live visits and completions in request order', async () => {
  const events = [];
  await searchNetwork(
    start,
    async (node) => node.id === '1' ? { node: [peer('2')] } : { value: 'yes' },
    (node, hops, parent) => events.push(['visit', node.id, hops, parent?.id]),
    100,
    (step) => events.push(['complete', step.node.id, step.hops, step.parent?.id]),
  );
  assert.deepEqual(events, [
    ['visit', '1', 0, undefined],
    ['complete', '1', 0, undefined],
    ['visit', '2', 1, '1'],
    ['complete', '2', 1, '1'],
  ]);
});
