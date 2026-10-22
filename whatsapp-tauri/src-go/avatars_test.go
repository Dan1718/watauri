package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAvatarMetadataPersistsOpaqueLocalURLs(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertContact(User{ID: "contact@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertChat(Chat{ID: "group@g.us", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	const contactID = "0123456789abcdef0123456789abcdef"
	const groupID = "fedcba9876543210fedcba9876543210"
	if err := store.SaveAvatar("contact@s.whatsapp.net", contactID, "contact-picture", false); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAvatar("group@g.us", groupID, "group-picture", true); err != nil {
		t.Fatal(err)
	}

	contacts, err := store.GetContacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Avatar != "/api/avatars/"+contactID {
		t.Fatalf("contact avatar URL = %q, want local opaque URL", contacts[0].Avatar)
	}
	assetID, pictureID, err := store.AvatarMetadata("contact@s.whatsapp.net", false)
	if err != nil || assetID != contactID || pictureID != "contact-picture" {
		t.Fatalf("contact avatar metadata = (%q, %q, %v)", assetID, pictureID, err)
	}
	chats, err := store.GetChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].Avatar == nil || *chats[0].Avatar != "/api/avatars/"+groupID {
		t.Fatalf("group avatar = %v, want local opaque URL", chats[0].Avatar)
	}
	removedID, err := store.ClearAvatar("contact@s.whatsapp.net", false)
	if err != nil || removedID != contactID {
		t.Fatalf("cleared contact asset = %q, err = %v", removedID, err)
	}
	contact, found, err := store.ResolveContact("contact@s.whatsapp.net")
	if err != nil || !found || contact.Avatar != "" {
		t.Fatalf("contact lookup after avatar removal = %+v, found=%v, err=%v", contact, found, err)
	}
}

func TestHandleAvatarServesOnlyOpaqueLocalAssets(t *testing.T) {
	newTestStore(t)
	store.avatarDir = t.TempDir()
	const id = "0123456789abcdef0123456789abcdef"
	image := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	if err := os.WriteFile(filepath.Join(store.avatarDir, id), image, 0600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/avatars/"+id, nil)
	response := httptest.NewRecorder()
	handleAvatar(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("GET avatar status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("avatar content type = %q, want image/png", got)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("avatar response is missing nosniff protection")
	}
	if !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatal("avatar response body differs from stored asset")
	}

	for _, path := range []string{
		"/api/avatars/../userdata.db",
		"/api/avatars/0123456789abcdef0123456789abcdef/extra",
		"/api/avatars/" + strings.Repeat("a", 31),
		"/api/avatars/" + strings.Repeat("g", 32),
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handleAvatar(response, req)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/avatars/"+id, nil)
	response = httptest.NewRecorder()
	handleAvatar(response, req)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST avatar status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestAvatarMigrationDropsRemoteURLs(t *testing.T) {
	newTestStore(t)
	if err := store.UpsertContact(User{ID: "contact@s.whatsapp.net", Avatar: "https://cdn.example/avatar.jpg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE contacts SET avatar = 'https://cdn.example/avatar.jpg' WHERE jid = 'contact@s.whatsapp.net'`); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(); err != nil {
		t.Fatal(err)
	}
	contacts, err := store.GetContacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Avatar != "" {
		t.Fatalf("legacy remote avatar was returned: %+v", contacts)
	}
	if strings.HasPrefix(contacts[0].Avatar, "http") {
		t.Fatalf("remote avatar URL escaped migration: %q", contacts[0].Avatar)
	}
}
