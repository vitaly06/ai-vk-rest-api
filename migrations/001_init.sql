-- 001_init.sql — начальная схема базы данных

-- Ссылки-приглашения (создаём первыми, т.к. users ссылается на них)
CREATE TABLE IF NOT EXISTS invites (
    id              BIGSERIAL PRIMARY KEY,
    token           TEXT NOT NULL UNIQUE,
    created_by_id   BIGINT NOT NULL,
    used_by_id      BIGINT,
    max_uses        INT NOT NULL DEFAULT 1,
    uses_count      INT NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invites_token ON invites(token);

-- Пользователи
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    vk_id           BIGINT NOT NULL UNIQUE,
    first_name      TEXT NOT NULL DEFAULT '',
    last_name       TEXT NOT NULL DEFAULT '',
    username        TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'guest'
                        CHECK (role IN ('admin','moderator','user','guest')),
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('active','banned','restricted','pending')),
    state           TEXT NOT NULL DEFAULT '',
    invite_id       BIGINT REFERENCES invites(id) ON DELETE SET NULL,
    request_count   INT NOT NULL DEFAULT 0,
    request_limit   INT NOT NULL DEFAULT 0,
    banned_until    TIMESTAMPTZ,
    consent_given   BOOLEAN NOT NULL DEFAULT FALSE,
    mailing_consent BOOLEAN NOT NULL DEFAULT FALSE,
    balance         NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_vk_id ON users(vk_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

-- Диалоги
CREATE TABLE IF NOT EXISTS dialogs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL DEFAULT 'main'
                    CHECK (type IN ('main','support')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dialogs_user_id ON dialogs(user_id);
CREATE INDEX IF NOT EXISTS idx_dialogs_type ON dialogs(type);

-- Сообщения
CREATE TABLE IF NOT EXISTS messages (
    id              BIGSERIAL PRIMARY KEY,
    dialog_id       BIGINT NOT NULL REFERENCES dialogs(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL
                        CHECK (role IN ('user','assistant','moderator','system')),
    type            TEXT NOT NULL DEFAULT 'text'
                        CHECK (type IN ('text','image','audio','video')),
    content         TEXT NOT NULL DEFAULT '',
    vk_message_id   INT NOT NULL DEFAULT 0,
    is_pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_dialog_id ON messages(dialog_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

-- Ответы анкеты
CREATE TABLE IF NOT EXISTS questionnaire_answers (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    answer      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qa_user_id ON questionnaire_answers(user_id);

-- Запросы на вступление
CREATE TABLE IF NOT EXISTS access_requests (
    id          BIGSERIAL PRIMARY KEY,
    vk_id       BIGINT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','approved','rejected')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Платежи
CREATE TABLE IF NOT EXISTS payments (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id         TEXT NOT NULL UNIQUE,
    amount              NUMERIC(12,2) NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'RUB',
    method              TEXT NOT NULL DEFAULT 'bank_card',
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','succeeded','failed','canceled')),
    description         TEXT NOT NULL DEFAULT '',
    confirmation_url    TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_external_id ON payments(external_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);

-- Товары/услуги
CREATE TABLE IF NOT EXISTS products (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price       NUMERIC(12,2) NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'RUB',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Настройки бота
CREATE TABLE IF NOT EXISTS bot_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO bot_settings (key, value) VALUES
    ('welcome_message', '👋 Добро пожаловать! Я ваш AI-помощник. Задайте любой вопрос.'),
    ('consent_text', '📋 Для работы с ботом необходимо ваше согласие на обработку персональных данных.'),
    ('faq_text', '❓ Часто задаваемые вопросы:\n\n1. Как начать? — Просто напишите вопрос.\n2. Как пополнить баланс? — Нажмите кнопку «Пополнить».'),
    ('about_text', 'ℹ️ Этот бот использует AI для ответов на ваши вопросы.'),
    ('default_request_limit', '100'),
    ('default_cooldown_secs', '60'),
    ('registration_open', 'true')
ON CONFLICT (key) DO NOTHING;

-- Журнал аудита
CREATE TABLE IF NOT EXISTS audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    BIGINT NOT NULL,
    target_id   BIGINT,
    action      TEXT NOT NULL,
    details     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
