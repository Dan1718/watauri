package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
	_ "modernc.org/sqlite"
)

type UserDataStore struct {
	db        *sql.DB
	avatarDir string
	mu        sync.RWMutex
}

func newUserDataStore() (*UserDataStore, error) {
	log.Println("[store] Opening userdata.db with WAL journal mode")
	db, err := sql.Open("sqlite", "file:userdata.db?_journal_mode=WAL&_pragma=foreign_keys(1)")
	if err != nil {
		log.Printf("[store] Failed to open userdata.db: %v", err)
		return nil, err
	}
	s := &UserDataStore{db: db, avatarDir: "avatars"}
	log.Println("[store] Running schema migration...")
	if err := s.migrate(); err != nil {
		log.Printf("[store] Migration failed: %v", err)
		return nil, err
	}
	if err := s.reconcileAvatarAssets(); err != nil {
		log.Printf("[store] Avatar reconciliation failed: %v", err)
		return nil, err
	}
	log.Println("[store] Schema migration complete")
	return s, nil
}

func (s *UserDataStore) reconcileAvatarAssets() error {
	type orphan struct{ table, jid string }
	var missing []orphan
	for _, table := range []string{"chats", "contacts"} {
		rows, err := s.db.Query(`SELECT jid, avatar_id FROM ` + table + ` WHERE avatar_id IS NOT NULL AND avatar_id <> ''`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var jid, id string
			if err := rows.Scan(&jid, &id); err != nil {
				rows.Close()
				return err
			}
			path, pathErr := s.avatarPath(id)
			if pathErr != nil {
				missing = append(missing, orphan{table, jid})
				continue
			}
			info, err := os.Lstat(path)
			if os.IsNotExist(err) || (err == nil && !info.Mode().IsRegular()) {
				missing = append(missing, orphan{table, jid})
			} else if err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	for _, item := range missing {
		if _, err := s.db.Exec(`UPDATE `+item.table+` SET avatar=NULL, avatar_id=NULL, avatar_picture_id=NULL, avatar_status='missing' WHERE jid=?`, item.jid); err != nil {
			return err
		}
	}
	return nil
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
			avatar_id TEXT,
			avatar_picture_id TEXT,
			avatar_status TEXT,
			avatar_updated_at TEXT,
			is_pinned INTEGER NOT NULL DEFAULT 0,
			is_muted INTEGER NOT NULL DEFAULT 0,
			mute_until TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS group_metadata (
			chat_jid TEXT PRIMARY KEY,
			description TEXT,
			owner_jid TEXT,
			created_at TEXT,
			is_locked INTEGER NOT NULL DEFAULT 0,
			is_announce INTEGER NOT NULL DEFAULT 0,
			is_ephemeral INTEGER NOT NULL DEFAULT 0,
			disappearing_timer INTEGER,
			is_join_approval_required INTEGER NOT NULL DEFAULT 0,
			parent_group_jid TEXT,
			linked_parent_jid TEXT,
			is_default_subgroup INTEGER NOT NULL DEFAULT 0,
			is_suspended INTEGER NOT NULL DEFAULT 0,
			is_deleted INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
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
			mentioned_jids TEXT NOT NULL DEFAULT '[]',
			revision INTEGER NOT NULL,
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			jid TEXT PRIMARY KEY,
			name TEXT,
			avatar TEXT,
			avatar_id TEXT,
			avatar_picture_id TEXT,
			avatar_status TEXT,
			avatar_updated_at TEXT,
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
		`CREATE TABLE IF NOT EXISTS jid_mappings (
			lid_jid TEXT PRIMARY KEY,
			phone_jid TEXT NOT NULL,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS participant_revision (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			revision INTEGER NOT NULL
		)`,
		`INSERT INTO participant_revision (singleton, revision) VALUES (1, 0) ON CONFLICT(singleton) DO NOTHING`,
		`CREATE TABLE IF NOT EXISTS participant_departures (
			chat_jid TEXT NOT NULL,
			user_jid TEXT NOT NULL,
			revision INTEGER NOT NULL,
			PRIMARY KEY (chat_jid, user_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS participant_departures_user_idx ON participant_departures(user_jid)`,
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
	for _, column := range []struct {
		name, definition string
	}{
		{"avatar", "TEXT"},
		{"avatar_id", "TEXT"},
		{"avatar_picture_id", "TEXT"},
		{"avatar_status", "TEXT"},
		{"avatar_updated_at", "TEXT"},
		{"is_pinned", "INTEGER NOT NULL DEFAULT 0"},
		{"is_muted", "INTEGER NOT NULL DEFAULT 0"},
		{"mute_until", "TEXT"},
	} {
		if err := addColumnIfMissing(tx, "chats", column.name, column.definition); err != nil {
			return err
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"avatar", "TEXT"},
		{"avatar_id", "TEXT"}, {"avatar_picture_id", "TEXT"},
		{"avatar_status", "TEXT"}, {"avatar_updated_at", "TEXT"},
	} {
		if err := addColumnIfMissing(tx, "contacts", column.name, column.definition); err != nil {
			return err
		}
	}
	// Rebuild API paths only from opaque local asset IDs; discard persisted remote URLs.
	if _, err := tx.Exec(`UPDATE chats SET avatar = CASE WHEN avatar_id IS NOT NULL AND length(avatar_id)=32 AND avatar_id NOT GLOB '*[^0-9a-f]*' THEN '/api/avatars/' || avatar_id ELSE NULL END`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE contacts SET avatar = CASE WHEN avatar_id IS NOT NULL AND length(avatar_id)=32 AND avatar_id NOT GLOB '*[^0-9a-f]*' THEN '/api/avatars/' || avatar_id ELSE NULL END`); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "messages", "timestamp_epoch", "INTEGER"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "messages", "revision", "INTEGER"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "messages", "mentioned_jids", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
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
		`CREATE UNIQUE INDEX IF NOT EXISTS jid_mappings_phone_jid_idx ON jid_mappings(phone_jid)`,
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
	if err := migrateParticipantAliases(tx); err != nil {
		return err
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
		`INSERT INTO chats (jid, name, last_message_id, last_message_text, last_message_timestamp, last_message_timestamp_epoch, last_message_sender, unread_count, is_group, is_archived, is_starred, is_community, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name=COALESCE(NULLIF(EXCLUDED.name, ''), name),
			last_message_id=COALESCE(NULLIF(EXCLUDED.last_message_id, ''), last_message_id),
			last_message_text=COALESCE(NULLIF(EXCLUDED.last_message_text, ''), last_message_text),
			last_message_timestamp=COALESCE(NULLIF(EXCLUDED.last_message_timestamp, ''), last_message_timestamp),
			last_message_timestamp_epoch=COALESCE(EXCLUDED.last_message_timestamp_epoch, last_message_timestamp_epoch),
			last_message_sender=COALESCE(NULLIF(EXCLUDED.last_message_sender, ''), last_message_sender),
			unread_count=CASE WHEN ? THEN EXCLUDED.unread_count ELSE chats.unread_count END,
			is_group=CASE WHEN EXCLUDED.is_group = 1 THEN 1 ELSE chats.is_group END,
			is_archived=CASE WHEN ? THEN EXCLUDED.is_archived ELSE chats.is_archived END,
			is_starred=CASE WHEN ? THEN EXCLUDED.is_starred ELSE chats.is_starred END,
			is_community=CASE WHEN ? THEN EXCLUDED.is_community ELSE chats.is_community END,
			updated_at=EXCLUDED.updated_at`,
		chat.ID, chat.Name,
		chat.LastMessageID, chat.LastMessageText, chat.LastMessageTimestamp, lastMessageEpoch, chat.LastMessageSender,
		chat.UnreadCount, boolToInt(chat.IsGroup),
		boolToInt(chat.IsArchived), boolToInt(chat.IsStarred), boolToInt(chat.IsCommunity),
		now, chat.UnreadCountSet, chat.IsArchivedSet, chat.IsStarredSet, chat.IsCommunitySet,
	)
	if err != nil {
		log.Printf("[store] UpsertChat(%s) error: %v (%v)", chat.ID, err, time.Since(start))
	}
	return err
}

func (s *UserDataStore) UpsertGroupMetadata(chatJID string, patch GroupMetadataPatch) error {
	if !hasGroupMetadataPatch(patch) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO chats (jid, is_group, updated_at) VALUES (?, 1, ?) ON CONFLICT(jid) DO UPDATE SET is_group = 1`,
		chatJID, time.Now().Format(time.RFC3339),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO group_metadata (chat_jid) VALUES (?) ON CONFLICT(chat_jid) DO NOTHING`, chatJID); err != nil {
		return err
	}

	var assignments []string
	var args []any
	set := func(column string, value any) {
		assignments = append(assignments, column+" = ?")
		args = append(args, value)
	}
	if patch.Description != nil {
		set("description", *patch.Description)
	}
	if patch.OwnerJID != nil {
		if patch.OwnerJIDFallback {
			assignments = append(assignments, "owner_jid = COALESCE(NULLIF(owner_jid, ''), ?)")
			args = append(args, *patch.OwnerJID)
		} else {
			set("owner_jid", *patch.OwnerJID)
		}
	}
	if patch.CreatedAt != nil {
		set("created_at", *patch.CreatedAt)
	}
	if patch.IsLocked != nil {
		set("is_locked", boolToInt(*patch.IsLocked))
	}
	if patch.IsAnnouncement != nil {
		set("is_announce", boolToInt(*patch.IsAnnouncement))
	}
	if patch.IsEphemeral != nil {
		set("is_ephemeral", boolToInt(*patch.IsEphemeral))
	}
	if patch.DisappearingTimer != nil {
		set("disappearing_timer", *patch.DisappearingTimer)
	}
	if patch.IsJoinApprovalRequired != nil {
		set("is_join_approval_required", boolToInt(*patch.IsJoinApprovalRequired))
	}
	if patch.ParentGroupJID != nil {
		set("parent_group_jid", *patch.ParentGroupJID)
	}
	if patch.LinkedParentJID != nil {
		set("linked_parent_jid", *patch.LinkedParentJID)
	}
	if patch.IsDefaultSubgroup != nil {
		set("is_default_subgroup", boolToInt(*patch.IsDefaultSubgroup))
	}
	if patch.IsSuspended != nil {
		set("is_suspended", boolToInt(*patch.IsSuspended))
	}
	if patch.IsDeleted != nil {
		set("is_deleted", boolToInt(*patch.IsDeleted))
	}
	if len(assignments) > 0 {
		args = append(args, chatJID)
		query := `UPDATE group_metadata SET ` + strings.Join(assignments, ", ") + ` WHERE chat_jid = ?`
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func hasGroupMetadataPatch(patch GroupMetadataPatch) bool {
	return patch.Description != nil || patch.OwnerJID != nil || patch.CreatedAt != nil ||
		patch.IsLocked != nil || patch.IsAnnouncement != nil || patch.IsEphemeral != nil ||
		patch.DisappearingTimer != nil || patch.IsJoinApprovalRequired != nil ||
		patch.ParentGroupJID != nil || patch.LinkedParentJID != nil ||
		patch.IsDefaultSubgroup != nil || patch.IsSuspended != nil || patch.IsDeleted != nil
}

func (s *UserDataStore) UpsertChatParticipant(chatJID, userJID string, rank int) error {
	return s.writeParticipant(chatJID, userJID, rank, participantWrite{source: participantLive})
}

func (s *UserDataStore) DeleteChatParticipant(chatJID, userJID string) error {
	return s.writeParticipant(chatJID, userJID, 0, participantWrite{remove: true})
}

func (s *UserDataStore) ResolveContact(jid string) (User, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolveContactLocked(jid)
}

func (s *UserDataStore) resolveContactLocked(jid string) (User, bool, error) {
	candidates := []string{jid}
	if strings.HasSuffix(jid, "@lid") {
		var phoneJID string
		err := s.db.QueryRow(
			`SELECT phone_jid FROM jid_mappings WHERE lid_jid = ?`,
			jid,
		).Scan(&phoneJID)
		if err != nil && err != sql.ErrNoRows {
			return User{}, false, err
		}
		if phoneJID != "" {
			candidates = append(candidates, phoneJID)
		}
	} else if strings.HasSuffix(jid, "@s.whatsapp.net") {
		var lidJID string
		err := s.db.QueryRow(
			`SELECT lid_jid FROM jid_mappings WHERE phone_jid = ?`,
			jid,
		).Scan(&lidJID)
		if err != nil && err != sql.ErrNoRows {
			return User{}, false, err
		}
		if lidJID != "" {
			candidates = append(candidates, lidJID)
		}
	}

	var best User
	bestScore := 0
	for _, candidate := range candidates {
		var contact User
		var name, avatar, pushName, status sql.NullString
		err := s.db.QueryRow(
			`SELECT jid, name, avatar, push_name, status FROM contacts WHERE jid = ?`,
			candidate,
		).Scan(&contact.ID, &name, &avatar, &pushName, &status)

		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return User{}, false, err
		}
		if name.Valid {
			contact.Name = name.String
		}
		if avatar.Valid {
			contact.Avatar = avatar.String
		}
		if pushName.Valid {
			contact.PushName = pushName.String
		}
		if status.Valid {
			contact.Status = status.String
		}
		score := contactScore(contact)
		if score > bestScore {
			best = contact
			bestScore = score
		}
	}
	return best, bestScore > 0, nil
}

func contactScore(contact User) int {
	if contact.Name != "" {
		return 3
	}
	if contact.PushName != "" {
		return 2
	}
	return 1
}

func contactDisplayName(contact User, fallbackJID string) string {
	if contact.Name != "" {
		return contact.Name
	}
	if contact.PushName != "" {
		return contact.PushName
	}
	return displayNameFromJID(fallbackJID)
}

func displayNameFromJID(jid string) string {
	user, _, ok := strings.Cut(jid, "@")
	if ok && user != "" {
		return user
	}
	if jid != "" {
		return jid
	}
	return "Unknown chat"
}

func (s *UserDataStore) CanonicalDirectChatJID(jid string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.canonicalDirectChatJIDLocked(jid)
}

func (s *UserDataStore) canonicalDirectChatJIDLocked(jid string) (string, error) {
	if !strings.HasSuffix(jid, "@lid") {
		return jid, nil
	}

	var phoneJID string
	err := s.db.QueryRow(
		`SELECT phone_jid FROM jid_mappings WHERE lid_jid = ?`,
		jid,
	).Scan(&phoneJID)
	if err == sql.ErrNoRows {
		return jid, nil
	}
	if err != nil {
		return "", err
	}
	if phoneJID == "" {
		return jid, nil
	}
	return phoneJID, nil
}

func (s *UserDataStore) UpsertJIDMapping(lidJID, phoneJID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM jid_mappings WHERE lid_jid <> ? AND phone_jid = ?`, lidJID, phoneJID); err != nil {
		log.Printf("[store] UpsertJIDMapping(%s -> %s) delete error: %v (%v)", lidJID, phoneJID, err, time.Since(start))
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO jid_mappings (lid_jid, phone_jid, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(lid_jid) DO UPDATE SET
			phone_jid = excluded.phone_jid,
			updated_at = excluded.updated_at`,
		lidJID, phoneJID, start.Format(time.RFC3339),
	); err != nil {
		log.Printf("[store] UpsertJIDMapping(%s -> %s) error: %v (%v)", lidJID, phoneJID, err, time.Since(start))
		return err
	}
	if err := mergeParticipantAlias(tx, lidJID, phoneJID, ""); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[store] UpsertJIDMapping(%s -> %s) commit error: %v (%v)", lidJID, phoneJID, err, time.Since(start))
		return err
	}

	return nil
}

func (s *UserDataStore) GetChats() ([]Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	rows, err := s.db.Query(`SELECT
		c.jid, c.name, c.avatar,
		m.id, m.chat_jid, m.sender_jid, m.text, m.timestamp, m.status, m.media_type, m.is_from_me, m.mentioned_jids,
		c.unread_count, c.is_group, c.is_archived, c.is_starred, c.is_community,
		gm.chat_jid, gm.description, gm.owner_jid, gm.created_at,
		gm.is_locked, gm.is_announce, gm.is_ephemeral, gm.disappearing_timer,
		gm.is_join_approval_required, gm.parent_group_jid, gm.linked_parent_jid,
		gm.is_default_subgroup, gm.is_suspended, gm.is_deleted
		FROM chats c
		LEFT JOIN messages m ON m.id = c.last_message_id AND m.chat_jid = c.jid
		LEFT JOIN group_metadata gm ON gm.chat_jid = c.jid
		WHERE c.jid <> ?
		ORDER BY c.last_message_timestamp_epoch DESC`, types.StatusBroadcastJID.String())
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
		var lastMsgID, lastMsgChatJID, lastMsgSender, lastMsgText, lastMsgTS, lastMsgStatus, lastMsgMediaType, lastMsgMentionedJIDs sql.NullString
		var lastMsgIsFromMe sql.NullInt64
		var isGroup, isArchived, isStarred, isCommunity int
		var metadataChatJID, metadataDescription, metadataOwnerJID, metadataCreatedAt sql.NullString
		var metadataIsLocked, metadataIsAnnouncement, metadataIsEphemeral, metadataDisappearingTimer sql.NullInt64
		var metadataNeedsJoinApproval sql.NullInt64
		var metadataParentJID, metadataLinkedParentJID sql.NullString
		var metadataIsDefaultSubgroup, metadataIsSuspended, metadataIsDeleted sql.NullInt64

		if err := rows.Scan(&c.ID, &name, &avatar,
			&lastMsgID, &lastMsgChatJID, &lastMsgSender, &lastMsgText, &lastMsgTS, &lastMsgStatus, &lastMsgMediaType, &lastMsgIsFromMe, &lastMsgMentionedJIDs,
			&c.UnreadCount, &isGroup, &isArchived, &isStarred, &isCommunity,
			&metadataChatJID, &metadataDescription, &metadataOwnerJID, &metadataCreatedAt,
			&metadataIsLocked, &metadataIsAnnouncement, &metadataIsEphemeral, &metadataDisappearingTimer,
			&metadataNeedsJoinApproval, &metadataParentJID, &metadataLinkedParentJID,
			&metadataIsDefaultSubgroup, &metadataIsSuspended, &metadataIsDeleted); err != nil {
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
			lastMessage := &Message{ID: lastMsgID.String}
			if lastMsgChatJID.Valid {
				lastMessage.ChatJID = lastMsgChatJID.String
			}
			if lastMsgSender.Valid {
				lastMessage.SenderID = lastMsgSender.String
			}
			if lastMsgText.Valid {
				lastMessage.Text = lastMsgText.String
			}
			if lastMsgTS.Valid {
				lastMessage.Timestamp = lastMsgTS.String
			}
			if lastMsgStatus.Valid {
				lastMessage.Status = lastMsgStatus.String
			}
			if lastMsgMediaType.Valid {
				lastMessage.MediaType = lastMsgMediaType.String
			}
			lastMessage.IsFromMe = lastMsgIsFromMe.Valid && intToBool(int(lastMsgIsFromMe.Int64))
			lastMessage.MentionedJIDs = decodeMentionedJIDs(lastMsgMentionedJIDs.String)
			c.LastMessage = lastMessage
		}
		c.IsGroup = intToBool(isGroup)
		c.IsArchived = intToBool(isArchived)
		c.IsStarred = intToBool(isStarred)
		c.IsCommunity = intToBool(isCommunity)
		c.CanSend = true
		c.IsPinned = false
		c.IsMuted = false
		c.IsFavorite = c.IsStarred
		if metadataChatJID.Valid {
			metadata := &GroupMetadata{
				IsLocked:               metadataIsLocked.Valid && intToBool(int(metadataIsLocked.Int64)),
				IsAnnouncement:         metadataIsAnnouncement.Valid && intToBool(int(metadataIsAnnouncement.Int64)),
				IsEphemeral:            metadataIsEphemeral.Valid && intToBool(int(metadataIsEphemeral.Int64)),
				IsJoinApprovalRequired: metadataNeedsJoinApproval.Valid && intToBool(int(metadataNeedsJoinApproval.Int64)),
				IsDefaultSubgroup:      metadataIsDefaultSubgroup.Valid && intToBool(int(metadataIsDefaultSubgroup.Int64)),
				IsSuspended:            metadataIsSuspended.Valid && intToBool(int(metadataIsSuspended.Int64)),
				IsDeleted:              metadataIsDeleted.Valid && intToBool(int(metadataIsDeleted.Int64)),
			}
			if metadataDescription.Valid {
				metadata.Description = &metadataDescription.String
			}
			if metadataOwnerJID.Valid {
				metadata.OwnerJID = &metadataOwnerJID.String
			}
			if metadataCreatedAt.Valid {
				metadata.CreatedAt = &metadataCreatedAt.String
			}
			if metadataDisappearingTimer.Valid {
				timer := int(metadataDisappearingTimer.Int64)
				metadata.DisappearingTimer = &timer
			}
			if metadataParentJID.Valid {
				metadata.ParentGroupJID = &metadataParentJID.String
			}
			if metadataLinkedParentJID.Valid {
				metadata.LinkedParentJID = &metadataLinkedParentJID.String
			}
			c.GroupMetadata = metadata
			c.IsAnnouncement = metadata.IsAnnouncement
			c.IsSuspended = metadata.IsSuspended
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[store] GetChats rows iteration error: %v (%v)", err, time.Since(start))
		return nil, err
	}
	if err := rows.Close(); err != nil {
		log.Printf("[store] GetChats rows close error: %v (%v)", err, time.Since(start))
		return nil, err
	}

	// Close the chat query before loading contacts or participants. Besides
	// avoiding nested queries on an open result set, this works with stores
	// configured to use a single SQLite connection (including the in-memory DB
	// used by tests).
	for i := range chats {
		if !chats[i].IsGroup {
			contact, ok, err := s.resolveContactLocked(chats[i].ID)
			if err != nil {
				log.Printf("[store] failed to resolve contact for chat %s: %v", chats[i].ID, err)
			} else if ok {
				displayName := contactDisplayName(contact, chats[i].ID)
				chats[i].Name = &displayName
			} else if chats[i].Name == nil || *chats[i].Name == "" {
				displayName := displayNameFromJID(chats[i].ID)
				chats[i].Name = &displayName
			}
		}

		if chats[i].IsGroup {
			participants, err := s.getChatParticipantsLocked(chats[i].ID)
			if err != nil {
				log.Printf("[store] failed to load participants for group chat %s: %v", chats[i].ID, err)
			} else {
				chats[i].Participants = participants
			}
		}
	}
	log.Printf("[store] GetChats -> %d chats (%v)", len(chats), time.Since(start))
	return chats, nil
}

func (s *UserDataStore) getChatParticipantsLocked(chatJID string) ([]User, error) {
	rows, err := s.db.Query(
		`SELECT
			p.user_jid,
			COALESCE(
				CASE WHEN p.user_jid LIKE '%@s.whatsapp.net' THEN p.user_jid END,
				lid_map.phone_jid
			) AS phone_jid,

			COALESCE(
				CASE WHEN p.user_jid LIKE '%@lid' THEN p.user_jid END,
				phone_map.lid_jid
			) AS lid_jid,
			COALESCE(NULLIF(c.name, ''), NULLIF(pc.name, ''), NULLIF(lc.name, '')) AS name,
			COALESCE(NULLIF(c.avatar, ''), NULLIF(pc.avatar, ''), NULLIF(lc.avatar, '')) AS avatar,
			COALESCE(NULLIF(c.push_name, ''), NULLIF(pc.push_name, ''), NULLIF(lc.push_name, '')) AS push_name,
			COALESCE(NULLIF(c.status, ''), NULLIF(pc.status, ''), NULLIF(lc.status, '')) AS status,
			p.rank
		FROM chat_participants p
		LEFT JOIN contacts c ON c.jid = p.user_jid
		LEFT JOIN jid_mappings lid_map ON lid_map.lid_jid = p.user_jid
		LEFT JOIN contacts pc ON pc.jid = lid_map.phone_jid
		LEFT JOIN jid_mappings phone_map ON phone_map.phone_jid = p.user_jid
		LEFT JOIN contacts lc ON lc.jid = phone_map.lid_jid
		WHERE p.chat_jid = ?
		ORDER BY p.rank DESC,
			COALESCE(
				NULLIF(c.name, ''),
				NULLIF(pc.name, ''),
				NULLIF(lc.name, ''),
				NULLIF(c.push_name, ''),
				NULLIF(pc.push_name, ''),
				NULLIF(lc.push_name, ''),
				p.user_jid
			)`,
		chatJID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	participants := []User{}

	for rows.Next() {
		var participant User
		var rank int
		var name, avatar, pushName, status, phoneJID, lidJID sql.NullString
		if err := rows.Scan(
			&participant.ID,
			&phoneJID,
			&lidJID,
			&name,
			&avatar,
			&pushName,
			&status,
			&rank,
		); err != nil {
			return nil, err
		}
		participant.Rank = &rank
		if name.Valid {
			participant.Name = name.String
		}
		if avatar.Valid {
			participant.Avatar = avatar.String
		}
		if pushName.Valid {
			participant.PushName = pushName.String
		}
		if status.Valid {
			participant.Status = status.String
		}
		if phoneJID.Valid {
			participant.PhoneJID = phoneJID.String
			participant.PhoneNumber = phoneNumberFromJID(phoneJID.String)
		}
		if participant.Name == "" {
			if participant.PushName != "" {
				participant.Name = participant.PushName
			} else if participant.PhoneNumber != "" {
				participant.Name = participant.PhoneNumber
			} else {
				participant.Name = displayNameFromJID(participant.ID)
			}
		}

		if lidJID.Valid {
			participant.LIDJID = lidJID.String
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return participants, nil
}

func phoneNumberFromJID(jid string) string {
	user, server, ok := strings.Cut(jid, "@")
	if ok && server == "s.whatsapp.net" && user != "" {
		return user
	}
	return ""
}

func (s *UserDataStore) InsertMessage(msg Message, incrementUnread bool) error {
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

	// Satisfy the messages.chat_jid foreign key before inserting the message.
	// The transaction rolls this placeholder back if the message insert fails.
	if _, err := tx.Exec(
		`INSERT INTO chats (jid, is_group, updated_at) VALUES (?, ?, ?) ON CONFLICT(jid) DO NOTHING`,
		msg.ChatJID, boolToInt(strings.HasSuffix(msg.ChatJID, "@g.us")), time.Now().Format(time.RFC3339),
	); err != nil {
		tx.Rollback()
		return err
	}

	mentionedJIDs, err := json.Marshal(msg.MentionedJIDs)
	if err != nil {
		tx.Rollback()
		return err
	}
	result, err := tx.Exec(
		`INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, text, timestamp, timestamp_epoch, status, media_type, is_from_me, mentioned_jids, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		msg.ID, msg.ChatJID, msg.SenderID, msg.Text, msg.Timestamp, epoch, msg.Status, msg.MediaType, boolToInt(msg.IsFromMe), string(mentionedJIDs),
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
		if incrementUnread && !msg.IsFromMe {
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

// InsertHistoryMessages stores one conversation's parsed history in a single
// SQLite transaction. It retains duplicate suppression and per-message
// revisions while applying the chat summary update only once.
func (s *UserDataStore) InsertHistoryMessages(messages []Message, senderPushNames map[string]string) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	type preparedMessage struct {
		message  Message
		epoch    int64
		mentions string
	}
	prepared := make([]preparedMessage, 0, len(messages))
	for _, message := range messages {
		epoch, err := timestampEpoch(message.Timestamp)
		if err != nil {
			return 0, err
		}
		mentions, err := json.Marshal(message.MentionedJIDs)
		if err != nil {
			return 0, err
		}
		prepared = append(prepared, preparedMessage{message: message, epoch: epoch, mentions: string(mentions)})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	chatJID := messages[0].ChatJID
	if _, err := tx.Exec(`INSERT INTO chats (jid, is_group, updated_at) VALUES (?, ?, ?) ON CONFLICT(jid) DO NOTHING`,
		chatJID, boolToInt(strings.HasSuffix(chatJID, "@g.us")), time.Now().Format(time.RFC3339)); err != nil {
		return 0, err
	}
	if len(senderPushNames) > 0 {
		contactStmt, err := tx.Prepare(`INSERT INTO contacts (jid, push_name) VALUES (?, ?) ON CONFLICT(jid) DO UPDATE SET push_name=COALESCE(NULLIF(EXCLUDED.push_name, ''), push_name)`)
		if err != nil {
			return 0, err
		}
		for jid, pushName := range senderPushNames {
			if _, err := contactStmt.Exec(jid, pushName); err != nil {
				contactStmt.Close()
				return 0, err
			}
		}
		if err := contactStmt.Close(); err != nil {
			return 0, err
		}
	}
	var lastRevision int64
	if err := tx.QueryRow(`UPDATE message_revision SET revision = revision + ? WHERE singleton = 1 RETURNING revision`, len(prepared)).Scan(&lastRevision); err != nil {
		return 0, err
	}
	firstRevision := lastRevision - int64(len(prepared)) + 1
	insertStmt, err := tx.Prepare(`INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, text, timestamp, timestamp_epoch, status, media_type, is_from_me, mentioned_jids, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer insertStmt.Close()
	inserted := 0
	var latest *preparedMessage
	for i := range prepared {
		item := &prepared[i]
		message := item.message
		if message.ChatJID != chatJID {
			return 0, fmt.Errorf("history message chat %q does not match conversation %q", message.ChatJID, chatJID)
		}
		result, err := insertStmt.Exec(message.ID, chatJID, message.SenderID, message.Text, message.Timestamp, item.epoch,
			message.Status, message.MediaType, boolToInt(message.IsFromMe), item.mentions, firstRevision+int64(i))
		if err != nil {
			log.Printf("[store] InsertHistoryMessages(%s) insert error: %v (%v)", message.ID, err, time.Since(start))
			return 0, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if rows == 0 {
			continue
		}
		inserted++
		if latest == nil || item.epoch >= latest.epoch {
			latest = item
		}
	}
	if latest != nil {
		message := latest.message
		_, err := tx.Exec(`UPDATE chats SET
			last_message_id = CASE WHEN last_message_timestamp_epoch IS NULL OR last_message_timestamp_epoch <= ? THEN ? ELSE last_message_id END,
			last_message_text = CASE WHEN last_message_timestamp_epoch IS NULL OR last_message_timestamp_epoch <= ? THEN ? ELSE last_message_text END,
			last_message_timestamp = CASE WHEN last_message_timestamp_epoch IS NULL OR last_message_timestamp_epoch <= ? THEN ? ELSE last_message_timestamp END,
			last_message_sender = CASE WHEN last_message_timestamp_epoch IS NULL OR last_message_timestamp_epoch <= ? THEN ? ELSE last_message_sender END,
			last_message_timestamp_epoch = MAX(COALESCE(last_message_timestamp_epoch, ?), ?),
			is_group = ?, updated_at = CASE WHEN last_message_timestamp_epoch IS NULL OR last_message_timestamp_epoch <= ? THEN ? ELSE updated_at END
			WHERE jid = ?`,
			latest.epoch, message.ID, latest.epoch, message.Text, latest.epoch, message.Timestamp,
			latest.epoch, message.SenderID, latest.epoch, latest.epoch,
			boolToInt(strings.HasSuffix(chatJID, "@g.us")), latest.epoch, time.Now().Format(time.RFC3339), chatJID)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (s *UserDataStore) GetUnreadInboundMessages(chatJID string, messageIDs []string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := time.Now()
	var rows *sql.Rows
	var err error
	if len(messageIDs) == 0 {
		rows, err = s.db.Query(`SELECT id, sender_jid, timestamp FROM messages WHERE chat_jid = ? AND is_from_me = 0 AND status != 'read' ORDER BY timestamp_epoch ASC, id ASC`, chatJID)
	} else {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(messageIDs)), ",")
		args := []any{chatJID}
		for _, id := range messageIDs {
			args = append(args, id)
		}
		rows, err = s.db.Query(`SELECT id, sender_jid, timestamp FROM messages WHERE chat_jid = ? AND id IN (`+placeholders+`) AND is_from_me = 0 AND status != 'read' ORDER BY timestamp_epoch ASC, id ASC`, args...)
	}
	if err != nil {
		log.Printf("[store] GetUnreadInboundMessages(%s) query error: %v (%v)", chatJID, err, time.Since(start))
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		message.ChatJID = chatJID
		if err := rows.Scan(&message.ID, &message.SenderID, &message.Timestamp); err != nil {
			log.Printf("[store] GetUnreadInboundMessages(%s) row scan error: %v (%v)", chatJID, err, time.Since(start))
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[store] GetUnreadInboundMessages(%s) rows error: %v (%v)", chatJID, err, time.Since(start))
		return nil, err
	}

	log.Printf("[store] GetUnreadInboundMessages(%s) -> %d messages (%v)", chatJID, len(messages), time.Since(start))
	return messages, nil
}

func (s *UserDataStore) MarkChatRead(chatJID string, messageIDs []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	if len(messageIDs) == 0 {
		var unreadCount int
		err := s.db.QueryRow(`SELECT unread_count FROM chats WHERE jid = ?`, chatJID).Scan(&unreadCount)
		return unreadCount, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[store] MarkChatRead begin tx error : %v (%v) ", err, time.Since(start))
		return 0, err
	}
	stmt, err := tx.Prepare(`UPDATE messages SET status = 'read', revision = ? WHERE chat_jid = ? AND id = ? AND is_from_me = 0 AND status != 'read'`)
	if err != nil {
		tx.Rollback()
		log.Printf("[store] MarkChatRead(%s) preparer errorr: %v (%v)", chatJID, err, time.Since(start))
		return 0, err
	}
	defer stmt.Close()

	updatedCount := int64(0)
	for _, id := range messageIDs {
		revision, err := nextMessageRevision(tx)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		result, err := stmt.Exec(revision, chatJID, id)
		if err != nil {
			tx.Rollback()
			log.Printf("[store] MarkChatRead(%s) update message %s error : %v (%v) ", chatJID, id, err, time.Since(start))
			return 0, err

		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			tx.Rollback()
			log.Printf("[store] MarkChatRead(%s) rows affected for message %s error : %v (%v) ", chatJID, id, err, time.Since(start))
			return 0, err
		}
		updatedCount += rowsAffected
	}
	var unreadCount int
	if _, err := tx.Exec(`
		UPDATE chats
		SET unread_count = MAX(unread_count - ?, 0),
		updated_at = ?
		WHERE jid = ?`, updatedCount, time.Now().Format(time.RFC3339), chatJID); err != nil {
		tx.Rollback()
		log.Printf("[store] MarkChatRead(%s) update chat error : %v (%v) ", chatJID, err, time.Since(start))
		return 0, err
	}
	if err := tx.QueryRow(`SELECT unread_count FROM chats WHERE jid = ?`, chatJID).Scan(&unreadCount); err != nil {
		tx.Rollback()
		log.Printf("[store] MarkChatRead(%s) unread count query error : %v (%v) ", chatJID, err, time.Since(start))
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[store] MarkChatRead(%s) commit error : %v (%v)", chatJID, err, time.Since(start))
		return 0, err
	}
	log.Printf("[store] MarkChatRead(%s) -> %d messages, %d unread remaining (%v)", chatJID, updatedCount, unreadCount, time.Since(start))
	return unreadCount, nil

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
	query := `SELECT id, sender_jid, text, timestamp, status, media_type, is_from_me, mentioned_jids, revision FROM messages WHERE chat_jid = ?`
	args := []any{chatJID}
	orderBy := ` ORDER BY timestamp_epoch DESC, id DESC`
	reverse := true

	if before != nil {
		query += ` AND (timestamp_epoch < ? OR (timestamp_epoch = ? AND id < ?))`
		args = append(args, before.TimestampEpoch, before.TimestampEpoch, before.ID)

	} else if after != nil && after.Mode == "after" {
		query += ` AND revision > ?`
		args = append(args, after.Revision)
		orderBy = ` ORDER BY revision ASC`
		reverse = false
	} else if after != nil && after.Mode == "afterTime" {
		query += ` AND (timestamp_epoch > ? OR (timestamp_epoch = ? AND id > ?))`
		args = append(args, after.TimestampEpoch, after.TimestampEpoch, after.ID)
		orderBy = ` ORDER BY timestamp_epoch ASC, id ASC`
		reverse = false
	}
	query += orderBy
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
		var mediaType, mentionedJIDs sql.NullString
		var isFromMe int

		if err := rows.Scan(&m.ID, &m.SenderID, &m.Text, &m.Timestamp, &m.Status, &mediaType, &isFromMe, &mentionedJIDs, &m.Revision); err != nil {
			log.Printf("[store] GetMessages(%s) row scan error: %v (%v)", chatJID, err, time.Since(start))
			return nil, false, 0, err
		}
		if mediaType.Valid {
			m.MediaType = mediaType.String
		}
		m.IsFromMe = intToBool(isFromMe)
		m.MentionedJIDs = decodeMentionedJIDs(mentionedJIDs.String)
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
	if reverse {
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
	return nil
}

func (s *UserDataStore) UpsertContact(contact User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO contacts (jid, name, push_name, status) VALUES (?, ?, ?, ?) ON CONFLICT(jid) DO UPDATE SET name=COALESCE(NULLIF(EXCLUDED.name, ''), name), push_name=COALESCE(NULLIF(EXCLUDED.push_name, ''), push_name), status=COALESCE(NULLIF(EXCLUDED.status, ''), status)`,
		contact.ID, contact.Name, contact.PushName, contact.Status,
	)
	if err != nil {
		log.Printf("[store] UpsertContact(%s) error: %v (%v)", contact.ID, err, time.Since(start))
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
		var name, avatar, pushName, status sql.NullString
		if err := rows.Scan(&u.ID, &name, &avatar, &pushName, &status); err != nil {
			log.Printf("[store] GetContacts row scan error: %v (%v)", err, time.Since(start))
			return nil, err
		}
		if name.Valid {
			u.Name = name.String
		}
		if avatar.Valid {
			u.Avatar = avatar.String
		}
		if pushName.Valid {
			u.PushName = pushName.String
		}
		if status.Valid {
			u.Status = status.String
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

func (s *UserDataStore) MissingAvatarCandidates() ([]avatarCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT jid FROM contacts
		WHERE COALESCE(avatar_id, '') = '' AND COALESCE(avatar_status, '') <> 'removed'
		UNION
		SELECT jid FROM chats
		WHERE is_group = 1 AND COALESCE(avatar_id, '') = '' AND COALESCE(avatar_status, '') <> 'removed'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]avatarCandidate, 0)
	for rows.Next() {
		var candidate avatarCandidate
		if err := rows.Scan(&candidate.jid); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (s *UserDataStore) AvatarMetadata(jid string, group bool) (assetID, pictureID string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	table := "contacts"
	if group {
		table = "chats"
	}
	err = s.db.QueryRow(`SELECT COALESCE(avatar_id,''), COALESCE(avatar_picture_id,'') FROM `+table+` WHERE jid = ?`, jid).Scan(&assetID, &pictureID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return
}

func (s *UserDataStore) SaveAvatar(jid, assetID, pictureID string, group bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	table := "contacts"
	if group {
		table = "chats"
	}
	_, err := s.db.Exec(`INSERT INTO `+table+` (jid, avatar, avatar_id, avatar_picture_id, avatar_status, avatar_updated_at)
		VALUES (?, ?, ?, ?, 'ready', ?) ON CONFLICT(jid) DO UPDATE SET avatar=excluded.avatar, avatar_id=excluded.avatar_id,
		avatar_picture_id=excluded.avatar_picture_id, avatar_status='ready', avatar_updated_at=excluded.avatar_updated_at`,
		jid, "/api/avatars/"+assetID, assetID, pictureID, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *UserDataStore) ClearAvatar(jid string, group bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	table := "contacts"
	if group {
		table = "chats"
	}
	var oldID string
	queryErr := s.db.QueryRow(`SELECT COALESCE(avatar_id,'') FROM `+table+` WHERE jid = ?`, jid).Scan(&oldID)
	if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
		return "", queryErr
	}
	_, err := s.db.Exec(`UPDATE `+table+` SET avatar=NULL, avatar_id=NULL, avatar_picture_id=NULL, avatar_status='removed', avatar_updated_at=? WHERE jid=?`, time.Now().UTC().Format(time.RFC3339), jid)
	return oldID, err
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

func (s *UserDataStore) GetMessagesAnchoredAtOldestUnread(chatJID string, limit int) ([]Message, bool, bool, int64, error) {
	s.mu.RLock()
	var unreadCount int
	err := s.db.QueryRow(`SELECT unread_count FROM chats WHERE jid = ?`, chatJID).Scan(&unreadCount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.mu.RUnlock()
		return nil, false, false, 0, err
	}
	if errors.Is(err, sql.ErrNoRows) || unreadCount <= 0 {
		s.mu.RUnlock()
		messages, hasMore, latestRevision, err := s.GetMessages(chatJID, limit, nil, nil)
		return messages, hasMore, false, latestRevision, err
	}

	var anchorID string
	var anchorEpoch int64
	err = s.db.QueryRow(`
		SELECT id, timestamp_epoch
		FROM messages
		WHERE chat_jid = ? AND is_from_me = 0
		ORDER BY timestamp_epoch DESC, id DESC
		LIMIT 1 OFFSET ?`, chatJID, unreadCount-1).Scan(&anchorID, &anchorEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		// A stale count can exceed the number of stored inbound messages. Anchor
		// at the oldest available inbound message rather than the latest page.
		err = s.db.QueryRow(`
			SELECT id, timestamp_epoch
			FROM messages
			WHERE chat_jid = ? AND is_from_me = 0
			ORDER BY timestamp_epoch ASC, id ASC
			LIMIT 1`, chatJID).Scan(&anchorID, &anchorEpoch)
	}
	if errors.Is(err, sql.ErrNoRows) {
		s.mu.RUnlock()
		messages, hasMore, latestRevision, err := s.GetMessages(chatJID, limit, nil, nil)
		return messages, hasMore, false, latestRevision, err
	}
	if err != nil {
		s.mu.RUnlock()
		return nil, false, false, 0, err
	}

	defer s.mu.RUnlock()

	var latestRevision int64
	if err := s.db.QueryRow(`
		SELECT COALESCE(MAX(revision), 0) FROM messages where chat_jid = ? `, chatJID).Scan(&latestRevision); err != nil {
		return nil, false, false, 0, err
	}
	beforeLimit := limit / 2
	afterLimit := limit - beforeLimit

	older, hasOlder, err := s.getMessagesBeforeAnchor(chatJID, anchorEpoch, anchorID, beforeLimit)
	if err != nil {
		return nil, false, false, 0, err
	}

	newer, hasNewer, err := s.getMessagesFromAnchor(chatJID, anchorEpoch, anchorID, afterLimit)
	if err != nil {
		return nil, false, false, 0, err
	}

	messages := append(older, newer...)
	return messages, hasOlder, hasNewer, latestRevision, nil

}

func (s *UserDataStore) getMessagesBeforeAnchor(chatJID string, anchorEpoch int64, anchorID string, limit int) ([]Message, bool, error) {
	if limit <= 0 {
		return []Message{}, false, nil
	}
	rows, err := s.db.Query(`
		SELECT id, sender_jid, text, timestamp, status, media_type, is_from_me, mentioned_jids, revision
		FROM messages
		WHERE chat_jid = ?
		  AND (timestamp_epoch < ? OR (timestamp_epoch = ? AND id < ?))
		ORDER BY timestamp_epoch DESC, id DESC
		LIMIT ?`,
		chatJID, anchorEpoch, anchorEpoch, anchorID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	messages, err := scanMessageRows(rows, chatJID)
	if err != nil {
		return nil, false, err
	}
	hasOlder := len(messages) > limit
	if hasOlder {
		messages = messages[:limit]
	}
	slices.Reverse(messages)
	return messages, hasOlder, nil
}

func (s *UserDataStore) getMessagesFromAnchor(chatJID string, anchorEpoch int64, anchorID string, limit int) ([]Message, bool, error) {
	if limit <= 0 {
		return []Message{}, false, nil
	}

	rows, err := s.db.Query(`
		SELECT id, sender_jid, text, timestamp, status, media_type, is_from_me, mentioned_jids, revision
		FROM messages
		WHERE chat_jid = ?
		  AND (timestamp_epoch > ? OR (timestamp_epoch = ? AND id >= ?))
		ORDER BY timestamp_epoch ASC, id ASC
		LIMIT ?`, chatJID, anchorEpoch, anchorEpoch, anchorID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	messages, err := scanMessageRows(rows, chatJID)
	if err != nil {
		return nil, false, err
	}

	hasNewer := len(messages) > limit
	if hasNewer {
		messages = messages[:limit]
	}
	return messages, hasNewer, nil
}

func scanMessageRows(rows *sql.Rows, chatJID string) ([]Message, error) {

	var messages []Message
	for rows.Next() {
		var message Message
		var mediaType, mentionedJIDs sql.NullString
		var isFromMe int
		if err := rows.Scan(
			&message.ID,
			&message.SenderID,
			&message.Text,
			&message.Timestamp,
			&message.Status,
			&mediaType,
			&isFromMe,
			&mentionedJIDs,
			&message.Revision,
		); err != nil {
			return nil, err
		}
		message.ChatJID = chatJID
		message.IsFromMe = intToBool(isFromMe)
		message.MentionedJIDs = decodeMentionedJIDs(mentionedJIDs.String)
		if mediaType.Valid {
			message.MediaType = mediaType.String
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil

}

func decodeMentionedJIDs(encoded string) []string {
	mentionedJIDs := []string{}
	if encoded == "" {
		return mentionedJIDs
	}
	if err := json.Unmarshal([]byte(encoded), &mentionedJIDs); err != nil || mentionedJIDs == nil {
		return []string{}
	}
	return mentionedJIDs
}
