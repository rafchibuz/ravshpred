import test from 'node:test';
import assert from 'node:assert/strict';
import { averageCastingScore, resizeCastingJudges } from '../src/castingScores.js';

test('averageCastingScore averages all submitted judge scores', () => {
  assert.equal(averageCastingScore({ 1: 7, 2: 8.5, 3: 9 }), 8.17);
  assert.equal(averageCastingScore({}), 0);
});

test('resizeCastingJudges preserves names and creates defaults', () => {
  assert.deepEqual(resizeCastingJudges([{ id: 1, name: 'Анна' }], 3), [
    { id: 1, name: 'Анна' },
    { id: 2, name: 'Судья 2' },
    { id: 3, name: 'Судья 3' },
  ]);
});
