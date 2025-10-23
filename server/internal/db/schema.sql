PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password TEXT NOT NULL,
  friend_code TEXT NOT NULL,
  created_at TEXT DEFAULT (datetime('now')),
  CONSTRAINT id_length CHECK (length(id) = 36),
  CONSTRAINT username_length CHECK (length(username) <= 20)
);

CREATE TABLE IF NOT EXISTS friendships (
  user_one TEXT NOT NULL,
  user_two TEXT NOT NULL,
  status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'blocked')),
  who_blocked TEXT CHECK (who_blocked IN ('one', 'two', 'both')),
  created_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY (user_one, user_two),
  FOREIGN KEY (user_one) REFERENCES users (id) ON DELETE CASCADE,
  FOREIGN KEY (user_two) REFERENCES users (id) ON DELETE CASCADE,
  CHECK (user_one < user_two) -- one row per friendship, application must enforce ordering on insert
);

CREATE TABLE IF NOT EXISTS invite_codes (
  id TEXT NOT NULL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  registered_user_id TEXT,
  created_at TEXT DEFAULT (datetime('now')),
  FOREIGN KEY (registered_user_id) REFERENCES users (id) ON DELETE CASCADE
);
