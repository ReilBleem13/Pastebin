CREATE UNIQUE INDEX idx_pastas_hash ON pastas(hash);

CREATE INDEX idx_pastas_user_id_created_at ON pastas(user_id, created_at DESC);

CREATE INDEX idx_pastas_public_created_at ON pastas(created_at DESC)
WHERE object_id LIKE 'public:%' AND (password_hash IS NULL OR password_hash = '');

CREATE INDEX idx_favorites_user_created_at ON favorites(user_id, created_at DESC);

