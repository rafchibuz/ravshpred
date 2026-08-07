const API_ROOT = '/api';

function csrfToken() {
  return document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith('ravshann_session_csrf='))
    ?.split('=')
    .slice(1)
    .join('=') || '';
}

async function request(path, { body, ...options } = {}) {
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/json');
  if (body !== undefined) headers.set('Content-Type', 'application/json');
  const csrf = csrfToken();
  if (csrf && !['GET', 'HEAD'].includes(options.method || 'GET')) {
    headers.set('X-CSRF-Token', csrf);
  }

  const response = await fetch(`${API_ROOT}${path}`, {
    credentials: 'include',
    ...options,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return null;
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(payload.error?.message || `Ошибка API (${response.status})`);
    error.code = payload.error?.code;
    error.status = response.status;
    throw error;
  }
  return payload;
}

function durationLabel(seconds) {
  const value = Number(seconds) || 0;
  if (!value) return '—';
  const hours = Math.floor(value / 3600);
  const minutes = Math.floor((value % 3600) / 60);
  const rest = value % 60;
  return hours
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`
    : `${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`;
}

function compactNumber(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) return '—';
  return new Intl.NumberFormat('ru-RU', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(number);
}

export function normalizeVideo(video) {
  return {
    id: video.id,
    youtubeId: video.youtube_id,
    youtubeUrl: video.youtube_url,
    thumbnailUrl: video.thumbnail_url,
    title: video.title,
    channel: video.channel_title,
    authorId: video.author?.id,
    author: video.author?.display_name || video.author?.login || 'Пользователь Twitch',
    authorAvatar: video.author?.avatar_url,
    category: video.category?.name || 'Без категории',
    categoryId: video.category?.id,
    createdAt: video.created_at,
    updatedAt: video.updated_at,
    duration: durationLabel(video.duration_seconds),
    views: compactNumber(video.view_count),
    youtubeLikes: compactNumber(video.youtube_like_count),
    status: video.status,
    watched: Boolean(video.watched),
    rating: Number(video.rating) || 0,
    userVote: Number(video.user_vote) || 0,
    version: Number(video.version) || 1,
    submitterComment: video.submitter_comment || '',
    moderatorComment: video.moderator_comment || '',
    tone: 'purple',
    votes: {},
  };
}

function normalizeUser(user) {
  return {
    id: user?.id,
    name: user?.display_name || user?.login || 'Пользователь Twitch',
    avatarUrl: user?.avatar_url || '',
    role: user?.role || 'user',
  };
}

function normalizeNewsComment(comment) {
  return {
    id: comment.id,
    postId: comment.post_id,
    body: comment.body,
    createdAt: comment.created_at,
    author: normalizeUser(comment.author),
  };
}

function normalizeNewsPost(post) {
  return {
    id: post.id,
    title: post.title,
    body: post.body,
    createdAt: post.created_at,
    updatedAt: post.updated_at,
    author: normalizeUser(post.author),
    comments: (post.comments || []).map(normalizeNewsComment),
  };
}

export async function loadWorkspace(role) {
  const [categoriesResponse, newsResponse] = await Promise.all([
    request('/categories'),
    request('/news?limit=50'),
  ]);
  const categoryRecords = categoriesResponse.data || [];
  const requests = [request('/videos?limit=100')];

  if (role === 'moderator' || role === 'owner') {
    requests.push(
      request('/moderation/submissions?status=pending&limit=100'),
      request('/moderation/submissions?status=approved&limit=100'),
      request('/moderation/submissions?status=rejected&limit=100'),
    );
  } else if (role === 'user') {
    requests.push(request('/submissions/mine?limit=100'));
  }

  const responses = await Promise.all(requests);
  const videosById = new Map();
  for (const response of responses) {
    for (const video of response.data || []) videosById.set(video.id, normalizeVideo(video));
  }
  const result = {
    categoryRecords,
    categories: categoryRecords.map((item) => item.name),
    videos: [...videosById.values()],
    news: (newsResponse.data || []).map(normalizeNewsPost),
  };
  if (['user', 'moderator', 'owner'].includes(role)) {
    const notifications = await request('/notifications?limit=100');
    result.notifications = (notifications.data || []).map((notice) => ({
      id: notice.id,
      title: notice.title,
      body: notice.body,
      createdAt: notice.created_at,
      read: Boolean(notice.read_at),
      tone: notice.type === 'moderation' && /отклон|исправ/i.test(notice.title) ? 'red' : 'green',
    }));
  }
  if (role === 'owner') {
    const [moderators, audit, settings] = await Promise.all([
      request('/owner/moderators'),
      request('/owner/audit?limit=100'),
      request('/owner/settings'),
    ]);
    result.moderatorRecords = moderators.data || [];
    result.moderators = result.moderatorRecords.map((user) => user.login || user.display_name);
    result.auditRecords = audit.data || [];
    result.audit = result.auditRecords.map((entry) => {
      const actor = entry.actor?.display_name || entry.actor?.login || 'Система';
      return `${actor}: ${entry.action} · ${entry.target_type} #${entry.target_id}`;
    });
    result.settings = {
      dailyLimit: settings.data.submission_daily_limit,
      commentLimit: settings.data.submission_comment_limit,
      publicFeed: settings.data.public_feed_enabled,
      allowSelfVote: settings.data.allow_self_vote,
    };
  }
  return result;
}

