export function averageCastingScore(scores) {
  const values = Object.values(scores || {}).map(Number).filter((value) => value >= 1 && value <= 10);
  if (!values.length) return 0;
  return Math.round((values.reduce((sum, value) => sum + value, 0) / values.length) * 100) / 100;
}

export function resizeCastingJudges(judges, count) {
  const bounded = Math.max(1, Math.min(20, Number(count) || 1));
  return Array.from({ length: bounded }, (_, index) => judges[index] || { id: index + 1, name: `Судья ${index + 1}` });
}
