-- 0003_admin_users.sql —— 管理后台账号表。
-- 密码只存 bcrypt 哈希，绝不存明文。
-- 该文件由 internal/store 的迁移器在单个事务中执行，这里不写 BEGIN/COMMIT。

CREATE TABLE IF NOT EXISTS admin_users (
    id            BIGSERIAL   PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    display_name  TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_admin_users_touch ON admin_users;
CREATE TRIGGER trg_admin_users_touch BEFORE UPDATE ON admin_users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
