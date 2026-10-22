package main

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func TestInsertHistoryMessagesPreservesDuplicatesRevisionsAndChatSummary(t *testing.T) {
	newTestStore(t)
	messages := []Message{
		{ID: "m1", ChatJID: "c1", SenderID: "sender", Text: "first", Timestamp: "2025-01-01T00:00:00Z", Status: "received", MentionedJIDs: []string{"a@s.whatsapp.net"}},
		{ID: "m2", ChatJID: "c1", SenderID: "sender", Text: "last", Timestamp: "2025-01-01T00:00:02Z", Status: "received"},
	}
	inserted, err := store.InsertHistoryMessages(messages, map[string]string{"sender": "Sender"})
	if err != nil || inserted != 2 {
		t.Fatalf("inserted = %d, err = %v; want 2 messages", inserted, err)
	}
	inserted, err = store.InsertHistoryMessages(messages, nil)
	if err != nil || inserted != 0 {
		t.Fatalf("duplicate insert count = %d, err = %v; want 0", inserted, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_jid = 'c1'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("message count = %d, err = %v; want 2", count, err)
	}
	var revision1, revision2 int64
	if err := store.db.QueryRow(`SELECT revision FROM messages WHERE id='m1'`).Scan(&revision1); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT revision FROM messages WHERE id='m2'`).Scan(&revision2); err != nil {
		t.Fatal(err)
	}
	if revision1 == 0 || revision2 != revision1+1 {
		t.Fatalf("revisions = %d, %d; want consecutive nonzero revisions", revision1, revision2)
	}
	var lastID, mention string
	if err := store.db.QueryRow(`SELECT last_message_id FROM chats WHERE jid='c1'`).Scan(&lastID); err != nil || lastID != "m2" {
		t.Fatalf("last message = %q, err = %v; want m2", lastID, err)
	}
	if err := store.db.QueryRow(`SELECT mentioned_jids FROM messages WHERE id='m1'`).Scan(&mention); err != nil || mention != `["a@s.whatsapp.net"]` {
		t.Fatalf("mentions = %q, err = %v", mention, err)
	}
	var pushName string
	if err := store.db.QueryRow(`SELECT push_name FROM contacts WHERE jid='sender'`).Scan(&pushName); err != nil || pushName != "Sender" {
		t.Fatalf("push name = %q, err = %v; want Sender", pushName, err)
	}
}

func TestWriteHistoryParticipantsKeepsHistoryInsertOnlyAndDepartures(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertChat(Chat{ID: "g@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.writeParticipant("g@g.us", "admin@s.whatsapp.net", 2, participantWrite{source: participantLive}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO participant_departures (chat_jid, user_jid, revision) VALUES ('g@g.us', 'left@s.whatsapp.net', 3)`); err != nil {
		t.Fatal(err)
	}
	participants := []historyParticipant{
		{jid: "admin@s.whatsapp.net", rank: 0},
		{jid: "new@s.whatsapp.net", rank: 1},
		{jid: "left@s.whatsapp.net", rank: 2},
	}
	if err := store.writeHistoryParticipants("g@g.us", participants); err != nil {
		t.Fatal(err)
	}
	var adminRank, newRank, leftCount int
	if err := store.db.QueryRow(`SELECT rank FROM chat_participants WHERE chat_jid='g@g.us' AND user_jid='admin@s.whatsapp.net'`).Scan(&adminRank); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT rank FROM chat_participants WHERE chat_jid='g@g.us' AND user_jid='new@s.whatsapp.net'`).Scan(&newRank); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chat_participants WHERE chat_jid='g@g.us' AND user_jid='left@s.whatsapp.net'`).Scan(&leftCount); err != nil {
		t.Fatal(err)
	}
	if adminRank != 2 || newRank != 1 || leftCount != 0 {
		t.Fatalf("admin rank=%d, new rank=%d, departed rows=%d", adminRank, newRank, leftCount)
	}
}

func benchmarkStore(b *testing.B) *UserDataStore {
	b.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	b.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		b.Fatal(err)
	}
	s := &UserDataStore{db: db}
	if err := s.migrate(); err != nil {
		b.Fatal(err)
	}
	return s
}

func benchmarkMessages(i int, count int) []Message {
	messages := make([]Message, count)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for j := range messages {
		messages[j] = Message{
			ID: fmt.Sprintf("m-%d-%d", i, j), ChatJID: "c1", SenderID: "sender",
			Text: "benchmark history message", Timestamp: base.Add(time.Duration(j) * time.Second).Format(time.RFC3339Nano),
			Status: "received", MentionedJIDs: []string{},
		}
	}
	return messages
}

func BenchmarkHistoryMessagePersistence(b *testing.B) {
	const batchSize = 100
	b.Run("per_message_transaction", func(b *testing.B) {
		s := benchmarkStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, message := range benchmarkMessages(i, batchSize) {
				if err := s.InsertMessage(message, false); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("conversation_batch", func(b *testing.B) {
		s := benchmarkStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := s.InsertHistoryMessages(benchmarkMessages(i, batchSize), nil); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkHistoryParticipantPersistence(b *testing.B) {
	const batchSize = 100
	participants := make([]historyParticipant, batchSize)
	for i := range participants {
		participants[i] = historyParticipant{jid: fmt.Sprintf("user-%d@s.whatsapp.net", i), rank: i % 3}
	}
	b.Run("per_participant_transaction", func(b *testing.B) {
		s := benchmarkStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			group := fmt.Sprintf("group-%d@g.us", i)
			b.StopTimer()
			if err := s.UpsertChat(Chat{ID: group, IsGroup: true}); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			for _, participant := range participants {
				if err := s.writeParticipant(group, participant.jid, participant.rank, participantWrite{source: participantHistory}); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("conversation_batch", func(b *testing.B) {
		s := benchmarkStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			group := fmt.Sprintf("group-%d@g.us", i)
			b.StopTimer()
			if err := s.UpsertChat(Chat{ID: group, IsGroup: true}); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			if err := s.writeHistoryParticipants(group, participants); err != nil {
				b.Fatal(err)
			}
		}
	})
}
