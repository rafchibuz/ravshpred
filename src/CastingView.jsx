import React, { useEffect, useMemo, useState } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronRight,
  ClipboardList,
  Crown,
  Download,
  ExternalLink,
  FileUp,
  Film,
  Medal,
  ShieldCheck,
  Star,
  Trophy,
  UserRound,
} from 'lucide-react';
import { parseYouTubeId } from './domain.js';
import * as api from './api.js';
import { importCastingWorkbook } from './castingImport.js';
import { averageCastingScore, resizeCastingJudges } from './castingScores.js';
import { exportCastingResults } from './castingExport.js';
import './casting.css';

const sectionIcons = [UserRound, Medal, Star, ShieldCheck, Trophy];
const castingFieldLabels = {
  'Обязательства': 'Обязательства, которые могут помешать участию',
  'Что готов изменить': 'Что готов изменить ради участия',
};
const hiddenCastingFields = new Set([
  'Фамилия',
  'Отчество',
  'Официальные соревнования',
  'Опыт реальных поединков',
  'Официальных боёв',
  'Тренер / контракт',
]);
const castingPositionKey = 'casting-active-participant';
function youtubeEmbed(url) {
  const id = parseYouTubeId(url);
  return id ? `https://www.youtube-nocookie.com/embed/${id}?rel=0&modestbranding=1&playsinline=1` : '';
}

function googleDriveEmbed(url) {
  const match = String(url || '').match(/drive\.google\.com\/file\/d\/([^/]+)/i);
  return match ? `https://drive.google.com/file/d/${match[1]}/preview` : '';
}

