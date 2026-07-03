# Walkthrough: PostgreSQL migrations (`migrations/*.sql`)

**Folder:** `migrations/`  
**Files:** `001_init.sql`, `002_crop_id.sql`, `003_feedback_analytics.sql`  
**Applied by:** Go server on startup (`server/postgres_store.go` → `runAllMigrations`)  
**DB:** PostgreSQL 16 (container `postgres` in `docker-compose.yml`)

---

## What is a migration in simple terms

A **migration** is an SQL script that **changes database structure** (tables, columns, indexes).

Why not edit DB manually in pgAdmin:

- same schema for you, colleagues, and server;
- changes in Git — visible history (“session 2 added messages”);
- on new deploy server applies scripts automatically.

You have **three numbered files** — schema evolution, not three different databases.

---

## How migrations run in this project

```mermaid
sequenceDiagram
    participant DC as docker compose up server
    participant Go as server (main.go + postgres_store.go)
    participant PG as PostgreSQL

    DC->>Go: start container
    Go->>PG: connect DATABASE_URL
    Go->>PG: CREATE TABLE IF NOT EXISTS schema_migrations
    Go->>PG: SELECT filename FROM schema_migrations
    Go->>Go: findMigrationsDir → /migrations, sort 001, 002, 003
    loop each pending .sql
        Go->>PG: BEGIN; execute file; INSERT INTO schema_migrations; COMMIT
    end
    Go->>Go: API ready
```

### Important details

1. **`schema_migrations` ledger table** records which files were applied (`filename`, `applied_at`).
2. On startup the server applies only **pending** `.sql` files in alphabetical order; already-recorded ones are skipped.
3. Each migration runs in a **transaction** together with its ledger insert — a failed migration is not recorded as applied.
4. Existing files still use `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS` (they predate the ledger, and an existing DB will have all three re-recorded as applied on first run under the new scheme without failing). New migrations do not have to be idempotent.

### Where files live in Docker

- `Dockerfile.server`: `COPY migrations /migrations`
- `docker-compose.yml`: `MIGRATIONS_DIR=/migrations`

Locally without Docker Go looks for `migrations` or `../migrations`.

---

## Basic SQL cheat sheet

### Comments

```sql
-- single line
```

### Data types (used here)

| Type | Meaning |
|------|---------|
| `BIGSERIAL` | auto-increment integer (message id, user id) |
| `BIGINT` | large integer (telegram_id) |
| `TEXT` | arbitrary-length string |
| `TIMESTAMPTZ` | date+time with timezone |
| `DOUBLE PRECISION` | float (CV confidence) |
| `SMALLINT` | small integer (-1, 1 for like) |
| `JSONB` | JSON in binary form (analytics) |

### `PRIMARY KEY`

Unique row identifier. One row — one id.

### `NOT NULL`

Field required (cannot be empty).

### `DEFAULT`

Default value on insert if not specified:

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
crop_id TEXT NOT NULL DEFAULT 'apple'
```

### `UNIQUE`

Value does not repeat in table (e.g. user `telegram_id`).

### `REFERENCES ... ON DELETE CASCADE`

**Foreign key:** row references another table.

- `messages.session_id` → `chat_sessions.id`
- On session delete **cascade** deletes all its messages.

`ON DELETE SET NULL` (in analytics): on user delete `user_id` becomes NULL, event remains.

### `CHECK`

Constraint on allowed values:

```sql
CHECK (role IN ('user', 'assistant'))
CHECK (rating IN (-1, 1))
```

### `CREATE INDEX`

Speeds search/sort on column (cost — disk space and slightly slower INSERT).

```sql
CREATE INDEX IF NOT EXISTS idx_messages_session_created
  ON messages (session_id, created_at);
