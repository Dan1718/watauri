package main

import (
	"encoding/json"
	"testing"
	"time"

	waHistorySync "go.mau.fi/whatsmeow/proto/waHistorySync"
	whatsmeowTypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestGroupMetadataPatchesPreserveOmittedFieldsAndExposeNestedAPI(t *testing.T) {
	newTestStore(t)
	groupJID := "123@g.us"
	parentJID := "456@g.us"
	ownerJID := "owner@s.whatsapp.net"
	createdAt := "2026-07-20T12:30:00Z"
	description := "Original description"
	timer := 86400
	if err := store.UpsertChat(Chat{
		ID:             groupJID,
		IsGroup:        true,
		IsArchived:     true,
		IsArchivedSet:  true,
		IsStarred:      true,
		IsStarredSet:   true,
		IsCommunity:    true,
		IsCommunitySet: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertGroupMetadata(groupJID, GroupMetadataPatch{
		Description:            &description,
		OwnerJID:               &ownerJID,
		CreatedAt:              &createdAt,
		IsLocked:               boolPointer(true),
		IsAnnouncement:         boolPointer(true),
		IsEphemeral:            boolPointer(true),
		DisappearingTimer:      &timer,
		IsJoinApprovalRequired: boolPointer(true),
		ParentGroupJID:         &parentJID,
		LinkedParentJID:        &parentJID,
		IsDefaultSubgroup:      boolPointer(true),
		IsSuspended:            boolPointer(true),
		IsDeleted:              boolPointer(false),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChat(Chat{ID: groupJID}); err != nil {
		t.Fatal(err)
	}

	updatedDescription := "Updated description"
	zeroTimer := 0
	if err := store.UpsertGroupMetadata(groupJID, GroupMetadataPatch{
		Description:       &updatedDescription,
		IsAnnouncement:    boolPointer(false),
		IsEphemeral:       boolPointer(false),
		DisappearingTimer: &zeroTimer,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChat(Chat{ID: "direct@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}

	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	var group, direct *Chat
	for i := range chats {
		switch chats[i].ID {
		case groupJID:
			group = &chats[i]
		case "direct@s.whatsapp.net":
			direct = &chats[i]
		}
	}
	if group == nil || group.GroupMetadata == nil {
		t.Fatalf("group metadata missing from chat response: %#v", group)
	}
	metadata := group.GroupMetadata
	if metadata.Description == nil || *metadata.Description != updatedDescription {
		t.Errorf("description = %v, want %q", metadata.Description, updatedDescription)
	}
	if metadata.OwnerJID == nil || *metadata.OwnerJID != ownerJID || metadata.CreatedAt == nil || *metadata.CreatedAt != createdAt {
		t.Errorf("owner/createdAt were not preserved: owner=%v createdAt=%v", metadata.OwnerJID, metadata.CreatedAt)
	}
	if !metadata.IsLocked || metadata.IsAnnouncement || metadata.IsEphemeral || metadata.DisappearingTimer == nil || *metadata.DisappearingTimer != 0 {
		t.Errorf("updated group flags/timer incorrect: %#v", metadata)
	}
	if !metadata.IsJoinApprovalRequired || metadata.ParentGroupJID == nil || *metadata.ParentGroupJID != parentJID ||
		metadata.LinkedParentJID == nil || *metadata.LinkedParentJID != parentJID || !metadata.IsDefaultSubgroup ||
		!metadata.IsSuspended || metadata.IsDeleted {
		t.Errorf("omitted group metadata was not preserved: %#v", metadata)
	}
	if group.IsAnnouncement || !group.IsSuspended {
		t.Errorf("legacy top-level aliases disagree with nested metadata: announcement=%v suspended=%v", group.IsAnnouncement, group.IsSuspended)
	}
	if !group.IsGroup || !group.IsCommunity || !group.IsArchived || !group.IsStarred {
		t.Errorf("placeholder upsert overwrote existing chat state: %#v", group)
	}
	if direct == nil || direct.GroupMetadata != nil {
		t.Errorf("direct chat should not contain groupMetadata: %#v", direct)
	}

	payload, err := json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if _, ok := response["groupMetadata"]; !ok {
		t.Fatalf("nested groupMetadata missing from JSON: %s", payload)
	}
}

func TestPersistJoinedGroupMetadata(t *testing.T) {
	newTestStore(t)
	groupJID := parseTestJID(t, "123@g.us")
	ownerJID := parseTestJID(t, "owner@s.whatsapp.net")
	parentJID := parseTestJID(t, "456@g.us")
	created := time.Date(2026, 7, 20, 12, 30, 0, 0, time.UTC)
	manager := &WAManager{store: store}
	_, err := manager.persistGroupInfo(whatsmeowTypes.GroupInfo{
		JID:      groupJID,
		OwnerJID: ownerJID,
		GroupName: whatsmeowTypes.GroupName{
			Name: "Design team",
		},
		GroupTopic: whatsmeowTypes.GroupTopic{
			Topic: "Project discussion",
		},
		GroupLocked:                 whatsmeowTypes.GroupLocked{IsLocked: true},
		GroupAnnounce:               whatsmeowTypes.GroupAnnounce{IsAnnounce: true},
		GroupEphemeral:              whatsmeowTypes.GroupEphemeral{IsEphemeral: true, DisappearingTimer: 604800},
		GroupMembershipApprovalMode: whatsmeowTypes.GroupMembershipApprovalMode{IsJoinApprovalRequired: true},
		GroupParent:                 whatsmeowTypes.GroupParent{IsParent: true},
		GroupLinkedParent:           whatsmeowTypes.GroupLinkedParent{LinkedParentJID: parentJID},
		GroupIsDefaultSub:           whatsmeowTypes.GroupIsDefaultSub{IsDefaultSubGroup: true},
		GroupCreated:                created,
		Suspended:                   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("got %d chats, want 1", len(chats))
	}
	chat := chats[0]
	if chat.Name == nil || *chat.Name != "Design team" || !chat.IsCommunity || !chat.IsAnnouncement || !chat.IsSuspended {
		t.Fatalf("group chat summary was not populated: %#v", chat)
	}
	if chat.GroupMetadata == nil || chat.GroupMetadata.Description == nil || *chat.GroupMetadata.Description != "Project discussion" ||
		chat.GroupMetadata.OwnerJID == nil || *chat.GroupMetadata.OwnerJID != ownerJID.String() ||
		chat.GroupMetadata.CreatedAt == nil || *chat.GroupMetadata.CreatedAt != created.Format(time.RFC3339Nano) ||
		!chat.GroupMetadata.IsLocked || !chat.GroupMetadata.IsAnnouncement || !chat.GroupMetadata.IsEphemeral ||
		chat.GroupMetadata.DisappearingTimer == nil || *chat.GroupMetadata.DisappearingTimer != 604800 ||
		!chat.GroupMetadata.IsJoinApprovalRequired || chat.GroupMetadata.LinkedParentJID == nil ||
		*chat.GroupMetadata.LinkedParentJID != parentJID.String() || !chat.GroupMetadata.IsDefaultSubgroup || !chat.GroupMetadata.IsSuspended {
		t.Fatalf("joined-group metadata was not persisted: %#v", chat.GroupMetadata)
	}
}

func TestHistoryConversationMetadataPatchOnlyIncludesPresentFields(t *testing.T) {
	created := uint64(1784550600)
	conv := &waHistorySync.Conversation{
		Description:         proto.String("History description"),
		CreatedBy:           proto.String("creator@s.whatsapp.net"),
		CreatedAt:           &created,
		Locked:              proto.Bool(false),
		EphemeralExpiration: proto.Uint32(3600),
		Suspended:           proto.Bool(true),
		Terminated:          proto.Bool(false),
		ParentGroupID:       proto.String("parent@g.us"),
		IsDefaultSubgroup:   proto.Bool(true),
	}
	patch := groupMetadataUpdateFromHistoryConversation(conv)
	if patch.Description == nil || *patch.Description != "History description" || patch.OwnerJID == nil ||
		*patch.OwnerJID != "creator@s.whatsapp.net" || patch.CreatedAt == nil || patch.IsLocked == nil || *patch.IsLocked ||
		patch.DisappearingTimer == nil || *patch.DisappearingTimer != 3600 || patch.IsEphemeral == nil || !*patch.IsEphemeral ||
		patch.IsSuspended == nil || !*patch.IsSuspended || patch.IsDeleted == nil || *patch.IsDeleted ||
		patch.ParentGroupJID == nil || *patch.ParentGroupJID != "parent@g.us" || patch.IsDefaultSubgroup == nil || !*patch.IsDefaultSubgroup {
		t.Fatalf("history metadata patch missing present fields: %#v", patch)
	}
	if patch.IsAnnouncement != nil || patch.IsJoinApprovalRequired != nil || patch.LinkedParentJID != nil {
		t.Fatalf("history patch should leave absent fields untouched: %#v", patch)
	}

	if got := groupMetadataUpdateFromHistoryConversation(&waHistorySync.Conversation{}); hasGroupMetadataPatch(got) {
		t.Fatalf("empty history conversation should not create/update metadata: %#v", got)
	}
}

func TestHistoryCreatedByOnlyFillsMissingOwner(t *testing.T) {
	newTestStore(t)
	groupJID := "123@g.us"
	actualOwner := "current-owner@s.whatsapp.net"
	historyCreator := "original-creator@s.whatsapp.net"
	if err := store.UpsertGroupMetadata(groupJID, GroupMetadataPatch{OwnerJID: &actualOwner}); err != nil {
		t.Fatal(err)
	}
	patch := groupMetadataUpdateFromHistoryConversation(&waHistorySync.Conversation{
		CreatedBy: proto.String(historyCreator),
	})
	if !patch.OwnerJIDFallback {
		t.Fatal("history CreatedBy should be marked as a fallback")
	}
	if err := store.UpsertGroupMetadata(groupJID, patch); err != nil {
		t.Fatal(err)
	}
	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].GroupMetadata == nil || chats[0].GroupMetadata.OwnerJID == nil || *chats[0].GroupMetadata.OwnerJID != actualOwner {
		t.Fatalf("history creator overwrote known actual owner: %#v", chats)
	}

	missingOwnerGroup := "456@g.us"
	if err := store.UpsertGroupMetadata(missingOwnerGroup, groupMetadataUpdateFromHistoryConversation(&waHistorySync.Conversation{
		CreatedBy: proto.String(historyCreator),
	})); err != nil {
		t.Fatal(err)
	}
	chats, err = store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	for _, chat := range chats {
		if chat.ID == missingOwnerGroup {
			if chat.GroupMetadata == nil || chat.GroupMetadata.OwnerJID == nil || *chat.GroupMetadata.OwnerJID != historyCreator {
				t.Fatalf("history creator was not used as missing-owner fallback: %#v", chat.GroupMetadata)
			}
			return
		}
	}
	t.Fatalf("missing-owner chat %q not returned", missingOwnerGroup)
}

func TestLiveGroupMetadataEventsMergeAndUpdateCommunityLinks(t *testing.T) {
	newTestStore(t)
	groupJID := parseTestJID(t, "123@g.us")
	childJID := parseTestJID(t, "789@g.us")
	manager := &WAManager{store: store}
	if _, err := manager.persistGroupInfo(whatsmeowTypes.GroupInfo{
		JID:         groupJID,
		GroupName:   whatsmeowTypes.GroupName{Name: "Community"},
		GroupParent: whatsmeowTypes.GroupParent{IsParent: true},
		GroupLocked: whatsmeowTypes.GroupLocked{IsLocked: true},
	}); err != nil {
		t.Fatal(err)
	}
	manager.handleGroupInfoEvent(&events.GroupInfo{
		JID:       groupJID,
		Topic:     &whatsmeowTypes.GroupTopic{Topic: "New description"},
		Locked:    &whatsmeowTypes.GroupLocked{IsLocked: false},
		Suspended: true,
		Link: &whatsmeowTypes.GroupLinkChange{
			Type:  whatsmeowTypes.GroupLinkChangeTypeSub,
			Group: whatsmeowTypes.GroupLinkTarget{JID: childJID},
		},
	})

	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	var community, child *Chat
	for i := range chats {
		switch chats[i].ID {
		case groupJID.String():
			community = &chats[i]
		case childJID.String():
			child = &chats[i]
		}
	}
	if community == nil || community.GroupMetadata == nil || community.GroupMetadata.Description == nil ||
		*community.GroupMetadata.Description != "New description" || community.GroupMetadata.IsLocked || !community.GroupMetadata.IsSuspended {
		t.Fatalf("live group event was not applied or wiped existing fields: %#v", community)
	}
	if child == nil || child.GroupMetadata == nil || child.GroupMetadata.LinkedParentJID == nil ||
		*child.GroupMetadata.LinkedParentJID != groupJID.String() || child.GroupMetadata.ParentGroupJID == nil ||
		*child.GroupMetadata.ParentGroupJID != groupJID.String() {
		t.Fatalf("live community link was not persisted: %#v", child)
	}

	manager.handleGroupInfoEvent(&events.GroupInfo{
		JID: groupJID,
		Unlink: &whatsmeowTypes.GroupLinkChange{
			Type:         whatsmeowTypes.GroupLinkChangeTypeSub,
			UnlinkReason: whatsmeowTypes.GroupUnlinkReasonDefault,
			Group:        whatsmeowTypes.GroupLinkTarget{JID: childJID},
		},
	})
	chats, err = store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	for i := range chats {
		if chats[i].ID == childJID.String() {
			if chats[i].GroupMetadata == nil || chats[i].GroupMetadata.ParentGroupJID == nil || *chats[i].GroupMetadata.ParentGroupJID != "" ||
				chats[i].GroupMetadata.LinkedParentJID == nil || *chats[i].GroupMetadata.LinkedParentJID != "" {
				t.Fatalf("unlink event did not clear the child relationship: %#v", chats[i].GroupMetadata)
			}
			return
		}
	}
	t.Fatal("child chat disappeared after unlink event")
}

func parseTestJID(t *testing.T, jid string) whatsmeowTypes.JID {
	t.Helper()
	parsed, err := whatsmeowTypes.ParseJID(jid)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
