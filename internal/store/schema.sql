-- lol-civil-war 数据库结构（SQLite）
-- 约定：时间统一存 UTC 毫秒时间戳；布尔用 0/1；列表/对象用 JSON 文本。
PRAGMA user_version = 1;

CREATE TABLE IF NOT EXISTS users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    qq_openid   TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    avatar      TEXT,
    is_dev      INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS groups (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    qq_group_number TEXT NOT NULL UNIQUE,
    public_key      TEXT NOT NULL UNIQUE,
    name            TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS group_members (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id      INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    qq_number     TEXT,
    nickname      TEXT NOT NULL,
    rank_region   TEXT,
    rank_tier     TEXT,
    rank_division INTEGER,
    rank_lp       INTEGER NOT NULL DEFAULT 0,
    positions     TEXT,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    UNIQUE (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_group_members_group ON group_members(group_id);

CREATE TABLE IF NOT EXISTS events (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id          INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    creator_member_id INTEGER NOT NULL REFERENCES group_members(id),
    title             TEXT,
    region            TEXT NOT NULL,
    best_of           INTEGER NOT NULL DEFAULT 1,
    has_bp            INTEGER NOT NULL DEFAULT 0,
    signup_deadline   INTEGER NOT NULL,
    starts_at         INTEGER NOT NULL,
    note              TEXT,
    ended_at          INTEGER,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_group_deadline ON events(group_id, signup_deadline);

CREATE TABLE IF NOT EXISTS registrations (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id          INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    member_id         INTEGER NOT NULL REFERENCES group_members(id) ON DELETE CASCADE,
    rank_region       TEXT NOT NULL,
    rank_tier         TEXT NOT NULL,
    rank_division     INTEGER,
    rank_lp           INTEGER NOT NULL DEFAULT 0,
    main_position     TEXT,
    sub_positions     TEXT,
    sub_rank_region   TEXT,
    sub_rank_tier     TEXT,
    sub_rank_division INTEGER,
    sub_rank_lp       INTEGER NOT NULL DEFAULT 0,
    note              TEXT,
    is_spectator      INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL,
    UNIQUE (event_id, member_id)
);

CREATE TABLE IF NOT EXISTS event_rosters (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id   INTEGER NOT NULL UNIQUE REFERENCES events(id) ON DELETE CASCADE,
    payload    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS event_spectators (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id   INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    member_id  INTEGER NOT NULL REFERENCES group_members(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (event_id, member_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id             INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    game_no              INTEGER NOT NULL,
    team1_side           TEXT NOT NULL DEFAULT 'blue',
    blue_win             INTEGER,
    duration_ms          INTEGER,
    status               TEXT NOT NULL DEFAULT 'draft',
    submitted_at         INTEGER,
    created_by_member_id INTEGER REFERENCES group_members(id) ON DELETE SET NULL,
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL,
    UNIQUE (event_id, game_no)
);

CREATE TABLE IF NOT EXISTS match_players (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id            INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    side                TEXT NOT NULL,
    position            TEXT NOT NULL,
    slot                INTEGER NOT NULL,
    member_id           INTEGER REFERENCES group_members(id) ON DELETE SET NULL,
    player_name         TEXT NOT NULL,
    champion            TEXT NOT NULL DEFAULT '',
    kills               INTEGER NOT NULL DEFAULT 0,
    deaths              INTEGER NOT NULL DEFAULT 0,
    assists             INTEGER NOT NULL DEFAULT 0,
    cs                  INTEGER NOT NULL DEFAULT 0,
    gold                INTEGER NOT NULL DEFAULT 0,
    damage_to_champions INTEGER NOT NULL DEFAULT 0,
    damage_taken        INTEGER NOT NULL DEFAULT 0,
    vision_score        INTEGER NOT NULL DEFAULT 0,
    win                 INTEGER NOT NULL DEFAULT 0,
    raw                 TEXT,
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_match_players_match ON match_players(match_id, side, slot);
CREATE INDEX IF NOT EXISTS idx_match_players_member ON match_players(member_id);

CREATE TABLE IF NOT EXISTS match_bans (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id   INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    side       TEXT NOT NULL,
    slot       INTEGER NOT NULL,
    champion   TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (match_id, side, slot)
);

CREATE TABLE IF NOT EXISTS match_player_ratings (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id         INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    rater_member_id  INTEGER NOT NULL REFERENCES group_members(id) ON DELETE CASCADE,
    target_member_id INTEGER NOT NULL REFERENCES group_members(id) ON DELETE CASCADE,
    score            INTEGER NOT NULL,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    UNIQUE (match_id, rater_member_id, target_member_id)
);
CREATE INDEX IF NOT EXISTS idx_ratings_target ON match_player_ratings(target_member_id);

CREATE TABLE IF NOT EXISTS rofl_uploads (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id             INTEGER REFERENCES matches(id) ON DELETE SET NULL,
    file_name            TEXT NOT NULL,
    file_size            INTEGER NOT NULL DEFAULT 0,
    sha256               TEXT NOT NULL UNIQUE,
    game_version         TEXT NOT NULL DEFAULT '',
    ddragon_version      TEXT NOT NULL DEFAULT 'latest',
    game_length_ms       INTEGER NOT NULL DEFAULT 0,
    blue_win             INTEGER,
    parsed               TEXT,
    created_by_member_id INTEGER REFERENCES group_members(id) ON DELETE SET NULL,
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id             TEXT PRIMARY KEY,
    user_id        INTEGER,
    csrf           TEXT NOT NULL,
    oauth_state    TEXT NOT NULL DEFAULT '',
    oauth_redirect TEXT NOT NULL DEFAULT '',
    expires_at     INTEGER NOT NULL,
    created_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
