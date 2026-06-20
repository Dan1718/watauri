package main

import (
	"database/sql"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type UserDataStore struct {
	db *sql.DB
	mu sync.RWMutex
}

func newUserDataStore() (*UserDataStore, error) {
	db, err := sql.Open("sqlite", "file:userdata.db?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	s := &UserDataStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *UserDataStore) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			avatar TEXT,
			last_message_id TEXT,
			last_message_text TEXT,
			last_message_timestamp TEXT,
			last_message_sender TEXT,
			unread_count INTEGER DEFAULT 0,
			is_group INTEGER DEFAULT 0,
			is_archived INTEGER DEFAULT 0,
			is_starred INTEGER DEFAULT 0,
			is_community INTEGER DEFAULT 0,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_jid TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			text TEXT,
			timestamp TEXT,
			status TEXT DEFAULT 'sent',
			media_type TEXT,
			is_from_me INTEGER DEFAULT 0,
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			jid TEXT PRIMARY KEY,
			name TEXT,
			avatar TEXT,
			push_name TEXT,
			status TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			text,
			content=messages,
			content_rowid=rowid
		)`,
		`CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text);
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserDataStore) UpsertChat(chat Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO chats (jid, name, avatar, last_message_id, last_message_text, last_message_timestamp, last_message_sender, unread_count, is_group, is_archived, is_starred, is_community, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name=COALESCE(NULLIF(EXCLUDED.name, ''), name),
			avatar=COALESCE(NULLIF(EXCLUDED.avatar, ''), avatar),
			last_message_id=COALESCE(NULLIF(EXCLUDED.last_message_id, ''), last_message_id),
			last_message_text=COALESCE(NULLIF(EXCLUDED.last_message_text, ''), last_message_text),
			last_message_timestamp=COALESCE(NULLIF(EXCLUDED.last_message_timestamp, ''), last_message_timestamp),
			last_message_sender=COALESCE(NULLIF(EXCLUDED.last_message_sender, ''), last_message_sender),
			unread_count=EXCLUDED.unread_count,
			is_archived=EXCLUDED.is_archived,
			is_starred=EXCLUDED.is_starred,
			is_community=EXCLUDED.is_community,
			updated_at=EXCLUDED.updated_at`,
		chat.ID, chat.Name, chat.Avatar,
		chat.LastMessageID, chat.LastMessageText, chat.LastMessageTimestamp, chat.LastMessageSender,
		chat.UnreadCount, boolToInt(chat.IsGroup),
		boolToInt(chat.IsArchived), boolToInt(chat.IsStarred), boolToInt(chat.IsCommunity),
		now,
	)
	return err
}

func (s *UserDataStore) GetChats() ([]Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT jid, name, avatar, last_message_id, last_message_text, last_message_timestamp, last_message_sender, unread_count, is_group, is_archived, is_starred, is_community FROM chats ORDER BY last_message_timestamp DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		var name, avatar sql.NullString
		var lastMsgID, lastMsgText, lastMsgTS, lastMsgSender sql.NullString
		var isGroup, isArchived, isStarred, isCommunity int

		if err := rows.Scan(&c.ID, &name, &avatar, &lastMsgID, &lastMsgText, &lastMsgTS, &lastMsgSender, &c.UnreadCount, &isGroup, &isArchived, &isStarred, &isCommunity); err != nil {
			return nil, err
		}

		if name.Valid {
			c.Name = &name.String
		}
		if avatar.Valid {
			c.Avatar = &avatar.String
		}
		if lastMsgID.Valid {
			c.LastMessage = &Message{ID: lastMsgID.String, Text: lastMsgText.String, Timestamp: lastMsgTS.String, SenderID: lastMsgSender.String}
		}
		c.IsGroup = intToBool(isGroup)
		c.IsArchived = intToBool(isArchived)
		c.IsStarred = intToBool(isStarred)
		c.IsCommunity = intToBool(isCommunity)

		chats = append(chats, c)
	}
	return chats, rows.Err()
}

func (s *UserDataStore) InsertMessage(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, text, timestamp, status, media_type, is_from_me) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ChatJID, msg.SenderID, msg.Text, msg.Timestamp, msg.Status, msg.MediaType, boolToInt(msg.IsFromMe),
	)
	return err
}

func (s *UserDataStore) GetMessages(chatJID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, sender_jid, text, timestamp, status, media_type, is_from_me FROM messages WHERE chat_jid = ? ORDER BY timestamp ASC`, chatJID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var mediaType sql.NullString
		var isFromMe int

		if err := rows.Scan(&m.ID, &m.SenderID, &m.Text, &m.Timestamp, &m.Status, &mediaType, &isFromMe); err != nil {
			return nil, err
		}
		if mediaType.Valid {
			m.MediaType = mediaType.String
		}
		m.IsFromMe = intToBool(isFromMe)
		m.ChatJID = chatJID
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *UserDataStore) UpdateMessageStatus(messageIDs []string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`UPDATE messages SET status = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range messageIDs {
		if _, err := stmt.Exec(status, id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *UserDataStore) UpsertContact(contact User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO contacts (jid, name, avatar, push_name, status) VALUES (?, ?, ?, ?, ?) ON CONFLICT(jid) DO UPDATE SET name=COALESCE(NULLIF(EXCLUDED.name, ''), name), avatar=COALESCE(NULLIF(EXCLUDED.avatar, ''), avatar), push_name=COALESCE(NULLIF(EXCLUDED.push_name, ''), push_name), status=COALESCE(NULLIF(EXCLUDED.status, ''), status)`,
		contact.ID, contact.Name, contact.Avatar, "", contact.Status,
	)
	return err
}

func (s *UserDataStore) GetContacts() ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT jid, name, avatar, status FROM contacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Avatar, &u.Status); err != nil {
			return nil, err
		}
		contacts = append(contacts, u)
	}
	return contacts, rows.Err()
}

// SearchMessages performs full-text search across messages with optional filters.
// q is the FTS5 query string (supports AND, OR, NOT, "exact phrase").
// Filters: senderJID, mediaType, and afterTS are optional (pass empty string to skip).
// Returns up to limit results starting at offset.
func (s *UserDataStore) SearchMessages(q, senderJID, mediaType, afterTS string, limit, offset int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT m.id, m.chat_jid, m.sender_jid, m.text, m.timestamp, m.status, m.media_type, m.is_from_me
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		WHERE messages_fts MATCH ?
		AND (? == '' OR m.sender_jid = ?)
		AND (? == '' OR m.media_type = ?)
		AND (? == '' OR m.timestamp >= ?)
		ORDER BY m.timestamp DESC
		LIMIT ? OFFSET ?`

	rows, err := s.db.Query(query, q, senderJID, senderJID, mediaType, mediaType, afterTS, afterTS, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var mediaType sql.NullString
		var isFromMe int

		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderID, &m.Text, &m.Timestamp, &m.Status, &mediaType, &isFromMe); err != nil {
			return nil, err
		}
		if mediaType.Valid {
			m.MediaType = mediaType.String
		}
		m.IsFromMe = intToBool(isFromMe)
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}
