import React, { useMemo, useState } from 'react';
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
  Film,
  Home,
  LayoutGrid,
  Link2,
  ListFilter,
  Menu,
  MoreVertical,
  Play,
  Plus,
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
import './styles.css';

const categories = ['Все', 'Смешное', 'Трейлеры', 'Фильмы и сериалы', 'Разоблачения'];

const initialVideos = [
  { id: 1, title: 'Неожиданный финал GTA 6', channel: 'Rockstar Games', author: 'renatko', category: 'Трейлеры', ago: '12 мин назад', duration: '10:36', views: '12,3K', likes: 342, dislikes: 18, watched: true, tone: 'pink', tag: 'НОВИНКА' },
  { id: 2, title: 'Самый смешной момент', channel: 'Funny Streams', author: 'game_over_90', category: 'Смешное', ago: '34 мин назад', duration: '02:35', views: '8,7K', likes: 198, dislikes: 12, watched: false, tone: 'blue', tag: 'В ТРЕНДЕ' },
  { id: 3, title: 'Разоблачение года', channel: 'Truth Channel', author: 'truth_seeker', category: 'Разоблачения', ago: '1 ч назад', duration: '28:42', views: '6,1K', likes: 156, dislikes: 25, watched: false, tone: 'red', tag: 'ПРЕМЬЕРА' },
  { id: 4, title: 'Трейлер, который стоит посмотреть', channel: 'Movie Clips', author: 'kino_maniak', category: 'Трейлеры', ago: '2 ч назад', duration: '05:47', views: '3,2K', likes: 89, dislikes: 4, watched: false, tone: 'blue', tag: '' },
  { id: 5, title: 'Лучший камбэк', channel: 'Warrior', author: 'warrior_tv', category: 'Смешное', ago: '3 ч назад', duration: '10:21', views: '5,2K', likes: 170, dislikes: 9, watched: true, tone: 'amber', tag: '' },
  { id: 6, title: 'Это нужно увидеть', channel: 'Memes Daily', author: 'kotjara_', category: 'Смешное', ago: '4 ч назад', duration: '01:05', views: '4,5K', likes: 128, dislikes: 7, watched: false, tone: 'purple', tag: '' },
];

const thumbs = {
  pink: 'linear-gradient(135deg,#743584 0%,#d64e7f 100%)',
  blue: 'linear-gradient(135deg,#304981 0%,#637cc7 100%)',
  red: 'linear-gradient(135deg,#7d314f 0%,#dc6575 100%)',
  amber: 'linear-gradient(135deg,#89614e 0%,#d9a06c 100%)',
  purple: 'linear-gradient(135deg,#512883 0%,#9c58c8 100%)',
};

function Avatar({ small = false }) {
  return <img className={`avatar ${small ? 'avatar-small' : ''}`} src="/assets/banner.png" alt="Ravshann" />;
}

function Logo() {
  return <div className="logo"><span>RAVSHANN</span><small>ПРЕДЛОЖКА</small></div>;
}

function Sidebar({ view, setView, signedIn, role }) {
  const nav = [
    { key: 'feed', label: 'Главная', icon: Home },
    { key: 'categories', label: 'Категории', icon: LayoutGrid },
    { key: 'submit', label: 'Предложить видео', icon: Plus },
    { key: 'notifications', label: 'Уведомления', icon: Bell, count: 3 },
    { key: 'profile', label: 'Профиль', icon: UserRound },
  ];
  if (role === 'moderator' || role === 'owner') nav.push({ key: 'moderation', label: 'Модерация', icon: ShieldCheck });
  if (role === 'owner') nav.push({ key: 'owner', label: 'Управление', icon: Settings });
  return <aside className="sidebar">
    <div className="sidebar-top"><Logo /><button className="icon-btn mobile-menu"><Menu size={19} /></button></div>
    <div className="role-label">{role === 'owner' ? 'ОСНОВАТЕЛЬ' : role === 'moderator' ? 'МОДЕРАТОР' : signedIn ? 'ПОЛЬЗОВАТЕЛЬ' : 'ГОСТЬ'}</div>
    <nav className="side-nav">{nav.map(({ key, label, icon: Icon, count }) => <button key={key} className={view === key ? 'active' : ''} onClick={() => setView(key)}><Icon size={16} /><span>{label}</span>{count && <b>{count}</b>}</button>)}</nav>
    <div className="side-bottom">
      {signedIn ? <button className="account-card" onClick={() => setView('profile')}><Avatar small /><span><strong>Ravshibiscus</strong><small>{role === 'owner' ? 'Основатель' : role === 'moderator' ? 'Модератор' : 'Пользователь'}</small></span><ChevronDown size={14} /></button> : <button className="twitch-btn" onClick={() => window.dispatchEvent(new Event('login'))}><Sparkles size={15} /> Войти через Twitch</button>}
    </div>
  </aside>;
}

