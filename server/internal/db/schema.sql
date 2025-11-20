-- dialect: postgres
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY,
  username VARCHAR(16) NOT NULL UNIQUE CHECK (length(username) > 1),
  password VARCHAR(60) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'friend_status') THEN
        CREATE TYPE friend_status AS ENUM ('pending', 'accepted', 'blocked');
    END IF;

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

CREATE INDEX idx_friendships_user_one_status ON friendships(user_one, status);

CREATE INDEX idx_friendships_user_two_status ON friendships(user_two, status);

CREATE TABLE IF NOT EXISTS invite_codes (
  id UUID PRIMARY KEY,
  code VARCHAR(6) NOT NULL UNIQUE CHECK (length(code) = 6),
  registered_user_id UUID,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (registered_user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_invite_codes_registered_user ON invite_codes (registered_user_id);

CREATE TABLE IF NOT EXISTS channels (
  id UUID PRIMARY KEY,
  owner_id UUID NOT NULL,
  name VARCHAR(20) NOT NULL,
  description VARCHAR(100),
  capacity INTEGER DEFAULT 6 CHECK (capacity BETWEEN 1 AND 10)
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE
  UNIQUE (owner_id, name)
);

CREATE INDEX IF NOT EXISTS idx_channels_owner ON channels (owner_id);

CREATE TABLE IF NOT EXISTS channel_members (
  channel_id UUID NOT NULL,
  user_id UUID NOT NULL,
  invited_by UUID NOT NULL,
  joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (channel_id, user_id),
  FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  FOREIGN KEY (invited_by) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_channel_members_user ON channel_members (user_id);
CREATE INDEX IF NOT EXISTS idx_channel_members_channel ON channel_members (channel_id);
