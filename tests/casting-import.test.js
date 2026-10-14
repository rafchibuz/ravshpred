import test from 'node:test';
import assert from 'node:assert/strict';
import { parseCastingRows } from '../src/castingImport.js';

test('parseCastingRows maps form columns and keeps text when video link is absent', () => {
  const headers = Array.from({ length: 30 }, (_, index) => `Колонка ${index + 1}`);
  headers[1] = 'Имя';
  headers[3] = 'Город';
  const row = Array.from({ length: 30 }, () => '');
  row[0] = 'Фамилия не должна попасть в результат';
  row[1] = 'Артур';
  row[2] = 'Отчество не должно попасть в результат';
  row[3] = 'Москва';
  row[6] = '20';
  row[7] = '183';
  row[8] = '78';
  row[18] = 'Да';
  row[21] = 'Переехать';
  row[22] = 'https://youtu.be/example';
  row[23] = 'Видео пока нет, но тренируюсь каждый день';
  row[25] = 'За свой счёт';
  row[26] = 'Нет';

  const [participant] = parseCastingRows([headers, row]);

  assert.equal(participant.full_name, 'Артур');
  assert.equal(participant.height, '183 см');
  assert.equal(participant.videos[0].url, 'https://youtu.be/example');
  assert.equal(participant.videos[1].url, '');
  assert.equal(participant.videos[1].text, 'Видео пока нет, но тренируюсь каждый день');
  assert.equal(participant.sections[2].fields[2].label, 'Что готов изменить ради участия');
  assert.equal(participant.sections[3].fields[0].value, 'Да');
  assert.equal(participant.sections[3].fields[2].label, 'Обязательства, которые могут помешать участию');
  assert.equal(JSON.stringify(participant).includes('Фамилия не должна'), false);
  assert.equal(JSON.stringify(participant).includes('Отчество не должно'), false);
});

test('parseCastingRows rejects an unexpected workbook layout', () => {
  assert.throws(() => parseCastingRows([['wrong'], ['value']]), /не соответствуют/);
});
