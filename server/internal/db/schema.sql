-- dialect: postgres
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY,
  username VARCHAR(16) NOT NULL CHECK (length(username) > 1),
  password VARCHAR(60) NOT NULL,
  friend_code VARCHAR(6) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'friend_status') THEN
        CREATE TYPE friend_status AS ENUM ('pending', 'accepted', 'blocked');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'blocked_status') THEN
        CREATE TYPE blocked_status AS ENUM ('user_one', 'user_two', 'both');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS friendships (
  user_one UUID NOT NULL,
  user_two UUID NOT NULL,
  status friend_status DEFAULT 'pending',
  whos_blocked blocked_status,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_one, user_two),
  FOREIGN KEY (user_one) REFERENCES users (id) ON DELETE CASCADE,
  FOREIGN KEY (user_two) REFERENCES users (id) ON DELETE CASCADE,
  CHECK (user_one < user_two) -- one row per friendship, application must enforce ordering on insert
);

CREATE TABLE IF NOT EXISTS invite_codes (
  id UUID PRIMARY KEY,
  code VARCHAR(6) NOT NULL UNIQUE CHECK (length(code) = 6),
  registered_user_id UUID,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (registered_user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_invite_codes_registered_user ON invite_codes (registered_user_id);
