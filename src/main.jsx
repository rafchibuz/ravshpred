import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  ArrowUpRight,
  Bell,
  Check,
  Clock3,
  ExternalLink,
  Eye,
  Filter,
  FileText,
  Home,
  LayoutGrid,
  Link2,
  List,
  Menu,
  MessageCircle,
  Newspaper,
  Play,
  Plus,
  Search,
  Send,
  Settings,
  ShieldCheck,
  Sparkles,
  ThumbsDown,
  ThumbsUp,
  Trophy,
  Trash2,
  UserRound,
  X,
} from 'lucide-react';
import {
  can,
  isDuplicate,
  newPendingSubmissions,
  parseYouTubeId,
  recentSubmissionCount,
  removeCategory,
  voteTotals,
} from './domain.js';
import * as api from './api.js';
import './styles.css';

const STORAGE_KEY = 'ravshann-predlozhka-local-v3';
const MODERATION_POLL_INTERVAL = 15000;
const ROLE_LABELS = {
  guest: 'Гость',
  user: 'Пользователь',
  moderator: 'Модератор',
  owner: 'Основатель',
};

function playNewSubmissionSound(audioContextRef) {
  const context = audioContextRef.current;
  if (!context || context.state === 'closed') return;

  const play = () => {
    const start = context.currentTime;
    [0, 0.16].forEach((offset, index) => {
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      oscillator.type = 'sine';
      oscillator.frequency.setValueAtTime(index === 0 ? 740 : 988, start + offset);
      gain.gain.setValueAtTime(0.0001, start + offset);
      gain.gain.exponentialRampToValueAtTime(0.13, start + offset + 0.025);
      gain.gain.exponentialRampToValueAtTime(0.0001, start + offset + 0.14);
      oscillator.connect(gain);
      gain.connect(context.destination);
      oscillator.start(start + offset);
      oscillator.stop(start + offset + 0.15);
    });
  };

  if (context.state === 'suspended') context.resume().then(play).catch(() => {});
  else play();
}

const STATUS_LABELS = {
  pending: 'На рассмотрении',
  approved: 'Одобрено',
  rejected: 'Отклонено',
  changes_requested: 'Нужно исправить',
  hidden: 'Скрыто',
};
const tones = {
  pink: 'linear-gradient(135deg,#743584 0%,#d64e7f 100%)',
  blue: 'linear-gradient(135deg,#304981 0%,#637cc7 100%)',
  red: 'linear-gradient(135deg,#7d314f 0%,#dc6575 100%)',
  amber: 'linear-gradient(135deg,#89614e 0%,#d9a06c 100%)',
  purple: 'linear-gradient(135deg,#512883 0%,#9c58c8 100%)',
};

const agoIso = (hours) => new Date(Date.now() - hours * 60 * 60 * 1000).toISOString();

function makeInitialState() {
  return {
    categories: ['Без категории', 'Смешное', 'Трейлеры', 'Фильмы и сериалы', 'Разоблачения'],
    moderators: ['moderator_live', 'lexapro_tv', 'shadowmff'],
    settings: { dailyLimit: 3, commentLimit: 500, publicFeed: true, socials: {}, socialLinks: [], supportLinks: [] },
    streamer: null,
    userStats: [],
    videos: [
      {
        id: 12451,
        youtubeId: 'dQw4w9WgXcQ',
        youtubeUrl: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
        title: 'Rick Astley — Never Gonna Give You Up',
        channel: 'Rick Astley',
        authorId: 'viewer',
        author: 'Ravshibiscus',
        category: 'Трейлеры',
        createdAt: agoIso(2),
        duration: '10:36',
        views: '12,3K',
        youtubeLikes: '842K',
        baseLikes: 342,
        baseDislikes: 18,
        votes: {},
        status: 'approved',
        watched: true,
        tone: 'pink',
        moderatorComment: 'Отличный момент для начала стрима.',
      },
      {
        id: 12450,
        youtubeId: 'M7lc1UVf-VE',
        youtubeUrl: 'https://www.youtube.com/watch?v=M7lc1UVf-VE',
        title: 'YouTube IFrame Player API',
        channel: 'YouTube Developers',
        authorId: 'game-over',
        author: 'game_over_90',
        category: 'Смешное',
        createdAt: agoIso(4),
        duration: '02:35',
        views: '8,7K',
        youtubeLikes: '214K',
        baseLikes: 198,
        baseDislikes: 12,
        votes: {},
        status: 'approved',
        watched: false,
        tone: 'blue',
      },
      {
        id: 12449,
        youtubeId: 'aqz-KE-bpKQ',
        youtubeUrl: 'https://www.youtube.com/watch?v=aqz-KE-bpKQ',
        title: 'Big Buck Bunny 60fps 4K',
        channel: 'Blender',
        authorId: 'truth',
        author: 'truth_seeker',
        category: 'Разоблачения',
        createdAt: agoIso(7),
        duration: '28:42',
        views: '6,1K',
        youtubeLikes: '91K',
        baseLikes: 156,
        baseDislikes: 25,
        votes: {},
        status: 'approved',
        watched: false,
        tone: 'red',
      },
      {
        id: 12448,
        youtubeId: 'ysz5S6PUM-U',
        youtubeUrl: 'https://www.youtube.com/watch?v=ysz5S6PUM-U',
        title: 'Chilled Serenity',
        channel: 'Nature Relaxation',
        authorId: 'kino',
        author: 'kino_maniak',
        category: 'Трейлеры',
        createdAt: agoIso(12),
        duration: '05:47',
        views: '3,2K',
        youtubeLikes: '48K',
        baseLikes: 89,
        baseDislikes: 4,
        votes: {},
        status: 'approved',
        watched: false,
        tone: 'blue',
      },
      {
        id: 12447,
        youtubeId: 'jNQXAC9IVRw',
        youtubeUrl: 'https://www.youtube.com/watch?v=jNQXAC9IVRw',
        title: 'Me at the zoo',
        channel: 'jawed',
        authorId: 'warrior',
        author: 'warrior_tv',
        category: 'Смешное',
        createdAt: agoIso(20),
        duration: '10:21',
        views: '5,2K',
        youtubeLikes: '126K',
        baseLikes: 170,
        baseDislikes: 9,
        votes: {},
        status: 'approved',
        watched: true,
        tone: 'amber',
      },
      {
        id: 12446,
        youtubeId: 'ScMzIvxBSi4',
        youtubeUrl: 'https://www.youtube.com/watch?v=ScMzIvxBSi4',
        title: 'Демо-видео для проверки превью',
        channel: 'YouTube',
        authorId: 'kotjara',
        author: 'kotjara_',
        category: 'Смешное',
        createdAt: agoIso(28),
        duration: '01:05',
        views: '4,5K',
        youtubeLikes: '73K',
        baseLikes: 128,
        baseDislikes: 7,
        votes: {},
        status: 'approved',
        watched: false,
        tone: 'purple',
      },
      {
        id: 12452,
        youtubeId: 'YE7VzlLtp-4',
        youtubeUrl: 'https://www.youtube.com/watch?v=YE7VzlLtp-4',
        title: 'Новая теория по сериалу',
        channel: 'Series Lab',
        authorId: 'viewer',
        author: 'Ravshibiscus',
        category: 'Фильмы и сериалы',
        createdAt: agoIso(1),
        duration: '14:12',
        views: '—',
        youtubeLikes: '—',
        baseLikes: 0,
        baseDislikes: 0,
        votes: {},
        status: 'pending',
        watched: false,
        tone: 'purple',
        submitterComment: 'Есть интересная деталь для обсуждения с чатом.',
      },
      {
        id: 12445,
        youtubeId: 'LXb3EKWsInQ',
        youtubeUrl: 'https://www.youtube.com/watch?v=LXb3EKWsInQ',
        title: 'Реклама под видом обзора',
        channel: 'Promo World',
        authorId: 'viewer',
        author: 'Ravshibiscus',
        category: 'Разоблачения',
        createdAt: agoIso(72),
        duration: '08:40',
        views: '—',
        youtubeLikes: '—',
        baseLikes: 0,
        baseDislikes: 0,
        votes: {},
        status: 'rejected',
        watched: false,
        tone: 'red',
        moderatorComment: 'Реклама и внешние ссылки не допускаются.',
      },
      {
        id: 12453,
        youtubeId: '9bZkp7q19f0',
        youtubeUrl: 'https://www.youtube.com/watch?v=9bZkp7q19f0',
        title: 'Клип для начала стрима',
        channel: 'Music Room',
        authorId: 'music-fan',
        author: 'music_fan',
        category: 'Смешное',
        createdAt: agoIso(0.5),
        duration: '04:13',
        views: '—',
        youtubeLikes: '—',
        baseLikes: 0,
        baseDislikes: 0,
        votes: {},
        status: 'pending',
        watched: false,
        tone: 'pink',
        submitterComment: 'Проверенный клип, хорошо зайдёт перед новостями.',
      },
    ],
    notifications: [
      {
        id: 1,
        title: 'Ваше видео одобрено',
        body: '«Rick Astley — Never Gonna Give You Up» появилось в общей ленте.',
        createdAt: agoIso(0.2),
        read: false,
        tone: 'green',
      },
      {
        id: 2,
        title: 'Видео отклонено',
        body: 'Причина: реклама и внешние ссылки не допускаются.',
        createdAt: agoIso(24),
        read: false,
        tone: 'red',
      },
      {
        id: 3,
        title: '100 лайков',
        body: 'Ваше видео набрало первые 100 лайков.',
        createdAt: agoIso(48),
        read: false,
        tone: 'purple',
      },
    ],
    audit: [
      'moderator_live одобрил видео #12451',
      'lexapro_tv отклонил видео #12445',
      'Ravshibiscus добавил shadowmff',
      'Категория «Разоблачения» изменена',
    ],
  };
}

function emptyServerState() {
  return {
    categories: [],
    categoryRecords: [],
    moderators: [],
    moderatorRecords: [],
    videos: [],
    notifications: [],
    news: [],
    audit: [],
    auditRecords: [],
    settings: { dailyLimit: 3, commentLimit: 500, publicFeed: true, allowSelfVote: false, socials: {}, socialLinks: [], supportLinks: [] },
    streamer: null,
    userStats: [],
  };
}

function useProjectState(role) {
  const [state, setState] = useState(emptyServerState);
  const [apiReady, setApiReady] = useState(false);

  useEffect(() => {
    let active = true;
    setApiReady(false);
    api.loadWorkspace(role)
      .then((workspace) => {
        if (!active) return;
        setState((current) => ({ ...current, ...workspace }));
        setApiReady(true);
      })
      .catch(() => {
        if (active) setApiReady(false);
      });
    return () => {
      active = false;
    };
  }, [role]);

  const reload = async () => {
    const workspace = await api.loadWorkspace(role);
    setState((current) => ({ ...current, ...workspace }));
    setApiReady(true);
  };

  return [state, setState, apiReady, reload];
}

