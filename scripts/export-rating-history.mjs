import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { once } from 'node:events';
import { DatabaseSync } from 'node:sqlite';

const sourceRoot = path.resolve(process.argv[2] || 'C:/Users/rafchibus/Documents/twitch-vips');
const outputFile = path.resolve(process.argv[3] || 'imports/ravshann-rating-history.sql.gz');
const chatDir = path.join(sourceRoot, 'data', 'historical', 'chats');
const sqliteFile = path.join(sourceRoot, 'data', 'activity.sqlite');

if (!fs.existsSync(chatDir) || !fs.existsSync(sqliteFile)) {
  throw new Error(`Не найдены архивы рейтинга в ${sourceRoot}`);
}

fs.mkdirSync(path.dirname(outputFile), { recursive: true });
const gzip = zlib.createGzip({ level: 9 });
const output = fs.createWriteStream(outputFile);
gzip.pipe(output);

async function write(value) {
  if (!gzip.write(value)) await once(gzip, 'drain');
}

function csv(value) {
  if (value == null) return '';
  const text = String(value);
  if (text === '') return '""';
  return /[",\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
}

function csvRow(values) {
  return `${values.map(csv).join(',')}\n`;
}

function bool(value) {
  return value ? 'true' : 'false';
}

function hashText(value) {
  return crypto.createHash('sha256').update(value.toLowerCase().replace(/\s+/g, ' ')).digest('hex');
}

function roleFromBadges(badges = []) {
  const names = new Set(badges.map((badge) => String(badge?._id || badge?.id || '').toLowerCase()));
  if (names.has('broadcaster')) return 'broadcaster';
  if (names.has('moderator')) return 'moderator';
  if (names.has('vip')) return 'vip';
  return 'viewer';
}

function isEmoteOnly(message) {
  const fragments = message?.fragments || [];
  return fragments.length > 0 && fragments.every((fragment) => fragment.emoticon);
}

function streamEnd(startedAt, seconds) {
  if (!startedAt || !seconds) return null;
  return new Date(new Date(startedAt).getTime() + Number(seconds) * 1000).toISOString();
}

const chatFiles = fs.readdirSync(chatDir).filter((name) => /^\d+\.json\.gz$/.test(name)).sort();
const historicalStreams = [];
const historicalMessages = [];
const historicalStreamIDs = new Set();

for (const fileName of chatFiles) {
  const chat = JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(chatDir, fileName))));
  const video = chat.video;
  if (!video?.id || !video.created_at) continue;
  const streamID = String(video.id);
  historicalStreamIDs.add(streamID);
  historicalStreams.push([
    streamID, 'ravshann', video.created_at, streamEnd(video.created_at, video.length),
    video.title || '', video.game || '', true,
  ]);
  const seen = new Map();
  for (const comment of chat.comments || []) {
    const userID = String(comment.commenter?._id || '');
    const login = String(comment.commenter?.name || '').trim().toLowerCase();
    const messageID = String(comment._id || '');
    if (!userID || !login || !messageID || !comment.created_at) continue;
    const body = String(comment.message?.body || '').trim();
    const textHash = hashText(body);
    const duplicateKey = `${userID}:${textHash}`;
    const sentAtMs = new Date(comment.created_at).getTime();
    const previous = seen.get(duplicateKey);
    const duplicate = previous != null && sentAtMs - previous < 10 * 60_000;
    seen.set(duplicateKey, sentAtMs);
    historicalMessages.push([
      messageID, 'ravshann', userID, login, comment.commenter?.display_name || login,
      comment.commenter?.logo || '', comment.created_at, streamID,
      roleFromBadges(comment.message?.user_badges), textHash, [...body].length,
      /^[!/]/.test(body), duplicate, isEmoteOnly(comment.message), false,
    ]);
  }
}

