package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func participantRanks(t *testing.T, group string) map[string]int {
	t.Helper()
	rows, err := store.db.Query(`SELECT user_jid, rank FROM chat_participants WHERE chat_jid = ?`, group)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := map[string]int{}
	for rows.Next() {
		var jid string
		var rank int
		if err := rows.Scan(&jid, &rank); err != nil {
			t.Fatal(err)
		}
		result[jid] = rank
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHistoryParticipantsAreGroupScopedAndConservative(t *testing.T) {
	newTestStore(t)
	manager := &WAManager{store: store}
	for _, group := range []string{"one@g.us", "two@g.us"} {
		if err := store.UpsertChat(Chat{ID: group, IsGroup: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertChatParticipant("one@g.us", "existing@s.whatsapp.net", 2); err != nil {
		t.Fatal(err)
	}
	conv := &waHistorySync.Conversation{ID: proto.String("one@g.us"), Participant: []*waHistorySync.GroupParticipant{
		{UserJID: proto.String("existing@s.whatsapp.net")},
		{UserJID: proto.String("admin@s.whatsapp.net"), Rank: waHistorySync.GroupParticipant_ADMIN.Enum()},
		{UserJID: proto.String("unknown@lid")},
		{UserJID: proto.String("bad@g.us")},
		nil,
	}}
	manager.persistHistoryParticipants(conv)
	manager.persistHistoryParticipants(conv)
	manager.persistHistoryParticipants(&waHistorySync.Conversation{ID: conv.ID})
	manager.persistHistoryParticipants(nil)
	got := participantRanks(t, "one@g.us")
	if len(got) != 3 || got["existing@s.whatsapp.net"] != 2 || got["admin@s.whatsapp.net"] != 1 {
		t.Fatalf("unexpected history participants: %#v", got)
	}
	if _, ok := got["unknown@lid"]; !ok {
		t.Fatal("unmapped LID missing")
	}
	if got := participantRanks(t, "two@g.us"); len(got) != 0 {
		t.Fatalf("wrong group modified: %#v", got)
	}
	if err := store.UpsertChat(Chat{ID: "direct@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}
	conv.ID = proto.String("direct@s.whatsapp.net")
	manager.persistHistoryParticipants(conv)
	if got := participantRanks(t, conv.GetID()); len(got) != 0 {
		t.Fatalf("direct participants modified: %#v", got)
	}
}

func TestLiveParticipantsAndChatInvalidation(t *testing.T) {
	newTestStore(t)
	manager := &WAManager{store: store}
	group := parseTestJID(t, "one@g.us")
	member := parseTestJID(t, "member@s.whatsapp.net")
	if err := store.UpsertChat(Chat{ID: "two@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChatParticipant("two@g.us", member.String(), 2); err != nil {
		t.Fatal(err)
	}
	updates, unsubscribe := realtimeHub.Subscribe()
	defer unsubscribe()
	for _, step := range []struct {
		name   string
		event  events.GroupInfo
		want   int
		absent bool
	}{
		{"join", events.GroupInfo{Join: []types.JID{member}}, 0, false},
		{"promote", events.GroupInfo{Promote: []types.JID{member}}, 1, false},
		{"demote", events.GroupInfo{Demote: []types.JID{member}}, 0, false},
		{"leave", events.GroupInfo{Leave: []types.JID{member}}, 0, true},
	} {
		t.Run(step.name, func(t *testing.T) {
			step.event.JID = group
			manager.handleGroupInfoEvent(&step.event)
			got := participantRanks(t, group.String())
			value, ok := got[member.String()]
			if ok == step.absent || value != step.want {
				t.Fatalf("participants: %#v", got)
			}
			select {
			case update := <-updates:
				if update.Type != "chat" || update.ChatID != group.String() {
					t.Fatalf("unexpected invalidation: %#v", update)
				}
			case <-time.After(time.Second):
				t.Fatal("missing chat invalidation")
			}
		})
	}
	if got := participantRanks(t, "two@g.us"); got[member.String()] != 2 {
		t.Fatalf("other group modified: %#v", got)
	}
}

func TestParticipantMappingsMergeAliasesAndAcceptAllSources(t *testing.T) {
	newTestStore(t)
	manager := &WAManager{store: store}
	group := "one@g.us"
	if err := store.UpsertChat(Chat{ID: group, IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	for _, participant := range []struct {
		jid  string
		rank int
	}{{"111@lid", 2}, {"123@s.whatsapp.net", 0}} {
		if err := store.UpsertChatParticipant(group, participant.jid, participant.rank); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertJIDMapping("111@lid", "123@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group); len(got) != 1 || got["123@s.whatsapp.net"] != 2 {
		t.Fatalf("aliases not merged: %#v", got)
	}
	manager.persistHistoryParticipants(&waHistorySync.Conversation{ID: proto.String(group), Participant: []*waHistorySync.GroupParticipant{{UserJID: proto.String("111@lid")}}})
	if got := participantRanks(t, group); len(got) != 1 || got["123@s.whatsapp.net"] != 2 {
		t.Fatalf("history downgraded rank: %#v", got)
	}
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: parseTestJID(t, group), Demote: []types.JID{parseTestJID(t, "111@lid")}})
	if got := participantRanks(t, group); len(got) != 1 || got["123@s.whatsapp.net"] != 0 {
		t.Fatalf("mapped demotion failed: %#v", got)
	}
	if err := store.UpsertChatParticipant(group, "123:4@c.us", 1); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group); len(got) != 1 || got["123@s.whatsapp.net"] != 1 {
		t.Fatalf("legacy/device normalization failed: %#v", got)
	}
	if err := store.DeleteChatParticipant(group, "111@lid"); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group); len(got) != 0 {
		t.Fatalf("mapped deletion failed: %#v", got)
	}

	// Joined-group snapshots carrying both identities remain idempotent.
	snapshot := types.GroupInfo{JID: parseTestJID(t, group), Participants: []types.GroupParticipant{{
		JID:         parseTestJID(t, "222@lid"), // Some snapshots omit the dedicated LID field.
		PhoneNumber: parseTestJID(t, "456@s.whatsapp.net"), IsAdmin: true,
	}}}
	for i := 0; i < 2; i++ {
		if _, err := manager.persistGroupInfo(snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if got := participantRanks(t, group); len(got) != 1 || got["456@s.whatsapp.net"] != 1 {
		t.Fatalf("snapshot not idempotent: %#v", got)
	}
}

func TestChatAPIParticipantsIncludeRanksAndOptionalContactEnrichment(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertChat(Chat{ID: "one@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChat(Chat{ID: "direct@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertJIDMapping("111@lid", "123@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertContact(User{ID: "111@lid", Name: "Alice", PushName: "Ali"}); err != nil {
		t.Fatal(err)
	}
	for _, jid := range []string{"111@lid", "unmapped@lid", "456@s.whatsapp.net"} {
		if err := store.UpsertChatParticipant("one@g.us", jid, 0); err != nil {
			t.Fatal(err)
		}
	}
	recorder := httptest.NewRecorder()
	handleChats(recorder, httptest.NewRequest(http.MethodGet, "/api/chats", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", recorder.Code, recorder.Body)
	}
	var chats []Chat
	if err := json.Unmarshal(recorder.Body.Bytes(), &chats); err != nil {
		t.Fatal(err)
	}
	for _, chat := range chats {
		if !chat.IsGroup {
			for _, participant := range chat.Participants {
				if participant.Rank != nil {
					t.Fatal("rank leaked to direct chat")
				}
			}
			continue
		}
		if len(chat.Participants) != 3 {
			t.Fatalf("missing participants: %#v", chat.Participants)
		}
		for _, participant := range chat.Participants {
			if participant.Rank == nil || *participant.Rank != 0 || participant.Name == "" {
				t.Fatalf("missing rank/fallback: %#v", participant)
			}
			if participant.ID == "123@s.whatsapp.net" && (participant.Name != "Alice" || participant.LIDJID != "111@lid") {
				t.Fatalf("missing enrichment: %#v", participant)
			}
		}
	}
}

func TestDepartureBlocksHistoryAndRoleChangesUntilRejoin(t *testing.T) {
	newTestStore(t)
	manager := &WAManager{store: store}
	group := parseTestJID(t, "one@g.us")
	member := parseTestJID(t, "member@s.whatsapp.net")
	conv := &waHistorySync.Conversation{ID: proto.String(group.String()), Participant: []*waHistorySync.GroupParticipant{{UserJID: proto.String(member.String())}}}
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Join: []types.JID{member}})
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{member}})
	// Repeated leaves are safe; absent members must be protected too.
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{member}})
	manager.persistHistoryParticipants(conv)
	manager.persistHistoryParticipants(conv)
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Promote: []types.JID{member}})
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Demote: []types.JID{member}})
	if got := participantRanks(t, group.String()); len(got) != 0 {
		t.Fatalf("departed member resurrected: %#v", got)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM participant_departures WHERE chat_jid = ?`, group.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("departure count = %d, want 1", count)
	}
	// The same person can still be a member of another group.
	if err := store.UpsertChat(Chat{ID: "two@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	other := proto.Clone(conv).(*waHistorySync.Conversation)
	other.ID = proto.String("two@g.us")
	manager.persistHistoryParticipants(other)
	if got := participantRanks(t, "two@g.us"); len(got) != 1 {
		t.Fatalf("departure leaked to another group: %#v", got)
	}
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Join: []types.JID{member}})
	manager.persistHistoryParticipants(conv)
	if got := participantRanks(t, group.String()); len(got) != 1 {
		t.Fatalf("explicit rejoin not persisted: %#v", got)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM participant_departures WHERE chat_jid = ?`, group.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rejoin did not clear departure marker")
	}
}

func TestSnapshotFreshnessGuardsDepartures(t *testing.T) {
	newTestStore(t)
	manager := &WAManager{store: store}
	group := parseTestJID(t, "one@g.us")
	member := parseTestJID(t, "member@s.whatsapp.net")
	if err := store.UpsertChat(Chat{ID: group.String(), IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	beforeLeave, err := store.participantRevision()
	if err != nil {
		t.Fatal(err)
	}
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{member}})
	snapshot := types.GroupInfo{JID: group, Participants: []types.GroupParticipant{{JID: member, IsAdmin: true}}}
	// A response for a request begun before the leave must not restore membership.
	if _, err := manager.persistGroupInfoAtRevision(snapshot, &beforeLeave); err != nil {
		t.Fatal(err)
	}
	// Unsolicited snapshots have no reliable request-start boundary.
	if _, err := manager.persistGroupInfo(snapshot); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group.String()); len(got) != 0 {
		t.Fatalf("stale/unknown snapshot restored departed member: %#v", got)
	}
	afterLeave, err := store.participantRevision()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.persistGroupInfoAtRevision(snapshot, &afterLeave); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group.String()); len(got) != 1 || got[member.String()] != 1 {
		t.Fatalf("fresh snapshot failed to restore member: %#v", got)
	}
	// Clearing the old marker must not reset the logical clock.
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{member}})
	if _, err := manager.persistGroupInfoAtRevision(snapshot, &afterLeave); err != nil {
		t.Fatal(err)
	}
	if got := participantRanks(t, group.String()); len(got) != 0 {
		t.Fatalf("snapshot crossed a second departure: %#v", got)
	}
}

