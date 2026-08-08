import test from 'node:test';
import assert from 'node:assert/strict';
import {
  can,
  deduplicateTwitchClips,
  isDuplicate,
  latestTwitchStreamClips,
  newPendingSubmissions,
  parseYouTubeId,
  recentSubmissionCount,
  removeCategory,
  toggleVote,
  transitionSubmission,
  voteTotals,
} from '../src/domain.js';

test('фильтр Twitch-клипов оставляет самый популярный клип одного момента', () => {
  const clips = [
    { id: 'low', video_id: 'vod-1', vod_offset: 100, view_count: 20 },
    { id: 'best', video_id: 'vod-1', vod_offset: 118, view_count: 200 },
    { id: 'other', video_id: 'vod-1', vod_offset: 220, view_count: 10 },
    { id: 'live-a', video_id: '', vod_offset: null, view_count: 5 },
    { id: 'live-b', video_id: '', vod_offset: null, view_count: 4 },
  ];
  assert.deepEqual(deduplicateTwitchClips(clips).map((clip) => clip.id), ['best', 'other', 'live-a', 'live-b']);
});

test('главная выбирает клипы последнего Twitch-стрима', () => {
  const clips = [
    { id: 'old', broadcaster_name: 'RavshanN', video_id: 'vod-old', created_at: '2026-08-07T18:00:00Z' },
    { id: 'latest-a', broadcaster_name: 'ravshanbtw', video_id: 'vod-new', created_at: '2026-08-09T18:00:00Z' },
    { id: 'latest-b', broadcaster_name: 'ravshanbtw', video_id: 'vod-new', created_at: '2026-08-09T18:10:00Z' },
  ];
  assert.deepEqual(latestTwitchStreamClips(clips).map((clip) => clip.id), ['latest-a', 'latest-b']);
  assert.deepEqual(latestTwitchStreamClips(clips, 'ravshann').map((clip) => clip.id), ['old']);
});

test('парсер понимает основные форматы YouTube URL', () => {
  const id = 'dQw4w9WgXcQ';
  assert.equal(parseYouTubeId(`https://www.youtube.com/watch?v=${id}&t=12`), id);
  assert.equal(parseYouTubeId(`https://youtu.be/${id}`), id);
  assert.equal(parseYouTubeId(`https://youtube.com/shorts/${id}`), id);
  assert.equal(parseYouTubeId(`https://youtube.com/live/${id}`), id);
  assert.equal(parseYouTubeId('https://example.com/video'), null);
});

test('дубликат определяется по YouTube ID', () => {
  assert.equal(isDuplicate([{ youtubeId: 'dQw4w9WgXcQ', status: 'approved' }], 'dQw4w9WgXcQ'), true);
  assert.equal(isDuplicate([{ youtubeId: 'dQw4w9WgXcQ', status: 'hidden' }], 'dQw4w9WgXcQ'), false);
});

test('звуковое уведомление выбирает только новые pending-видео', () => {
  const videos = [
    { id: 'old', status: 'pending' },
    { id: 'new', status: 'pending' },
    { id: 'approved', status: 'approved' },
  ];
  assert.deepEqual(newPendingSubmissions(new Set(['old']), videos), [{ id: 'new', status: 'pending' }]);
});

test('суточный лимит считает только последние 24 часа', () => {
  const now = Date.parse('2026-08-06T12:00:00.000Z');
  const videos = [
    { authorId: 'u1', createdAt: '2026-08-06T11:00:00.000Z', status: 'pending' },
    { authorId: 'u1', createdAt: '2026-08-05T13:00:00.000Z', status: 'approved' },
    { authorId: 'u1', createdAt: '2026-08-05T10:00:00.000Z', status: 'approved' },
    { authorId: 'u2', createdAt: '2026-08-06T10:00:00.000Z', status: 'approved' },
  ];
  assert.equal(recentSubmissionCount(videos, 'u1', now), 2);
});

test('RBAC отделяет гостя, модератора и основателя', () => {
  assert.equal(can('guest', 'vote'), false);
  assert.equal(can('moderator', 'moderate'), true);
  assert.equal(can('moderator', 'manage'), false);
  assert.equal(can('owner', 'manage'), true);
  assert.equal(can('guest', 'comment_news'), false);
  assert.equal(can('user', 'comment_news'), true);
});

test('переход модерации сбрасывает отсмотрено вне approved', () => {
  const approved = transitionSubmission({ status: 'pending', watched: false }, 'approved', 'moderator');
  assert.equal(approved.status, 'approved');
  const rejected = transitionSubmission({ status: 'approved', watched: true }, 'rejected', 'owner');
  assert.equal(rejected.watched, false);
  assert.throws(() => transitionSubmission({ status: 'pending' }, 'approved', 'user'));
});

test('один пользователь имеет только один актуальный голос', () => {
  const video = { baseLikes: 10, baseDislikes: 2, votes: {} };
  const liked = toggleVote(video, 'u1', 'up');
  assert.deepEqual(voteTotals(liked), { likes: 11, dislikes: 2, score: 9 });
  const changed = toggleVote(liked, 'u1', 'down');
  assert.deepEqual(voteTotals(changed), { likes: 10, dislikes: 3, score: 7 });
  const removed = toggleVote(changed, 'u1', 'down');
  assert.deepEqual(voteTotals(removed), { likes: 10, dislikes: 2, score: 8 });
});

test('удаление категории переносит видео в системную категорию', () => {
  const state = {
    categories: ['Без категории', 'Трейлеры'],
    videos: [{ id: 1, category: 'Трейлеры' }, { id: 2, category: 'Без категории' }],
  };
  const next = removeCategory(state, 'Трейлеры');
  assert.deepEqual(next.categories, ['Без категории']);
  assert.equal(next.videos[0].category, 'Без категории');
  assert.throws(() => removeCategory(state, 'Без категории'));
});
