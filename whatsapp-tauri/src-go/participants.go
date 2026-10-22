package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Participant normalization is deliberately separate from direct-chat identity.
func normalizeParticipantJID(raw string) (string, error) {
	jid, err := types.ParseJID(raw)
	if err != nil {
		return "", err
	}
	jid = jid.ToNonAD()
	if jid.Server == types.LegacyUserServer {
		jid.Server = types.DefaultUserServer
	}
	if jid.User == "" || (jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
		return "", fmt.Errorf("invalid participant JID %q", raw)
	}
	return jid.String(), nil
}

// mergeParticipantAlias also handles mappings learned after participants were
// stored. Preserve richer ranks when two previously independent identities merge;
// an authoritative live/snapshot update can subsequently set any rank.
func mergeParticipantAlias(tx *sql.Tx, lid, phone, group string) error {
	var valid bool
	lid, phone, valid = normalizedParticipantMapping(lid, phone)
	if !valid {
		return nil
	}
	// Unknown aliases may have accumulated conflicting membership evidence.
	// Conservatively retain departure markers until a known join/fresh snapshot.
	if _, err := tx.Exec(`INSERT INTO participant_departures (chat_jid, user_jid, revision)
		SELECT chat_jid, ?, revision FROM participant_departures WHERE user_jid = ? AND (? = '' OR chat_jid = ?)
		ON CONFLICT(chat_jid, user_jid) DO UPDATE SET revision = MAX(participant_departures.revision, excluded.revision)`,
		phone, lid, group, group); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM participant_departures WHERE user_jid = ? AND (? = '' OR chat_jid = ?)`, lid, group, group); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO contacts (jid) SELECT ? WHERE EXISTS (
		SELECT 1 FROM chat_participants WHERE user_jid = ? AND (? = '' OR chat_jid = ?)
		) ON CONFLICT(jid) DO NOTHING`, phone, lid, group, group); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO chat_participants (chat_jid, user_jid, rank, updated_at)
		SELECT chat_jid, ?, rank, updated_at FROM chat_participants WHERE user_jid = ? AND (? = '' OR chat_jid = ?)
		ON CONFLICT(chat_jid, user_jid) DO UPDATE SET
			rank = MAX(chat_participants.rank, excluded.rank),
			updated_at = MAX(chat_participants.updated_at, excluded.updated_at)`, phone, lid, group, group)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM chat_participants WHERE user_jid = ? AND (? = '' OR chat_jid = ?)`, lid, group, group)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM chat_participants WHERE user_jid = ? AND (? = '' OR chat_jid = ?)
		AND EXISTS (SELECT 1 FROM participant_departures d WHERE d.chat_jid = chat_participants.chat_jid AND d.user_jid = chat_participants.user_jid)`, phone, group, group)
	return err
}

func normalizedParticipantMapping(lid, phone string) (string, string, bool) {
	lid, lidErr := normalizeParticipantJID(lid)
	phone, phoneErr := normalizeParticipantJID(phone)
	return lid, phone, lidErr == nil && phoneErr == nil &&
		strings.HasSuffix(lid, "@"+types.HiddenUserServer) && strings.HasSuffix(phone, "@"+types.DefaultUserServer)
}

// Older versions could persist both forms even when the mapping was already
// known. Reconcile them on startup, without touching direct-chat identity.
func migrateParticipantAliases(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT lid_jid, phone_jid FROM jid_mappings m
		WHERE EXISTS (SELECT 1 FROM chat_participants p WHERE p.user_jid = m.lid_jid)
		OR EXISTS (SELECT 1 FROM participant_departures d WHERE d.user_jid = m.lid_jid)`)
	if err != nil {
		return err
	}
	var mappings [][2]string
	for rows.Next() {
		var mapping [2]string
		if err := rows.Scan(&mapping[0], &mapping[1]); err != nil {
			rows.Close()
			return err
		}
		mappings = append(mappings, mapping)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, mapping := range mappings {
		if err := mergeParticipantAlias(tx, mapping[0], mapping[1], ""); err != nil {
			return err
		}
	}
	return nil
}

type participantSource int

const (
	participantLive participantSource = iota // Explicit join or authoritative local upsert.
	participantHistory
	participantSnapshot
	participantRoleChange // Promotion/demotion is not an explicit rejoin.
)

type participantWrite struct {
	source participantSource
	remove bool
	// Captured before requesting a snapshot; nil means freshness is unknown.
	snapshotRevision *int64
}

func (s *UserDataStore) participantRevision() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var revision int64
	err := s.db.QueryRow(`SELECT revision FROM participant_revision WHERE singleton = 1`).Scan(&revision)
	return revision, err
}

