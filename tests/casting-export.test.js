import test from 'node:test';
import assert from 'node:assert/strict';
import { buildCastingExportTable } from '../src/castingExport.js';

test('buildCastingExportTable creates judge, average and winner columns', () => {
  const table = buildCastingExportTable(
    [{ id: 1, full_name: 'Анна', city: 'Москва' }, { id: 2, full_name: 'Игорь', city: 'Казань' }],
    [{ id: 1, name: 'Судья Антон' }, { id: 2, name: 'Судья Мария' }],
    { 1: { 1: 8, 2: 9 }, 2: { 1: 7, 2: 7.5 } },
    [2],
  );

  assert.deepEqual(table[0], ['Место', 'Участник', 'Город', 'Судья Антон', 'Судья Мария', 'Общая оценка', 'Победитель']);
  assert.deepEqual(table[1], [1, 'Анна', 'Москва', 8, 9, 8.5, 'Нет']);
  assert.deepEqual(table[2], [2, 'Игорь', 'Казань', 7, 7.5, 7.25, 'Да']);
});