```

### `CREATE TABLE IF NOT EXISTS`

Create table only if missing — safe on migration rerun.

### `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`

Add column to existing table (migration 002), without breaking old data.

---

## File `001_init.sql` — foundation (session 2)

Three tables + relations.

### `users` — who chats

| Column | Purpose |
|--------|---------|
| `id` | internal DB id |
| `telegram_id` | Telegram id, **UNIQUE** |
| `username`, `first_name`, `last_name` | profile |
| `created_at`, `updated_at` | timestamps |

### `chat_sessions` — one “dialog”

| Column | Purpose |
|--------|---------|
| `id` | TEXT (random hex from Go), not auto-increment |
| `user_id` | → `users.id`, CASCADE on user delete |
| `created_at`, `updated_at` | session open/update time |

Index `idx_chat_sessions_user_id` — fast lookup of user sessions.

### `messages` — messages in session

| Column | Purpose |
|--------|---------|
| `id` | BIGSERIAL |
| `session_id` | → `chat_sessions.id` |
| `role` | `user` or `assistant` |
| `content` | text |
| `kind` | type (text/photo etc. — logic in Go) |
| `image_token` | photo file reference on disk, not base64 in DB |
| `class_prediction`, `class_confidence` | CV result |
| `created_at` | chat order |

Index `(session_id, created_at)` — history by time.

### Relation diagram

```
users (1) ──< chat_sessions (N) ──< messages (N)
```

---

## File `002_crop_id.sql` — multi-crop (session 3)

Does not create a new table, **extends** `chat_sessions`:

```sql
ALTER TABLE chat_sessions
    ADD COLUMN IF NOT EXISTS crop_id TEXT NOT NULL DEFAULT 'apple';
```

- Each session remembers selected crop (apple, pear…).
- Old sessions without column get `'apple'` via DEFAULT.
- Index on `crop_id` — if crop analytics needed.

File order matters: **002 only after 001**, otherwise `chat_sessions` does not exist.

---

## File `003_feedback_analytics.sql` — UX and metrics (session 5)

### `message_feedback` — 👍 / 👎

| Column | Purpose |
|--------|---------|
| `message_id` | → `messages.id`, CASCADE |
| `user_id` | → `users.id` |
| `rating` | `-1` or `1` |
| `UNIQUE (message_id, user_id)` | one vote per user per message |

### `analytics_events` — statistics events

| Column | Purpose |
|--------|---------|
| `event_type` | event code string (onboarding, etc.) |
| `payload` | JSONB — arbitrary fields |
| `user_id` | optional, SET NULL if user deleted |

Index `(event_type, created_at DESC)` — “latest events of type X”.

Depends on **001**: needs `users` and `messages`.

---

## Order and file naming

```
001_init.sql
002_crop_id.sql
003_feedback_analytics.sql
```

Go does `sort.Strings` → order by name. Prefix `001_`, `002_` — **team convention**, not PostgreSQL magic.

**New migration:** `004_something.sql`, do not change old files after merge to prod (only add new ones).

---

## How Go uses these tables (where to look)

| Table | Code example |
|-------|--------------|
| `users` | `UpsertUser` in `postgres_store.go` |
| `chat_sessions` | session creation, `crop_id` |
| `messages` | chat save, CV fields |
| `message_feedback` | `server/feedback.go` |
| `analytics_events` | `server/analytics_store.go` |

---

## Practice: check DB manually

```bash
docker compose exec postgres psql -U gardener -d gardener
```

```sql
\dt                    -- list tables
\d messages            -- table structure
SELECT COUNT(*) FROM messages;
SELECT rating, COUNT(*) FROM message_feedback GROUP BY rating;
```

---

## FAQ

### Deleted postgres volume — what happens?

Empty DB. On server start 001→002→003 run again, tables recreated. **Chat data lost** (unless volume was backed up).

### Can I change `001_init.sql` after deploy?

On existing DB — **risky**: `CREATE TABLE IF NOT EXISTS` does not update old schema. Correct: new file `004_...sql` with `ALTER TABLE`.

### Why is `session_id` TEXT, not integer?

Go generates random hex (`newSessionID`) — convenient for API without sequential id.

### Migrations and RAG/Chroma

**Unrelated.** Articles — files + RAG indexes (Chroma, BM25); migrations — PostgreSQL only (chat, users, feedback).

---

## Brief summary

| File | Adds |
|------|------|
| **001** | users, chat_sessions, messages + indexes |
| **002** | `crop_id` column on session |
| **003** | message_feedback, analytics_events |

Migrations are **versioned DB schema in SQL**. Go applies pending ones on startup and records them in `schema_migrations` (transaction per file). Understanding `CREATE`, `ALTER`, `REFERENCES`, `CHECK`, `INDEX` — base for reading any new `00N_*.sql`.