function useRoute(defaultRoute) {
  const read = () => window.location.hash.replace(/^#\/?/, '') || defaultRoute;
  const [route, setRoute] = useState(read);
  useEffect(() => {
    const onHash = () => setRoute(read());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);
  const navigate = (next) => {
    if (read() === next) setRoute(next);
    else window.location.hash = `/${next}`;
  };
  return [route, navigate];
}

function Avatar({ small = false, src = '', name = '' }) {
  const className = `avatar ${small ? 'avatar-small' : ''}`;
  if (src) {
    return <img className={className} src={src} alt={`Аватар ${name || 'пользователя Twitch'}`} referrerPolicy="no-referrer" />;
  }
  return (
    <span className={`${className} avatar-fallback`} aria-hidden="true">
      {(name.trim()[0] || '?').toUpperCase()}
    </span>
  );
}

function Logo() {
  return (
    <div className="logo" aria-label="RAVSHANN Предложка">
      <span>RAVSHANN</span>
      <small>ПРЕДЛОЖКА</small>
    </div>
  );
}

function Sidebar({ route, navigate, role, unread, actor }) {
  const nav = [
    { key: 'home', label: 'Главная', icon: Home },
    { key: 'feed', label: 'Предложка', icon: Play },
    { key: 'news', label: 'Новости', icon: Newspaper },
    ...(can(role, 'submit') ? [{ key: 'submit', label: 'Предложить', icon: Plus }] : []),
    ...(can(role, 'view_profile')
      ? [
          { key: 'notifications', label: 'Уведомления', icon: Bell, count: unread },
          { key: 'profile', label: 'Профиль', icon: UserRound },
        ]
      : []),
    ...(can(role, 'moderate') ? [{ key: 'moderation', label: 'Модерация', icon: ShieldCheck }] : []),
    ...(can(role, 'manage') ? [{ key: 'owner', label: 'Управление', icon: Settings }] : []),
  ];

  return (
    <aside className="sidebar">
      <div className="sidebar-top">
        <Logo />
      </div>
      <div className="role-label">{ROLE_LABELS[role].toUpperCase()}</div>
      <nav className="side-nav" aria-label="Основная навигация">
        {nav.map(({ key, label, icon: Icon, count }) => (
          <button
            key={key}
            className={route === key ? 'active' : ''}
            onClick={() => navigate(key)}
            aria-current={route === key ? 'page' : undefined}
          >
            <Icon size={16} />
            <span>{label}</span>
            {count > 0 && <b>{count}</b>}
          </button>
        ))}
      </nav>
      <div className="side-bottom">
        <div className="account-card" aria-label={`Текущая роль: ${ROLE_LABELS[role]}`}>
          <Avatar small src={actor?.avatarUrl} name={actor?.name} />
          <span>
            <strong>{role === 'guest' ? 'Без авторизации' : actor?.name || ROLE_LABELS[role]}</strong>
            <small>{ROLE_LABELS[role]}</small>
          </span>
        </div>
      </div>
    </aside>
  );
}

function SiteFooter({ navigate }) {
  return (
    <footer className="site-footer">
      <span>© {new Date().getFullYear()} RAVSHANN Предложка</span>
      <nav aria-label="Служебные страницы">
        <button onClick={() => navigate('rules')}>Правила</button>
        <button onClick={() => navigate('privacy')}>Конфиденциальность</button>
        <button onClick={() => navigate('terms')}>Условия использования</button>
      </nav>
    </footer>
  );
}

function Topbar({ role, search, setSearch, navigate, openAuth, onSignOut, unread, actor }) {
  return (
    <header className="topbar">
      <div className="search-wrap">
        <Search size={16} />
        <input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Поиск по видео, автору или категории..."
          aria-label="Поиск"
        />
      </div>
      <div className="top-actions">
        {can(role, 'submit') && (
          <button className="outline-btn hide-mobile" onClick={() => navigate('submit')}>
            <Plus size={15} /> Предложить видео
          </button>
        )}
        {can(role, 'view_profile') && (
          <button className="icon-btn" aria-label={`Уведомления: ${unread} непрочитанных`} onClick={() => navigate('notifications')}>
            <Bell size={17} />
            {unread > 0 && <i />}
          </button>
        )}
        {role === 'guest' ? (
          <button className="twitch-login" onClick={openAuth}>
            <Sparkles size={15} />
            <span className="desktop-login-label">Войти через Twitch</span>
            <span className="mobile-login-label">Twitch</span>
          </button>
        ) : (
          <>
            <button className="user-pill role-pill" onClick={() => navigate('profile')}>
              <Avatar small src={actor?.avatarUrl} name={actor?.name} />
              <span>{actor?.name || ROLE_LABELS[role]}</span>
            </button>
            <button className="ghost-btn signout-button" onClick={onSignOut}>Выйти</button>
          </>
        )}
      </div>
    </header>
  );
}

function AuthModal({ onContinue, onClose }) {
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section className="role-modal auth-modal" role="dialog" aria-modal="true" aria-labelledby="auth-title" onMouseDown={(event) => event.stopPropagation()}>
        <button className="modal-close icon-btn" onClick={onClose} aria-label="Закрыть"><X size={17} /></button>
        <span className="panel-kicker">БЕЗОПАСНЫЙ ВХОД</span>
        <h2 id="auth-title">Войти через Twitch</h2>
        <p>Вход нужен только тем, кто отправляет видео или голосует. Смотреть ленту можно без аккаунта.</p>
        <div className="auth-points">
          <p><ShieldCheck size={15} /> Сайт получает Twitch ID, ник и аватар — пароль Twitch не передаётся сайту.</p>
          <p><UserRound size={15} /> Если основатель назначил вас модератором, защищённый раздел появится автоматически.</p>
        </div>
        <button className="primary-btn full" onClick={onContinue}>
          <Sparkles size={15} /> Продолжить через Twitch
        </button>
        <small>Вы перейдёте на официальный экран Twitch, а сайт не увидит ваш пароль.</small>
      </section>
    </div>
  );
}

function Thumb({ video, large = false }) {
  const status = video.status;
  const fallbackThumbnail = video.thumbnailUrl || `https://i.ytimg.com/vi/${video.youtubeId}/hqdefault.jpg`;
  return (
    <div className={`thumb ${large ? 'thumb-large' : ''}`} style={{ background: tones[video.tone] || tones.purple }}>
      <img
        className="youtube-thumb"
        src={`https://i.ytimg.com/vi/${video.youtubeId}/maxresdefault.jpg`}
        data-fallback={fallbackThumbnail}
        alt={`Превью «${video.title}»`}
        loading={large ? 'eager' : 'lazy'}
        onError={(event) => {
          if (event.currentTarget.dataset.fallbackTried !== '1') {
            event.currentTarget.dataset.fallbackTried = '1';
            event.currentTarget.src = event.currentTarget.dataset.fallback;
          } else {
            event.currentTarget.hidden = true;
          }
        }}
      />
      <div className="thumb-top">
        <span className={`mini-tag status-${status}`}>{STATUS_LABELS[status]?.toUpperCase() || 'ВИДЕО'}</span>
        {video.watched && (
          <span className="watched-chip">
            <Eye size={11} /> ОТСМОТРЕНО
          </span>
        )}
      </div>
      <span className="duration">{video.duration}</span>
      {large && (
        <span className="play">
          <Play size={23} fill="white" />
        </span>
      )}
    </div>
  );
}

function VideoCard({ video, role, onVote, onOpen, onManage, list }) {
  const totals = voteTotals(video);
  const vote = video.userVote === 1 ? 'up' : video.userVote === -1 ? 'down' : null;
  return (
    <article className={`video-card ${list ? 'video-card-list' : ''}`}>
      <button className="card-link" onClick={() => onOpen(video)} aria-label={`Открыть «${video.title}» на YouTube`}>
        <Thumb video={video} />
      </button>
      <div className="card-body">
        <h3>{video.title}</h3>
        <div className="author-row">
          <Avatar small src={video.authorAvatar} name={video.author} />
          <span>{video.author}</span>
          <em>·</em>
          <span>{new Date(video.createdAt).toLocaleDateString('ru-RU')}</span>
        </div>
        <div className="channel">YouTube: {video.channel}</div>
        {(video.movieTitle || video.movieYear || video.movieStudio || video.movieRating !== '' || video.kinopoiskUrl) && (
          <div className="movie-card-info">
            {video.movieTitle && <strong>{video.movieTitle}</strong>}
            <div>
              {video.movieYear && <span>{video.movieYear}</span>}
              {video.movieStudio && <span>{video.movieStudio}</span>}
              {video.movieRating !== '' && <span>Рейтинг: {video.movieRating}</span>}
            </div>
            {video.kinopoiskUrl && <a href={video.kinopoiskUrl} target="_blank" rel="noreferrer">Открыть на Кинопоиске <ExternalLink size={11} /></a>}
          </div>
        )}
        {video.submitterComment && <p className="viewer-wish"><MessageCircle size={12} /> {video.submitterComment}</p>}
        <div className="metrics">
          <div className="youtube-metrics" aria-label="Метрики YouTube">
            <span title="Просмотры на YouTube"><Eye size={12} /> {video.views}</span>
            <span title="Лайки на YouTube"><ThumbsUp size={12} /> {video.youtubeLikes ?? '—'}</span>
          </div>
          <div className="rating-control" aria-label={`Рейтинг сайта: ${totals.score}`}>
            <strong className={totals.score < 0 ? 'negative' : ''}>{totals.score > 0 ? `+${totals.score}` : totals.score}</strong>
            <button className={vote === 'up' ? 'voted' : ''} onClick={() => onVote(video.id, 'up')} aria-label="Поставить лайк">
              <ThumbsUp size={13} />
            </button>
            <button className={vote === 'down' ? 'voted down' : ''} onClick={() => onVote(video.id, 'down')} aria-label="Поставить дизлайк">
              <ThumbsDown size={13} />
            </button>
          </div>
        </div>
        {(!can(role, 'vote') || can(role, 'moderate')) && (
          <div className="card-footer">
            {!can(role, 'vote') && <div className="login-hint">Войдите через Twitch, чтобы голосовать</div>}
            {can(role, 'moderate') && (
              <button className="manage-video-btn" onClick={() => onManage(video)}>
                <ShieldCheck size={14} /> Отсмотрено / удалить
              </button>
            )}
          </div>
        )}
      </div>
    </article>
  );
}

function formatStreamDuration(startedAt, now) {
  if (!startedAt) return '—';
  const seconds = Math.max(0, Math.floor((now - new Date(startedAt).getTime()) / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  return `${hours}:${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`;
}

function LinkDirectory({ title, links }) {
  const items = links || [];
  return (
    <section className="stream-socials social-directory-section">
      <header><h2>{title}</h2><span>{String(items.length).padStart(2, '0')}</span></header>
      <div className="social-directory">
        {items.map((item, index) => (
          <a key={`${item.name}-${item.url}`} href={item.url} target="_blank" rel="noreferrer">
            <span>{String(index + 1).padStart(2, '0')}</span>
            <div><strong>{item.name}</strong><small>{item.url.replace(/^https?:\/\//, '').replace(/\/$/, '')}</small></div>
            <ArrowUpRight size={16} />
          </a>
        ))}
        {!items.length && <div className="social-directory-empty">Ссылки появятся здесь после добавления в настройках</div>}
      </div>
    </section>
  );
}

function StreamerHome({ streamer }) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  if (!streamer) return null;
  const parent = window.location.hostname || 'localhost';
  const socials = streamer.socials || {};
  const links = (streamer.social_links?.length ? streamer.social_links : [
    { name: 'Twitch', url: socials.twitch || 'https://www.twitch.tv/ravshann' },
    { name: 'YouTube', url: socials.youtube },
    { name: 'Telegram', url: socials.telegram },
    { name: 'VK', url: socials.vk },
  ]).filter((item) => item.url);
  const supportLinks = (streamer.support_links || []).filter((item) => item.url);
  return (
    <main className="main-content stream-home">
      <section className={`streamer-hero streamer-cinema ${streamer.live ? 'is-live' : ''}`}>
      {streamer.live && (
        <div className="twitch-live-shell">
          <div className="twitch-player-column">
            <iframe title="Стрим RavshanN" src={`https://player.twitch.tv/?channel=ravshann&parent=${encodeURIComponent(parent)}&autoplay=false`} allowFullScreen />
            <div className="stream-info-bar">
              <div className="stream-channel-info">
                <Avatar src={streamer.avatar_url} name={streamer.display_name || 'RavshanN'} />
                <div><h1>{streamer.display_name || 'RavshanN'}</h1><strong>{streamer.title}</strong><span>{streamer.game_name || 'Twitch'}</span></div>
              </div>
              <div className="stream-live-metrics">
                <span className="stream-state"><i /> В ЭФИРЕ</span>
                <b>{Number(streamer.viewer_count || 0).toLocaleString('ru-RU')} онлайн</b>
                <time>{formatStreamDuration(streamer.started_at, now)}</time>
              </div>
            </div>
          </div>
          <iframe className="twitch-chat-frame" title="Чат RavshanN" src={`https://www.twitch.tv/embed/ravshann/chat?parent=${encodeURIComponent(parent)}&darkpopout`} />
        </div>
      )}
      {!streamer.live && (
        <div className="stream-offline">
          <Avatar src={streamer.avatar_url} name={streamer.display_name || 'RavshanN'} />
          <span className="stream-state"><i /> СЕЙЧАС НЕ В ЭФИРЕ</span>
          <h1>{streamer.display_name || 'RavshanN'}</h1>
          <p>Когда эфир начнётся, здесь автоматически появятся плеер и чат.</p>
        </div>
      )}
      {!streamer.live && <div className="stream-info-bar">
        <div className="stream-channel-info">
          <Avatar src={streamer.avatar_url} name={streamer.display_name || 'RavshanN'} />
          <div><h1>{streamer.display_name || 'RavshanN'}</h1><strong>{streamer.live ? streamer.title : 'Канал сейчас офлайн'}</strong><span>{streamer.game_name || 'Twitch'}</span></div>
        </div>
        <div className="stream-live-metrics">
          <span className="stream-state"><i /> {streamer.live ? 'В ЭФИРЕ' : 'ОФЛАЙН'}</span>
          {streamer.live && <><b>{Number(streamer.viewer_count || 0).toLocaleString('ru-RU')} онлайн</b><time>{formatStreamDuration(streamer.started_at, now)}</time></>}
        </div>
      </div>}
      </section>
      <LinkDirectory title="Соцсети" links={links} />
      <LinkDirectory title="Поддержка" links={supportLinks} />
    </main>
  );
}

function Feed({ videos, categories, role, search, onVote, onOpen, navigate, onLogin }) {
  const [category, setCategory] = useState('Все');
  const [watchedOnly, setWatchedOnly] = useState(false);
  const [visibleStatuses, setVisibleStatuses] = useState(['approved']);
  const [sort, setSort] = useState('Новые');
  const [list, setList] = useState(false);
  const approved = videos.filter((video) => video.status === 'approved');
  const toggleStatus = (status) => {
    setVisibleStatuses((current) => current.includes(status)
      ? current.filter((item) => item !== status)
      : [...current, status]);
  };
  const manageVideo = (video) => {
    window.sessionStorage.setItem('ravshann-moderation-target', video.id);
    navigate('moderation');
  };
  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return videos
      .filter(
        (video) =>
          visibleStatuses.includes(video.status) &&
          (category === 'Все' || video.category === category) &&
          (!watchedOnly || video.watched) &&
          [video.title, video.author, video.category, video.channel].join(' ').toLowerCase().includes(query),
      )
      .sort((a, b) => {
        if (sort === 'Новые') return new Date(b.createdAt) - new Date(a.createdAt);
        if (sort === 'Рейтингу') return voteTotals(b).score - voteTotals(a).score;
        return voteTotals(b).score - voteTotals(a).score;
      });
  }, [videos, visibleStatuses, category, watchedOnly, sort, search]);

  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow">
            <span className="live-dot" /> ПУБЛИЧНАЯ ЛЕНТА
          </div>
          <h1>Предложка Равшана</h1>
          <p>Предложенные видео сообщества — по умолчанию показаны только одобренные</p>
        </div>
        <div className="heading-actions">
          <button className="ghost-btn" onClick={() => setList((value) => !value)} aria-pressed={list}>
            {list ? <LayoutGrid size={15} /> : <List size={15} />} {list ? 'Плитка' : 'Список'}
          </button>
          <select value={sort} onChange={(event) => setSort(event.target.value)} aria-label="Сортировка">
            <option>Новые</option>
            <option>Популярности</option>
            <option>Рейтингу</option>
          </select>
        </div>
      </section>
      <div className="chips">
        <div className="chip-bar">
          <div className="chip-scroll">
            {['Все', ...categories].map((item) => (
              <button key={item} className={category === item ? 'selected' : ''} onClick={() => setCategory(item)}>
                {item}
              </button>
            ))}
          </div>
          <details className="feed-filter-menu">
            <summary>
              <Filter size={13} /> Показать
              <span>{visibleStatuses.length + (watchedOnly ? 1 : 0)}</span>
            </summary>
            <div className="feed-filter-panel">
              {[
                ['approved', 'Одобренные'],
                ['pending', 'На рассмотрении'],
                ['rejected', 'Отказанные'],
              ].map(([status, label]) => (
                <button
                  type="button"
                  key={status}
                  className={visibleStatuses.includes(status) ? 'selected' : ''}
                  aria-pressed={visibleStatuses.includes(status)}
                  onClick={() => toggleStatus(status)}
                >
                  <span className={`filter-status-dot ${status}`} /> {label}
                </button>
              ))}
              <button
                type="button"
                className={watchedOnly ? 'selected' : ''}
                aria-pressed={watchedOnly}
                onClick={() => setWatchedOnly((value) => !value)}
              >
                <Eye size={13} /> Только отсмотренные
              </button>
            </div>
          </details>
        </div>
      </div>
      <div className="content-grid">
        <section className={`feed-grid ${list ? 'feed-list' : ''}`}>
          {filtered.map((video) => (
            <VideoCard key={video.id} video={video} role={role} onVote={onVote} onOpen={onOpen} onManage={manageVideo} list={list} />
          ))}
          {!filtered.length && (
            <div className="empty-state">
              <Search size={28} />
              <h3>Ничего не нашли</h3>
              <p>Измените запрос или фильтры</p>
            </div>
          )}
        </section>
        <aside className="feed-aside">
          <div className="promo-card">
            <div className="promo-image" />
            <div className="promo-content">
              <span className="eyebrow">КАК ЭТО РАБОТАЕТ</span>
              <h3>Предлагайте — Равшан смотрит</h3>
              <p>Зрители отправляют YouTube-ссылки, а модераторы допускают безопасные ролики в ленту.</p>
              <button className="primary-btn" onClick={() => can(role, 'submit') ? navigate('submit') : onLogin()}>
                {can(role, 'submit') ? 'Предложить видео' : 'Войти через Twitch'} <ArrowUpRight size={15} />
              </button>
            </div>
          </div>
          <div className="side-card">
            <div className="side-card-head">
              <h3>
                <Trophy size={16} /> Топ недели
              </h3>
              <span>обновлено сейчас</span>
            </div>
            {[...approved]
              .sort((a, b) => voteTotals(b).score - voteTotals(a).score)
              .slice(0, 5)
              .map((video, index) => (
                <button type="button" className="rank-row" key={video.id} onClick={() => onOpen(video)} aria-label={`Открыть «${video.title}» на YouTube`}>
                  <b>{index + 1}</b>
                  <span>{video.title}</span>
                  <strong>{voteTotals(video).score > 0 ? '+' : ''}{voteTotals(video).score}</strong>
                </button>
              ))}
          </div>
        </aside>
      </div>
    </main>
  );
}

function NewsView({
  posts,
  role,
  actor,
  onLogin,
  onCreatePost,
  onDeletePost,
  onCreateComment,
  onDeleteComment,
  notify,
}) {
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [commentDrafts, setCommentDrafts] = useState({});
  const [posting, setPosting] = useState(false);
  const [deleteArmed, setDeleteArmed] = useState('');

  const publish = async (event) => {
    event.preventDefault();
    if (!title.trim() || !body.trim()) return;
    setPosting(true);
    try {
      await onCreatePost(title.trim(), body.trim());
      setTitle('');
      setBody('');
      notify('Новость опубликована');
    } catch (error) {
      notify(error.message);
    } finally {
      setPosting(false);
    }
  };

  const comment = async (event, postId) => {
    event.preventDefault();
    const value = (commentDrafts[postId] || '').trim();
    if (!value) return;
    try {
      await onCreateComment(postId, value);
      setCommentDrafts((current) => ({ ...current, [postId]: '' }));
    } catch (error) {
      notify(error.message);
    }
  };

  return (
    <main className="main-content news-page">
      <section className="page-heading">
        <div>
          <div className="eyebrow"><span className="live-dot" /> НОВОСТИ ПРОЕКТА</div>
          <h1>Что нового в предложке</h1>
          <p>Обновления, планы и важные объявления от основателя проекта</p>
        </div>
      </section>

      {role === 'owner' && (
        <form className="panel news-editor" onSubmit={publish}>
          <div className="panel-head">
            <div>
              <span className="panel-kicker">НОВАЯ ПУБЛИКАЦИЯ</span>
              <h2>Написать новость</h2>
            </div>
            <Newspaper size={20} />
          </div>
          <label>
            Заголовок
            <input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={160} placeholder="Коротко о главном" required />
          </label>
          <label>
            Текст
            <textarea value={body} onChange={(event) => setBody(event.target.value)} maxLength={5000} placeholder="Расскажите пользователям об обновлении…" required />
          </label>
          <div className="news-editor-actions">
            <span>{body.length}/5000</span>
            <button className="primary-btn" disabled={posting || !title.trim() || !body.trim()}>
              <Send size={14} /> {posting ? 'Публикуем…' : 'Опубликовать'}
            </button>
          </div>
        </form>
      )}

      <section className="news-list">
        {posts.map((post) => (
          <article className="panel news-post" key={post.id}>
            <header>
              <Avatar src={post.author.avatarUrl} name={post.author.name} />
              <div>
                <strong>{post.author.name}</strong>
                <span>Основатель · {new Date(post.createdAt).toLocaleString('ru-RU')}</span>
              </div>
              {role === 'owner' && (
                <button
                  className={deleteArmed === post.id ? 'danger-confirm' : 'icon-btn'}
                  aria-label={deleteArmed === post.id ? 'Подтвердить удаление новости' : 'Удалить новость'}
                  onClick={async () => {
                    if (deleteArmed !== post.id) return setDeleteArmed(post.id);
                    try {
                      await onDeletePost(post.id);
                      notify('Новость удалена');
                    } catch (error) {
                      notify(error.message);
                    }
                    setDeleteArmed('');
                  }}
                >
                  {deleteArmed === post.id ? 'Удалить' : <Trash2 size={14} />}
                </button>
              )}
            </header>
            <h2>{post.title}</h2>
            <div className="news-body">{post.body}</div>

            <section className="comments-block" aria-label={`Комментарии к новости «${post.title}»`}>
              <div className="comments-title">
                <MessageCircle size={15} />
                <strong>Комментарии</strong>
                <span>{post.comments.length}</span>
              </div>
              <div className="comment-list">
                {post.comments.map((item) => (
                  <div className="comment-row" key={item.id}>
                    <Avatar small src={item.author.avatarUrl} name={item.author.name} />
                    <div>
                      <div className="comment-meta">
                        <strong>{item.author.name}</strong>
                        <span>{new Date(item.createdAt).toLocaleString('ru-RU')}</span>
                      </div>
                      <p>{item.body}</p>
                    </div>
                    {(role === 'owner' || actor?.id === item.author.id) && (
                      <button
                        className="comment-delete"
                        aria-label="Удалить комментарий"
                        onClick={() => onDeleteComment(item.id).catch((error) => notify(error.message))}
                      >
                        <Trash2 size={12} />
                      </button>
                    )}
                  </div>
                ))}
                {!post.comments.length && <p className="no-comments">Комментариев пока нет. Можно быть первым.</p>}
              </div>
              {can(role, 'comment_news') ? (
                <form className="comment-form" onSubmit={(event) => comment(event, post.id)}>
                  <Avatar small src={actor?.avatarUrl} name={actor?.name} />
                  <textarea
                    value={commentDrafts[post.id] || ''}
                    onChange={(event) => setCommentDrafts((current) => ({ ...current, [post.id]: event.target.value }))}
                    maxLength={1000}
                    rows={2}
                    placeholder="Написать комментарий…"
                    aria-label="Комментарий"
                  />
                  <button className="primary-btn" disabled={!(commentDrafts[post.id] || '').trim()} aria-label="Отправить комментарий">
                    <Send size={14} />
                  </button>
                </form>
              ) : (
                <button className="news-login-prompt" onClick={onLogin}>
                  Войдите через Twitch, чтобы оставить комментарий
                </button>
              )}
            </section>
          </article>
        ))}
        {!posts.length && (
          <div className="empty-state news-empty">
            <Newspaper size={30} />
            <h3>Новостей пока нет</h3>
            <p>Первое объявление появится здесь.</p>
          </div>
        )}
      </section>
    </main>
  );
}

const LEGAL_PAGES = {
  rules: {
    kicker: 'ПРАВИЛА СООБЩЕСТВА',
    title: 'Правила предложки',
    intro: 'Эти правила помогают модераторам безопасно отбирать видео для просмотра на стриме.',
    sections: [
      ['Что можно отправлять', ['Публичные YouTube-видео, доступные по обычной ссылке.', 'Материалы, которые подходят для совместного просмотра и обсуждения на стриме.', 'Один ролик отправляется один раз и размещается в подходящей категории.']],
      ['Что запрещено', ['Незаконный контент, угрозы, травля, шокирующие материалы и разглашение личных данных.', 'Реклама, скам, вредоносные ссылки, накрутка голосов и обход ограничений сайта.', 'Материалы, нарушающие права третьих лиц или правила YouTube.']],
      ['Модерация', ['Модератор может одобрить, отклонить, скрыть или удалить предложение.', 'Комментарий при отказе может отсутствовать; решение отображается в профиле автора.', 'Повторные нарушения могут привести к ограничению доступа к функциям проекта.']],
      ['Комментарии', ['Обсуждайте тему новости без оскорблений, спама и рекламы.', 'Автор может удалить свой комментарий, а основатель — любой комментарий или новость.']],
    ],
  },
  privacy: {
    kicker: 'ДАННЫЕ И БЕЗОПАСНОСТЬ',
    title: 'Политика конфиденциальности',
    intro: 'Мы собираем только данные, необходимые для авторизации и работы предложки.',
    sections: [
      ['Какие данные хранятся', ['Twitch User ID, логин, отображаемый ник и публичный аватар.', 'Отправленные ссылки, голоса, комментарии, уведомления и действия модерации.', 'Серверные сессии в защищённых cookie и технические журналы без паролей Twitch.']],
      ['Для чего используются данные', ['Для входа, назначения ролей, отображения профиля и защиты от злоупотреблений.', 'Для работы предложений, голосования, комментариев, уведомлений и аудита решений.', 'Данные не продаются и не используются для рекламного профилирования.']],
      ['Хранение и защита', ['Пароль Twitch и пользовательский OAuth-токен не сохраняются.', 'Доступ к административным данным ограничен ролями; изменяющие запросы защищены CSRF-проверкой.', 'Резервные копии базы создаются автоматически и хранятся с ограниченной ротацией.']],
      ['Ваши возможности', ['Можно выйти из аккаунта и прекратить использование сайта в любой момент.', 'Запрос на удаление профиля и связанных персональных данных направляется администрации проекта.', 'Часть записей аудита может сохраняться в обезличенном виде для безопасности проекта.']],
    ],
  },
  terms: {
    kicker: 'УСЛОВИЯ СЕРВИСА',
    title: 'Условия использования',
    intro: 'Используя сайт, вы соглашаетесь соблюдать правила проекта и требования Twitch и YouTube.',
    sections: [
      ['Назначение сервиса', ['Сайт хранит ссылки и метаданные, но не загружает и не раздаёт видеофайлы.', 'Видео открываются непосредственно на YouTube и подчиняются правилам этой платформы.']],
      ['Ответственность пользователя', ['Отправляйте только материалы, которыми вы вправе делиться.', 'Не пытайтесь нарушить работу сайта, получить чужой доступ или обойти лимиты.', 'Пользователь отвечает за содержание своих предложений и комментариев.']],
      ['Работа сервиса', ['Администрация может изменять категории, правила и функциональность проекта.', 'Доступность сайта не гарантируется непрерывно: возможны обновления и технические перерывы.', 'Нарушающий правила контент и связанные записи могут быть удалены без предварительного уведомления.']],
      ['Изменение условий', ['Актуальная версия публикуется на этой странице.', 'Продолжение использования сайта после обновления означает принятие новой редакции.']],
    ],
  },
};

function LegalView({ kind }) {
  const page = LEGAL_PAGES[kind];
  return (
    <main className="main-content legal-page">
      <section className="legal-hero">
        <FileText size={30} />
        <div>
          <span className="panel-kicker">{page.kicker}</span>
          <h1>{page.title}</h1>
          <p>{page.intro}</p>
          <small>Обновлено 7 августа 2026 года</small>
        </div>
      </section>
      <div className="legal-sections">
        {page.sections.map(([title, items]) => (
          <section className="panel" key={title}>
            <h2>{title}</h2>
            <ul>
              {items.map((item) => <li key={item}>{item}</li>)}
            </ul>
          </section>
        ))}
      </div>
    </main>
  );
}

function SubmitView({ state, actor, navigate, notify, onSubmit }) {
  const [url, setUrl] = useState('');
  const [category, setCategory] = useState('');
  const [comment, setComment] = useState('');
  const [movie, setMovie] = useState({ title: '', year: '', studio: '', rating: '', url: '' });
  const [error, setError] = useState('');
  const [sentId, setSentId] = useState(null);
  const count = recentSubmissionCount(state.videos, actor.id);
  const youtubeId = parseYouTubeId(url);
  const trailerCategory = /трейлер|фильм|сериал/i.test(category);

  const [submitting, setSubmitting] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setError('');
    if (!youtubeId) return setError('Проверьте ссылку: нужен корректный URL YouTube.');
    if (!category) return setError('Выберите категорию.');
    if (trailerCategory && !/^https:\/\/(?:www\.)?kinopoisk\.ru\//i.test(movie.url.trim())) return setError('Для фильма добавьте корректную ссылку на Кинопоиск.');
    if (isDuplicate(state.videos, youtubeId)) return setError('Это видео уже есть в предложке.');
    if (count >= state.settings.dailyLimit) return setError(`Достигнут лимит: ${state.settings.dailyLimit} отправки за 24 часа.`);
    setSubmitting(true);
    try {
      if (comment.trim().length > state.settings.commentLimit) return setError(`Пожелание должно занимать не более ${state.settings.commentLimit} символов.`);
      const video = await onSubmit({ url, category, comment: comment.trim(), movie: trailerCategory ? movie : null });
      setSentId(video.id);
      notify('Видео добавлено в очередь модерации');
    } catch (submissionError) {
      setError(submissionError.message || 'Не удалось отправить видео');
    } finally {
      setSubmitting(false);
    }
  };

  if (sentId) {
    return (
      <main className="main-content narrow">
        <div className="success-panel">
          <div className="success-icon">
            <Check />
          </div>
          <h2>Отправка #{sentId} принята</h2>
          <p>Решение модератора появится в уведомлениях и профиле.</p>
          <div className="success-actions">
            <button className="primary-btn" onClick={() => navigate('profile')}>
              Открыть профиль <ArrowUpRight size={15} />
            </button>
            <button className="ghost-btn" onClick={() => navigate('feed')}>Вернуться в ленту</button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="main-content narrow">
      <section className="page-heading">
        <div>
          <div className="eyebrow">НОВАЯ ОТПРАВКА</div>
          <h1>Предложить видео</h1>
          <p>Отправьте ссылку — видеофайл останется на YouTube</p>
        </div>
        <button className="ghost-btn" onClick={() => navigate('feed')}>Назад к ленте</button>
      </section>
      <form className="submit-layout" onSubmit={submit}>
        <section className="panel form-panel">
          <div className="panel-head">
            <div>
              <span className="panel-kicker">ШАГ 1 ИЗ 2</span>
              <h2>Ссылка и категория</h2>
            </div>
            <span className="limit">
              <Clock3 size={14} /> {count} из {state.settings.dailyLimit} за 24 часа
            </span>
          </div>
          <label>
            Категория
            <select value={category} onChange={(event) => setCategory(event.target.value)}>
              <option value="">Выберите категорию</option>
              {state.categories.map((item) => <option key={item}>{item}</option>)}
            </select>
          </label>
          {trailerCategory ? (
            <div className="movie-links-grid">
              <label>Трейлер
                <input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="Ссылка на YouTube-трейлер" inputMode="url" />
              </label>
              <label>Кинопоиск
                <input type="url" placeholder="https://www.kinopoisk.ru/film/..." value={movie.url} onChange={(event) => setMovie({ ...movie, url: event.target.value })} />
              </label>
            </div>
          ) : (
            <label>Ссылка на видео
              <input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://www.youtube.com/watch?v=..." inputMode="url" />
            </label>
          )}
          <label>
            Пожелание Равшану <span>необязательно, будет видно на сайте</span>
            <textarea
              maxLength={state.settings.commentLimit}
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="Что именно посмотреть или на что обратить внимание?"
            />
          </label>
          <div className="counter">{comment.length}/{state.settings.commentLimit}</div>
          {trailerCategory && (
            <div className="movie-fields">
              <div className="panel-kicker">ДАННЫЕ ФИЛЬМА · НЕОБЯЗАТЕЛЬНО</div>
              <label>Название<input value={movie.title} onChange={(event) => setMovie({ ...movie, title: event.target.value })} /></label>
              <label>Год<input inputMode="numeric" maxLength="4" value={movie.year} onChange={(event) => setMovie({ ...movie, year: event.target.value.replace(/\D/g, '') })} /></label>
              <label>Кинокомпания / студия<input value={movie.studio} onChange={(event) => setMovie({ ...movie, studio: event.target.value })} /></label>
              <label>Рейтинг<input placeholder="например, 7.8" value={movie.rating} onChange={(event) => setMovie({ ...movie, rating: event.target.value })} /></label>
            </div>
          )}
          {error && <div className="form-error" role="alert">{error}</div>}
          <button className="primary-btn full" type="submit" disabled={submitting}>
            {submitting ? 'Получаем данные YouTube…' : 'Проверить и отправить'} <ArrowUpRight size={15} />
          </button>
          <p className="form-note">
            <ShieldCheck size={14} /> Локальная версия проверяет формат, дубликаты и суточный лимит.
          </p>
        </section>
        <section className="preview-panel panel">
          <div className="panel-head">
            <div>
              <span className="panel-kicker">ПРЕДПРОСМОТР</span>
              <h2>Проверка ссылки</h2>
            </div>
          </div>
          <div className={`preview-placeholder ${url && !youtubeId ? 'invalid' : ''}`}>
            {youtubeId ? (
              <>
                <Link2 size={24} />
                <strong>YouTube ID: {youtubeId}</strong>
                <span>Ссылка распознана, карточка готова к отправке</span>
              </>
            ) : (
              <>
                <Play size={24} />
                <strong>{url ? 'Ссылка не распознана' : 'Вставьте ссылку на видео'}</strong>
                <span>Поддерживаются watch, youtu.be, Shorts и live</span>
              </>
            )}
          </div>
          <div className="rules">
            <h3>Перед отправкой</h3>
            <p><Check size={14} /> Видео должно быть доступно на YouTube</p>
            <p><Check size={14} /> Дубликаты блокируются по video ID</p>
            <p><Check size={14} /> Максимум {state.settings.dailyLimit} видео за 24 часа</p>
          </div>
        </section>
      </form>
    </main>
  );
}

function ProfileView({ videos, actor, navigate }) {
  const [filter, setFilter] = useState('all');
  const own = videos.filter((video) => video.authorId === actor.id);
  const filtered = filter === 'all' ? own : own.filter((video) => video.status === filter);
  return (
    <main className="main-content">
      <section className="profile-hero">
        <Avatar src={actor.avatarUrl} name={actor.name} />
        <div>
          <div className="eyebrow">ВАШ ПРОФИЛЬ</div>
          <h1>{actor.name}</h1>
          <p>{own.length} отправки · решения и отметка «Отсмотрено» видны здесь</p>
        </div>
        <button className="ghost-btn" onClick={() => navigate('submit')}>
          <Plus size={15} /> Предложить видео
        </button>
      </section>
      <section className="panel submissions">
        <div className="panel-head">
          <div>
            <span className="panel-kicker">ИСТОРИЯ</span>
            <h2>Мои видео</h2>
          </div>
          <span className="filter-button"><Filter size={14} /> {filtered.length} записей</span>
        </div>
        <div className="tabs" role="tablist">
          {[
            ['all', 'Все'],
            ['pending', 'На рассмотрении'],
            ['approved', 'Одобрено'],
            ['rejected', 'Отклонено'],
          ].map(([key, label]) => (
            <button key={key} className={filter === key ? 'selected' : ''} onClick={() => setFilter(key)} role="tab" aria-selected={filter === key}>
              {label}
            </button>
          ))}
        </div>
        {filtered.map((video) => (
          <div className="submission-row" key={video.id}>
            <Thumb video={video} />
            <div className="submission-info">
              <strong>{video.title}</strong>
              <span>{video.category} · {new Date(video.createdAt).toLocaleString('ru-RU')}</span>
            </div>
            <span className={`status ${video.status}`}>{STATUS_LABELS[video.status]}{video.watched && <small>Отсмотрено</small>}</span>
            <span className="decision">{video.moderatorComment || 'Ожидает решения модератора'}</span>
          </div>
        ))}
        {!filtered.length && <div className="compact-empty">В этом разделе пока нет видео</div>}
      </section>
    </main>
  );
}

function NotificationView({ notifications, markAllRead, markRead }) {
  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow">ЦЕНТР УВЕДОМЛЕНИЙ</div>
          <h1>Уведомления</h1>
          <p>Решения модерации и важные события</p>
        </div>
        <button className="ghost-btn" onClick={markAllRead}>Прочитать все</button>
      </section>
      <div className="panel notification-page">
        {notifications.map((notice) => (
          <button className={`notice-row big ${notice.read ? 'is-read' : ''}`} key={notice.id} onClick={() => !notice.read && markRead(notice.id)}>
            <i className={notice.tone === 'red' ? 'red' : ''} />
            <div>
              <strong>{notice.title}</strong>
              <p>{notice.body}</p>
            </div>
            <span>{new Date(notice.createdAt).toLocaleString('ru-RU')}</span>
          </button>
        ))}
      </div>
    </main>
  );
}

function ModeratorMovieEditor({ video, onSave, notify }) {
  const [movie, setMovie] = useState({ url: '', title: '', year: '', studio: '', rating: '' });
  const [saving, setSaving] = useState(false);
  useEffect(() => setMovie({
    url: video.kinopoiskUrl || '', title: video.movieTitle || '', year: video.movieYear || '',
    studio: video.movieStudio || '', rating: video.movieRating ?? '',
  }), [video.id, video.kinopoiskUrl, video.movieTitle, video.movieYear, video.movieStudio, video.movieRating]);
  const relevant = /трейлер|фильм|сериал/i.test(video.category) || video.kinopoiskUrl || video.movieTitle;
  if (!relevant) return null;
  return (
    <section className="moderator-movie-editor">
      <div><span className="panel-kicker">КАРТОЧКА ФИЛЬМА</span><small>Можно заполнить или исправить данные пользователя</small></div>
      <div className="movie-links-grid">
        <label>Кинопоиск<input type="url" value={movie.url} placeholder="https://www.kinopoisk.ru/film/..." onChange={(event) => setMovie({ ...movie, url: event.target.value })} /></label>
        <label>Название<input value={movie.title} onChange={(event) => setMovie({ ...movie, title: event.target.value })} /></label>
      </div>
      <div className="movie-fields moderator-movie-fields">
        <label>Год<input inputMode="numeric" value={movie.year} onChange={(event) => setMovie({ ...movie, year: event.target.value.replace(/\D/g, '').slice(0, 4) })} /></label>
        <label>Кинокомпания / студия<input value={movie.studio} onChange={(event) => setMovie({ ...movie, studio: event.target.value })} /></label>
        <label>Рейтинг<input type="number" min="0" max="10" step="0.1" value={movie.rating} onChange={(event) => setMovie({ ...movie, rating: event.target.value })} /></label>
      </div>
      <button className="outline-btn" disabled={saving} onClick={async () => {
        setSaving(true);
        try { await onSave(video.id, movie, video.version); notify('Данные фильма сохранены'); }
        catch (error) { notify(error.message); }
        finally { setSaving(false); }
      }}>{saving ? 'Сохраняем…' : 'Сохранить данные фильма'}</button>
    </section>
  );
}

function ModerationView({ state, role, onDecision, onWatched, onDelete, onCategoryChange, onMovieUpdate, notify }) {
  const [tab, setTab] = useState('pending');
  const items = state.videos.filter((video) => video.status === tab);
  const [selectedId, setSelectedId] = useState(items[0]?.id);
  const [comment, setComment] = useState('');
  const [deleteArmed, setDeleteArmed] = useState(false);
  const selected = state.videos.find((video) => video.id === selectedId && video.status === tab) || items[0];

  useEffect(() => {
    if (!selected && items[0]) setSelectedId(items[0].id);
  }, [selected, items]);
  useEffect(() => {
    const targetId = window.sessionStorage.getItem('ravshann-moderation-target');
    const target = state.videos.find((video) => String(video.id) === targetId);
    if (!target) return;
    setTab(target.status);
    setSelectedId(target.id);
    window.sessionStorage.removeItem('ravshann-moderation-target');
  }, [state.videos]);
  useEffect(() => setDeleteArmed(false), [selected?.id]);

  const decide = (status) => {
    if (!selected) return;
    onDecision(selected.id, status, comment.trim());
    setComment('');
    notify(status === 'approved' ? 'Видео опубликовано в ленте' : 'Решение сохранено и отправлено автору');
  };

  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow"><span className="live-dot orange" /> ЗАЩИЩЁННЫЙ РАЗДЕЛ</div>
          <h1>Модерация видео</h1>
          <p>Очередь, решение, категория и публичная отметка «Отсмотрено»</p>
        </div>
      </section>
      <div className="moderation-layout">
        <section className="panel queue">
          <div className="queue-tabs">
            {[
              ['pending', 'На рассмотрении'],
              ['approved', 'Главное'],
              ['rejected', 'Отказано'],
            ].map(([status, label]) => (
              <button
                key={status}
                className={tab === status ? 'selected' : ''}
                onClick={() => {
                  setTab(status);
                  setSelectedId(state.videos.find((video) => video.status === status)?.id);
                }}
              >
                {label} <span>{state.videos.filter((video) => video.status === status).length}</span>
              </button>
            ))}
          </div>
          {items.map((video) => (
            <button className={`queue-item ${selected?.id === video.id ? 'selected' : ''}`} key={video.id} onClick={() => setSelectedId(video.id)}>
              <Thumb video={video} />
              <div>
                <strong>{video.title}</strong>
                <span>{video.author} · #{video.id}</span>
              </div>
              <span className="queue-status">{STATUS_LABELS[video.status].toUpperCase()}</span>
            </button>
          ))}
          {!items.length && <div className="compact-empty">Очередь пуста</div>}
        </section>
        {selected ? (
          <section className="panel review">
            <div className="review-head">
              <div>
                <span className="panel-kicker">ID #{selected.id} · {new Date(selected.createdAt).toLocaleString('ru-RU')}</span>
                <h2>{selected.title}</h2>
              </div>
              <span className={`status ${selected.status}`}>{STATUS_LABELS[selected.status]}</span>
            </div>
            <a href={selected.youtubeUrl} target="_blank" rel="noopener noreferrer" aria-label="Открыть видео на YouTube">
              <Thumb video={selected} large />
            </a>
            <div className="review-copy">
              <span className="panel-kicker">КОММЕНТАРИЙ ПОЛЬЗОВАТЕЛЯ</span>
              <p>«{selected.submitterComment || 'Комментарий не оставлен'}»</p>
              <div className="meta-grid">
                <span>Категория <b>{selected.category}</b></span>
                <span>Канал <b>{selected.channel} · {selected.views} просмотров</b></span>
                <span>Отправитель <b>{selected.author}</b></span>
              </div>
            </div>
            <ModeratorMovieEditor video={selected} onSave={onMovieUpdate} notify={notify} />
            {selected.status === 'pending' && (
              <>
                <label className="moderation-comment">
                  Комментарий / причина отказа <span>необязательно</span>
                  <textarea value={comment} onChange={(event) => setComment(event.target.value)} maxLength={500} />
                </label>
                <div className="review-actions">
                  <button className="approve" onClick={() => decide('approved')}><Check size={15} /> Одобрить</button>
                  <button className="reject" onClick={() => decide('rejected')}><X size={15} /> Отклонить</button>
                  <a className="outline-btn" href={selected.youtubeUrl} target="_blank" rel="noopener noreferrer">
                    YouTube <ExternalLink size={14} />
                  </a>
                </div>
              </>
            )}
            {selected.status === 'approved' && (
              <div className="watched-row">
                <button className={`watched-toggle ${selected.watched ? 'is-on' : ''}`} onClick={() => onWatched(selected.id)}>
                  <Eye size={14} /> {selected.watched ? 'Отсмотрено' : 'Отметить как отсмотренное'}
                </button>
                <span className="public-note">Плашку увидят все посетители</span>
              </div>
            )}
            {selected.status === 'rejected' && selected.moderatorComment && (
              <div className="moderator-decision">
                <span className="panel-kicker">КОММЕНТАРИЙ МОДЕРАТОРА</span>
                <p>{selected.moderatorComment}</p>
              </div>
            )}
            <div className="video-management">
              <label>
                Категория
                <select value={selected.category} onChange={(event) => onCategoryChange(selected.id, event.target.value)}>
                  {state.categories.map((category) => <option key={category}>{category}</option>)}
                </select>
              </label>
              {selected.status === 'rejected' && (
                <button className="outline-btn" onClick={() => decide('pending')}>Вернуть на рассмотрение</button>
              )}
              <button
                className={deleteArmed ? 'danger-confirm' : 'danger-btn'}
                onClick={() => {
                  if (!deleteArmed) return setDeleteArmed(true);
                  onDelete(selected.id);
                  setDeleteArmed(false);
                  notify('Видео удалено из предложки');
                }}
              >
                {deleteArmed ? 'Подтвердить удаление' : 'Удалить видео'}
              </button>
            </div>
          </section>
        ) : (
          <section className="panel review compact-empty">Выберите видео из очереди</section>
        )}
        <aside className="review-aside">
          <div className="side-card stats">
            <h3>Сводка</h3>
            <p>На рассмотрении <b className="amber-text">{state.videos.filter((video) => video.status === 'pending').length}</b></p>
            <p>Одобрено <b className="green">{state.videos.filter((video) => video.status === 'approved').length}</b></p>
            <p>Отклонено <b className="red-text">{state.videos.filter((video) => video.status === 'rejected').length}</b></p>
          </div>
          <div className="side-card check-list">
            <h3>Автопроверка</h3>
            <p>✓ Ссылка YouTube</p>
            <p>✓ Видео доступно</p>
            <p>✓ Дубликат не найден</p>
            <p className="warn">! Содержание проверить вручную</p>
          </div>
          <div className="side-card">
            <h3>Права</h3>
            <p>{role === 'owner' ? 'Основатель видит управление ролями и аудит.' : 'Модератор принимает решения, но не назначает роли.'}</p>
          </div>
        </aside>
      </div>
    </main>
  );
}