const database = new DatabaseSync(sqliteFile, { readOnly: true });
const placeholders = [...historicalStreamIDs].map(() => '?').join(',');
const recentStreams = database.prepare(`
  SELECT s.id, s.started_at, s.ended_at, s.title, s.game_name
  FROM streams s
  WHERE s.observed_live = 1
    AND s.id NOT IN (${placeholders})
    AND EXISTS (SELECT 1 FROM messages m WHERE m.stream_id=s.id AND m.is_live=1)
  ORDER BY s.started_at
`).all(...historicalStreamIDs);
const recentStreamIDs = new Set(recentStreams.map((stream) => String(stream.id)));
const recentMessages = database.prepare(`
  SELECT id, user_id, login, display_name, sent_at, stream_id, is_vip_badge,
         text_hash, text_length, is_command, is_duplicate, is_emote_only, is_reply
  FROM messages
  WHERE is_live=1 AND stream_id IS NOT NULL AND stream_id NOT IN (${placeholders})
  ORDER BY sent_at
`).all(...historicalStreamIDs).filter((message) => recentStreamIDs.has(String(message.stream_id)));
database.close();

const streamRows = [
  ...historicalStreams,
  ...recentStreams.map((stream) => [
    stream.id, 'ravshann', stream.started_at, stream.ended_at || null, stream.title || '',
    stream.game_name || '', true,
  ]),
];
const messageRows = [
  ...historicalMessages,
  ...recentMessages.map((message) => [
    message.id, 'ravshann', message.user_id, message.login,
    message.display_name || message.login, '', message.sent_at, message.stream_id,
    message.is_vip_badge ? 'vip' : 'viewer', message.text_hash || '', message.text_length || 0,
    Boolean(message.is_command), Boolean(message.is_duplicate), Boolean(message.is_emote_only), Boolean(message.is_reply),
  ]),
];

await write(`\\set ON_ERROR_STOP on
BEGIN;
CREATE TEMP TABLE rating_import_streams (LIKE twitch_rating_streams INCLUDING DEFAULTS) ON COMMIT DROP;
CREATE TEMP TABLE rating_import_messages (LIKE twitch_rating_messages INCLUDING DEFAULTS) ON COMMIT DROP;
COPY rating_import_streams (id,channel_login,started_at,ended_at,title,game_name,observed_live) FROM STDIN WITH (FORMAT csv, HEADER true);
`);
await write(csvRow(['id', 'channel_login', 'started_at', 'ended_at', 'title', 'game_name', 'observed_live']));
for (const row of streamRows) await write(csvRow(row.map((value, index) => index === 6 ? bool(value) : value)));
await write(`\\.
COPY rating_import_messages (id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply) FROM STDIN WITH (FORMAT csv, HEADER true);
`);
await write(csvRow(['id', 'channel_login', 'user_id', 'login', 'display_name', 'avatar_url', 'sent_at', 'stream_id', 'role', 'text_hash', 'text_length', 'is_command', 'is_duplicate', 'is_emote_only', 'is_reply']));
for (const row of messageRows) {
  await write(csvRow(row.map((value, index) => index >= 11 ? bool(value) : value)));
}
await write(`\\.
INSERT INTO twitch_rating_streams(id,channel_login,started_at,ended_at,title,game_name,observed_live)
SELECT id,channel_login,started_at,ended_at,title,game_name,observed_live FROM rating_import_streams
ON CONFLICT(channel_login,id) DO UPDATE SET
  started_at=excluded.started_at,
  ended_at=COALESCE(excluded.ended_at,twitch_rating_streams.ended_at),
  title=excluded.title,
  game_name=excluded.game_name,
  observed_live=twitch_rating_streams.observed_live OR excluded.observed_live;
INSERT INTO twitch_rating_messages(id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply)
SELECT id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply
FROM rating_import_messages
ON CONFLICT(id) DO NOTHING;
SELECT count(*) AS imported_streams FROM rating_import_streams;
SELECT count(*) AS imported_messages FROM rating_import_messages;
COMMIT;
`);
gzip.end();
await once(output, 'close');

console.log(JSON.stringify({
  outputFile,
  historicalStreams: historicalStreams.length,
  recentStreams: recentStreams.length,
  streams: streamRows.length,
  historicalMessages: historicalMessages.length,
  recentMessages: recentMessages.length,
  messages: messageRows.length,
  bytes: fs.statSync(outputFile).size,
}, null, 2));