export function createSubmission(input) {
  return request('/submissions', { method: 'POST', body: input })
    .then((payload) => normalizeVideo(payload.data));
}

export function vote(videoId, value) {
  return request(`/videos/${videoId}/vote`, { method: 'PUT', body: { value } });
}

export function decide(videoId, status, comment, version) {
  return request(`/moderation/submissions/${videoId}`, {
    method: 'PATCH',
    body: { status, comment, reason_code: '', version },
  }).then((payload) => normalizeVideo(payload.data));
}

export function setWatched(videoId, watched) {
  return request(`/moderation/submissions/${videoId}/watched`, {
    method: 'PATCH',
    body: { watched },
  });
}

export function deleteVideo(videoId) {
  return request(`/moderation/submissions/${videoId}`, { method: 'DELETE' });
}

export function setVideoCategory(videoId, categoryId) {
  return request(`/moderation/submissions/${videoId}/category`, {
    method: 'PATCH',
    body: { category_id: categoryId },
  });
}

export function createCategory(name) {
  return request('/owner/categories', {
    method: 'POST',
    body: { name },
  }).then((payload) => payload.data);
}

export function deleteCategory(categoryId) {
  return request(`/owner/categories/${categoryId}`, { method: 'DELETE' });
}

export function assignModerator(twitchLogin) {
  return request('/owner/moderators', {
    method: 'POST',
    body: { twitch_login: twitchLogin },
  }).then((payload) => payload.data);
}

export function removeModerator(userId) {
  return request(`/owner/moderators/${userId}`, { method: 'DELETE' });
}

export function readNotification(notificationId) {
  return request(`/notifications/${notificationId}/read`, { method: 'POST' });
}

export function readAllNotifications() {
  return request('/notifications/read-all', { method: 'POST' });
}

export function updateSettings(settings) {
  return request('/owner/settings', {
    method: 'PUT',
    body: {
      submission_daily_limit: settings.dailyLimit,
      submission_comment_limit: settings.commentLimit,
      public_feed_enabled: settings.publicFeed,
      allow_self_vote: Boolean(settings.allowSelfVote),
    },
  });
}

export function createNewsPost(title, body) {
  return request('/owner/news', {
    method: 'POST',
    body: { title, body },
  }).then((payload) => normalizeNewsPost(payload.data));
}

export function deleteNewsPost(postId) {
  return request(`/owner/news/${postId}`, { method: 'DELETE' });
}

export function createNewsComment(postId, body) {
  return request(`/news/${postId}/comments`, {
    method: 'POST',
    body: { body },
  }).then((payload) => normalizeNewsComment(payload.data));
}

export function deleteNewsComment(commentId) {
  return request(`/news/comments/${commentId}`, { method: 'DELETE' });
}

export async function currentUser() {
  const payload = await request('/me');
  return payload.user;
}

export function logout() {
  return request('/logout', { method: 'POST' });
}