function OwnerView({ state, setState, notify, onAddCategory, onDeleteCategory, onAddModerator, onDeleteModerator, onUpdateSettings }) {
  const [moderator, setModerator] = useState('');
  const [category, setCategory] = useState('');
  const [categoryDeleteArmed, setCategoryDeleteArmed] = useState('');
  const [settings, setSettings] = useState(state.settings);
  useEffect(() => setSettings(state.settings), [state.settings]);

  const addModerator = async () => {
    const value = moderator.trim().replace(/^@/, '');
    if (!value || state.moderators.includes(value)) return notify('Введите новый точный Twitch-ник');
    if (onAddModerator) {
      try {
        await onAddModerator(value);
        setModerator('');
        notify('Модератор добавлен');
      } catch (error) {
        notify(error.message);
      }
      return;
    }
    setState((current) => ({
      ...current,
      moderators: [...current.moderators, value],
      audit: [`Ravshann добавил ${value}`, ...current.audit],
    }));
    setModerator('');
    notify('Модератор добавлен');
  };
  const addCategory = async () => {
    const value = category.trim();
    if (!value || state.categories.includes(value)) return notify('Введите новое название категории');
    if (onAddCategory) {
      try {
        await onAddCategory(value);
        setCategory('');
        notify('Категория создана');
      } catch (error) {
        notify(error.message);
      }
      return;
    }
    setState((current) => ({
      ...current,
      categories: [...current.categories, value],
      audit: [`Создана категория «${value}»`, ...current.audit],
    }));
    setCategory('');
    notify('Категория создана');
  };

  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow">ПАНЕЛЬ ОСНОВАТЕЛЯ</div>
          <h1>Управление предложкой</h1>
          <p>Роли, категории, журнал действий и глобальные ограничения</p>
        </div>
      </section>
      <div className="stat-strip">
        <div><b className="purple-text">{state.videos.filter((video) => video.status === 'pending').length}</b><span>Видео ждут проверки</span></div>
        <div><b className="green">{state.videos.filter((video) => video.status === 'approved').length}</b><span>Видео в публичной ленте</span></div>
        <div><b className="amber-text">{state.moderators.length}</b><span>Активных модератора</span></div>
      </div>
      <div className="owner-columns">
        <section className="panel owner-panel">
          <div className="panel-head"><h2>Модераторы</h2><span className="panel-kicker">{state.moderators.length} АКТИВНЫХ</span></div>
          <div className="add-row">
            <input value={moderator} onChange={(event) => setModerator(event.target.value)} placeholder="Точный Twitch-ник" />
            <button className="primary-btn" onClick={addModerator}>Добавить модератора</button>
          </div>
          {state.moderators.map((item) => (
            <div className="moderator-row" key={item}>
              <Avatar small name={item} />
              <strong>{item}</strong>
              <b>Модератор</b>
              <button
                className="danger-btn"
                onClick={async () => {
                  if (onDeleteModerator) {
                    try {
                      await onDeleteModerator(item);
                      notify('Модератор удалён');
                    } catch (error) {
                      notify(error.message);
                    }
                    return;
                  }
                  setState((current) => ({
                    ...current,
                    moderators: current.moderators.filter((name) => name !== item),
                    audit: [`Ravshann удалил ${item}`, ...current.audit],
                  }));
                  notify('Модератор удалён');
                }}
              >
                Удалить
              </button>
            </div>
          ))}
        </section>
        <section className="panel owner-panel">
          <div className="panel-head"><h2>Журнал действий</h2><span className="panel-kicker">ЛОКАЛЬНЫЙ АУДИТ</span></div>
          {state.audit.slice(0, 7).map((line, index) => (
            <div className="audit-row" key={`${line}-${index}`}>
              <i className={`audit-dot a${index % 4}`} />
              <div><strong>{line}</strong><span>Локальная сессия</span></div>
            </div>
          ))}
        </section>
      </div>
      <div className="owner-columns lower">
        <section className="panel owner-panel">
          <div className="panel-head"><h2>Категории</h2><span className="panel-kicker">{state.categories.length} КАТЕГОРИИ</span></div>
          <div className="add-row">
            <input value={category} onChange={(event) => setCategory(event.target.value)} placeholder="Название новой категории" />
            <button className="primary-btn" onClick={addCategory}>Создать категорию</button>
          </div>
          <p className="category-note">При удалении категории её видео перейдут в «Без категории».</p>
          {state.categories.map((item) => {
            const count = state.videos.filter((video) => video.category === item).length;
            return (
              <div className="category-row" key={item}>
                <strong>{item}</strong>
                <span>{count} видео</span>
                <button
                  className={categoryDeleteArmed === item ? 'danger-confirm-link' : 'danger-link'}
                  disabled={item === 'Без категории'}
                  title={item === 'Без категории' ? 'Системную категорию удалить нельзя' : undefined}
                  onClick={async () => {
                    if (categoryDeleteArmed !== item) return setCategoryDeleteArmed(item);
                    if (onDeleteCategory) {
                      try {
                        await onDeleteCategory(item);
                        setCategoryDeleteArmed('');
                        notify(`Категория удалена, ${count} видео перенесено`);
                      } catch (error) {
                        notify(error.message);
                      }
                      return;
                    }
                    setState((current) => {
                      const next = removeCategory(current, item);
                      return { ...next, audit: [`Ravshann удалил категорию «${item}»`, ...next.audit] };
                    });
                    setCategoryDeleteArmed('');
                    notify(`Категория удалена, ${count} видео перенесено`);
                  }}
                >
                  {categoryDeleteArmed === item ? 'Подтвердить' : 'Удалить'}
                </button>
              </div>
            );
          })}
        </section>
        <section className="panel owner-panel settings-panel">
          <h2>Глобальные настройки</h2>
          <label>
            Лимит отправок за 24 часа
            <input type="number" min="1" max="20" value={settings.dailyLimit} onChange={(event) => setSettings({ ...settings, dailyLimit: Number(event.target.value) })} />
          </label>
          <label>
            Максимальный комментарий
            <input type="number" min="100" max="2000" value={settings.commentLimit} onChange={(event) => setSettings({ ...settings, commentLimit: Number(event.target.value) })} />
          </label>
          <label>
            Публичная лента
            <select value={settings.publicFeed ? 'open' : 'closed'} onChange={(event) => setSettings({ ...settings, publicFeed: event.target.value === 'open' })}>
              <option value="open">Открыта</option>
              <option value="closed">Временно закрыта</option>
            </select>
          </label>
          <label className="setting-check">
            <input type="checkbox" checked={Boolean(settings.allowSelfVote)} onChange={(event) => setSettings({ ...settings, allowSelfVote: event.target.checked })} />
            Разрешить голосовать за собственные видео
          </label>
          <h3>Социальные сети Равшана</h3>
          <div className="social-settings-list">
            {(settings.socialLinks || []).map((item, index) => (
              <div className="social-setting-row" key={index}>
                <label>Название
                  <input value={item.name} maxLength={40} placeholder="Telegram" onChange={(event) => setSettings({ ...settings, socialLinks: settings.socialLinks.map((link, linkIndex) => linkIndex === index ? { ...link, name: event.target.value } : link) })} />
                </label>
                <label>Ссылка
                  <input type="url" value={item.url} placeholder="https://..." onChange={(event) => setSettings({ ...settings, socialLinks: settings.socialLinks.map((link, linkIndex) => linkIndex === index ? { ...link, url: event.target.value } : link) })} />
                </label>
                <button type="button" className="danger-btn" onClick={() => setSettings({ ...settings, socialLinks: settings.socialLinks.filter((_, linkIndex) => linkIndex !== index) })}>Удалить</button>
              </div>
            ))}
            <button type="button" className="ghost-btn" onClick={() => setSettings({ ...settings, socialLinks: [...(settings.socialLinks || []), { name: '', url: '' }] })}><Plus size={14} /> Добавить соцсеть</button>
          </div>
          <h3>Поддержка проекта</h3>
          <div className="social-settings-list">
            {(settings.supportLinks || []).map((item, index) => (
              <div className="social-setting-row" key={index}>
                <label>Название
                  <input value={item.name} maxLength={40} placeholder="DonationAlerts" onChange={(event) => setSettings({ ...settings, supportLinks: settings.supportLinks.map((link, linkIndex) => linkIndex === index ? { ...link, name: event.target.value } : link) })} />
                </label>
                <label>Ссылка
                  <input type="url" value={item.url} placeholder="https://..." onChange={(event) => setSettings({ ...settings, supportLinks: settings.supportLinks.map((link, linkIndex) => linkIndex === index ? { ...link, url: event.target.value } : link) })} />
                </label>
                <button type="button" className="danger-btn" onClick={() => setSettings({ ...settings, supportLinks: settings.supportLinks.filter((_, linkIndex) => linkIndex !== index) })}>Удалить</button>
              </div>
            ))}
            <button type="button" className="ghost-btn" onClick={() => setSettings({ ...settings, supportLinks: [...(settings.supportLinks || []), { name: '', url: '' }] })}><Plus size={14} /> Добавить ссылку поддержки</button>
          </div>
          <button
            className="primary-btn"
            onClick={async () => {
              if (onUpdateSettings) {
                try {
                  await onUpdateSettings(settings);
                  notify('Настройки сохранены');
                } catch (error) {
                  notify(error.message);
                }
                return;
              }
              setState((current) => ({ ...current, settings, audit: ['Ravshann обновил глобальные настройки', ...current.audit] }));
              notify('Настройки сохранены');
            }}
          >
            Сохранить настройки
          </button>
        </section>
      </div>
      <section className="panel owner-panel users-panel">
        <div className="panel-head"><h2>Пользователи</h2><span className="panel-kicker">{state.userStats?.length || 0} АККАУНТОВ</span></div>
        <div className="users-table-wrap">
          <table className="users-table">
            <thead><tr><th>Пользователь</th><th>Роль</th><th>Всего</th><th>На рассмотрении</th><th>Одобрено</th><th>Отказано</th><th>Отсмотрено</th><th>Комментарии</th><th>Последний вход</th></tr></thead>
            <tbody>{(state.userStats || []).map((item) => (
              <tr key={item.user.id}>
                <td><span className="user-cell"><Avatar small src={item.user.avatar_url} name={item.user.display_name} /><b>{item.user.display_name || item.user.login}</b></span></td>
                <td>{ROLE_LABELS[item.user.role] || item.user.role}</td><td>{item.total}</td><td>{item.pending}</td><td>{item.approved}</td><td>{item.rejected}</td><td>{item.watched}</td><td>{item.comments}</td>
                <td>{item.last_login_at ? new Date(item.last_login_at).toLocaleString('ru-RU') : '—'}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      </section>
      <section className="panel owner-panel users-panel">
        <div className="panel-head"><h2>Все действия пользователей</h2><span className="panel-kicker">ПОСЛЕДНИЕ {state.auditRecords?.length || 0}</span></div>
        <div className="users-table-wrap">
          <table className="users-table audit-table">
            <thead><tr><th>Время</th><th>Пользователь</th><th>Действие</th><th>Объект</th><th>ID</th><th>Детали</th></tr></thead>
            <tbody>{(state.auditRecords || []).map((entry) => (
              <tr key={entry.id}>
                <td>{new Date(entry.created_at).toLocaleString('ru-RU')}</td>
                <td>{entry.actor?.display_name || entry.actor?.login || 'Система'}</td>
                <td><b>{entry.action}</b></td><td>{entry.target_type}</td><td>{entry.target_id}</td>
                <td>{entry.metadata && JSON.stringify(entry.metadata) !== '{}' ? JSON.stringify(entry.metadata) : '—'}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      </section>
    </main>
  );
}

function AccessDenied({ role, onAccess }) {
  return (
    <main className="main-content narrow">
      <div className="success-panel access-panel">
        <ShieldCheck size={38} />
        <h2>Этот раздел недоступен роли «{ROLE_LABELS[role]}»</h2>
        <p>Войдите через Twitch. Если вам выданы права модератора, раздел появится автоматически.</p>
        <button className="primary-btn" onClick={onAccess}>Войти через Twitch</button>
      </div>
    </main>
  );
}

function App() {
  const defaultRoute = 'home';
  const [route, navigate] = useRoute(defaultRoute);
  const [role, setRole] = useState('guest');
  const [sessionUser, setSessionUser] = useState(null);
  const [state, setState, apiReady, reload] = useProjectState(role);
  const [search, setSearch] = useState('');
  const [authOpen, setAuthOpen] = useState(false);
  const [toast, setToast] = useState('');
  const moderationAudioRef = useRef(null);
  const pendingSubmissionIdsRef = useRef(new Set());
  const actor = sessionUser
    ? {
        id: sessionUser.id,
        name: sessionUser.display_name || sessionUser.login,
        avatarUrl: sessionUser.avatar_url || '',
      }
    : role === 'owner'
      ? { id: 'owner', name: 'rafchibiskus' }
      : role === 'moderator'
        ? { id: 'moderator', name: 'moderator_live' }
        : { id: 'viewer', name: 'Ravshibiscus' };
  const unread = state.notifications.filter((notice) => !notice.read).length;

  useEffect(() => {
    api.currentUser()
      .then((user) => {
        setSessionUser(user);
        setRole(user?.role || 'guest');
      })
      .catch(() => {
        setSessionUser(null);
        setRole('guest');
      });
  }, []);

  useEffect(() => {
    if (!apiReady) return undefined;
    const refreshStreamer = () => api.loadStreamer()
      .then((streamer) => setState((current) => ({ ...current, streamer })))
      .catch(() => {});
    const timer = window.setInterval(refreshStreamer, 60000);
    return () => window.clearInterval(timer);
  }, [apiReady]);

  const notify = useCallback((message) => {
    setToast(message);
    window.clearTimeout(window.__ravshannToast);
    window.__ravshannToast = window.setTimeout(() => setToast(''), 2800);
  }, []);

  useEffect(() => {
    if (!can(role, 'moderate')) return undefined;
    const armAudio = () => {
      if (!moderationAudioRef.current || moderationAudioRef.current.state === 'closed') {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        if (AudioContextClass) moderationAudioRef.current = new AudioContextClass();
      }
      moderationAudioRef.current?.resume().catch(() => {});
    };
    window.addEventListener('pointerdown', armAudio, { once: true });
    window.addEventListener('keydown', armAudio, { once: true });
    return () => {
      window.removeEventListener('pointerdown', armAudio);
      window.removeEventListener('keydown', armAudio);
    };
  }, [role]);

  useEffect(() => {
    if (!can(role, 'moderate') || !apiReady) return undefined;
    let active = true;
    pendingSubmissionIdsRef.current = new Set(
      state.videos.filter((video) => video.status === 'pending').map((video) => video.id),
    );

    const pollPending = async () => {
      try {
        const pending = await api.loadPendingSubmissions();
        if (!active) return;
        const additions = newPendingSubmissions(pendingSubmissionIdsRef.current, pending);
        pendingSubmissionIdsRef.current = new Set(pending.map((video) => video.id));
        if (!additions.length) return;

        setState((current) => {
          const existingIds = new Set(current.videos.map((video) => video.id));
          return {
            ...current,
            videos: [...additions.filter((video) => !existingIds.has(video.id)), ...current.videos],
          };
        });
        playNewSubmissionSound(moderationAudioRef);
        notify(additions.length === 1
          ? 'Новое видео поступило на рассмотрение'
          : `Новых видео на рассмотрении: ${additions.length}`);
      } catch {
        // Следующая фоновая проверка повторит запрос без вмешательства пользователя.
      }
    };

    const timer = window.setInterval(pollPending, MODERATION_POLL_INTERVAL);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [apiReady, role]);

  const vote = async (id, direction) => {
    if (!can(role, 'vote')) return setAuthOpen(true);
    if (!apiReady) return notify('Сервер временно недоступен');
    try {
      const result = await api.vote(id, direction === 'up' ? 1 : -1);
      setState((current) => ({
        ...current,
        videos: current.videos.map((video) => video.id === id
          ? { ...video, rating: result.rating, userVote: result.user_vote }
          : video),
      }));
    } catch (error) {
      notify(error.message);
    }
  };
  const decide = async (id, status, comment) => {
    if (!apiReady) return notify('Сервер временно недоступен');
    const currentVideo = state.videos.find((item) => item.id === id);
    try {
      const updated = await api.decide(id, status, comment, currentVideo.version);
      setState((current) => ({
        ...current,
        videos: current.videos.map((item) => item.id === id ? updated : item),
      }));
    } catch (error) {
      notify(error.message);
      await reload().catch(() => {});
    }
  };
  const toggleWatched = async (id) => {
    const currentVideo = state.videos.find((item) => item.id === id);
    if (!apiReady || !currentVideo) return notify('Сервер временно недоступен');
    try {
      await api.setWatched(id, !currentVideo.watched);
      setState((current) => ({
        ...current,
        videos: current.videos.map((video) => video.id === id ? { ...video, watched: !video.watched } : video),
      }));
      notify('Публичная отметка обновлена');
    } catch (error) {
      notify(error.message);
    }
  };
  const openVideo = (video) => window.open(video.youtubeUrl, '_blank', 'noopener,noreferrer');
  const openAuth = () => setAuthOpen(true);
  const localTwitchLogin = () => {
    window.location.assign('/api/auth/twitch/start?return_to=/');
  };
  const signOut = async () => {
    await api.logout().catch(() => {});
    setSessionUser(null);
    setRole('guest');
    navigate('home');
    notify('Вы вышли из аккаунта');
  };
  const deleteVideo = async (id) => {
    if (!apiReady) return notify('Сервер временно недоступен');
    try {
      await api.deleteVideo(id);
      setState((current) => ({
        ...current,
        videos: current.videos.filter((video) => video.id !== id),
      }));
    } catch (error) {
      notify(error.message);
    }
  };
  const changeVideoCategory = async (id, category) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === category);
    if (!apiReady || !categoryRecord) return notify('Сервер временно недоступен');
    try {
      await api.setVideoCategory(id, categoryRecord.id);
      setState((current) => ({
        ...current,
        videos: current.videos.map((video) => video.id === id
          ? { ...video, category, categoryId: categoryRecord.id, version: video.version + 1 }
          : video),
      }));
      notify('Категория видео обновлена');
    } catch (error) {
      notify(error.message);
    }
  };
  const submitVideo = async ({ url, category, comment, movie }) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === category);
    if (!apiReady || !categoryRecord) throw new Error('API пока недоступен — повторите через несколько секунд');
    const video = await api.createSubmission({
      url,
      category_id: categoryRecord.id,
      comment,
      kinopoisk_url: movie?.url?.trim() || '',
      movie_title: movie?.title?.trim() || '',
      movie_year: movie?.year ? Number(movie.year) : null,
      movie_studio: movie?.studio?.trim() || '',
      movie_rating: movie?.rating !== '' && movie?.rating != null ? Number(movie.rating) : null,
    });
    setState((current) => ({ ...current, videos: [video, ...current.videos] }));
    return video;
  };
  const updateMovieMetadata = async (id, movie, version) => {
    if (!apiReady) throw new Error('API пока недоступен');
    const updated = await api.setMovieMetadata(id, movie, version);
    setState((current) => ({ ...current, videos: current.videos.map((video) => video.id === id ? updated : video) }));
    return updated;
  };
  const addCategory = async (name) => {
    if (!apiReady) throw new Error('API пока недоступен');
    const categoryRecord = await api.createCategory(name);
    setState((current) => ({
      ...current,
      categoryRecords: [...(current.categoryRecords || []), categoryRecord],
      categories: [...current.categories, categoryRecord.name],
    }));
  };
  const deleteCategory = async (name) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === name);
    if (!apiReady || !categoryRecord) throw new Error('Категория не найдена');
    await api.deleteCategory(categoryRecord.id);
    const fallback = state.categoryRecords?.find((item) => item.is_system);
    setState((current) => ({
      ...current,
      categoryRecords: current.categoryRecords.filter((item) => item.id !== categoryRecord.id),
      categories: current.categories.filter((item) => item !== name),
      videos: current.videos.map((video) => video.category === name
        ? { ...video, category: fallback?.name || 'Без категории', categoryId: fallback?.id }
        : video),
    }));
  };
  const addModerator = async (login) => {
    if (!apiReady) throw new Error('API пока недоступен');
    const user = await api.assignModerator(login);
    setState((current) => ({
      ...current,
      moderatorRecords: [...(current.moderatorRecords || []), user],
      moderators: [...current.moderators.filter((item) => item !== login), user.login || user.display_name],
    }));
  };
  const deleteModerator = async (login) => {
    const user = state.moderatorRecords?.find((item) => (item.login || item.display_name) === login);
    if (!user) throw new Error('Сначала пользователь должен войти через Twitch');
    await api.removeModerator(user.id);
    setState((current) => ({
      ...current,
      moderatorRecords: current.moderatorRecords.filter((item) => item.id !== user.id),
      moderators: current.moderators.filter((item) => item !== login),
    }));
  };
  const updateSettings = async (settings) => {
    if (!apiReady) throw new Error('API пока недоступен');
    await api.updateSettings(settings);
    setState((current) => ({ ...current, settings }));
  };
  const markNotification = async (id) => {
    if (apiReady) await api.readNotification(id);
    setState((current) => ({
      ...current,
      notifications: current.notifications.map((notice) => notice.id === id ? { ...notice, read: true } : notice),
    }));
  };
  const markAllNotifications = async () => {
    if (apiReady) await api.readAllNotifications();
    setState((current) => ({
      ...current,
      notifications: current.notifications.map((notice) => ({ ...notice, read: true })),
    }));
  };
  const createNewsPost = async (title, body) => {
    if (!apiReady) throw new Error('Сервер временно недоступен');
    const post = await api.createNewsPost(title, body);
    setState((current) => ({ ...current, news: [post, ...current.news] }));
  };
  const deleteNewsPost = async (postId) => {
    if (!apiReady) throw new Error('Сервер временно недоступен');
    await api.deleteNewsPost(postId);
    setState((current) => ({ ...current, news: current.news.filter((post) => post.id !== postId) }));
  };
  const createNewsComment = async (postId, body) => {
    if (!apiReady) throw new Error('Сервер временно недоступен');
    const comment = await api.createNewsComment(postId, body);
    setState((current) => ({
      ...current,
      news: current.news.map((post) => post.id === postId
        ? { ...post, comments: [...post.comments, comment] }
        : post),
    }));
  };
  const deleteNewsComment = async (commentId) => {
    if (!apiReady) throw new Error('Сервер временно недоступен');
    await api.deleteNewsComment(commentId);
    setState((current) => ({
      ...current,
      news: current.news.map((post) => ({
        ...post,
        comments: post.comments.filter((comment) => comment.id !== commentId),
      })),
    }));
  };
  let page;
  if (route === 'home') page = <StreamerHome streamer={state.streamer} />;
  else if (route === 'feed') page = <Feed videos={state.videos} categories={state.categories} role={role} search={search} onVote={vote} onOpen={openVideo} navigate={navigate} onLogin={openAuth} />;
  else if (route === 'news') page = <NewsView posts={state.news} role={role} actor={actor} onLogin={openAuth} onCreatePost={createNewsPost} onDeletePost={deleteNewsPost} onCreateComment={createNewsComment} onDeleteComment={deleteNewsComment} notify={notify} />;
  else if (route === 'submit') page = can(role, 'submit') ? <SubmitView state={state} actor={actor} navigate={navigate} notify={notify} onSubmit={submitVideo} /> : <AccessDenied role={role} onAccess={openAuth} />;
  else if (route === 'profile') page = can(role, 'view_profile') ? <ProfileView videos={state.videos} actor={actor} navigate={navigate} /> : <AccessDenied role={role} onAccess={openAuth} />;
  else if (route === 'notifications') page = can(role, 'view_profile') ? <NotificationView notifications={state.notifications} markAllRead={markAllNotifications} markRead={markNotification} /> : <AccessDenied role={role} onAccess={openAuth} />;
  else if (route === 'moderation') page = can(role, 'moderate') ? <ModerationView state={state} role={role} onDecision={decide} onWatched={toggleWatched} onDelete={deleteVideo} onCategoryChange={changeVideoCategory} onMovieUpdate={updateMovieMetadata} notify={notify} /> : <AccessDenied role={role} onAccess={openAuth} />;
  else if (route === 'owner') page = can(role, 'manage') ? <OwnerView state={state} setState={setState} notify={notify} onAddCategory={apiReady ? addCategory : null} onDeleteCategory={apiReady ? deleteCategory : null} onAddModerator={apiReady ? addModerator : null} onDeleteModerator={apiReady ? deleteModerator : null} onUpdateSettings={apiReady ? updateSettings : null} /> : <AccessDenied role={role} onAccess={openAuth} />;
  else if (['rules', 'privacy', 'terms'].includes(route)) page = <LegalView kind={route} />;
  else page = <StreamerHome streamer={state.streamer} />;

  return (
    <div className="app-shell">
      <Sidebar route={route} navigate={navigate} role={role} unread={unread} actor={actor} />
      <div className="app-body">
        <Topbar role={role} search={search} setSearch={setSearch} navigate={navigate} openAuth={openAuth} onSignOut={signOut} unread={unread} actor={actor} />
        {page}
        <SiteFooter navigate={navigate} />
      </div>
      {authOpen && <AuthModal onContinue={localTwitchLogin} onClose={() => setAuthOpen(false)} />}
      {toast && <div className="toast" role="status" aria-live="polite"><Check size={15} /> {toast}</div>}
    </div>
  );
}

createRoot(document.getElementById('root')).render(<App />);
