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

const avatarRefreshAttempts = 3

type avatarRefreshFlight struct {
	done chan struct{}
	err  error
}

type avatarCandidate struct {
	jid string
}

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
	ctx, finish := traceStart(ctx, "avatar.fetch_missing", map[string]any{"target_type": avatarTargetType(jid)})
	var result error
	defer func() { finish(result) }()
	if wa.store == nil || jid.IsEmpty() {
		return
	}
	jid = jid.ToNonAD()
	group := jid.Server == types.GroupServer
	assetID, _, err := wa.store.AvatarMetadataContext(ctx, jid.String(), group)
	if err != nil {
		result = err
		log.Printf("[avatar] metadata lookup for %s failed: %v", jid, err)
		return
	}
	if assetID != "" {
		return
	}
	if err := wa.refreshAvatarWithRetries(ctx, jid, true); err != nil {
		result = err
		log.Printf("[avatar] giving up after %d attempts for %s: %v", avatarRefreshAttempts, jid, err)
	}
}

func (wa *WAManager) refreshAvatar(ctx context.Context, jid types.JID) {
	if err := wa.refreshAvatarWithRetries(ctx, jid, false); err != nil {
		log.Printf("[avatar] refresh for %s failed: %v", jid, err)
	}
}

func (wa *WAManager) refreshAvatarWithRetries(ctx context.Context, jid types.JID, onlyIfMissing bool) (result error) {
	ctx, finish := traceStart(ctx, "avatar.refresh", map[string]any{"target_type": avatarTargetType(jid), "only_if_missing": onlyIfMissing})
	defer func() { finish(result) }()
	if wa.store == nil || wa.client == nil || jid.IsEmpty() {
		return nil
	}
	jid = jid.ToNonAD()
	flightValue, loaded := wa.avatarRefreshes.LoadOrStore(jid.String(), &avatarRefreshFlight{done: make(chan struct{})})
	flight := flightValue.(*avatarRefreshFlight)
	if loaded {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-flight.done:
			return flight.err
		}
	}
	jidKey := jid.String()
	defer func() {
		wa.avatarRefreshes.Delete(jidKey)
		close(flight.done)
	}()

	for attempt := 1; attempt <= avatarRefreshAttempts; attempt++ {
		_, finishAttempt := traceStart(ctx, "avatar.attempt", map[string]any{"attempt": attempt, "target_type": avatarTargetType(jid)})
		if err := wa.refreshAvatarAttempt(ctx, jid, onlyIfMissing); err == nil {
			finishAttempt(nil)
			return nil
		} else if attempt == avatarRefreshAttempts {
			flight.err = err
			finishAttempt(err)
			return err
		} else {
			flight.err = err
			finishAttempt(err)
		}
		timer := time.NewTimer(time.Duration(attempt) * 500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (wa *WAManager) refreshAvatarAttempt(ctx context.Context, jid types.JID, onlyIfMissing bool) (result error) {
	if wa.store == nil || wa.client == nil || jid.IsEmpty() {
		return nil
	}
	jid = jid.ToNonAD()
	ctx, finish := traceStart(ctx, "avatar.refresh_attempt", map[string]any{"target_type": avatarTargetType(jid), "only_if_missing": onlyIfMissing})
	defer func() { finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, avatarRefreshTimeout)
	defer cancel()

	if wa.avatarSlots != nil {
		queueStarted := time.Now()
		select {
		case wa.avatarSlots <- struct{}{}:
			traceEvent(ctx, "avatar.queue_acquired", map[string]any{"wait_ns": time.Since(queueStarted).Nanoseconds()})
			defer func() { <-wa.avatarSlots }()
		case <-ctx.Done():
			traceEvent(ctx, "avatar.queue_timeout", map[string]any{"wait_ns": time.Since(queueStarted).Nanoseconds()})
			return ctx.Err()
		}
	}
	avatarLockStarted := time.Now()
	unlock := wa.lockAvatar(jid)
	traceEvent(ctx, "avatar.jid_lock_acquired", map[string]any{"wait_ns": time.Since(avatarLockStarted).Nanoseconds()})
	defer unlock()
	group := jid.Server == types.GroupServer
	oldAssetID, previousPictureID, err := wa.store.AvatarMetadataContext(ctx, jid.String(), group)
	if err != nil {
		log.Printf("[avatar] metadata lookup for %s failed: %v", jid, err)
		return err
	}
	if onlyIfMissing && oldAssetID != "" {
		return nil
	}
	if oldAssetID == "" {
		previousPictureID = "" // Without a local file, force a full lookup even if legacy metadata has an ID.
	}
	lookupStarted := time.Now()
	info, err := wa.client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{ExistingID: previousPictureID})
	traceEvent(ctx, "avatar.whatsapp_lookup", map[string]any{"duration_ns": time.Since(lookupStarted).Nanoseconds(), "failed": err != nil, "error_type": errorType(err)})
	if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
		wa.removeAvatarLockedContext(ctx, jid, group)
		return nil
	}
	if err != nil {
		return err
	}
	if info == nil {
		if oldAssetID == "" {
			err := fmt.Errorf("picture lookup returned no data for missing avatar")
			return err
		}
		return nil
	} // The stored picture ID is still current.
	if info.ID == "" || info.URL == "" {
		err := fmt.Errorf("picture lookup returned incomplete data")
		return err
	}
	downloadStarted := time.Now()
	data, err := downloadAvatar(ctx, info.URL)
	traceEvent(ctx, "avatar.download", map[string]any{"duration_ns": time.Since(downloadStarted).Nanoseconds(), "bytes": len(data), "failed": err != nil, "error_type": errorType(err)})
	if err != nil {
		return err
	}
	assetID, err := newAvatarID()
	if err != nil {
		return err
	}
	path, err := wa.store.avatarPath(assetID)
	if err != nil {
		return err
	}
	fileStarted := time.Now()
	if err := writeAvatarAtomically(path, data); err != nil {
		traceEvent(ctx, "avatar.file_write", map[string]any{"duration_ns": time.Since(fileStarted).Nanoseconds(), "bytes": len(data), "failed": true, "error_type": errorType(err)})
		return err
	}
	traceEvent(ctx, "avatar.file_write", map[string]any{"duration_ns": time.Since(fileStarted).Nanoseconds(), "bytes": len(data)})
	dbStarted := time.Now()
	if err := wa.store.SaveAvatarContext(ctx, jid.String(), assetID, info.ID, group); err != nil {
		traceEvent(ctx, "avatar.metadata_save", map[string]any{"duration_ns": time.Since(dbStarted).Nanoseconds(), "failed": true, "error_type": errorType(err)})
		_ = os.Remove(path)
		return err
	}
	traceEvent(ctx, "avatar.metadata_save", map[string]any{"duration_ns": time.Since(dbStarted).Nanoseconds()})
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
	return nil
}