// writeParticipant serializes mapping resolution and mutation so a concurrently
// learned mapping cannot leave behind an alias row. History is insert-only: old
// chunks must not overwrite ranks supplied by snapshots or live events.
func (s *UserDataStore) writeParticipant(chatJID, raw string, rank int, write participantWrite) error {
	group, err := types.ParseJID(chatJID)
	if err != nil || group.User == "" || group.Server != types.GroupServer {
		return fmt.Errorf("invalid participant group %q", chatJID)
	}
	user, err := normalizeParticipantJID(raw)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lid, phone string
	err = tx.QueryRow(`SELECT lid_jid, phone_jid FROM jid_mappings WHERE lid_jid = ? OR phone_jid = ?`, user, user).Scan(&lid, &phone)
	if err != nil && err != sql.ErrNoRows {
		// Mapping lookup must not prevent persisting a valid fallback identity.
		log.Printf("[store] participant mapping lookup %s: %v", user, err)
	}
	if err == nil {
		if mappedLID, mappedPhone, valid := normalizedParticipantMapping(lid, phone); valid {
			if err := mergeParticipantAlias(tx, mappedLID, mappedPhone, group.ToNonAD().String()); err != nil {
				return err
			}
			user = mappedPhone
		}
	}
	chatJID = group.ToNonAD().String()
	if write.remove {
		var revision int64
		if err := tx.QueryRow(`UPDATE participant_revision SET revision = revision + 1 WHERE singleton = 1 RETURNING revision`).Scan(&revision); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO participant_departures (chat_jid, user_jid, revision) VALUES (?, ?, ?)
			ON CONFLICT(chat_jid, user_jid) DO UPDATE SET revision = excluded.revision`, chatJID, user, revision); err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM chat_participants WHERE chat_jid = ? AND user_jid = ?`, chatJID, user)
	} else {
		var departureRevision int64
		departureErr := tx.QueryRow(`SELECT revision FROM participant_departures WHERE chat_jid = ? AND user_jid = ?`, chatJID, user).Scan(&departureRevision)
		if departureErr != nil && departureErr != sql.ErrNoRows {
			return departureErr
		}
		if departureErr == nil && (write.source == participantHistory || write.source == participantRoleChange ||
			(write.source == participantSnapshot && (write.snapshotRevision == nil || departureRevision > *write.snapshotRevision))) {
			return tx.Commit()
		}
		if _, err := tx.Exec(`DELETE FROM participant_departures WHERE chat_jid = ? AND user_jid = ?`, chatJID, user); err != nil {
			return err
		}
		// The schema references contacts, but contact enrichment is optional.
		if _, err := tx.Exec(`INSERT INTO contacts (jid) VALUES (?) ON CONFLICT(jid) DO NOTHING`, user); err != nil {
			return err
		}
		conflict := `DO UPDATE SET rank = excluded.rank, updated_at = excluded.updated_at`
		if write.source == participantHistory {
			conflict = `DO NOTHING`
		}
		_, err = tx.Exec(`INSERT INTO chat_participants (chat_jid, user_jid, rank, updated_at)
			VALUES (?, ?, ?, ?) ON CONFLICT(chat_jid, user_jid) `+conflict,
			chatJID, user, rank, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (wa *WAManager) persistHistoryParticipants(conv *waHistorySync.Conversation) {
	if conv == nil || wa.store == nil {
		return
	}
	for _, participant := range conv.GetParticipant() {
		if participant == nil {
			continue
		}
		if err := wa.store.writeParticipant(conv.GetID(), participant.GetUserJID(), int(participant.GetRank()), participantWrite{source: participantHistory}); err != nil {
			log.Printf("[wa] failed to persist history participant in %s: %v", conv.GetID(), err)
		}
	}
}

func (wa *WAManager) persistLiveParticipants(evt *events.GroupInfo) {
	for _, change := range []struct {
		members []types.JID
		rank    int
		remove  bool
		source  participantSource
	}{
		{evt.Join, 0, false, participantLive},
		{evt.Promote, 1, false, participantRoleChange},
		{evt.Demote, 0, false, participantRoleChange},
		{evt.Leave, 0, true, participantLive},
	} {
		for _, jid := range change.members {
			if err := wa.store.writeParticipant(evt.JID.String(), jid.String(), change.rank, participantWrite{source: change.source, remove: change.remove}); err != nil {
				log.Printf("[wa] failed to persist live participant %s in %s: %v", jid, evt.JID, err)
			}
		}
	}
}
