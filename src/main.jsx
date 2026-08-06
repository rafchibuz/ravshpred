import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  ArrowUpRight,
  Bell,
  Check,
  ChevronDown,
  Clock3,
  ExternalLink,
  Eye,
  Filter,
  Home,
  LayoutGrid,
  Link2,
  List,
  Menu,
  Play,
  Plus,
  RotateCcw,
  Search,
  Settings,
  ShieldCheck,
  Sparkles,
  ThumbsDown,
  ThumbsUp,
  Trophy,
  UserRound,
  X,
} from 'lucide-react';
import {
  can,
  isDuplicate,
  parseYouTubeId,
  recentSubmissionCount,
  removeCategory,
  toggleVote,
  transitionSubmission,
  voteTotals,
} from './domain.js';
import * as api from './api.js';
import './styles.css';

const STORAGE_KEY = 'ravshann-predlozhka-local-v3';
const ROLE_LABELS = {
  guest: 'Гость',
  user: 'Пользователь',
  moderator: 'Модератор',
  owner: 'Основатель',
};
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
    settings: { dailyLimit: 3, commentLimit: 500, publicFeed: true },
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

function useProjectState(role, demoMode) {
  const [state, setState] = useState(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      return stored ? JSON.parse(stored) : makeInitialState();
    } catch {
      return makeInitialState();
    }
  });
  const [apiReady, setApiReady] = useState(false);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }, [state]);

  useEffect(() => {
    let active = true;
    api.loadWorkspace(role, demoMode ? role : undefined)
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
  }, [role, demoMode]);

  const reload = async () => {
    const workspace = await api.loadWorkspace(role, demoMode ? role : undefined);
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

function Avatar({ small = false }) {
  return <img className={`avatar ${small ? 'avatar-small' : ''}`} src="/assets/banner.png" alt="" />;
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
    { key: 'feed', label: 'Главная', icon: Home },
    { key: 'categories', label: 'Категории', icon: LayoutGrid },
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
          <Avatar small />
          <span>
            <strong>{role === 'guest' ? 'Без авторизации' : actor?.name || ROLE_LABELS[role]}</strong>
            <small>{ROLE_LABELS[role]}</small>
          </span>
        </div>
      </div>
    </aside>
  );
}

