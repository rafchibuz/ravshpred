import { averageCastingScore } from './castingScores.js';

export function buildCastingExportTable(participants, judges, scores, winners) {
  const winnerSet = new Set((winners || []).map(Number));
  const ranked = (participants || []).map((participant) => ({
    participant,
    average: averageCastingScore(scores?.[participant.id]),
    judgeScores: judges.map((judge) => Number(scores?.[participant.id]?.[judge.id]) || null),
  })).sort((a, b) => b.average - a.average || a.participant.id - b.participant.id);

  const headers = ['Место', 'Участник', 'Город', ...judges.map((judge) => judge.name), 'Общая оценка', 'Победитель'];
  let ratedPlace = 0;
  const rows = ranked.map(({ participant, average, judgeScores }) => {
    if (average > 0) ratedPlace += 1;
    return [average > 0 ? ratedPlace : null, participant.full_name, participant.city || '', ...judgeScores, average || null, winnerSet.has(Number(participant.id)) ? 'Да' : 'Нет'];
  });
  return [headers, ...rows];
}

export async function exportCastingResults(participants, judges, scores, winners) {
  const XLSX = await import('xlsx');
  const table = buildCastingExportTable(participants, judges, scores, winners);
  const worksheet = XLSX.utils.aoa_to_sheet(table);
  worksheet['!cols'] = [
    { wch: 8 }, { wch: 28 }, { wch: 20 },
    ...judges.map(() => ({ wch: 16 })),
    { wch: 16 }, { wch: 14 },
  ];
  worksheet['!autofilter'] = { ref: `A1:${XLSX.utils.encode_col(table[0].length - 1)}${table.length}` };
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, 'Рейтинг');
  XLSX.writeFile(workbook, `Рейтинг-кастинга-${new Date().toISOString().slice(0, 10)}.xlsx`, { compression: true });
}