function Topbar({ signedIn, setSignedIn, setView, search, setSearch }) {
  return <header className="topbar">
    <div className="search-wrap"><Search size={16} /><input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Поиск по видео, автору или категории..." /></div>
    <div className="top-actions"><button className="outline-btn hide-mobile" onClick={() => setView('submit')}><Plus size={15} /> Предложить видео</button><button className="icon-btn" aria-label="Уведомления" onClick={() => setView('notifications')}><Bell size={17} />{signedIn && <i />}</button>{signedIn ? <button className="user-pill" onClick={() => setView('profile')}><Avatar small /><span>Ravshibiscus</span><ChevronDown size={13} /></button> : <button className="outline-btn login-top" onClick={() => setSignedIn(true)}><Sparkles size={14} /> Войти через Twitch</button>}</div>
  </header>;
}

function Thumb({ video, large = false }) {
  return <div className={`thumb ${large ? 'thumb-large' : ''}`} style={{ background: thumbs[video.tone] }}><div className="orb orb-a" /><div className="orb orb-b" /><div className="thumb-top"><span className="mini-tag">{video.tag || 'ОДОБРЕНО'}</span>{video.watched && <span className="watched-chip"><Eye size={11} /> ОТСМОТРЕНО</span>}</div><div className="thumb-title">{video.title}</div><span className="duration">{video.duration}</span>{large && <span className="play"><Play size={23} fill="white" /></span>}</div>;
}

function VideoCard({ video, signedIn, onVote, onOpen }) {
  return <article className="video-card">
    <button className="card-link" onClick={() => onOpen(video)} aria-label={`Открыть ${video.title}`}><Thumb video={video} /></button>
    <div className="card-body"><h3>{video.title}</h3><div className="author-row"><Avatar small /><span>{video.author}</span><em>·</em><span>{video.ago}</span></div><div className="channel">YouTube: {video.channel}</div><div className="metrics"><span><Eye size={12} /> {video.views}</span><button className={video.userVote === 'up' ? 'voted' : ''} onClick={() => onVote(video.id, 'up')}><ThumbsUp size={12} /> {video.likes}</button><button className={video.userVote === 'down' ? 'voted down' : ''} onClick={() => onVote(video.id, 'down')}><ThumbsDown size={12} /> {video.dislikes}</button><button className="more"><MoreVertical size={14} /></button></div>{!signedIn && <button className="login-hint" onClick={() => window.dispatchEvent(new Event('login'))}>Войдите, чтобы голосовать</button>}</div>
  </article>;
}

