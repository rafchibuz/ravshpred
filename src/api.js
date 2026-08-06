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

async function request(path, { role, body, ...options } = {}) {
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/json');
  if (body !== undefined) headers.set('Content-Type', 'application/json');
  if (role && role !== 'guest') headers.set('X-Dev-Role', role);
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
  if (!Number.isFinite(number)) return '—';
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

export async function loadWorkspace(role, authRole = role) {
  const categoriesResponse = await request('/categories', { role: authRole });
  const categoryRecords = categoriesResponse.data || [];
  const requests = [request('/videos?limit=100', { role: authRole })];

  if (role === 'moderator' || role === 'owner') {
    requests.push(
      request('/moderation/submissions?status=pending&limit=100', { role: authRole }),
      request('/moderation/submissions?status=approved&limit=100', { role: authRole }),
      request('/moderation/submissions?status=rejected&limit=100', { role: authRole }),
    );
  } else if (role === 'user') {
    requests.push(request('/submissions/mine?limit=100', { role: authRole }));
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
  };
  if (['user', 'moderator', 'owner'].includes(role)) {
    const notifications = await request('/notifications?limit=100', { role: authRole });
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
      request('/owner/moderators', { role: authRole }),
      request('/owner/audit?limit=100', { role: authRole }),
      request('/owner/settings', { role: authRole }),
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

export function createSubmission(role, input) {
  return request('/submissions', { method: 'POST', role, body: input })
    .then((payload) => normalizeVideo(payload.data));
}

export function vote(role, videoId, value) {
  return request(`/videos/${videoId}/vote`, { method: 'PUT', role, body: { value } });
}

export function decide(role, videoId, status, comment, version) {
  return request(`/moderation/submissions/${videoId}`, {
    method: 'PATCH',
    role,
    body: { status, comment, reason_code: '', version },
  }).then((payload) => normalizeVideo(payload.data));
}

export function setWatched(role, videoId, watched) {
  return request(`/moderation/submissions/${videoId}/watched`, {
    method: 'PATCH',
    role,
    body: { watched },
  });
}

export function deleteVideo(role, videoId) {
  return request(`/moderation/submissions/${videoId}`, { method: 'DELETE', role });
}

export function setVideoCategory(role, videoId, categoryId) {
  return request(`/moderation/submissions/${videoId}/category`, {
    method: 'PATCH',
    role,
    body: { category_id: categoryId },
  });
}

export function createCategory(role, name) {
  return request('/owner/categories', {
    method: 'POST',
    role,
    body: { name },
  }).then((payload) => payload.data);
}

export function deleteCategory(role, categoryId) {
  return request(`/owner/categories/${categoryId}`, { method: 'DELETE', role });
}

export function assignModerator(role, twitchLogin) {
  return request('/owner/moderators', {
    method: 'POST',
    role,
    body: { twitch_login: twitchLogin },
  }).then((payload) => payload.data);
}

export function removeModerator(role, userId) {
  return request(`/owner/moderators/${userId}`, { method: 'DELETE', role });
}

export function readNotification(role, notificationId) {
  return request(`/notifications/${notificationId}/read`, { method: 'POST', role });
}

export function readAllNotifications(role) {
  return request('/notifications/read-all', { method: 'POST', role });
}

export function updateSettings(role, settings) {
  return request('/owner/settings', {
    method: 'PUT',
    role,
    body: {
      submission_daily_limit: settings.dailyLimit,
      submission_comment_limit: settings.commentLimit,
      public_feed_enabled: settings.publicFeed,
      allow_self_vote: Boolean(settings.allowSelfVote),
    },
  });
}

export async function currentUser() {
  const payload = await request('/me');
  return payload.user;
}

export function logout() {
  return request('/logout', { method: 'POST' });
}

export function founderLogin(secret) {
  return request('/founder/session', {
    method: 'POST',
    body: { secret },
  }).then((payload) => payload.user);
}
