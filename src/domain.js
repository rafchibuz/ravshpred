const YOUTUBE_ID_PATTERN = /^[A-Za-z0-9_-]{11}$/;

export function parseYouTubeId(value) {
  const input = String(value || '').trim();
  if (!input) return null;

  try {
    const url = new URL(input);
    const host = url.hostname.replace(/^www\./, '').toLowerCase();

    if (host === 'youtu.be') {
      const id = url.pathname.split('/').filter(Boolean)[0];
      return YOUTUBE_ID_PATTERN.test(id || '') ? id : null;
    }

    if (host === 'youtube.com' || host === 'm.youtube.com' || host === 'music.youtube.com') {
      const pathParts = url.pathname.split('/').filter(Boolean);
      const candidate =
        url.pathname === '/watch'
          ? url.searchParams.get('v')
          : ['shorts', 'embed', 'live'].includes(pathParts[0])
            ? pathParts[1]
            : null;
      return YOUTUBE_ID_PATTERN.test(candidate || '') ? candidate : null;
    }
  } catch {
    return YOUTUBE_ID_PATTERN.test(input) ? input : null;
  }

  return null;
}

export function isDuplicate(videos, youtubeId) {
  return videos.some((video) => video.youtubeId === youtubeId && video.status !== 'hidden');
}

export function recentSubmissionCount(videos, authorId, now = Date.now()) {
  const windowStart = now - 24 * 60 * 60 * 1000;
  return videos.filter(
    (video) =>
      video.authorId === authorId &&
      new Date(video.createdAt).getTime() >= windowStart &&
      video.status !== 'hidden',
  ).length;
}

export function can(role, action) {
  const matrix = {
    guest: ['view_feed', 'open_video'],
    user: ['view_feed', 'open_video', 'submit', 'vote', 'view_profile', 'comment_news'],
    moderator: [
      'view_feed',
      'open_video',
      'submit',
      'vote',
      'view_profile',
      'moderate',
      'mark_watched',
      'comment_news',
    ],
    owner: [
      'view_feed',
      'open_video',
      'submit',
      'vote',
      'view_profile',
      'moderate',
      'mark_watched',
      'manage',
      'comment_news',
    ],
  };
  return matrix[role]?.includes(action) ?? false;
}

export function transitionSubmission(video, nextStatus, role) {
  if (!can(role, 'moderate')) throw new Error('Недостаточно прав для модерации');
  if (!['pending', 'approved', 'rejected', 'changes_requested', 'hidden'].includes(nextStatus)) {
    throw new Error('Неизвестный статус');
  }
  if (video.status === 'hidden' && nextStatus !== 'approved') {
    throw new Error('Скрытое видео можно только вернуть в ленту');
  }
  return {
    ...video,
    status: nextStatus,
    watched: nextStatus === 'approved' ? Boolean(video.watched) : false,
    updatedAt: new Date().toISOString(),
  };
}

export function toggleVote(video, userId, direction) {
  if (!['up', 'down'].includes(direction)) throw new Error('Неизвестная реакция');
  const votes = { ...(video.votes || {}) };
  votes[userId] = votes[userId] === direction ? undefined : direction;
  if (!votes[userId]) delete votes[userId];
  return { ...video, votes };
}

export function voteTotals(video) {
  if (Number.isFinite(Number(video.rating))) {
    return { likes: 0, dislikes: 0, score: Number(video.rating) };
  }
  const values = Object.values(video.votes || {});
  const likes = (video.baseLikes || 0) + values.filter((value) => value === 'up').length;
  const dislikes = (video.baseDislikes || 0) + values.filter((value) => value === 'down').length;
  return { likes, dislikes, score: likes - dislikes };
}

export function removeCategory(state, category, fallback = 'Без категории') {
  if (category === fallback) throw new Error('Системную категорию удалить нельзя');
  if (!state.categories.includes(category)) return state;
  return {
    ...state,
    categories: state.categories.filter((item) => item !== category),
    videos: state.videos.map((video) =>
      video.category === category ? { ...video, category: fallback } : video,
    ),
  };
}