function Topbar({ role, search, setSearch, navigate, demoMode, openRoleSwitcher, openAuth, onSignOut, unread, actor }) {
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
        {demoMode ? (
          <button className="user-pill role-pill qa-role-pill" onClick={openRoleSwitcher} aria-label={`QA-роль: ${ROLE_LABELS[role]}`}>
            <ShieldCheck size={15} />
            <span>QA · {ROLE_LABELS[role]}</span>
            <ChevronDown size={13} />
          </button>
        ) : role === 'guest' ? (
          <button className="twitch-login" onClick={openAuth}>
            <Sparkles size={15} />
            <span className="desktop-login-label">Войти через Twitch</span>
            <span className="mobile-login-label">Twitch</span>
          </button>
        ) : (
          <>
            <button className="user-pill role-pill" onClick={() => navigate('profile')}>
              <Avatar small />
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

function FounderLogin({ onSuccess }) {
  const [secret, setSecret] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const user = await api.founderLogin(secret);
      onSuccess(user);
    } catch (loginError) {
      setError(loginError.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="main-content narrow">
      <form className="panel form-panel founder-login" onSubmit={submit}>
        <span className="panel-kicker">ЗАКРЫТЫЙ ВХОД</span>
        <h1>Доступ основателя</h1>
        <p>Этот адрес не показан в навигации. Сессия основателя не связана с Twitch-аккаунтом стримера.</p>
        <label>
          Ключ доступа
          <input type="password" autoComplete="current-password" value={secret} onChange={(event) => setSecret(event.target.value)} />
        </label>
        {error && <div className="form-error" role="alert">{error}</div>}
        <button className="primary-btn full" disabled={busy}>{busy ? 'Проверяем…' : 'Войти'}</button>
      </form>
    </main>
  );
}

function RoleSwitcher({ role, onSelect, onReset, onClose }) {
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section className="role-modal" role="dialog" aria-modal="true" aria-labelledby="role-title" onMouseDown={(event) => event.stopPropagation()}>
        <button className="modal-close icon-btn" onClick={onClose} aria-label="Закрыть">
          <X size={17} />
        </button>
        <span className="panel-kicker">ЛОКАЛЬНАЯ ПРОВЕРКА</span>
        <h2 id="role-title">Выберите роль</h2>
        <p>Можно пройти все сценарии из ТЗ без реального Twitch-входа.</p>
        <div className="role-options">
          {Object.entries(ROLE_LABELS).map(([key, label]) => (
            <button key={key} className={role === key ? 'selected' : ''} onClick={() => onSelect(key)}>
              <strong>{label}</strong>
              <span>
                {key === 'guest'
                  ? 'Лента и открытие роликов'
                  : key === 'user'
                    ? 'Отправка, реакции и профиль'
                    : key === 'moderator'
                      ? 'Очередь и решения'
                      : 'Роли, категории и аудит'}
              </span>
              {role === key && <Check size={16} />}
            </button>
          ))}
        </div>
        <button className="reset-button" onClick={onReset}>
          <RotateCcw size={15} /> Сбросить локальные данные
        </button>
      </section>
    </div>
  );
}

function Thumb({ video, large = false, publicCard = false }) {
  const status = publicCard ? 'approved' : video.status;
  return (
    <div className={`thumb ${large ? 'thumb-large' : ''}`} style={{ background: tones[video.tone] || tones.purple }}>
      <img
        className="youtube-thumb"
        src={video.thumbnailUrl || `https://i.ytimg.com/vi/${video.youtubeId}/hqdefault.jpg`}
        alt=""
        loading={large ? 'eager' : 'lazy'}
        onError={(event) => {
          event.currentTarget.hidden = true;
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

function VideoCard({ video, role, onVote, onOpen, list }) {
  const totals = voteTotals(video);
  const vote = video.userVote === 1 ? 'up' : video.userVote === -1 ? 'down' : null;
  return (
    <article className={`video-card ${list ? 'video-card-list' : ''}`}>
      <button className="card-link" onClick={() => onOpen(video)} aria-label={`Открыть «${video.title}» на YouTube`}>
        <Thumb video={video} publicCard />
      </button>
      <div className="card-body">
        <h3>{video.title}</h3>
        <div className="author-row">
          <Avatar small />
          <span>{video.author}</span>
          <em>·</em>
          <span>{new Date(video.createdAt).toLocaleDateString('ru-RU')}</span>
        </div>
        <div className="channel">YouTube: {video.channel}</div>
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
        {!can(role, 'vote') && <div className="login-hint">Войдите через Twitch, чтобы голосовать</div>}
      </div>
    </article>
  );
}

function Feed({ videos, categories, role, search, onVote, onOpen, navigate, onLogin }) {
  const [category, setCategory] = useState('Все');
  const [watchedOnly, setWatchedOnly] = useState(false);
  const [sort, setSort] = useState('Популярности');
  const [list, setList] = useState(false);
  const approved = videos.filter((video) => video.status === 'approved');
  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return approved
      .filter(
        (video) =>
          (category === 'Все' || video.category === category) &&
          (!watchedOnly || video.watched) &&
          [video.title, video.author, video.category, video.channel].join(' ').toLowerCase().includes(query),
      )
      .sort((a, b) => {
        if (sort === 'Новые') return new Date(b.createdAt) - new Date(a.createdAt);
        if (sort === 'Рейтингу') return voteTotals(b).score - voteTotals(a).score;
        return voteTotals(b).score - voteTotals(a).score;
      });
  }, [approved, category, watchedOnly, sort, search]);

  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow">
            <span className="live-dot" /> ПУБЛИЧНАЯ ЛЕНТА
          </div>
          <h1>Предложка Равшана</h1>
          <p>Одобренные модерацией видео, готовые к просмотру на стриме</p>
        </div>
        <div className="heading-actions">
          <button className="ghost-btn" onClick={() => setList((value) => !value)} aria-pressed={list}>
            {list ? <LayoutGrid size={15} /> : <List size={15} />} {list ? 'Плитка' : 'Список'}
          </button>
          <select value={sort} onChange={(event) => setSort(event.target.value)} aria-label="Сортировка">
            <option>Популярности</option>
            <option>Новые</option>
            <option>Рейтингу</option>
          </select>
        </div>
      </section>
      <div className="chips">
        <div className="chip-scroll">
          {['Все', ...categories].map((item) => (
            <button key={item} className={category === item ? 'selected' : ''} onClick={() => setCategory(item)}>
              {item}
            </button>
          ))}
          <button className={watchedOnly ? 'selected watched-filter' : 'watched-filter'} onClick={() => setWatchedOnly((value) => !value)}>
            <Eye size={13} /> Отсмотрено
          </button>
        </div>
      </div>
      <div className="content-grid">
        <section className={`feed-grid ${list ? 'feed-list' : ''}`}>
          {filtered.map((video) => (
            <VideoCard key={video.id} video={video} role={role} onVote={onVote} onOpen={onOpen} list={list} />
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
                <div className="rank-row" key={video.id}>
                  <b>{index + 1}</b>
                  <span>{video.title}</span>
                  <strong>{voteTotals(video).score > 0 ? '+' : ''}{voteTotals(video).score}</strong>
                </div>
              ))}
          </div>
          <div className="side-card stream-card">
            <span className="eyebrow">РЕЖИМ ДЛЯ СТРИМА</span>
            <h3>Минимум лишнего</h3>
            <p>В ленте только одобренные ролики. «Отсмотрено» не скрывает видео и не отключает реакции.</p>
          </div>
        </aside>
      </div>
    </main>
  );
}

function CategoriesView({ videos, categories, navigate }) {
  const approved = videos.filter((video) => video.status === 'approved');
  return (
    <main className="main-content">
      <section className="page-heading">
        <div>
          <div className="eyebrow">КАТАЛОГ</div>
          <h1>Категории</h1>
          <p>Быстрый переход к одобренным видео по темам</p>
        </div>
      </section>
      <section className="category-cards">
        {categories.map((category, index) => {
          const items = approved.filter((video) => video.category === category);
          return (
            <button key={category} className={`category-card tone-${index % 4}`} onClick={() => navigate('feed')}>
              <span>{String(items.length).padStart(2, '0')} видео</span>
              <strong>{category}</strong>
              <small>Открыть ленту <ArrowUpRight size={14} /></small>
            </button>
          );
        })}
      </section>
    </main>
  );
}

function SubmitView({ state, actor, navigate, notify, onSubmit }) {
  const [url, setUrl] = useState('');
  const [category, setCategory] = useState('');
  const [comment, setComment] = useState('');
  const [error, setError] = useState('');
  const [sentId, setSentId] = useState(null);
  const count = recentSubmissionCount(state.videos, actor.id);
  const youtubeId = parseYouTubeId(url);

  const [submitting, setSubmitting] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setError('');
    if (!youtubeId) return setError('Проверьте ссылку: нужен корректный URL YouTube.');
    if (!category) return setError('Выберите категорию.');
    if (isDuplicate(state.videos, youtubeId)) return setError('Это видео уже есть в предложке.');
    if (count >= state.settings.dailyLimit) return setError(`Достигнут лимит: ${state.settings.dailyLimit} отправки за 24 часа.`);
    setSubmitting(true);
    try {
      const video = await onSubmit({ url, category, comment: comment.trim() });
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
            Ссылка на видео
            <input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://www.youtube.com/watch?v=..." inputMode="url" />
          </label>
          <label>
            Категория
            <select value={category} onChange={(event) => setCategory(event.target.value)}>
              <option value="">Выберите категорию</option>
              {state.categories.map((item) => <option key={item}>{item}</option>)}
            </select>
          </label>
          <label>
            Комментарий модератору <span>необязательно</span>
            <textarea
              maxLength={state.settings.commentLimit}
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="Почему это видео стоит посмотреть на стриме?"
            />
          </label>
          <div className="counter">{comment.length}/{state.settings.commentLimit}</div>
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
        <Avatar />
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

function ModerationView({ state, role, onDecision, onWatched, onDelete, onCategoryChange, notify }) {
  const [tab, setTab] = useState('pending');
  const items = state.videos.filter((video) => video.status === tab);
  const [selectedId, setSelectedId] = useState(items[0]?.id);
  const [comment, setComment] = useState('');
  const [deleteArmed, setDeleteArmed] = useState(false);
  const selected = state.videos.find((video) => video.id === selectedId && video.status === tab) || items[0];

  useEffect(() => {
    if (!selected && items[0]) setSelectedId(items[0].id);
  }, [selected, items]);
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
              <Avatar small />
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
    </main>
  );
}

function AccessDenied({ role, demoMode, onAccess }) {
  return (
    <main className="main-content narrow">
      <div className="success-panel access-panel">
        <ShieldCheck size={38} />
        <h2>Этот раздел недоступен роли «{ROLE_LABELS[role]}»</h2>
        <p>{demoMode ? 'Переключите локальную QA-роль, чтобы пройти защищённый сценарий.' : 'Войдите через Twitch. Если вам выданы права модератора, раздел появится автоматически.'}</p>
        <button className="primary-btn" onClick={onAccess}>{demoMode ? 'Выбрать QA-роль' : 'Войти через Twitch'}</button>
      </div>
    </main>
  );
}

function App() {
  const query = new URLSearchParams(window.location.search);
  const queryRole = query.get('role');
  const demoMode = (import.meta.env.DEV || import.meta.env.VITE_QA_MODE === '1') && (query.get('demo') === '1' || Boolean(queryRole));
  const initialRole = demoMode && ROLE_LABELS[queryRole] ? queryRole : 'guest';
  const defaultRoute = initialRole === 'owner' ? 'owner' : initialRole === 'moderator' ? 'moderation' : 'feed';
  const [route, navigate] = useRoute(defaultRoute);
  const [role, setRole] = useState(initialRole);
  const [sessionUser, setSessionUser] = useState(null);
  const [state, setState, apiReady, reload] = useProjectState(role, demoMode);
  const [search, setSearch] = useState('');
  const [roleOpen, setRoleOpen] = useState(false);
  const [authOpen, setAuthOpen] = useState(false);
  const [toast, setToast] = useState('');
  const actor = sessionUser
    ? { id: sessionUser.id, name: sessionUser.display_name || sessionUser.login }
    : role === 'owner'
      ? { id: 'owner', name: 'Ravshann' }
      : role === 'moderator'
        ? { id: 'moderator', name: 'moderator_live' }
        : { id: 'viewer', name: 'Ravshibiscus' };
  const unread = state.notifications.filter((notice) => !notice.read).length;

  useEffect(() => {
    if (demoMode) return;
    api.currentUser()
      .then((user) => {
        setSessionUser(user);
        setRole(user?.role || 'guest');
      })
      .catch(() => {
        setSessionUser(null);
        setRole('guest');
      });
  }, [demoMode]);

  const notify = (message) => {
    setToast(message);
    window.clearTimeout(window.__ravshannToast);
    window.__ravshannToast = window.setTimeout(() => setToast(''), 2800);
  };
  const selectRole = (nextRole) => {
    setRole(nextRole);
    const url = new URL(window.location.href);
    url.searchParams.set('demo', '1');
    if (nextRole === 'guest') url.searchParams.delete('role');
    else url.searchParams.set('role', nextRole);
    window.history.replaceState({}, '', url);
    setRoleOpen(false);
    if (nextRole === 'owner') navigate('owner');
    else if (nextRole === 'moderator') navigate('moderation');
    else navigate('feed');
    notify(`Режим: ${ROLE_LABELS[nextRole]}`);
  };
  const apiRole = demoMode ? role : undefined;
  const vote = async (id, direction) => {
    if (!can(role, 'vote')) return demoMode ? setRoleOpen(true) : setAuthOpen(true);
    if (apiReady) {
      try {
        const result = await api.vote(apiRole, id, direction === 'up' ? 1 : -1);
        setState((current) => ({
          ...current,
          videos: current.videos.map((video) => video.id === id
            ? { ...video, rating: result.rating, userVote: result.user_vote }
            : video),
        }));
      } catch (error) {
        notify(error.message);
      }
      return;
    }
    setState((current) => ({
      ...current,
      videos: current.videos.map((video) => (video.id === id ? toggleVote(video, actor.id, direction) : video)),
    }));
  };
  const decide = async (id, status, comment) => {
    if (apiReady) {
      const currentVideo = state.videos.find((item) => item.id === id);
      try {
        const updated = await api.decide(apiRole, id, status, comment, currentVideo.version);
        setState((current) => ({
          ...current,
          videos: current.videos.map((item) => item.id === id ? updated : item),
        }));
      } catch (error) {
        notify(error.message);
        await reload().catch(() => {});
      }
      return;
    }
    setState((current) => {
      const video = current.videos.find((item) => item.id === id);
      const updated = transitionSubmission(video, status, role);
      const title = status === 'approved' ? 'Ваше видео одобрено' : status === 'rejected' ? 'Видео отклонено' : 'Видео возвращено на рассмотрение';
      const body =
        status === 'approved'
          ? `«${video.title}» появилось в общей ленте.`
          : status === 'rejected'
            ? comment
              ? `«${video.title}»: ${comment}`
              : `«${video.title}» отклонено без комментария.`
            : `«${video.title}» снова ожидает решения.`;
      const verb = status === 'approved' ? 'одобрил' : status === 'rejected' ? 'отклонил' : 'вернул на рассмотрение';
      return {
        ...current,
        videos: current.videos.map((item) => (item.id === id ? { ...updated, moderatorComment: comment || (status === 'rejected' ? 'Без комментария' : '') } : item)),
        notifications:
          video.authorId === 'viewer' && status !== 'pending'
            ? [{ id: Date.now(), title, body, createdAt: new Date().toISOString(), read: false, tone: status === 'approved' ? 'green' : 'red' }, ...current.notifications]
            : current.notifications,
        audit: [`${actor.name} ${verb} видео #${id}`, ...current.audit],
      };
    });
  };
  const toggleWatched = async (id) => {
    const currentVideo = state.videos.find((item) => item.id === id);
    if (apiReady && currentVideo) {
      try {
        await api.setWatched(apiRole, id, !currentVideo.watched);
        setState((current) => ({
          ...current,
          videos: current.videos.map((video) => video.id === id ? { ...video, watched: !video.watched } : video),
        }));
        notify('Публичная отметка обновлена');
      } catch (error) {
        notify(error.message);
      }
      return;
    }
    setState((current) => ({
      ...current,
      videos: current.videos.map((video) => (video.id === id && video.status === 'approved' ? { ...video, watched: !video.watched } : video)),
      audit: [`${actor.name} изменил отметку «Отсмотрено» у видео #${id}`, ...current.audit],
    }));
    notify('Публичная отметка обновлена');
  };
  const openVideo = (video) => window.open(video.youtubeUrl, '_blank', 'noopener,noreferrer');
  const openRoleSwitcher = () => setRoleOpen(true);
  const openAuth = () => setAuthOpen(true);
  const localTwitchLogin = () => {
    window.location.assign('/api/auth/twitch/start?return_to=/');
  };
  const signOut = async () => {
    if (!demoMode) await api.logout().catch(() => {});
    setSessionUser(null);
    setRole('guest');
    navigate('feed');
    notify('Вы вышли из аккаунта');
  };
  const deleteVideo = async (id) => {
    if (apiReady) {
      try {
        await api.deleteVideo(apiRole, id);
        setState((current) => ({
          ...current,
          videos: current.videos.filter((video) => video.id !== id),
        }));
      } catch (error) {
        notify(error.message);
      }
      return;
    }
    setState((current) => ({
      ...current,
      videos: current.videos.filter((video) => video.id !== id),
      audit: [`${actor.name} удалил видео #${id}`, ...current.audit],
    }));
  };
  const changeVideoCategory = async (id, category) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === category);
    if (apiReady && categoryRecord) {
      try {
        await api.setVideoCategory(apiRole, id, categoryRecord.id);
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
      return;
    }
    setState((current) => ({
      ...current,
      videos: current.videos.map((video) => video.id === id ? { ...video, category } : video),
      audit: [`${actor.name} изменил категорию видео #${id} на «${category}»`, ...current.audit],
    }));
    notify('Категория видео обновлена');
  };
  const submitVideo = async ({ url, category, comment }) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === category);
    if (!apiReady || !categoryRecord) throw new Error('API пока недоступен — повторите через несколько секунд');
    const video = await api.createSubmission(apiRole, {
      url,
      category_id: categoryRecord.id,
      comment,
    });
    setState((current) => ({ ...current, videos: [video, ...current.videos] }));
    return video;
  };
  const addCategory = async (name) => {
    if (!apiReady) throw new Error('API пока недоступен');
    const categoryRecord = await api.createCategory(apiRole, name);
    setState((current) => ({
      ...current,
      categoryRecords: [...(current.categoryRecords || []), categoryRecord],
      categories: [...current.categories, categoryRecord.name],
    }));
  };
  const deleteCategory = async (name) => {
    const categoryRecord = state.categoryRecords?.find((item) => item.name === name);
    if (!apiReady || !categoryRecord) throw new Error('Категория не найдена');
    await api.deleteCategory(apiRole, categoryRecord.id);
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
    const user = await api.assignModerator(apiRole, login);
    setState((current) => ({
      ...current,
      moderatorRecords: [...(current.moderatorRecords || []), user],
      moderators: [...current.moderators.filter((item) => item !== login), user.login || user.display_name],
    }));
  };
  const deleteModerator = async (login) => {
    const user = state.moderatorRecords?.find((item) => (item.login || item.display_name) === login);
    if (!user) throw new Error('Сначала пользователь должен войти через Twitch');
    await api.removeModerator(apiRole, user.id);
    setState((current) => ({
      ...current,
      moderatorRecords: current.moderatorRecords.filter((item) => item.id !== user.id),
      moderators: current.moderators.filter((item) => item !== login),
    }));
  };
  const updateSettings = async (settings) => {
    if (!apiReady) throw new Error('API пока недоступен');
    await api.updateSettings(apiRole, settings);
    setState((current) => ({ ...current, settings }));
  };
  const markNotification = async (id) => {
    if (apiReady) await api.readNotification(apiRole, id);
    setState((current) => ({
      ...current,
      notifications: current.notifications.map((notice) => notice.id === id ? { ...notice, read: true } : notice),
    }));
  };
  const markAllNotifications = async () => {
    if (apiReady) await api.readAllNotifications(apiRole);
    setState((current) => ({
      ...current,
      notifications: current.notifications.map((notice) => ({ ...notice, read: true })),
    }));
  };
  const accessAction = demoMode ? openRoleSwitcher : openAuth;

  let page;
  if (route === 'founder-access' && !demoMode) page = <FounderLogin onSuccess={(user) => { setSessionUser(user); setRole('owner'); navigate('owner'); notify('Закрытая сессия основателя открыта'); }} />;
  else if (route === 'feed') page = <Feed videos={state.videos} categories={state.categories} role={role} search={search} onVote={vote} onOpen={openVideo} navigate={navigate} onLogin={accessAction} />;
  else if (route === 'categories') page = <CategoriesView videos={state.videos} categories={state.categories} navigate={navigate} />;
  else if (route === 'submit') page = can(role, 'submit') ? <SubmitView state={state} actor={actor} navigate={navigate} notify={notify} onSubmit={submitVideo} /> : <AccessDenied role={role} demoMode={demoMode} onAccess={accessAction} />;
  else if (route === 'profile') page = can(role, 'view_profile') ? <ProfileView videos={state.videos} actor={actor} navigate={navigate} /> : <AccessDenied role={role} demoMode={demoMode} onAccess={accessAction} />;
  else if (route === 'notifications') page = can(role, 'view_profile') ? <NotificationView notifications={state.notifications} markAllRead={markAllNotifications} markRead={markNotification} /> : <AccessDenied role={role} demoMode={demoMode} onAccess={accessAction} />;
  else if (route === 'moderation') page = can(role, 'moderate') ? <ModerationView state={state} role={role} onDecision={decide} onWatched={toggleWatched} onDelete={deleteVideo} onCategoryChange={changeVideoCategory} notify={notify} /> : <AccessDenied role={role} demoMode={demoMode} onAccess={accessAction} />;
  else if (route === 'owner') page = can(role, 'manage') ? <OwnerView state={state} setState={setState} notify={notify} onAddCategory={apiReady ? addCategory : null} onDeleteCategory={apiReady ? deleteCategory : null} onAddModerator={apiReady ? addModerator : null} onDeleteModerator={apiReady ? deleteModerator : null} onUpdateSettings={apiReady ? updateSettings : null} /> : <AccessDenied role={role} demoMode={demoMode} onAccess={accessAction} />;
  else page = <Feed videos={state.videos} categories={state.categories} role={role} search={search} onVote={vote} onOpen={openVideo} navigate={navigate} onLogin={accessAction} />;

  return (
    <div className="app-shell">
      <Sidebar route={route} navigate={navigate} role={role} unread={unread} actor={actor} />
      <div className="app-body">
        <Topbar role={role} search={search} setSearch={setSearch} navigate={navigate} demoMode={demoMode} openRoleSwitcher={openRoleSwitcher} openAuth={openAuth} onSignOut={signOut} unread={unread} actor={actor} />
        {page}
      </div>
      {demoMode && roleOpen && (
        <RoleSwitcher
          role={role}
          onSelect={selectRole}
          onReset={() => {
            setState(makeInitialState());
            notify('Локальные данные сброшены');
            setRoleOpen(false);
          }}
          onClose={() => setRoleOpen(false)}
        />
      )}
      {authOpen && <AuthModal onContinue={localTwitchLogin} onClose={() => setAuthOpen(false)} />}
      {toast && <div className="toast" role="status" aria-live="polite"><Check size={15} /> {toast}</div>}
    </div>
  );
}

createRoot(document.getElementById('root')).render(<App />);
