-- Session 3: crop_id bound to chat session

ALTER TABLE chat_sessions
    ADD COLUMN IF NOT EXISTS crop_id TEXT NOT NULL DEFAULT 'apple';

CREATE INDEX IF NOT EXISTS idx_chat_sessions_crop_id ON chat_sessions (crop_id);
