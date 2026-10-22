package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const maxAvatarBytes = 8 << 20
const avatarRefreshTimeout = 30 * time.Second

var avatarIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s *UserDataStore) avatarPath(id string) (string, error) {
	if !avatarIDPattern.MatchString(id) {
		return "", os.ErrNotExist
	}
	dir := s.avatarDir
	if dir == "" {
		dir = "avatars"
	}
	return filepath.Join(dir, id), nil
}

func newAvatarID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func (wa *WAManager) lockAvatar(jid types.JID) func() {
	value, _ := wa.avatarLocks.LoadOrStore(jid.String(), &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (wa *WAManager) fetchMissingAvatar(ctx context.Context, jid types.JID) {
	if wa.store == nil || jid.IsEmpty() {
		return
	}
	jid = jid.ToNonAD()
	group := jid.Server == types.GroupServer
	assetID, _, err := wa.store.AvatarMetadata(jid.String(), group)
	if err != nil {
		log.Printf("[avatar] metadata lookup for %s failed: %v", jid, err)
		return
	}
	if assetID == "" {
		wa.refreshAvatar(ctx, jid)
	}
}

func (wa *WAManager) refreshAvatar(ctx context.Context, jid types.JID) {
	if wa.store == nil || wa.client == nil || jid.IsEmpty() {
		return
	}
	jid = jid.ToNonAD()
	ctx, cancel := context.WithTimeout(ctx, avatarRefreshTimeout)
	defer cancel()
	unlock := wa.lockAvatar(jid)
	defer unlock()
	group := jid.Server == types.GroupServer
	oldAssetID, previousPictureID, err := wa.store.AvatarMetadata(jid.String(), group)
	if err != nil {
		log.Printf("[avatar] metadata lookup for %s failed: %v", jid, err)
		return
	}
	if oldAssetID == "" {
		previousPictureID = "" // Without a local file, force a full lookup even if legacy metadata has an ID.
	}
	info, err := wa.client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{ExistingID: previousPictureID})
	if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
		wa.removeAvatarLocked(jid, group)
		return
	}
	if err != nil {
		log.Printf("[avatar] picture lookup for %s failed; preserving cached avatar: %v", jid, err)
		return
	}
	if info == nil {
		return
	} // The stored picture ID is still current.
	if info.ID == "" || info.URL == "" {
		log.Printf("[avatar] picture lookup for %s returned incomplete data", jid)
		return
	}
	data, err := downloadAvatar(ctx, info.URL)
	if err != nil {
		log.Printf("[avatar] download for %s failed; preserving cached avatar: %v", jid, err)
		return
	}
	assetID, err := newAvatarID()
	if err != nil {
		log.Printf("[avatar] ID generation for %s failed: %v", jid, err)
		return
	}
	path, err := wa.store.avatarPath(assetID)
	if err != nil {
		log.Printf("[avatar] invalid stored asset ID for %s: %v", jid, err)
		return
	}
	if err := writeAvatarAtomically(path, data); err != nil {
		log.Printf("[avatar] storage for %s failed: %v", jid, err)
		return
	}
	if err := wa.store.SaveAvatar(jid.String(), assetID, info.ID, group); err != nil {
		log.Printf("[avatar] metadata update for %s failed: %v", jid, err)
		_ = os.Remove(path)
		return
	}
	if oldAssetID != "" {
		if oldPath, err := wa.store.avatarPath(oldAssetID); err == nil {
			_ = os.Remove(oldPath)
		}
	}
	eventType := "contact"
	if group {
		eventType = "chat"
	}
	publishRealtime(RealtimeEvent{Type: eventType, ChatID: jid.String()})
}

func (wa *WAManager) removeAvatar(jid types.JID, group bool) {
	jid = jid.ToNonAD()
	unlock := wa.lockAvatar(jid)
	defer unlock()
	wa.removeAvatarLocked(jid, group)
}

func (wa *WAManager) removeAvatarLocked(jid types.JID, group bool) {
	oldID, err := wa.store.ClearAvatar(jid.String(), group)
	if err != nil {
		log.Printf("[avatar] failed to clear metadata for %s: %v", jid, err)
		return
	}
	if oldID != "" {
		if path, err := wa.store.avatarPath(oldID); err == nil {
			_ = os.Remove(path)
		}
		eventType := "contact"
		if group {
			eventType = "chat"
		}
		publishRealtime(RealtimeEvent{Type: eventType, ChatID: jid.String()})
	}
}

func downloadAvatar(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || !allowedAvatarHost(parsed.Hostname()) {
		return nil, fmt.Errorf("rejected avatar URL")
	}
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || !allowedAvatarHost(req.URL.Hostname()) || len(via) > 5 {
				return fmt.Errorf("rejected avatar redirect")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar server returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatarBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxAvatarBytes {
		return nil, fmt.Errorf("avatar size is invalid")
	}
	contentType := http.DetectContentType(data)
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/gif" && mediaType != "image/webp" {
		return nil, fmt.Errorf("downloaded content is not a supported raster image")
	}
	return data, nil
}

func allowedAvatarHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "whatsapp.net" || strings.HasSuffix(host, ".whatsapp.net") || host == "whatsapp.com" || strings.HasSuffix(host, ".whatsapp.com")
}

func writeAvatarAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".avatar-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func handleAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/avatars/")
	if id == r.URL.Path || strings.Contains(id, "/") || !avatarIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	path, err := store.avatarPath(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		http.Error(w, "failed to read avatar", http.StatusInternalServerError)
		return
	}
	contentType := http.DetectContentType(buf[:n])
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/webp" {
		http.NotFound(w, r)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "failed to read avatar", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "avatar", time.Time{}, file)
}