function Feed({ videos, signedIn, setView, search, setSearch, onVote, onOpen }) {
  const [category, setCategory] = useState('Все');
  const [watchedOnly, setWatchedOnly] = useState(false);
  const [sort, setSort] = useState('Популярности');
  const filtered = useMemo(() => videos.filter((v) => (category === 'Все' || v.category === category) && (!watchedOnly || v.watched) && [v.title, v.author, v.category, v.channel].join(' ').toLowerCase().includes(search.toLowerCase())).sort((a, b) => sort === 'Новые' ? b.id - a.id : sort === 'Лайкам' ? b.likes - a.likes : b.likes + b.views.localeCompare(a.views)), [videos, category, watchedOnly, sort, search]);
  return <main className="main-content">
    <section className="page-heading"><div><div className="eyebrow"><span className="live-dot" /> ПУБЛИЧНАЯ ЛЕНТА</div><h1>Предложка Равшана</h1><p>Одобренные модерацией видео, готовые к просмотру на стриме</p></div><div className="heading-actions"><button className="ghost-btn"><ListFilter size={15} /> Вид</button><select value={sort} onChange={(e) => setSort(e.target.value)} aria-label="Сортировка"><option>Популярности</option><option>Новые</option><option>Лайкам</option></select></div></section>
    <div className="chips"><div className="chip-scroll">{categories.map((c) => <button key={c} className={category === c ? 'selected' : ''} onClick={() => setCategory(c)}>{c}</button>)}<button className={watchedOnly ? 'selected watched-filter' : 'watched-filter'} onClick={() => setWatchedOnly((v) => !v)}><Eye size={13} /> Отсмотрено</button></div></div>
    <div className="content-grid"><section className="feed-grid">{filtered.map((video) => <VideoCard key={video.id} video={video} signedIn={signedIn} onVote={onVote} onOpen={onOpen} />)}{!filtered.length && <div className="empty-state"><Search size={28} /><h3>Ничего не нашли</h3><p>Попробуйте другой запрос или категорию</p></div>}</section><aside className="feed-aside"><div className="promo-card"><div className="promo-image" /><div className="promo-content"><span className="eyebrow">КАК ЭТО РАБОТАЕТ</span><h3>Предлагайте — Равшann смотрит</h3><p>Зрители отправляют YouTube-ссылки. Модераторы проверяют содержание и допускают безопасные ролики в ленту.</p><button className="primary-btn" onClick={() => setSignedIn(true)}>Войти через Twitch <ArrowUpRight size={15} /></button></div></div><div className="side-card"><div className="side-card-head"><h3><Trophy size={16} /> Топ недели</h3><span>обновлено сейчас</span></div>{videos.slice(0, 5).map((v, i) => <div className="rank-row" key={v.id}><b>{i + 1}</b><span>{v.title}</span><strong>♥ {v.likes}</strong></div>)}</div><div className="side-card stream-card"><span className="eyebrow">РЕЖИМ ДЛЯ СТРИМА</span><h3>Минимум лишнего</h3><p>Главная страница показывает только одобренные ролики. Нажатие на карточку откроет оригинал на YouTube.</p></div></aside></div>
  </main>;
}