func TestDepartureMappingsPreventAliasResurrection(t *testing.T) {
	for _, leaveAsLID := range []bool{true, false} {
		t.Run(map[bool]string{true: "leave_as_lid", false: "leave_as_phone"}[leaveAsLID], func(t *testing.T) {
			newTestStore(t)
			manager := &WAManager{store: store}
			group := parseTestJID(t, "one@g.us")
			lid := parseTestJID(t, "111@lid")
			phone := parseTestJID(t, "123@s.whatsapp.net")
			departed, historical := phone, lid
			if leaveAsLID {
				departed, historical = lid, phone
			}
			manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{departed}})
			// Without a mapping, these cannot safely be identified as one person.
			conv := &waHistorySync.Conversation{ID: proto.String(group.String()), Participant: []*waHistorySync.GroupParticipant{{UserJID: proto.String(historical.String())}}}
			manager.persistHistoryParticipants(conv)
			if got := participantRanks(t, group.String()); len(got) != 1 {
				t.Fatalf("valid unmapped fallback lost: %#v", got)
			}
			if err := store.UpsertJIDMapping(lid.String(), phone.String()); err != nil {
				t.Fatal(err)
			}
			manager.persistHistoryParticipants(conv)
			if got := participantRanks(t, group.String()); len(got) != 0 {
				t.Fatalf("mapped alias resurrected departed member: %#v", got)
			}
			var user string
			if err := store.db.QueryRow(`SELECT user_jid FROM participant_departures WHERE chat_jid = ?`, group.String()).Scan(&user); err != nil {
				t.Fatal(err)
			}
			if user != phone.String() {
				t.Fatalf("departure alias = %s, want phone", user)
			}
			manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Join: []types.JID{lid}})
			if got := participantRanks(t, group.String()); len(got) != 1 {
				t.Fatalf("mapped rejoin failed: %#v", got)
			}
		})
	}
}

