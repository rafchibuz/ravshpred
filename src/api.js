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
    sourceUrl: video.source_url || video.youtube_url || '',
    contentKind: video.content_kind || 'video',
    sourceType: video.source_type || 'youtube',
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
    ravshtokStatus: video.ravshtok_status || '',
    ravshtokError: video.ravshtok_error || '',
    ravshtokAttempts: Number(video.ravshtok_attempts) || 0,
    kinopoiskUrl: video.kinopoisk_url || '',
    movieTitle: video.movie_title || '',
    movieYear: video.movie_year ?? '',
    movieStudio: video.movie_studio || '',
    movieRating: video.movie_rating ?? '',
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

function normalizeNotification(notice) {
  return {
    id: notice.id,
    title: notice.title,
    body: notice.body,
    createdAt: notice.created_at,
    read: Boolean(notice.read_at),
    tone: /отклон|исправ|дубликат/i.test(notice.title) ? 'red' : 'green',
  };
}

export async function loadWorkspace(role) {
  const [categoriesResponse, newsResponse, streamerResponse, publicSettingsResponse] = await Promise.all([
    request('/categories'),
    request('/news?limit=50'),
    request('/streamer'),
    request('/settings'),
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
    streamer: streamerResponse.data || null,
    settings: {
      dailyLimit: publicSettingsResponse.data?.submission_daily_limit || 3,
      ravshtokDailyLimit: publicSettingsResponse.data?.ravshtok_daily_limit || 10,
      commentLimit: publicSettingsResponse.data?.submission_comment_limit || 500,
      publicFeed: publicSettingsResponse.data?.public_feed_enabled !== false,
      allowSelfVote: false,
      socials: {}, socialLinks: [], supportLinks: [], partnerLinks: {}, siteLinks: [],
    },
  };
  if (['user', 'moderator', 'owner'].includes(role)) {
    const notifications = await request('/notifications?limit=100');
    result.notifications = (notifications.data || []).map(normalizeNotification);
  }
  if (role === 'owner') {
    const [moderators, settings] = await Promise.all([
      request('/owner/moderators'),
      request('/owner/settings'),
    ]);
    result.moderatorRecords = moderators.data || [];
    result.moderators = result.moderatorRecords.map((user) => user.login || user.display_name);
    const legacySocials = settings.data.socials || {};
    const legacySocialLinks = Object.entries(legacySocials)
      .filter(([, url]) => url)
      .map(([name, url]) => ({ name: name === 'vk' ? 'VK' : `${name.charAt(0).toUpperCase()}${name.slice(1)}`, url }));
    const socialLinks = (settings.data.social_links?.length ? settings.data.social_links : legacySocialLinks)
      .map((item) => ({ ...item, section: item.section || 'primary' }));
    const supportLinks = (settings.data.support_links || [])
      .map((item) => ({ ...item, section: 'support' }));
    result.settings = {
      dailyLimit: settings.data.submission_daily_limit,
      ravshtokDailyLimit: settings.data.ravshtok_daily_limit || 10,
      commentLimit: settings.data.submission_comment_limit,
      publicFeed: settings.data.public_feed_enabled,
      allowSelfVote: settings.data.allow_self_vote,
      socials: legacySocials,
      socialLinks,
      supportLinks,
      partnerLinks: settings.data.partner_links || {},
      siteLinks: [...socialLinks, ...supportLinks],
    };
  }
  return result;
}

export function createSubmission(input) {
  return request('/submissions', { method: 'POST', body: input })
    .then((payload) => normalizeVideo(payload.data));
}

export function loadMyUnbanAppeals() {
  return request('/unban-appeals/mine?limit=100').then((payload) => payload.data || []);
}

export function loadNotifications() {
  return request('/notifications?limit=100').then((payload) => (payload.data || []).map(normalizeNotification));
}

export function loadViewerRating(channel = 'all', period = '30d') {
  const query = new URLSearchParams({ channel, period });
  return request(`/rating?${query}`).then((payload) => payload.data);
}

function normalizeRavshTOKItem(item) {
  return {
    id: item.id,
    title: item.title || 'Без названия',
    description: item.description || '',
    sourceUrl: item.source_url || '',
    platform: item.platform || 'tiktok',
    mediaStatus: item.media_status || 'ready',
    playbackUrl: item.playback_url || '',
    posterUrl: item.poster_url || '',
    durationSeconds: Number(item.duration_seconds) || 0,
    width: Number(item.width) || 0,
    height: Number(item.height) || 0,
    likes: Number(item.likes) || 0,
    dislikes: Number(item.dislikes) || 0,
    userVote: Number(item.user_vote) || 0,
    userViewed: Boolean(item.user_viewed),
    streamerWatched: Boolean(item.streamer_watched),
    createdAt: item.created_at,
    author: normalizeUser(item.author),
  };
}

export function loadRavshTOK({ mode = 'new', platform = 'all', sort = 'new', offset = 0, limit = 12 } = {}) {
  const query = new URLSearchParams({ mode, platform, sort, offset: String(offset), limit: String(limit) });
  return request(`/ravshtok?${query}`).then((payload) => ({
    items: (payload.data?.items || []).map(normalizeRavshTOKItem),
    hasMore: Boolean(payload.data?.has_more),
  }));
}

export function markRavshTOKViewed(itemId) {
  return request(`/ravshtok/${encodeURIComponent(itemId)}/view`, { method: 'POST' });
}

export function voteRavshTOK(itemId, value) {
  return request(`/ravshtok/${encodeURIComponent(itemId)}/vote`, {
    method: 'PUT',
    body: { value },
  });
}

export function createUnbanAppeal(input) {
  return request('/unban-appeals', { method: 'POST', body: input }).then((payload) => payload.data);
}

export function withdrawUnbanAppeal(appealId) {
  return request(`/unban-appeals/${encodeURIComponent(appealId)}/withdraw`, { method: 'POST' }).then((payload) => payload.data);
}

export function loadUnbanAppeals(filters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => { if (value !== '' && value != null) params.set(key, value); });
  return request(`/moderation/unban-appeals?${params}`).then((payload) => ({ items: payload.data || [], total: Number(payload.meta?.total || 0) }));
}