func (wa *WAManager) backfillMissingAvatars(ctx context.Context) {
	ctx, finish := traceStart(ctx, "avatar.backfill", nil)
	defer func() { finish(nil) }()
	wa.avatarBackfillMu.Lock()
	if wa.avatarBackfillRun {
		wa.avatarBackfillMu.Unlock()
		return
	}
	wa.avatarBackfillRun = true
	wa.avatarBackfillMu.Unlock()
	defer func() {
		wa.avatarBackfillMu.Lock()
		wa.avatarBackfillRun = false
		wa.avatarBackfillMu.Unlock()
	}()

	candidates, err := wa.store.MissingAvatarCandidates()
	traceEvent(ctx, "avatar.backfill_candidates", map[string]any{"count": len(candidates), "failed": err != nil, "error_type": errorType(err)})
	if err != nil {
		log.Printf("[avatar] failed to list backfill candidates: %v", err)
		return
	}
	var workers sync.WaitGroup
	jobs := make(chan avatarCandidate)
	workerCount := 4
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for candidate := range jobs {
				jid, err := types.ParseJID(candidate.jid)
				if err != nil {
					continue
				}
				wa.fetchMissingAvatar(ctx, jid)
			}
		}()
	}
	for _, candidate := range candidates {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		case jobs <- candidate:
		}
	}
	close(jobs)
	workers.Wait()
	log.Printf("[avatar] backfill completed: candidates=%d", len(candidates))
}

func avatarTargetType(jid types.JID) string {
	if jid.Server == types.GroupServer {
		return "group"
	}
	return "contact"
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}

func (wa *WAManager) removeAvatar(jid types.JID, group bool) {
	jid = jid.ToNonAD()
	unlock := wa.lockAvatar(jid)
	defer unlock()
	wa.removeAvatarLockedContext(context.Background(), jid, group)
}

func (wa *WAManager) removeAvatarLockedContext(ctx context.Context, jid types.JID, group bool) {
	oldID, err := wa.store.ClearAvatarContext(ctx, jid.String(), group)
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