func TestDeparturePersistsAcrossStoreRestart(t *testing.T) {
	oldStore := store
	t.Cleanup(func() { store = oldStore })
	path := filepath.Join(t.TempDir(), "participants.db")
	open := func() *UserDataStore {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			t.Fatal(err)
		}
		s := &UserDataStore{db: db}
		if err := s.migrate(); err != nil {
			t.Fatal(err)
		}
		return s
	}
	store = open()
	manager := &WAManager{store: store}
	group := parseTestJID(t, "one@g.us")
	member := parseTestJID(t, "member@lid")
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Leave: []types.JID{member}})
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	store = open()
	t.Cleanup(func() { store.db.Close() })
	manager = &WAManager{store: store}
	manager.persistHistoryParticipants(&waHistorySync.Conversation{ID: proto.String(group.String()), Participant: []*waHistorySync.GroupParticipant{{UserJID: proto.String(member.String())}}})
	if got := participantRanks(t, group.String()); len(got) != 0 {
		t.Fatalf("restart lost departure marker: %#v", got)
	}
	if revision, err := store.participantRevision(); err != nil || revision == 0 {
		t.Fatalf("restart reset revision: %d, %v", revision, err)
	}
	manager.handleGroupInfoEvent(&events.GroupInfo{JID: group, Join: []types.JID{member}})
	if got := participantRanks(t, group.String()); len(got) != 1 {
		t.Fatalf("rejoin after restart failed: %#v", got)
	}
}

