-- lol-civil-war 数据库结构（MySQL 8.0+ / 8.4 LTS）
-- 约定：时间统一存 UTC 毫秒时间戳；布尔用 0/1；列表/对象用 JSON 文本。
-- 应用启动时按分号切分执行，所有语句都必须幂等（IF NOT EXISTS）。

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT NOT NULL AUTO_INCREMENT,
    user_no       VARCHAR(16) NOT NULL,
    email         VARCHAR(255) DEFAULT NULL,
    password_hash VARCHAR(255) DEFAULT NULL,
    qq_openid     VARCHAR(128) DEFAULT NULL,
    name          VARCHAR(64) NOT NULL,
    avatar        VARCHAR(255) DEFAULT NULL,
    is_dev        TINYINT(1) NOT NULL DEFAULT 0,
    created_at    BIGINT NOT NULL,
    updated_at    BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_user_no (user_no),
    UNIQUE KEY uk_users_email (email),
    UNIQUE KEY uk_users_openid (qq_openid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS qq_groups (
    id              BIGINT NOT NULL AUTO_INCREMENT,
    qq_group_number VARCHAR(20) NOT NULL,
    public_key      VARCHAR(64) NOT NULL,
    name            VARCHAR(64) DEFAULT NULL,
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_groups_number (qq_group_number),
    UNIQUE KEY uk_groups_key (public_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS group_members (
    id            BIGINT NOT NULL AUTO_INCREMENT,
    group_id      BIGINT NOT NULL,
    user_id       BIGINT NOT NULL,
    qq_number     VARCHAR(20) DEFAULT NULL,
    nickname      VARCHAR(64) NOT NULL,
    rank_region   VARCHAR(32) DEFAULT NULL,
    rank_tier     VARCHAR(16) DEFAULT NULL,
    rank_division TINYINT UNSIGNED DEFAULT NULL,
    rank_lp       INT UNSIGNED NOT NULL DEFAULT 0,
    positions     LONGTEXT DEFAULT NULL,
    created_at    BIGINT NOT NULL,
    updated_at    BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_group_members (group_id, user_id),
    UNIQUE KEY uk_group_members_qq (group_id, qq_number),
    KEY idx_group_members_group (group_id),
    CONSTRAINT fk_group_members_group FOREIGN KEY (group_id) REFERENCES qq_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_group_members_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS events (
    id                BIGINT NOT NULL AUTO_INCREMENT,
    group_id          BIGINT NOT NULL,
    creator_member_id BIGINT NOT NULL,
    title             VARCHAR(80) DEFAULT NULL,
    region            VARCHAR(32) NOT NULL,
    best_of           TINYINT UNSIGNED NOT NULL DEFAULT 1,
    has_bp            TINYINT(1) NOT NULL DEFAULT 0,
    signup_deadline   BIGINT NOT NULL,
    starts_at         BIGINT NOT NULL,
    note              TEXT,
    ended_at          BIGINT DEFAULT NULL,
    created_at        BIGINT NOT NULL,
    updated_at        BIGINT NOT NULL,
    PRIMARY KEY (id),
    KEY idx_events_group_deadline (group_id, signup_deadline),
    CONSTRAINT fk_events_group FOREIGN KEY (group_id) REFERENCES qq_groups (id) ON DELETE CASCADE,
    CONSTRAINT fk_events_creator FOREIGN KEY (creator_member_id) REFERENCES group_members (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS registrations (
    id                BIGINT NOT NULL AUTO_INCREMENT,
    event_id          BIGINT NOT NULL,
    member_id         BIGINT NOT NULL,
    rank_region       VARCHAR(32) NOT NULL,
    rank_tier         VARCHAR(16) NOT NULL,
    rank_division     TINYINT UNSIGNED DEFAULT NULL,
    rank_lp           INT UNSIGNED NOT NULL DEFAULT 0,
    main_position     VARCHAR(16) DEFAULT NULL,
    sub_positions     LONGTEXT DEFAULT NULL,
    sub_rank_region   VARCHAR(32) DEFAULT NULL,
    sub_rank_tier     VARCHAR(16) DEFAULT NULL,
    sub_rank_division TINYINT UNSIGNED DEFAULT NULL,
    sub_rank_lp       INT UNSIGNED NOT NULL DEFAULT 0,
    note              TEXT,
    is_spectator      TINYINT(1) NOT NULL DEFAULT 0,
    created_at        BIGINT NOT NULL,
    updated_at        BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_registrations (event_id, member_id),
    CONSTRAINT fk_registrations_event FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
    CONSTRAINT fk_registrations_member FOREIGN KEY (member_id) REFERENCES group_members (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS event_rosters (
    id         BIGINT NOT NULL AUTO_INCREMENT,
    event_id   BIGINT NOT NULL,
    payload    LONGTEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_event_rosters (event_id),
    CONSTRAINT fk_event_rosters_event FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS event_spectators (
    id         BIGINT NOT NULL AUTO_INCREMENT,
    event_id   BIGINT NOT NULL,
    member_id  BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_event_spectators (event_id, member_id),
    CONSTRAINT fk_event_spectators_event FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
    CONSTRAINT fk_event_spectators_member FOREIGN KEY (member_id) REFERENCES group_members (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS matches (
    id                   BIGINT NOT NULL AUTO_INCREMENT,
    event_id             BIGINT NOT NULL,
    game_no              TINYINT UNSIGNED NOT NULL,
    team1_side           VARCHAR(8) NOT NULL DEFAULT 'blue',
    blue_win             TINYINT(1) DEFAULT NULL,
    duration_ms          BIGINT DEFAULT NULL,
    status               VARCHAR(16) NOT NULL DEFAULT 'draft',
    submitted_at         BIGINT DEFAULT NULL,
    created_by_member_id BIGINT DEFAULT NULL,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_matches (event_id, game_no),
    CONSTRAINT fk_matches_event FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
    CONSTRAINT fk_matches_creator FOREIGN KEY (created_by_member_id) REFERENCES group_members (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS match_players (
    id                  BIGINT NOT NULL AUTO_INCREMENT,
    match_id            BIGINT NOT NULL,
    side                VARCHAR(8) NOT NULL,
    position            VARCHAR(16) NOT NULL,
    slot                TINYINT UNSIGNED NOT NULL,
    member_id           BIGINT DEFAULT NULL,
    player_name         VARCHAR(64) NOT NULL,
    champion            VARCHAR(64) NOT NULL DEFAULT '',
    kills               INT UNSIGNED NOT NULL DEFAULT 0,
    deaths              INT UNSIGNED NOT NULL DEFAULT 0,
    assists             INT UNSIGNED NOT NULL DEFAULT 0,
    cs                  INT UNSIGNED NOT NULL DEFAULT 0,
    gold                BIGINT UNSIGNED NOT NULL DEFAULT 0,
    damage_to_champions BIGINT UNSIGNED NOT NULL DEFAULT 0,
    damage_taken        BIGINT UNSIGNED NOT NULL DEFAULT 0,
    vision_score        INT UNSIGNED NOT NULL DEFAULT 0,
    win                 TINYINT(1) NOT NULL DEFAULT 0,
    raw                 LONGTEXT DEFAULT NULL,
    created_at          BIGINT NOT NULL,
    updated_at          BIGINT NOT NULL,
    PRIMARY KEY (id),
    KEY idx_match_players_match (match_id, side, slot),
    KEY idx_match_players_member (member_id),
    CONSTRAINT fk_match_players_match FOREIGN KEY (match_id) REFERENCES matches (id) ON DELETE CASCADE,
    CONSTRAINT fk_match_players_member FOREIGN KEY (member_id) REFERENCES group_members (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS match_bans (
    id         BIGINT NOT NULL AUTO_INCREMENT,
    match_id   BIGINT NOT NULL,
    side       VARCHAR(8) NOT NULL,
    slot       TINYINT UNSIGNED NOT NULL,
    champion   VARCHAR(64) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_match_bans (match_id, side, slot),
    CONSTRAINT fk_match_bans_match FOREIGN KEY (match_id) REFERENCES matches (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS match_player_ratings (
    id               BIGINT NOT NULL AUTO_INCREMENT,
    match_id         BIGINT NOT NULL,
    rater_member_id  BIGINT NOT NULL,
    target_member_id BIGINT NOT NULL,
    score            TINYINT UNSIGNED NOT NULL,
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_match_player_ratings (match_id, rater_member_id, target_member_id),
    KEY idx_ratings_target (target_member_id),
    CONSTRAINT fk_ratings_match FOREIGN KEY (match_id) REFERENCES matches (id) ON DELETE CASCADE,
    CONSTRAINT fk_ratings_rater FOREIGN KEY (rater_member_id) REFERENCES group_members (id) ON DELETE CASCADE,
    CONSTRAINT fk_ratings_target FOREIGN KEY (target_member_id) REFERENCES group_members (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS rofl_uploads (
    id                   BIGINT NOT NULL AUTO_INCREMENT,
    match_id             BIGINT DEFAULT NULL,
    file_name            VARCHAR(255) NOT NULL,
    file_size            BIGINT UNSIGNED NOT NULL DEFAULT 0,
    sha256               CHAR(64) NOT NULL,
    game_version         VARCHAR(32) NOT NULL DEFAULT '',
    ddragon_version      VARCHAR(32) NOT NULL DEFAULT 'latest',
    game_length_ms       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    blue_win             TINYINT(1) DEFAULT NULL,
    parsed               LONGTEXT,
    created_by_member_id BIGINT DEFAULT NULL,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_rofl_sha256 (sha256),
    CONSTRAINT fk_rofl_match FOREIGN KEY (match_id) REFERENCES matches (id) ON DELETE SET NULL,
    CONSTRAINT fk_rofl_creator FOREIGN KEY (created_by_member_id) REFERENCES group_members (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sessions (
    id                 VARCHAR(64) NOT NULL,
    user_id            BIGINT DEFAULT NULL,
    csrf               VARCHAR(64) NOT NULL,
    oauth_state        VARCHAR(64) NOT NULL DEFAULT '',
    oauth_redirect     VARCHAR(255) NOT NULL DEFAULT '',
    qq_pending_openid  VARCHAR(128) NOT NULL DEFAULT '',
    qq_pending_user_id BIGINT DEFAULT NULL,
    qq_pending_name    VARCHAR(64) NOT NULL DEFAULT '',
    qq_pending_avatar  VARCHAR(255) NOT NULL DEFAULT '',
    qq_bind_user_id    BIGINT DEFAULT NULL,
    expires_at         BIGINT NOT NULL,
    created_at         BIGINT NOT NULL,
    PRIMARY KEY (id),
    KEY idx_sessions_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS allowed_groups (
    qq_group_number VARCHAR(20) NOT NULL,
    note            VARCHAR(255) DEFAULT NULL,
    created_at      BIGINT NOT NULL,
    PRIMARY KEY (qq_group_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