function SubmitView({ setView }) {
  const [url, setUrl] = useState(''); const [category, setCategory] = useState(''); const [comment, setComment] = useState(''); const [sent, setSent] = useState(false);
  return <main className="main-content narrow"><section className="page-heading"><div><div className="eyebrow">НОВАЯ ОТПРАВКА</div><h1>Предложить видео</h1><p>Поделитесь роликом, который стоит посмотреть на стриме</p></div><button className="ghost-btn" onClick={() => setView('feed')}>Назад к ленте</button></section>{sent ? <div className="success-panel"><div className="success-icon"><Check /></div><h2>Отправка принята</h2><p>Видео добавлено в очередь модерации. Мы покажем решение в уведомлениях.</p><button className="primary-btn" onClick={() => setView('profile')}>Открыть профиль <ArrowUpRight size={15} /></button></div> : <div className="submit-layout"><section className="panel form-panel"><div className="panel-head"><div><span className="panel-kicker">ШАГ 1 ИЗ 2</span><h2>Ссылка на YouTube</h2></div><span className="limit"><Clock3 size={14} /> 2 из 3 сегодня</span></div><label>Ссылка на видео<input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://www.youtube.com/watch?v=..." /></label><label>Категория<select value={category} onChange={(e) => setCategory(e.target.value)}><option value="">Выберите категорию</option>{categories.slice(1).map((c) => <option key={c}>{c}</option>)}</select></label><label>Комментарий модератору <span>необязательно</span><textarea maxLength={500} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Почему это видео стоит посмотреть на стриме?" /></label><div className="counter">{comment.length}/500</div><button className="primary-btn full" disabled={!url || !category} onClick={() => setSent(true)}>Проверить и отправить <ArrowUpRight size={15} /></button><p className="form-note"><ShieldCheck size={14} /> Мы не храним видеофайлы — только ссылку и метаданные.</p></section><section className="preview-panel panel"><div className="panel-head"><div><span className="panel-kicker">ПРЕДПРОСМОТР</span><h2>Так будет выглядеть карточка</h2></div></div><div className="preview-placeholder">{url ? <><Link2 size={24} /><strong>Метаданные YouTube появятся здесь</strong><span>После проверки ссылки</span></> : <><Play size={24} /><strong>Вставьте ссылку на видео</strong><span>Название, канал и превью загрузятся автоматически</span></>}</div><div className="rules"><h3>Перед отправкой</h3><p><Check size={14} /> Видео должно быть доступно на YouTube</p><p><Check size={14} /> Не отправляйте дубликаты и рекламу</p><p><Check size={14} /> Максимум 3 видео за 24 часа</p></div></section></div>}</main>;
}

function ProfileView({ setView }) {
  return <main className="main-content"><section className="profile-hero"><Avatar /><div><div className="eyebrow">ВАШ ПРОФИЛЬ</div><h1>Ravshibiscus</h1><p>Зритель с марта 2023 · 3 отправки за последние 30 дней</p></div><button className="ghost-btn" onClick={() => setView('submit')}><Plus size={15} /> Предложить видео</button></section><div className="profile-grid"><section className="panel submissions"><div className="panel-head"><div><span className="panel-kicker">ИСТОРИЯ</span><h2>Мои видео</h2></div><button className="filter-button"><Filter size={14} /> Все статусы <ChevronDown size={13} /></button></div><div className="tabs"><button className="selected">Все <span>3</span></button><button>На рассмотрении</button><button>Одобрено</button><button>Отклонено</button></div>{initialVideos.slice(0, 3).map((v, i) => <div className="submission-row" key={v.id}><Thumb video={v} /><div className="submission-info"><strong>{v.title}</strong><span>{v.category} · {i === 0 ? 'Сегодня, 14:32' : i === 1 ? 'Вчера, 20:15' : '3 авг., 18:45'}</span></div><span className={`status ${i === 0 ? 'approved' : i === 1 ? 'pending' : 'rejected'}`}>{i === 0 ? 'Одобрено' : i === 1 ? 'На рассмотрении' : 'Отклонено'}{i === 0 && v.watched && <small>Отсмотрено</small>}</span><span className="decision">{i === 0 ? 'Отличный момент' : i === 1 ? 'Ожидает модератора' : 'Реклама и ссылки'}</span></div>)}</section><section className="panel notifications"><div className="panel-head"><div><span className="panel-kicker">АКТИВНОСТЬ</span><h2>Последние уведомления</h2></div><span className="unread">3 новых</span></div>{['Ваше видео одобрено','Видео отклонено','Новый лайк'].map((n, i) => <div className="notice-row" key={n}><i className={i === 1 ? 'red' : ''} /><div><strong>{n}</strong><p>{i === 0 ? '«Смешная нарезка со стрима» появилась в общей ленте.' : i === 1 ? 'Нажмите, чтобы посмотреть причину модератора.' : 'Ваше видео набрало 100 лайков.'}</p></div><span>{i === 0 ? '10 мин' : 'вчера'}</span></div>)}</section></div></main>;
}

function ModerationView({ owner = false }) {
  const [selected, setSelected] = useState(initialVideos[0]); const [watched, setWatched] = useState(selected.watched); return <main className="main-content"><section className="page-heading"><div><div className="eyebrow"><span className="live-dot orange" /> ЗАЩИЩЁННЫЙ РАЗДЕЛ</div><h1>{owner ? 'Управление предложкой' : 'Модерация видео'}</h1><p>{owner ? 'Роли, категории, журнал действий и глобальные ограничения' : 'Проверка отправленных ссылок перед публикацией в общей ленте'}</p></div><div className="heading-actions"><button className="outline-btn">Экспорт CSV <ArrowUpRight size={14} /></button></div></section>{owner ? <OwnerDashboard /> : <div className="moderation-layout"><section className="panel queue"><div className="queue-tabs"><button className="selected">На рассмотрении <span>15</span></button><button>Одобрено</button><button>Отклонено</button></div>{initialVideos.map((v) => <button className={`queue-item ${selected.id === v.id ? 'selected' : ''}`} key={v.id} onClick={() => { setSelected(v); setWatched(v.watched); }}><Thumb video={v} /><div><strong>{v.title}</strong><span>{v.author} · {v.ago}</span></div><span className="queue-status">НА ПРОВЕРКЕ</span></button>)}</section><section className="panel review"><div className="review-head"><div><span className="panel-kicker">ID #{selected.id} · сегодня, 14:32</span><h2>{selected.title}</h2></div><span className="status pending">НА МОДЕРАЦИИ</span></div><Thumb video={selected} large /><div className="review-copy"><span className="panel-kicker">КОММЕНТАРИЙ ПОЛЬЗОВАТЕЛЯ</span><p>«Посмотрим этот трейлер в начале стрима, там много новых деталей. Думаю, чат оценит»</p><div className="meta-grid"><span>Категория <b>{selected.category}</b></span><span>Канал <b>{selected.channel} · {selected.views} просмотров</b></span><span>Отправитель <b>{selected.author} · 8 отправок</b></span></div></div><div className="review-actions"><button className="approve"><Check size={15} /> Одобрить</button><button className="reject"><X size={15} /> Отклонить</button><button className="outline-btn">YouTube <ExternalLink size={14} /></button></div><button className={`watched-toggle ${watched ? 'is-on' : ''}`} onClick={() => setWatched((v) => !v)}><Eye size={14} /> {watched ? 'Отсмотрено' : 'Отметить как отсмотренное'}</button><span className="public-note">Плашку увидят все</span></section><aside className="review-aside"><div className="side-card stats"><h3>Сегодня</h3><p>Проверено <b>58</b></p><p>Одобрено <b className="green">46</b></p><p>Отклонено <b className="red-text">12</b></p><p>В очереди <b className="purple-text">15</b></p></div><div className="side-card"><h3>Отправитель</h3><div className="sender"><Avatar small /><strong>{selected.author}</strong></div><p>6 из 8 видео одобрено</p><p>Предупреждения: 0. Отклонено ранее: 2.</p></div><div className="side-card check-list"><h3>Проверка</h3><p>✓ Ссылка YouTube</p><p>✓ Видео доступно</p><p>✓ Дубликат не найден</p><p>✓ Длительность допустима</p><p className="warn">! Содержание проверить вручную</p></div></aside></div>}</main>;
}

function OwnerDashboard() { return <div className="owner-dashboard"><div className="stat-strip"><div><b className="purple-text">15</b><span>Видео ждут проверки</span></div><div><b className="green">256</b><span>Видео в публичной ленте</span></div><div><b className="amber-text">3</b><span>Активных модератора</span></div></div><div className="owner-columns"><section className="panel owner-panel"><div className="panel-head"><h2>Модераторы</h2><span className="panel-kicker">3 АКТИВНЫХ</span></div><div className="add-row"><input placeholder="Введите точный Twitch-ник" /><button className="primary-btn">Добавить модератора</button></div>{['moderator_live', 'lexapro_tv', 'shadowmff'].map((m) => <div className="moderator-row" key={m}><Avatar small /><strong>{m}</strong><span>Добавлен 12.07.2026</span><b>Модератор</b><button className="danger-btn">Удалить</button></div>)}</section><section className="panel owner-panel"><div className="panel-head"><h2>Журнал действий</h2><button className="ghost-btn">Смотреть всё</button></div>{['moderator_live одобрил видео #12451','lexapro_tv отклонил видео #12449','Ravshibiscus добавил shadowmff','Категория «Разоблачения» изменена'].map((line, i) => <div className="audit-row" key={line}><i className={`audit-dot a${i}`} /><div><strong>{line}</strong><span>{i < 2 ? 'Сегодня' : 'Вчера'}, {i + 10}:4{i}</span></div></div>)}</section></div><div className="owner-columns lower"><section className="panel owner-panel"><div className="panel-head"><h2>Категории</h2><span className="panel-kicker">250 ВИДЕО</span></div><div className="add-row"><input placeholder="Название новой категории" /><button className="primary-btn">Создать категорию</button></div>{['Смешное', 'Трейлеры', 'Фильмы и сериалы'].map((c, i) => <div className="category-row" key={c}><strong>{c}</strong><span>{120 - i * 26} видео</span><button>Изменить</button><button className="danger-link">Удалить</button></div>)}</section><section className="panel owner-panel settings-panel"><h2>Глобальные настройки</h2><label>Лимит отправок в сутки<input value="3" readOnly /></label><label>Максимальный комментарий<input value="500 символов" readOnly /></label><label>Публичная лента<select defaultValue="Открыта"><option>Открыта</option><option>Только для Twitch</option></select></label><button className="primary-btn">Сохранить настройки</button></section></div></div> }

function NotificationView() { return <main className="main-content"><section className="page-heading"><div><div className="eyebrow">ЦЕНТР УВЕДОМЛЕНИЙ</div><h1>Уведомления</h1><p>Здесь появятся решения модерации и важные события</p></div><button className="ghost-btn">Прочитать все</button></section><div className="panel notification-page">{['Ваше видео одобрено','Видео отклонено','Новый лайк'].map((n, i) => <div className="notice-row big" key={n}><i className={i === 1 ? 'red' : ''} /><div><strong>{n}</strong><p>{i === 0 ? '«Смешная нарезка со стрима» появилась в общей ленте.' : i === 1 ? 'Модератор оставил комментарий: реклама и ссылки не допускаются.' : 'Ваше видео набрало 100 лайков.'}</p></div><span>{i === 0 ? 'Сегодня, 12:42' : 'Вчера, 21:10'}</span></div>)}</div></main> }

function App() {
  const demoRole = new URLSearchParams(window.location.search).get('role');
  const [view, setView] = useState('feed'); const [signedIn, setSignedIn] = useState(demoRole === 'moderator' || demoRole === 'owner'); const [role, setRole] = useState(demoRole === 'moderator' || demoRole === 'owner' ? demoRole : 'guest'); const [search, setSearch] = useState(''); const [videos, setVideos] = useState(initialVideos); const [toast, setToast] = useState('');
  React.useEffect(() => { const login = () => { setSignedIn(true); setRole('user'); setToast('Вы вошли через Twitch'); setTimeout(() => setToast(''), 2600); }; window.addEventListener('login', login); return () => window.removeEventListener('login', login); }, []);
  const auth = () => { if (!signedIn) { setSignedIn(true); setRole('user'); setToast('Вы вошли через Twitch'); setTimeout(() => setToast(''), 2600); } };
  const vote = (id, type) => {
    if (!signedIn) return auth();
    setVideos((prev) => prev.map((v) => {
      if (v.id !== id) return v;
      const previous = v.userVote;
      if (previous === type) return { ...v, userVote: undefined };
      return {
        ...v,
        userVote: type,
        likes: v.likes + (type === 'up' ? 1 : 0) - (previous === 'up' ? 1 : 0),
        dislikes: v.dislikes + (type === 'down' ? 1 : 0) - (previous === 'down' ? 1 : 0),
      };
    }));
  };
  const current = view === 'categories' ? 'feed' : view;
  return <div className="app-shell"><Sidebar view={current} setView={(v) => { if ((v === 'profile' || v === 'submit' || v === 'notifications') && !signedIn) return auth(); setView(v); }} signedIn={signedIn} role={role} /><div className="app-body"><Topbar signedIn={signedIn} setSignedIn={auth} setView={setView} search={search} setSearch={setSearch} />{view === 'feed' || view === 'categories' || view === 'video' ? <Feed videos={videos} signedIn={signedIn} setView={setView} search={search} setSearch={setSearch} onVote={vote} onOpen={(video) => window.open(`https://www.youtube.com/results?search_query=${encodeURIComponent(`${video.title} ${video.channel}`)}`, '_blank', 'noopener,noreferrer')} /> : view === 'submit' ? <SubmitView setView={setView} /> : view === 'profile' ? <ProfileView setView={setView} /> : view === 'notifications' ? <NotificationView /> : view === 'moderation' ? <ModerationView /> : view === 'owner' ? <ModerationView owner /> : null}</div>{toast && <div className="toast"><Check size={15} /> {toast}</div>}</div>;
}

createRoot(document.getElementById('root')).render(<App />);