func TestDepartureAndMembershipMutationAreAtomic(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertChat(Chat{ID: "one@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChatParticipant("one@g.us", "member@lid", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_member_delete BEFORE DELETE ON chat_participants BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteChatParticipant("one@g.us", "member@lid"); err == nil {
		t.Fatal("expected deletion failure")
	}
	if got := participantRanks(t, "one@g.us"); len(got) != 1 {
		t.Fatal("failed leave deleted member")
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM participant_departures`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed leave persisted a departure marker")
	}
	if _, err := store.db.Exec(`DROP TRIGGER fail_member_delete`); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteChatParticipant("one@g.us", "member@lid"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_member_insert BEFORE INSERT ON chat_participants BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChatParticipant("one@g.us", "member@lid", 0); err == nil {
		t.Fatal("expected insertion failure")
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM participant_departures`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("failed rejoin cleared departure marker")
	}
}

func TestMigrationReconcilesExistingParticipantAliases(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertChat(Chat{ID: "one@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChat(Chat{ID: "111@lid"}); err != nil {
		t.Fatal(err)
	}
	for _, jid := range []string{"111@lid", "123@s.whatsapp.net"} {
		if err := store.UpsertContact(User{ID: jid}); err != nil {
			t.Fatal(err)
		}
	}
	// Reproduce the old storage representation, bypassing normalized writers.
	if _, err := store.db.Exec(`INSERT INTO chat_participants (chat_jid, user_jid, rank) VALUES ('one@g.us', '111@lid', 2), ('one@g.us', '123@s.whatsapp.net', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO jid_mappings (lid_jid, phone_jid) VALUES ('111@lid', '123@s.whatsapp.net')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := store.migrate(); err != nil {
			t.Fatal(err)
		}
	}
	if got := participantRanks(t, "one@g.us"); len(got) != 1 || got["123@s.whatsapp.net"] != 2 {
		t.Fatalf("migration failed to reconcile aliases: %#v", got)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chats WHERE jid = '111@lid'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("participant migration changed direct-chat identity")
	}
}

func TestContactAndParticipantResponsesIncludePhoneAliasesAndSavedName(t *testing.T) {
	newTestStore(t)
	const lid = "111@lid"
	const phone = "919876543210@s.whatsapp.net"
	if err := store.UpsertContact(User{ID: lid, PushName: "~Yashaswi"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertContact(User{ID: phone, Name: "Yashaswi Priya IIITH+0"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertJIDMapping(lid, phone); err != nil {
		t.Fatal(err)
	}
	contacts, err := store.GetContacts()
	if err != nil {
		t.Fatal(err)
	}
	var mapped *User
	for i := range contacts {
		if contacts[i].ID == lid {
			mapped = &contacts[i]
			break
		}
	}
	if mapped == nil || mapped.PhoneJID != phone || mapped.PhoneNumber != "919876543210" {
		t.Fatalf("mapped contact = %+v, want phone alias and phone number", mapped)
	}
	if err := store.UpsertChat(Chat{ID: "group@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO chat_participants (chat_jid, user_jid, rank) VALUES (?, ?, 0)`, "group@g.us", lid); err != nil {
		t.Fatal(err)
	}
	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || len(chats[0].Participants) != 1 {
		t.Fatalf("chats/participants = %+v, want one group participant", chats)
	}
	participant := chats[0].Participants[0]
	if participant.Name != "Yashaswi Priya IIITH+0" || participant.PushName != "~Yashaswi" || !participant.IsSaved {
		t.Fatalf("participant = %+v, want saved name, separate push name, and isSaved", participant)
	}
}

func TestParticipantMappingLookupFailureKeepsValidFallback(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertChat(Chat{ID: "one@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	// Simulate a mapping-store failure without breaking participant storage.
	if _, err := store.db.Exec(`DROP TABLE jid_mappings`); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChatParticipant("one@g.us", "111:3@lid", 0); err != nil {
		t.Fatal(err)
	}
	got := participantRanks(t, "one@g.us")
	if _, ok := got["111@lid"]; !ok {
		t.Fatalf("fallback was not retained: %#v", got)
	}
}