export function reviewUnbanAppeal(appealId, input) {
  return request(`/moderation/unban-appeals/${encodeURIComponent(appealId)}`, { method: 'PATCH', body: input }).then((payload) => payload.data);
}

export async function loadPendingSubmissions() {
  return loadModerationSubmissions('pending');
}

export async function loadModerationSubmissions(status) {
  const payload = await request(`/moderation/submissions?status=${encodeURIComponent(status)}&limit=100`);
  return (payload.data || []).map(normalizeVideo);
}

export async function loadMySubmissions() {
  const payload = await request('/submissions/mine?limit=100');
  return (payload.data || []).map(normalizeVideo);
}

export function loadStreamer() {
  return request('/streamer').then((payload) => payload.data || null);
}

export function loadTwitchClips({ channel = 'all', period = 'week', from = '', to = '' } = {}) {
  const params = new URLSearchParams({ channel, period });
  if (period === 'custom') {
    params.set('from', from);
    params.set('to', to);
  }
  return request(`/twitch/clips?${params}`).then((payload) => payload.data || []);
}

export function loadTwitchVideos(channel = 'all') {
  return request(`/twitch/videos?${new URLSearchParams({ channel })}`).then((payload) => payload.data || []);
}

export function loadTwitchVideo(videoId) {
  return request(`/twitch/videos/${encodeURIComponent(videoId)}`).then((payload) => payload.data || null);
}

export function loadTwitchCacheStatus() {
  return request('/twitch/cache-status').then((payload) => payload.data || { items: [], refreshing: false });
}

export function refreshTwitchCache() {
  return request('/owner/twitch-cache/refresh', { method: 'POST' }).then((payload) => payload.data || null);
}

export function loadOwnerUsers(filters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => { if (value !== '' && value != null) params.set(key, value); });
  return request(`/owner/users?${params}`).then((payload) => ({ items: payload.data || [], total: Number(payload.meta?.total || 0) }));
}

export function loadOwnerUser(userId) {
  return request(`/owner/users/${encodeURIComponent(userId)}`).then((payload) => payload.data || null);
}

export function loadOwnerAudit(filters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => { if (value !== '' && value != null) params.set(key, value); });
  return request(`/owner/audit?${params}`).then((payload) => ({ items: payload.data || [], total: Number(payload.meta?.total || 0) }));
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

export function setMovieMetadata(videoId, movie, version) {
  return request(`/moderation/submissions/${videoId}/movie`, {
    method: 'PATCH',
    body: {
      kinopoisk_url: movie.url?.trim() || '',
      movie_title: movie.title?.trim() || '',
      movie_year: movie.year ? Number(movie.year) : null,
      movie_studio: movie.studio?.trim() || '',
      movie_rating: movie.rating !== '' && movie.rating != null ? Number(String(movie.rating).replace(',', '.')) : null,
      version,
    },
  }).then((payload) => normalizeVideo(payload.data));
}

export function setSubmissionContent(videoId, content, version) {
  return request(`/moderation/submissions/${videoId}/content`, {
    method: 'PATCH',
    body: {
      title: content.title?.trim() || '',
      source_url: content.sourceUrl?.trim() || '',
      comment: content.comment?.trim() || '',
      version,
    },
  }).then((payload) => normalizeVideo(payload.data));
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
  const siteLinks = settings.siteLinks || [];
  return request('/owner/settings', {
    method: 'PUT',
    body: {
      submission_daily_limit: settings.dailyLimit,
      ravshtok_daily_limit: settings.ravshtokDailyLimit,
      submission_comment_limit: settings.commentLimit,
      public_feed_enabled: settings.publicFeed,
      allow_self_vote: Boolean(settings.allowSelfVote),
      socials: settings.socials || {},
      social_links: siteLinks.filter((item) => item.section !== 'support'),
      support_links: siteLinks.filter((item) => item.section === 'support'),
      partner_links: settings.partnerLinks || {},
    },
  });
}

export function createNewsPost(title, body, notifyUsers = true) {
  return request('/owner/news', {
    method: 'POST',
    body: { title, body, notify_users: Boolean(notifyUsers) },
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