function isDirectVideo(url) {
  return /\.(?:mp4|mov|webm)(?:[?#].*)?$/i.test(String(url || ''));
}

function linkify(value) {
  const parts = String(value || '').split(/\n+/).map((item) => item.trim()).filter(Boolean);
  return parts.map((part, index) => {
    const match = part.match(/https?:\/?\/?[^\s]+/i);
    if (!match) return <span key={`${part}-${index}`}>{part}</span>;
    let href = match[0];
    if (/^https:t\.me/i.test(href)) href = href.replace(/^https:/i, 'https://');
    return <a key={`${part}-${index}`} href={href} target="_blank" rel="noreferrer">{part}<ExternalLink size={12} /></a>;
  });
}

function InfoSection({ section, index }) {
  const Icon = sectionIcons[index] || ClipboardList;
  return (
    <section className="casting-card casting-info-card">
      <header><Icon size={19} /><h2>{section.title}</h2></header>
      <div className="casting-field-grid">
        {section.fields.filter((item) => !hiddenCastingFields.has(item.label)).map((item) => (
          <div className="casting-field" key={item.label}>
            <span>{castingFieldLabels[item.label] || item.label}</span>
            <strong className={item.label === 'Ссылки' ? 'casting-links' : ''}>
              {item.label === 'Ссылки' ? linkify(item.value) : item.value || '—'}
            </strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function JudgeSlider({ judge, value, saving, canManage, onNameChange, onChange }) {
  const [draft, setDraft] = useState(value || 1);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setDraft(value || 1);
    setDirty(false);
  }, [value]);

  const commit = (event) => {
    const score = Number(event.currentTarget.value);
    setDirty(false);
    if (!saving && score !== value) onChange(score);
  };

  return (
    <div className="casting-judge-slider">
      <div>{canManage ? <input value={judge.name} maxLength="80" aria-label={`Имя судьи ${judge.id}`} onChange={(event) => onNameChange(event.target.value)} /> : <strong>{judge.name}</strong>}<b>{dirty ? draft : value || '—'}</b></div>
      <div className="casting-score-slider">
        <input type="range" min="1" max="10" step="0.5" value={draft} disabled={saving} aria-label={`Оценка судьи ${judge.name}`}
          onChange={(event) => { setDraft(Number(event.target.value)); setDirty(true); }} onPointerUp={commit} onKeyUp={commit} />
        <div className="casting-score-ticks" aria-hidden="true">{Array.from({ length: 10 }, (_, index) => <span key={index}>{index + 1}</span>)}</div>
      </div>
    </div>
  );
}

function RatingControl({ judges, scores, savingKey, canManage, onChange, onSaveJudges }) {
  const [draftJudges, setDraftJudges] = useState(judges);
  const [judgesDirty, setJudgesDirty] = useState(false);
  const [savingJudges, setSavingJudges] = useState(false);

  useEffect(() => {
    if (!judgesDirty) setDraftJudges(judges);
  }, [judges, judgesDirty]);

  const average = averageCastingScore(scores);
  const scoredCount = Object.values(scores || {}).filter((score) => Number(score) > 0).length;

  const saveJudges = async () => {
    if (!judgesDirty || savingJudges) return;
    if (draftJudges.length < judges.length && !window.confirm('Оценки удалённых судей будут удалены. Продолжить?')) return;
    setSavingJudges(true);
    try {
      await onSaveJudges(draftJudges.map((judge) => judge.name.trim() || `Судья ${judge.id}`));
      setJudgesDirty(false);
    } catch {
      // The parent keeps the current form and shows the API error.
    } finally {
      setSavingJudges(false);
    }
  };

  return (
    <section className="casting-card casting-score-card">
      <header><Star size={20} /><h2>Оценка участника</h2></header>
      <div className="casting-score-value"><b>{average || '—'}</b><span>/ 10</span></div>
      <div className="casting-score-summary">Средняя оценка · выставлено {scoredCount} из {judges.length}</div>
      {canManage && (
        <div className="casting-judges-config">
          <label>Количество судей<input type="number" min="1" max="20" value={draftJudges.length} onChange={(event) => { setDraftJudges((current) => resizeCastingJudges(current, event.target.value)); setJudgesDirty(true); }} /></label>
          <button disabled={!judgesDirty || savingJudges} onClick={saveJudges}>{savingJudges ? 'Сохраняем…' : 'Сохранить судей'}</button>
        </div>
      )}
      <div className="casting-judge-list">
        {(canManage ? draftJudges : judges).map((judge, index) => <JudgeSlider key={judge.id} judge={judge} value={Number(scores?.[judge.id]) || 0} saving={Boolean(savingKey) || judge.id > judges.length} canManage={canManage} onNameChange={(name) => { setDraftJudges((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, name } : item)); setJudgesDirty(true); }} onChange={(score) => onChange(judge.id, score)} />)}
      </div>
      <p>{savingKey ? 'Сохраняем…' : scoredCount ? <><Check size={14} /> Все оценки и среднее видны проверяющим.</> : 'Передвиньте ползунок и отпустите его.'}</p>
    </section>
  );
}

function Ranking({ participants, judges, scores, winners, winnerSaving, onToggleWinner, onExport, onOpen, onBack }) {
  const winnerSet = useMemo(() => new Set((winners || []).map(Number)), [winners]);
  const ranked = useMemo(() => participants
    .map((participant) => ({ ...participant, average: averageCastingScore(scores[participant.id]) }))
    .filter((participant) => participant.average > 0)
    .sort((a, b) => b.average - a.average || a.id - b.id), [participants, scores]);
  return (
    <section className="casting-ranking">
      <div className="casting-ranking-head">
        <div><span>ОБЩИЙ РЕЙТИНГ</span><h1>Оценённые участники</h1><p>Здесь отображаются общие оценки всех проверяющих. Последнее изменение оценки видят все.</p></div>
        <div className="casting-ranking-actions"><strong>{ranked.length} из {participants.length}</strong><button onClick={onExport}><Download size={15} /> Выгрузить Excel</button><button onClick={onBack}><Film size={15} /> К анкетам</button></div>
      </div>
      {!ranked.length ? <div className="casting-empty">Пока нет оценок. Вернитесь к просмотру заявок.</div> : (
        <div className="casting-ranking-list">
          {ranked.map((participant, index) => {
            const winner = winnerSet.has(Number(participant.id));
            return (
              <div className={`casting-ranking-row ${winner ? 'winner' : ''}`} key={participant.id}>
                <button className="casting-ranking-open" onClick={() => onOpen(participant.id)}>
                  <b className={index < 3 ? `place place-${index + 1}` : 'place'}>#{index + 1}</b>
                  <span><strong>{participant.full_name}</strong><small>@{participant.twitch} · {participant.city}</small></span>
                  <em>{participant.average}<small>/10</small></em>
                  <ChevronRight size={18} />
                </button>
                <button className={`casting-winner-toggle ${winner ? 'selected' : ''}`} disabled={Boolean(winnerSaving)} onClick={() => onToggleWinner(participant.id, !winner)}><Crown size={15} />{winner ? 'Победитель' : 'Выбрать'}</button>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

export function CastingView({ notify, canManage = false }) {
  const [payload, setPayload] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeIndex, setActiveIndex] = useState(() => {
    const saved = Number.parseInt(window.localStorage.getItem(castingPositionKey), 10);
    return Number.isFinite(saved) && saved > 0 ? saved - 1 : 0;
  });
  const [videoIndex, setVideoIndex] = useState(0);
  const [mode, setMode] = useState('review');
  const [savingKey, setSavingKey] = useState('');
  const [winnerSaving, setWinnerSaving] = useState(0);
  const [participantNumber, setParticipantNumber] = useState('1');

  useEffect(() => {
    let active = true;
    document.body.classList.add('casting-open');
    api.loadCasting()
      .then((data) => { if (active) setPayload(data); })
      .catch((requestError) => { if (active) setError(requestError.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; document.body.classList.remove('casting-open'); };
  }, []);

  useEffect(() => {
    setParticipantNumber(String(activeIndex + 1));
  }, [activeIndex]);

  const participantCount = payload?.participants?.length || 0;
  useEffect(() => {
    if (!participantCount) return;
    const bounded = Math.max(0, Math.min(activeIndex, participantCount - 1));
    if (bounded !== activeIndex) {
      setActiveIndex(bounded);
      return;
    }
    window.localStorage.setItem(castingPositionKey, String(activeIndex + 1));
  }, [activeIndex, participantCount]);

  if (loading) return <main className="casting-page casting-state"><span className="clips-loader" /><h2>Загружаем анкеты</h2></main>;
  if (error || !payload) return <main className="casting-page casting-state"><ShieldCheck size={34} /><h2>Кастинг недоступен</h2><p>{error || 'Не удалось получить данные.'}</p></main>;

  const participants = payload.participants || [];
  const judges = payload.judges?.length ? payload.judges : [{ id: 1, name: 'Судья 1' }];
  const scores = payload.scores || {};
  const winners = payload.winners || [];
  if (!participants.length) return <main className="casting-page casting-state"><FileUp size={34} /><h2>Участники ещё не загружены</h2><p>Владелец может загрузить Excel в панели управления.</p></main>;
  const displayedIndex = Math.max(0, Math.min(activeIndex, participants.length - 1));
  const participant = participants[displayedIndex];
  const changeParticipant = (next) => {
    const bounded = Math.max(0, Math.min(next, participants.length - 1));
    setActiveIndex(bounded);
    setParticipantNumber(String(bounded + 1));
    setVideoIndex(0);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };
  const jumpToParticipant = () => {
    const requested = Number.parseInt(participantNumber, 10);
    if (!Number.isFinite(requested)) {
      setParticipantNumber(String(activeIndex + 1));
      return;
    }
    changeParticipant(requested - 1);
  };
  const saveScore = async (judgeId, score) => {
    if (!participant || savingKey) return;
    const previous = scores[participant.id]?.[judgeId];
    setPayload((current) => ({ ...current, scores: { ...current.scores, [participant.id]: { ...current.scores?.[participant.id], [judgeId]: score } } }));
    setSavingKey(String(judgeId));
    try {
      await api.saveCastingScore(participant.id, judgeId, score);
      notify?.(`Оценка ${score}/10 сохранена`);
    } catch (requestError) {
      setPayload((current) => ({ ...current, scores: { ...current.scores, [participant.id]: { ...current.scores?.[participant.id], [judgeId]: previous } } }));
      notify?.(requestError.message);
    } finally {
      setSavingKey('');
    }
  };
  const saveJudges = async (names) => {
    try {
      const updatedJudges = await api.replaceCastingJudges(names);
      const updated = await api.loadCasting();
      setPayload({ ...updated, judges: updatedJudges });
      notify?.('Состав судей сохранён');
    } catch (requestError) {
      notify?.(requestError.message);
      throw requestError;
    }
  };
  const toggleWinner = async (participantId, selected) => {
    if (winnerSaving) return;
    const previous = winners;
    const next = selected ? [...new Set([...winners, participantId])] : winners.filter((id) => Number(id) !== Number(participantId));
    setPayload((current) => ({ ...current, winners: next }));
    setWinnerSaving(participantId);
    try {
      await api.setCastingWinner(participantId, selected);
      notify?.(selected ? 'Победитель выбран' : 'Отметка победителя снята');
    } catch (requestError) {
      setPayload((current) => ({ ...current, winners: previous }));
      notify?.(requestError.message);
    } finally {
      setWinnerSaving(0);
    }
  };
  const downloadResults = async () => {
    try {
      await exportCastingResults(participants, judges, scores, winners);
      notify?.('Excel-файл сформирован');
    } catch (requestError) {
      notify?.(requestError.message || 'Не удалось сформировать Excel-файл');
    }
  };

  return (
    <main className="casting-page">
      {mode === 'ranking' ? <Ranking participants={participants} judges={judges} scores={scores} winners={winners} winnerSaving={winnerSaving} onToggleWinner={toggleWinner} onExport={downloadResults} onBack={() => setMode('review')} onOpen={(id) => { setActiveIndex(participants.findIndex((item) => item.id === id)); setVideoIndex(0); setMode('review'); }} /> : (
        <>
          <div className="casting-layout">
            <div className="casting-column">{participant.sections.slice(0, 2).map((section, index) => <InfoSection key={section.title} section={section} index={index} />)}</div>
            <div className="casting-column casting-media-column">
              <section className="casting-person-head">
                <div className="casting-person-avatar">{participant.full_name.split(/\s+/).slice(0, 2).map((part) => part[0]).join('')}</div>
                <div className="casting-person-copy"><span>УЧАСТНИК {displayedIndex + 1} ИЗ {participants.length}</span><h2>{participant.full_name}{participant.city ? `, ${participant.city}` : ''}</h2></div>
                <nav className="casting-top-pagination" aria-label="Навигация по анкетам">
                  <button disabled={displayedIndex === 0} onClick={() => changeParticipant(displayedIndex - 1)}><ArrowLeft size={17} /><span>Предыдущий</span></button>
                  <form className="casting-participant-jump" onSubmit={(event) => { event.preventDefault(); jumpToParticipant(); }}>
                    <input type="number" min="1" max={participants.length} value={participantNumber} aria-label="Номер участника" onChange={(event) => setParticipantNumber(event.target.value)} onBlur={jumpToParticipant} onWheel={(event) => event.currentTarget.blur()} />
                    <span>/ {participants.length}</span>
                  </form>
                  <button className="next" disabled={displayedIndex === participants.length - 1} onClick={() => changeParticipant(displayedIndex + 1)}><span>Следующий</span><ArrowRight size={17} /></button>
                </nav>
                <button className="casting-ranking-button" onClick={() => setMode('ranking')}><Trophy size={16} /> Рейтинг</button>
              </section>
              <section className="casting-card casting-video-card">
                <header><Film size={19} /><h2>Видео участника</h2></header>
                <div className="casting-video-tabs">{participant.videos.map((video, index) => <button className={index === videoIndex ? 'active' : ''} key={video.title} onClick={() => setVideoIndex(index)}><b>{index + 1}</b>{video.title}</button>)}</div>
                <div className="casting-video-frame">
                  {youtubeEmbed(participant.videos[videoIndex]?.url) ? (
                    <iframe title={`${participant.full_name}: ${participant.videos[videoIndex].title}`} src={youtubeEmbed(participant.videos[videoIndex].url)} allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerPolicy="strict-origin-when-cross-origin" allowFullScreen />
                  ) : googleDriveEmbed(participant.videos[videoIndex]?.url) ? (
                    <iframe title={`${participant.full_name}: ${participant.videos[videoIndex].title}`} src={googleDriveEmbed(participant.videos[videoIndex].url)} allow="autoplay; encrypted-media" allowFullScreen />
                  ) : isDirectVideo(participant.videos[videoIndex]?.url) ? (
                    <video src={participant.videos[videoIndex].url} controls playsInline preload="metadata" />
                  ) : participant.videos[videoIndex]?.url ? (
                    <div className="casting-external-video"><p>Видео размещено на внешнем сервисе.</p><a href={participant.videos[videoIndex].url} target="_blank" rel="noreferrer">Открыть видео</a></div>
                  ) : (
                    <div className="casting-video-text"><strong>Ответ участника</strong><p>{participant.videos[videoIndex]?.text || 'Ссылка или описание не указаны'}</p></div>
                  )}
                </div>
              </section>
              <InfoSection section={participant.sections[2]} index={2} />
            </div>
            <aside className="casting-column casting-side-column">
              <RatingControl judges={judges} scores={scores[participant.id] || {}} savingKey={savingKey} canManage={canManage} onChange={saveScore} onSaveJudges={saveJudges} />
              <InfoSection section={participant.sections[3]} index={3} />
              <InfoSection section={participant.sections[4]} index={4} />
            </aside>
          </div>
        </>
      )}
    </main>
  );
}

export function CastingReviewersPanel({ notify }) {
  const [reviewers, setReviewers] = useState([]);
  const [login, setLogin] = useState('');
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [importedCount, setImportedCount] = useState(null);
  useEffect(() => {
    let active = true;
    api.loadCastingReviewers().then((items) => { if (active) setReviewers(items); }).catch((error) => notify?.(error.message)).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [notify]);
  const add = async () => {
    const value = login.trim().replace(/^@/, '');
    if (!value) return notify?.('Введите Twitch-ник');
    try {
      const user = await api.assignCastingReviewer(value);
      setReviewers((current) => [...current.filter((item) => item.id !== user.id), user]);
      setLogin('');
      notify?.('Доступ к кастингу выдан');
    } catch (error) { notify?.(error.message); }
  };
  const uploadParticipants = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file || importing) return;
    try {
      setImporting(true);
      const participants = await importCastingWorkbook(file);
      if (!window.confirm(`Загрузить ${participants.length} участников? Текущий список и все выставленные оценки будут заменены.`)) return;
      const result = await api.replaceCastingParticipants(participants);
      setImportedCount(result.count);
      notify?.(`Загружено участников: ${result.count}`);
    } catch (error) {
      notify?.(error.message);
    } finally {
      setImporting(false);
    }
  };
  return (
    <section className="panel owner-panel casting-reviewers-panel">
      <div className="panel-head"><div><h2>Доступ к кастингу</h2><p>Пользователь должен хотя бы один раз войти на сайт через Twitch.</p></div><span className="panel-kicker">{reviewers.length} ДОПУЩЕНО</span></div>
      <div className="casting-import-row">
        <div><strong>Участники кастинга</strong><p>Загрузите исходный Excel формы. Фамилии и отчества не сохраняются. Новая загрузка заменит список и сбросит оценки.</p>{importedCount !== null && <small>Последняя загрузка: {importedCount} участников</small>}</div>
        <label className={`primary-btn casting-import-button ${importing ? 'disabled' : ''}`}><FileUp size={16} /> {importing ? 'Загружаем…' : 'Загрузить Excel'}<input type="file" accept=".xlsx" disabled={importing} onChange={uploadParticipants} /></label>
      </div>
      <div className="add-row"><input value={login} onChange={(event) => setLogin(event.target.value)} placeholder="Точный Twitch-ник" onKeyDown={(event) => { if (event.key === 'Enter') add(); }} /><button className="primary-btn" onClick={add}>Выдать доступ</button></div>
      {loading && <div className="owner-table-loading">Загружаем список…</div>}
      {!loading && !reviewers.length && <div className="owner-table-loading">Отдельный доступ пока никому не выдан. Основатель видит кастинг всегда.</div>}
      {reviewers.map((user) => <div className="moderator-row" key={user.id}><div className="casting-reviewer-avatar">{(user.display_name || user.login || '?')[0]}</div><strong>{user.display_name || user.login}</strong><b>@{user.login}</b><button className="danger-btn" onClick={async () => { try { await api.removeCastingReviewer(user.id); setReviewers((current) => current.filter((item) => item.id !== user.id)); notify?.('Доступ отозван'); } catch (error) { notify?.(error.message); } }}>Отозвать</button></div>)}
    </section>
  );
}
