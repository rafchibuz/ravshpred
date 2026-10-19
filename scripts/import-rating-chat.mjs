import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { once } from 'node:events';

const inputFile = path.resolve(process.argv[2] || '');
const channel = String(process.argv[3] || 'ravshann').trim().toLowerCase();
const outputFile = path.resolve(process.argv[4] || 'imports/missed-rating-stream.sql.gz');

if (!inputFile || !fs.existsSync(inputFile)) {
  throw new Error('Использование: node scripts/import-rating-chat.mjs <chat.json|chat.json.gz> [channel] [output.sql.gz]');
}
if (!/^[a-z0-9_]{1,25}$/.test(channel)) {
  throw new Error(`Некорректный Twitch-канал: ${channel}`);
}

const source = fs.readFileSync(inputFile);
const chat = JSON.parse(inputFile.endsWith('.gz') ? zlib.gunzipSync(source) : source);
const comments = Array.isArray(chat.comments) ? chat.comments : [];
const video = chat.video || {};
const firstCommentAt = comments.find((comment) => comment?.created_at)?.created_at;
const lastCommentAt = [...comments].reverse().find((comment) => comment?.created_at)?.created_at;
const streamID = String(video.id || path.basename(inputFile).match(/\d+/)?.[0] || '').trim();
const startedAt = video.created_at || firstCommentAt;
const durationSeconds = Number(video.length || 0);
const endedAt = startedAt && durationSeconds > 0
  ? new Date(new Date(startedAt).getTime() + durationSeconds * 1000).toISOString()
  : lastCommentAt;

if (!streamID || !startedAt || !endedAt) {
  throw new Error('В файле не удалось определить ID, начало или конец стрима');
}
if (!Number.isFinite(new Date(startedAt).getTime()) || !Number.isFinite(new Date(endedAt).getTime())) {
  throw new Error('В файле указана некорректная дата стрима');
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

function sql(value) {
  return `'${String(value).replaceAll("'", "''")}'`;
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

const messages = [];
const seen = new Map();
for (const comment of comments) {
  const userID = String(comment.commenter?._id || '').trim();
  const login = String(comment.commenter?.name || '').trim().toLowerCase();
  const messageID = String(comment._id || '').trim();
  if (!userID || !login || !messageID || !comment.created_at) continue;
  const body = String(comment.message?.body || '').trim();
  const textHash = hashText(body);
  const duplicateKey = `${userID}:${textHash}`;
  const sentAt = new Date(comment.created_at).getTime();
  const previous = seen.get(duplicateKey);
  const duplicate = previous != null && sentAt - previous < 10 * 60_000;
  seen.set(duplicateKey, sentAt);
  messages.push([
    messageID, channel, userID, login, comment.commenter?.display_name || login,
    comment.commenter?.logo || '', comment.created_at, streamID,
    roleFromBadges(comment.message?.user_badges), textHash, [...body].length,
    /^[!/]/.test(body), duplicate, isEmoteOnly(comment.message), false,
  ]);
}

if (messages.length === 0) {
  throw new Error('В файле не найдено ни одного пригодного сообщения чата');
}

fs.mkdirSync(path.dirname(outputFile), { recursive: true });
const gzip = zlib.createGzip({ level: 9 });
const output = fs.createWriteStream(outputFile);
gzip.pipe(output);
const write = async (value) => {
  if (!gzip.write(value)) await once(gzip, 'drain');
};

await write(`\\set ON_ERROR_STOP on
BEGIN;
CREATE TEMP TABLE rating_import_messages (LIKE twitch_rating_messages INCLUDING DEFAULTS) ON COMMIT DROP;
INSERT INTO twitch_rating_streams(id,channel_login,started_at,ended_at,title,game_name,observed_live)
VALUES(${sql(streamID)}, ${sql(channel)}, ${sql(new Date(startedAt).toISOString())}, ${sql(new Date(endedAt).toISOString())}, ${sql(video.title || '')}, ${sql(video.game || '')}, true)
ON CONFLICT(channel_login,id) DO UPDATE SET
  started_at=excluded.started_at,
  ended_at=COALESCE(excluded.ended_at,twitch_rating_streams.ended_at),
  title=excluded.title,
  game_name=excluded.game_name,
  observed_live=true;
COPY rating_import_messages (id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply) FROM STDIN WITH (FORMAT csv, HEADER true);
`);
await write(csvRow(['id', 'channel_login', 'user_id', 'login', 'display_name', 'avatar_url', 'sent_at', 'stream_id', 'role', 'text_hash', 'text_length', 'is_command', 'is_duplicate', 'is_emote_only', 'is_reply']));
for (const row of messages) {
  await write(csvRow(row.map((value, index) => index >= 11 ? String(Boolean(value)) : value)));
}
await write(`\\.
INSERT INTO twitch_rating_messages(id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply)
SELECT id,channel_login,user_id,login,display_name,avatar_url,sent_at,stream_id,role,text_hash,text_length,is_command,is_duplicate,is_emote_only,is_reply
FROM rating_import_messages
ON CONFLICT(id) DO NOTHING;
UPDATE twitch_rating_state state
SET last_event_at=GREATEST(COALESCE(state.last_event_at, 'epoch'::timestamptz), imported.last_event_at), updated_at=now()
FROM (SELECT channel_login, max(sent_at) AS last_event_at FROM rating_import_messages GROUP BY channel_login) imported
WHERE state.channel_login=imported.channel_login;
DELETE FROM twitch_rating_snapshots WHERE split_part(key, ':', 1) IN ('${channel}', 'all');
SELECT count(*) AS messages_in_file FROM rating_import_messages;
COMMIT;
`);
gzip.end();
await once(output, 'close');

console.log(JSON.stringify({ inputFile, outputFile, channel, streamID, startedAt, endedAt, messages: messages.length }, null, 2));
