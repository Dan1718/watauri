package main

import (
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type UserDataStore struct {
	db *sql.DB
	mu sync.RWMutex
}

func newUserDataStore() (*UserDataStore, error) {
	log.Println("[store] Opening userdata.db with WAL journal mode")
	db, err := sql.Open("sqlite", "file:userdata.db?_journal_mode=WAL")
	if err != nil {
		log.Printf("[store] Failed to open userdata.db: %v", err)
		return nil, err
	}
	s := &UserDataStore{db: db}
	log.Println("[store] Running schema migration...")
	if err := s.migrate(); err != nil {
		log.Printf("[store] Migration failed: %v", err)
		return nil, err
	}
	log.Println("[store] Schema migration complete")
	return s, nil
}

func (s *UserDataStore) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			avatar TEXT,
			last_message_id TEXT,
			last_message_text TEXT,
			last_message_timestamp TEXT,
			last_message_timestamp_epoch INTEGER,
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
			timestamp_epoch INTEGER NOT NULL,
			status TEXT DEFAULT 'sent',
			media_type TEXT,
			is_from_me INTEGER DEFAULT 0,
			revision INTEGER NOT NULL,
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			jid TEXT PRIMARY KEY,
			name TEXT,
			avatar TEXT,
			push_name TEXT,
			status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS chat_participants (
			chat_jid TEXT NOT NULL,
			user_jid TEXT NOT NULL,
			rank INTEGER DEFAULT 0,
			updated_at TEXT,
			PRIMARY KEY (chat_jid, user_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid),
			FOREIGN KEY (user_jid) REFERENCES contacts(jid)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			text,
			content=messages,
			content_rowid=rowid
		)`,
	}
	for i, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			log.Printf("[store] Migration step %d failed: %v\nQuery: %.100s", i, err, q)
			return err
		}
	}
	for _, trigger := range []string{"messages_ai", "messages_ad", "messages_au"} {
		if _, err := tx.Exec(`DROP TRIGGER IF EXISTS ` + trigger); err != nil {
			return err
		}
	}
	if err := addColumnIfMissing(tx, "chats", "last_message_timestamp_epoch", "INTEGER"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "messages", "timestamp_epoch", "INTEGER"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "messages", "revision", "INTEGER"); err != nil {
		return err
	}
	if err := backfillEpochs(tx); err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE messages SET revision = rowid WHERE revision IS NULL OR revision = 0`,
		`CREATE TABLE IF NOT EXISTS message_revision (singleton INTEGER PRIMARY KEY CHECK (singleton = 1), revision INTEGER NOT NULL)`,
		`INSERT INTO message_revision (singleton, revision) SELECT 1, COALESCE(MAX(revision), 0) FROM messages WHERE true ON CONFLICT(singleton) DO UPDATE SET revision = MAX(message_revision.revision, excluded.revision)`,
		`DROP INDEX IF EXISTS messages_chat_newest_idx`,
		`CREATE INDEX IF NOT EXISTS messages_chat_newest_idx ON messages(chat_jid, timestamp_epoch DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS messages_chat_revision_idx ON messages(chat_jid, revision)`,
		`CREATE INDEX IF NOT EXISTS chat_participants_user_idx ON chat_participants(user_jid)`,
		`INSERT INTO messages_fts(messages_fts) VALUES('rebuild')`,
		`CREATE TRIGGER messages_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
		`CREATE TRIGGER messages_ad AFTER DELETE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text);
		END`,
		`CREATE TRIGGER messages_au AFTER UPDATE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text);
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func addColumnIfMissing(tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		found = found || name == column
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func backfillEpochs(tx *sql.Tx) error {
	for _, table := range []struct {
		name, timestampColumn, epochColumn string
	}{
		{"messages", "timestamp", "timestamp_epoch"},
		{"chats", "last_message_timestamp", "last_message_timestamp_epoch"},
	} {
		rows, err := tx.Query(fmt.Sprintf(`SELECT rowid, %s FROM %s WHERE %s IS NULL AND %s IS NOT NULL AND %s != ''`, table.timestampColumn, table.name, table.epochColumn, table.timestampColumn, table.timestampColumn))
		if err != nil {
			return err
		}
		var updates [][2]int64
		for rows.Next() {
			var rowID int64
			var timestamp string
			if err := rows.Scan(&rowID, &timestamp); err != nil {
				rows.Close()
				return err
			}
			epoch, err := timestampEpoch(timestamp)
			if err != nil {
				rows.Close()
				return fmt.Errorf("migrate %s row %d timestamp: %w", table.name, rowID, err)
			}
			updates = append(updates, [2]int64{rowID, epoch})
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, update := range updates {
			if _, err := tx.Exec(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE rowid = ?`, table.name, table.epochColumn), update[1], update[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

func timestampEpoch(timestamp string) (int64, error) {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0, err
	}
	return parsed.UnixNano(), nil
}

func (s *UserDataStore) UpsertChat(chat Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	now := time.Now().Format(time.RFC3339)
	var lastMessageEpoch *int64
	if chat.LastMessageTimestamp != "" {
		epoch, err := timestampEpoch(chat.LastMessageTimestamp)
		if err != nil {
			return err
		}
		lastMessageEpoch = &epoch
	}
	_, err := s.db.Exec(
		`INSERT INTO chats (jid, name, avatar, last_message_id, last_message_text, last_message_timestamp, last_message_timestamp_epoch, last_message_sender, unread_count, is_group, is_archived, is_starred, is_community, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name=COALESCE(NULLIF(EXCLUDED.name, ''), name),
			avatar=COALESCE(NULLIF(EXCLUDED.avatar, ''), avatar),
			last_message_id=COALESCE(NULLIF(EXCLUDED.last_message_id, ''), last_message_id),
			last_message_text=COALESCE(NULLIF(EXCLUDED.last_message_text, ''), last_message_text),
			last_message_timestamp=COALESCE(NULLIF(EXCLUDED.last_message_timestamp, ''), last_message_timestamp),
			last_message_timestamp_epoch=COALESCE(EXCLUDED.last_message_timestamp_epoch, last_message_timestamp_epoch),
			last_message_sender=COALESCE(NULLIF(EXCLUDED.last_message_sender, ''), last_message_sender),
			unread_count=EXCLUDED.unread_count,
			is_archived=EXCLUDED.is_archived,
			is_starred=EXCLUDED.is_starred,
			is_community=EXCLUDED.is_community,
			updated_at=EXCLUDED.updated_at`,
		chat.ID, chat.Name, chat.Avatar,
		chat.LastMessageID, chat.LastMessageText, chat.LastMessageTimestamp, lastMessageEpoch, chat.LastMessageSender,
		chat.UnreadCount, boolToInt(chat.IsGroup),
		boolToInt(chat.IsArchived), boolToInt(chat.IsStarred), boolToInt(chat.IsCommunity),
		now,
	)
	if err != nil {
		log.Printf("[store] UpsertChat(%s) error: %v (%v)", chat.ID, err, time.Since(start))
	} else {
		log.Printf("[store] UpsertChat(%s) OK (%v)", chat.ID, time.Since(start))
	}
	return err
}

func (s *UserDataStore) UpsertChatParticipant(chatJID, userJID string, rank int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO chat_participants (chat_jid, user_jid, rank, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(chat_jid, user_jid) DO UPDATE SET
			rank = excluded.rank,
			updated_at = excluded.updated_at`,
		chatJID, userJID, rank, start.Format(time.RFC3339),
	)
	if err != nil {
		log.Printf("[store] UpsertChatParticipant(%s -> %s) error: %v (%v)", userJID, chatJID, err, time.Since(start))
		return err
	}

	log.Printf("[store] UpsertChatParticipant(%s -> %s) OK (%v)", userJID, chatJID, time.Since(start))
	return nil
}

func (s *UserDataStore) GetChats() ([]Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	rows, err := s.db.Query(`SELECT jid, name, avatar, last_message_id, last_message_text, last_message_timestamp, last_message_sender, unread_count, is_group, is_archived, is_starred, is_community FROM chats ORDER BY last_message_timestamp_epoch DESC`)
	if err != nil {
		log.Printf("[store] GetChats query error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		c.Participants = []User{}
		var name, avatar sql.NullString
		var lastMsgID, lastMsgText, lastMsgTS, lastMsgSender sql.NullString
		var isGroup, isArchived, isStarred, isCommunity int

		if err := rows.Scan(&c.ID, &name, &avatar, &lastMsgID, &lastMsgText, &lastMsgTS, &lastMsgSender, &c.UnreadCount, &isGroup, &isArchived, &isStarred, &isCommunity); err != nil {
			log.Printf("[store] GetChats row scan error: %v (%v)", err, time.Since(start))
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
	if err := rows.Err(); err != nil {
		log.Printf("[store] GetChats rows iteration error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	log.Printf("[store] GetChats -> %d chats (%v)", len(chats), time.Since(start))
	return chats, nil
}

func (s *UserDataStore) InsertMessage(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	epoch, err := timestampEpoch(msg.Timestamp)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	result, err := tx.Exec(
		`INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, text, timestamp, timestamp_epoch, status, media_type, is_from_me, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		msg.ID, msg.ChatJID, msg.SenderID, msg.Text, msg.Timestamp, epoch, msg.Status, msg.MediaType, boolToInt(msg.IsFromMe),
	)
	if err != nil {
		tx.Rollback()
		log.Printf("[store] InsertMessage(%s) error: %v (%v)", msg.ID, err, time.Since(start))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}
	if rows > 0 {
		revision, err := nextMessageRevision(tx)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`UPDATE messages SET revision = ? WHERE id = ?`, revision, msg.ID); err != nil {
			tx.Rollback()
			return err
		}
		unreadIncrement := 0
		if !msg.IsFromMe {
			unreadIncrement = 1
		}
		_, err = tx.Exec(`
    INSERT INTO chats (
        jid,
        last_message_id,
        last_message_text,
        last_message_timestamp,
		last_message_timestamp_epoch,
        last_message_sender,
        unread_count,
        is_group,
        updated_at
    )
	    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(jid) DO UPDATE SET
        last_message_id = CASE
	            WHEN chats.last_message_timestamp_epoch IS NULL
	                OR excluded.last_message_timestamp_epoch >= chats.last_message_timestamp_epoch
            THEN excluded.last_message_id
            ELSE chats.last_message_id
        END,
        last_message_text = CASE
	            WHEN chats.last_message_timestamp_epoch IS NULL
	                OR excluded.last_message_timestamp_epoch >= chats.last_message_timestamp_epoch
            THEN excluded.last_message_text
            ELSE chats.last_message_text
        END,
        last_message_timestamp = CASE
	            WHEN chats.last_message_timestamp_epoch IS NULL
	                OR excluded.last_message_timestamp_epoch >= chats.last_message_timestamp_epoch
            THEN excluded.last_message_timestamp
	            ELSE chats.last_message_timestamp
	        END,
		last_message_timestamp_epoch = MAX(COALESCE(chats.last_message_timestamp_epoch, excluded.last_message_timestamp_epoch), excluded.last_message_timestamp_epoch),
	        last_message_sender = CASE
	            WHEN chats.last_message_timestamp_epoch IS NULL
	                OR excluded.last_message_timestamp_epoch >= chats.last_message_timestamp_epoch
            THEN excluded.last_message_sender
            ELSE chats.last_message_sender
        END,
        unread_count = chats.unread_count + excluded.unread_count,
        is_group = excluded.is_group,
        updated_at = CASE
	            WHEN chats.last_message_timestamp_epoch IS NULL
	                OR excluded.last_message_timestamp_epoch >= chats.last_message_timestamp_epoch
            THEN excluded.updated_at
            ELSE chats.updated_at
        END
		`,
			msg.ChatJID,
			msg.ID,
			msg.Text,
			msg.Timestamp,
			epoch,
			msg.SenderID,
			unreadIncrement,
			boolToInt(strings.HasSuffix(msg.ChatJID, "@g.us")),
			time.Now().Format(time.RFC3339),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

type messageCursor struct {
	Version        int    `json:"v"`
	Mode           string `json:"mode"`
	TimestampEpoch int64  `json:"timestampEpoch,omitempty"`
	ID             string `json:"id,omitempty"`
	Revision       int64  `json:"revision,omitempty"`
}

func (s *UserDataStore) GetMessages(chatJID string, limit int, before, after *messageCursor) ([]Message, bool, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	var latestRevision int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(revision), 0) FROM messages WHERE chat_jid = ?`, chatJID).Scan(&latestRevision); err != nil {
		return nil, false, 0, err
	}
	query := `SELECT id, sender_jid, text, timestamp, status, media_type, is_from_me, revision FROM messages WHERE chat_jid = ?`
	args := []any{chatJID}
	ascending := after != nil
	if before != nil {
		query += ` AND (timestamp_epoch < ? OR (timestamp_epoch = ? AND id < ?))`
		args = append(args, before.TimestampEpoch, before.TimestampEpoch, before.ID)
	} else if after != nil {
		query += ` AND revision > ?`
		args = append(args, after.Revision)
	}
	if ascending {
		query += ` ORDER BY revision ASC`
	} else {
		query += ` ORDER BY timestamp_epoch DESC, id DESC`
	}
	query += ` LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("[store] GetMessages(%s) query error: %v (%v)", chatJID, err, time.Since(start))
		return nil, false, 0, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var mediaType sql.NullString
		var isFromMe int

		if err := rows.Scan(&m.ID, &m.SenderID, &m.Text, &m.Timestamp, &m.Status, &mediaType, &isFromMe, &m.Revision); err != nil {
			log.Printf("[store] GetMessages(%s) row scan error: %v (%v)", chatJID, err, time.Since(start))
			return nil, false, 0, err
		}
		if mediaType.Valid {
			m.MediaType = mediaType.String
		}
		m.IsFromMe = intToBool(isFromMe)
		m.ChatJID = chatJID
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[store] GetMessages(%s) rows iteration error: %v (%v)", chatJID, err, time.Since(start))
		return nil, false, 0, err
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	if !ascending {
		slices.Reverse(messages)
	}
	log.Printf("[store] GetMessages(%s) -> %d messages (%v)", chatJID, len(messages), time.Since(start))
	return messages, hasMore, latestRevision, nil
}

func nextMessageRevision(tx *sql.Tx) (int64, error) {
	var revision int64
	err := tx.QueryRow(`UPDATE message_revision SET revision = revision + 1 WHERE singleton = 1 RETURNING revision`).Scan(&revision)
	return revision, err
}

func (s *UserDataStore) UpdateMessageStatus(messageIDs []string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[store] UpdateMessageStatus begin tx error: %v (%v)", err, time.Since(start))
		return err
	}
	stmt, err := tx.Prepare(`UPDATE messages SET status = ?, revision = ? WHERE id = ? AND status != ?`)
	if err != nil {
		log.Printf("[store] UpdateMessageStatus prepare error: %v (%v)", err, time.Since(start))
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, id := range messageIDs {
		revision, err := nextMessageRevision(tx)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err := stmt.Exec(status, revision, id, status); err != nil {
			log.Printf("[store] UpdateMessageStatus exec for %s error: %v (%v)", id, err, time.Since(start))
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[store] UpdateMessageStatus commit error: %v (%v)", err, time.Since(start))
		return err
	}
	log.Printf("[store] UpdateMessageStatus(%d ids -> %s) OK (%v)", len(messageIDs), status, time.Since(start))
	return nil
}

func (s *UserDataStore) UpsertContact(contact User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO contacts (jid, name, avatar, push_name, status) VALUES (?, ?, ?, ?, ?) ON CONFLICT(jid) DO UPDATE SET name=COALESCE(NULLIF(EXCLUDED.name, ''), name), avatar=COALESCE(NULLIF(EXCLUDED.avatar, ''), avatar), push_name=COALESCE(NULLIF(EXCLUDED.push_name, ''), push_name), status=COALESCE(NULLIF(EXCLUDED.status, ''), status)`,
		contact.ID, contact.Name, contact.Avatar, contact.PushName, contact.Status,
	)
	if err != nil {
		log.Printf("[store] UpsertContact(%s) error: %v (%v)", contact.ID, err, time.Since(start))
	} else {
		log.Printf("[store] UpsertContact(%s) OK (%v)", contact.ID, time.Since(start))
	}
	return err
}
func (s *UserDataStore) GetContacts() ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	rows, err := s.db.Query(`SELECT jid, name, avatar, push_name, status FROM contacts`)
	if err != nil {
		log.Printf("[store] GetContacts query error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	defer rows.Close()

	var contacts []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Avatar, &u.PushName, &u.Status); err != nil {
			log.Printf("[store] GetContacts row scan error: %v (%v)", err, time.Since(start))
			return nil, err
		}
		contacts = append(contacts, u)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[store] GetContacts rows iteration error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	log.Printf("[store] GetContacts -> %d contacts (%v)", len(contacts), time.Since(start))
	return contacts, nil
}

// SearchMessages performs full-text search across messages with optional filters.
// q is the FTS5 query string (supports AND, OR, NOT, "exact phrase").
// Filters: senderJID, mediaType, and afterTS are optional (pass empty string to skip).
// Returns up to limit results starting at offset.
func (s *UserDataStore) SearchMessages(q, senderJID, mediaType, afterTS string, limit, offset int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	log.Printf("[store] SearchMessages q=%q sender=%q media=%q after=%q limit=%d offset=%d", q, senderJID, mediaType, afterTS, limit, offset)

	var afterEpoch int64
	if afterTS != "" {
		var err error
		afterEpoch, err = timestampEpoch(afterTS)
		if err != nil {
			return nil, err
		}
	}
	query := `SELECT m.id, m.chat_jid, m.sender_jid, m.text, m.timestamp, m.status, m.media_type, m.is_from_me
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		WHERE messages_fts MATCH ?
		AND (? == '' OR m.sender_jid = ?)
		AND (? == '' OR m.media_type = ?)
		AND (? == '' OR m.timestamp_epoch >= ?)
		ORDER BY m.timestamp_epoch DESC
		LIMIT ? OFFSET ?`

	rows, err := s.db.Query(query, q, senderJID, senderJID, mediaType, mediaType, afterTS, afterEpoch, limit, offset)
	if err != nil {
		log.Printf("[store] SearchMessages query error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var mediaType sql.NullString
		var isFromMe int

		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderID, &m.Text, &m.Timestamp, &m.Status, &mediaType, &isFromMe); err != nil {
			log.Printf("[store] SearchMessages row scan error: %v (%v)", err, time.Since(start))
			return nil, err
		}
		if mediaType.Valid {
			m.MediaType = mediaType.String
		}
		m.IsFromMe = intToBool(isFromMe)
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[store] SearchMessages rows iteration error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	log.Printf("[store] SearchMessages -> %d results (%v)", len(messages), time.Since(start))
	return messages, nil
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
