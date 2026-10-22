package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	wastore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	errInvalidChatID  = errors.New("invalid chat ID")
	errWAUnavailable  = errors.New("WhatsApp client is unavailable")
	errPersistMessage = errors.New("failed to store message")
)

type WAManager struct {
	client            *whatsmeow.Client
	status            string // "unauthenticated" | "connecting" | "connected"
	authError         string
	qrCode            string // base64 PNG data URL of the current QR
	store             *UserDataStore
	mu                sync.RWMutex
	avatarLocks       sync.Map // JID string -> *sync.Mutex; serializes fetches and removal per avatar.
	pairingAttempt    uint64
	pairingQRTimeout  time.Duration // zero uses pairingQRTimeout
	getQRChannel      func(context.Context) (<-chan whatsmeow.QRChannelItem, error)
	connectForPairing func() error
}

const pairingQRTimeout = 30 * time.Second
const latestWAVersionTimeout = 10 * time.Second

const clientOutdatedError = "WhatsApp rejected the current client version. Restart the backend to fetch the latest version and try again."

func newWAManager(store *UserDataStore) (*WAManager, error) {
	log.Println("[wa] Opening session store: wa-session.db")
	storeContainer, err := sqlstore.New(context.Background(), "sqlite3", "file:wa-session.db?_foreign_keys=on", nil)
	if err != nil {
		return nil, err
	}

	log.Println("[wa] Looking up stored device...")
	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		log.Printf("[wa] GetFirstDevice error: %v", err)
		return nil, err
	}

	if device.ID != nil {
		log.Printf("[wa] Found stored device (ID: %v)", device.ID)
	} else {
		log.Println("[wa] No stored device found, will need QR pairing")
	}
	wastore.SetOSInfo("WaTauri", [3]uint32{0, 1, 0})
	log.Println("[wa] Linked device name set to WaTauri")

	device.Platform = "Tauri"
	log.Printf("[wa] Device platform set to %s", device.Platform)

	versionCtx, cancelVersionFetch := context.WithTimeout(context.Background(), latestWAVersionTimeout)
	version, err := whatsmeow.GetLatestVersion(versionCtx, nil)
	cancelVersionFetch()
	if err != nil {
		log.Printf("[wa] GetLatestVersion failed, using built-in version: %v", err)
	} else {
		wastore.SetWAVersion(*version)
		log.Printf("[wa] Updated WhatsApp Web version to %v", *version)
	}

	client := whatsmeow.NewClient(device, nil)
	wa := &WAManager{client: client, status: "unauthenticated", store: store}
	if device.ID != nil {
		wa.status = "connected"
		log.Println("[wa] Auto-connecting with stored session...")
		go func() {
			if err := client.Connect(); err != nil {
				log.Printf("[wa] Auto-connect error: %v", err)
			}
		}()
	}
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			wa.mu.Lock()
			wa.status = "connected"
			wa.authError = ""
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Println("[wa] Connected to Whatsapp")
			go wa.SyncGroups(context.Background())
			go wa.SyncContacts(context.Background())
		case *events.Disconnected:
			wa.mu.Lock()
			if wa.status != "client_outdated" {
				wa.status = "unauthenticated"
				wa.authError = ""
			}
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Println("[wa] Disconnected from WhatsApp")
		case *events.ClientOutdated:
			wa.mu.Lock()
			wa.status = "client_outdated"
			wa.authError = clientOutdatedError
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Printf("[wa] Client version rejected by WhatsApp: %s", clientOutdatedError)
		case *events.Message:
			if wa.store == nil {
				log.Println("[wa] Skipping message: store is nil")
				break
			}

			chatJID := v.Info.Chat.String()
			canonicalChatJID, err := wa.store.CanonicalDirectChatJID(chatJID)
			if err != nil {
				log.Printf("[wa] Failed to canonicalize chat %s: %v", chatJID, err)
				break
			}
			placeholder := Chat{
				ID: canonicalChatJID,
			}
			if err := wa.store.UpsertChat(placeholder); err != nil {
				log.Printf("[wa] Failed to upsert placeholder chat %s: %v", canonicalChatJID, err)
				break
			}
			wa.storeMessageEvent(v, "live")
		case *events.Receipt:
			log.Printf("[wa] Event: receipt type=%s ids=%v chat=%s sender=%s", v.Type, v.MessageIDs, v.Chat, v.Sender)
			if len(v.MessageIDs) > 0 && wa.store != nil {
				newStatus := "delivered"
				if v.Type == "read" || v.Type == "read-self" {
					newStatus = "read"
				}
				if err := wa.store.UpdateMessageStatus(v.MessageIDs, newStatus); err != nil {
					log.Printf("[wa] Failed to update receipt status for %v: %v", v.MessageIDs, err)
				} else {
					chatJID := v.Chat.String()
					if canonical, err := wa.store.CanonicalDirectChatJID(chatJID); err == nil {
						chatJID = canonical
					}
					publishRealtime(RealtimeEvent{Type: "receipt", ChatID: chatJID, MessageIDs: v.MessageIDs})
				}
			}
		case *events.Presence:
			log.Printf("[wa] Event: presence from=%s unavailable=%v lastSeen=%v", v.From, v.Unavailable, v.LastSeen)
		case *events.Picture:
			jid := v.JID.ToNonAD()
			if v.Remove {
				go wa.removeAvatar(jid, jid.Server == types.GroupServer)
			} else {
				go wa.refreshAvatar(context.Background(), jid)
			}
		case *events.GroupInfo:
			wa.handleGroupInfoEvent(v)
		case *events.JoinedGroup:
			if _, err := wa.persistGroupInfo(v.GroupInfo); err != nil {
				log.Printf("[wa] failed to persist joined group %s: %v", v.JID, err)
			}
		case *events.HistorySync:
			if wa.store == nil {
				log.Println("[wa] Skipping history sync: store is nil")
				break
			}

			data := v.Data
			if data == nil {
				log.Printf("[wa] history sync skipped: nil data")
				break
			}
			dumpHistorySync(data)

			pushnamesStored := 0
			inlineContactsStored := 0
			for _, push := range data.GetPushnames() {
				jid := push.GetID()
				pushName := push.GetPushname()
				if jid == "" {
					continue
				}
				if err := wa.store.UpsertContact(User{
					ID:       jid,
					PushName: pushName,
				}); err != nil {
					log.Printf("[wa] failed to upsert history pushname %s: %v", jid, err)
					continue
				}
				pushnamesStored++
			}

			conversationsSeen := len(data.GetConversations())
			conversationsStored := 0
			conversationsSkipped := 0
			messagesSeen := 0
			messagesStored := 0
			messagesSkipped := 0
			jidMappingsStored := 0
			jidMappingsSkipped := 0
			for _, mapping := range data.GetPhoneNumberToLidMappings() {
				pn, err := types.ParseJID(mapping.GetPnJID())
				if err != nil {
					log.Printf("[wa] history sync invalid phone jid mapping pn=%q lid=%q: %v", mapping.GetPnJID(), mapping.GetLidJID(), err)
					jidMappingsSkipped++
					continue
				}
				if pn.Server == types.LegacyUserServer {
					pn.Server = types.DefaultUserServer
				}
				lid, err := types.ParseJID(mapping.GetLidJID())
				if err != nil {
					log.Printf("[wa] history sync invalid lid mapping pn=%q lid=%q: %v", mapping.GetPnJID(), mapping.GetLidJID(), err)
					jidMappingsSkipped++
					continue
				}
				if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer {
					log.Printf("[wa] history sync skipped unexpected jid mapping pn=%s lid=%s", pn, lid)
					jidMappingsSkipped++
					continue
				}

				if err := wa.store.UpsertJIDMapping(lid.String(), pn.String()); err != nil {
					log.Printf("[wa] failed to upsert history jid mapping %s -> %s: %v", lid, pn, err)
					jidMappingsSkipped++
					continue
				}

				jidMappingsStored++
			}
			log.Printf("[wa] Event: historySync type=%v chunk=%d progress=%d conversations=%d statusMessages=%d pushnames=%d/%d inlineContacts=%d/%d jidMappings=%d/%d skipped=%d",
				data.GetSyncType(), data.GetChunkOrder(), data.GetProgress(), conversationsSeen, len(data.GetStatusV3Messages()), pushnamesStored, len(data.GetPushnames()), inlineContactsStored, len(data.GetInlineContacts()), jidMappingsStored, len(data.GetPhoneNumberToLidMappings()), jidMappingsSkipped)

			for _, conv := range data.GetConversations() {
				chatJID, err := types.ParseJID(conv.GetID())
				if err != nil {
					log.Printf("[wa] history sync invalid chat jid %q : %v", conv.GetID(), err)
					conversationsSkipped++
					continue
				}
				canonicalChatJID := chatJID.String()
				if chatJID.Server != types.GroupServer {
					canonicalChatJID, err = wa.store.CanonicalDirectChatJID(chatJID.String())
					if err != nil {
						log.Printf("[wa] failed to canonicalize history chat %s: %v", chatJID, err)
						conversationsSkipped++
						continue
					}
				}
				name := conv.GetName()
				if name == "" {
					name = conv.GetDisplayName()
				}
				chat := Chat{
					ID:             canonicalChatJID,
					UnreadCount:    int(conv.GetUnreadCount()),
					UnreadCountSet: true,
					IsGroup:        chatJID.Server == "g.us",
					IsArchived:     conv.GetArchived(),
					IsArchivedSet:  conv.Archived != nil,
				}
				if chat.IsGroup && conv.IsParentGroup != nil {
					chat.IsCommunity = conv.GetIsParentGroup()
					chat.IsCommunitySet = true
				}

				if name != "" {
					chat.Name = &name
				}
				if err := wa.store.UpsertChat(chat); err != nil {
					log.Printf("[wa] failed to upsert history chat %s: %v", chatJID, err)
					conversationsSkipped++
					continue
				}
				if chat.IsGroup {
					wa.persistHistoryParticipants(conv)
					if err := wa.store.UpsertGroupMetadata(canonicalChatJID, groupMetadataUpdateFromHistoryConversation(conv)); err != nil {
						log.Printf("[wa] failed to upsert history group metadata %s: %v", canonicalChatJID, err)
					}
				}
				conversationsStored++

				for _, historymsg := range conv.GetMessages() {
					messagesSeen++
					evt, err := client.ParseWebMessage(chatJID, historymsg.GetMessage())
					if err != nil {
						log.Printf("[wa] failed to parse history message in %s: %v ", chatJID, err)
						messagesSkipped++
						continue
					}
					if evt == nil {
						log.Printf("[wa] skipped nil parsed history message in %s", chatJID)
						messagesSkipped++
						continue
					}
					if wa.storeMessageEvent(evt, "history") {
						messagesStored++
					} else {
						messagesSkipped++
					}
				}
			}
			go wa.SyncContacts(context.Background())
			log.Printf("[wa] historySync done type=%v chunk=%d progress=%d conversationsSeen=%d conversationsStored=%d conversationsSkipped=%d messagesSeen=%d messagesStored=%d messagesSkipped=%d",
				data.GetSyncType(), data.GetChunkOrder(), data.GetProgress(), conversationsSeen, conversationsStored, conversationsSkipped, messagesSeen, messagesStored, messagesSkipped)
			progress := int(data.GetProgress())
			publishRealtime(RealtimeEvent{Type: "sync", Progress: &progress, SyncType: data.GetSyncType().String()})
			if messagesStored > 0 {
				publishRealtime(RealtimeEvent{Type: "resync"})
			} else if conversationsStored > 0 || pushnamesStored > 0 || inlineContactsStored > 0 || jidMappingsStored > 0 {
				publishRealtime(RealtimeEvent{Type: "chat"})
			}
		case *events.PushName:
			log.Printf("[wa] Event: pushName jid=%s old=%q new=%q", v.JID, v.OldPushName, v.NewPushName)
			if wa.store != nil {
				if err := wa.store.UpsertContact(User{
					ID:       v.JID.String(),
					PushName: v.NewPushName,
				}); err == nil {
					publishRealtime(RealtimeEvent{Type: "chat"})
				}
			}
		case *events.LoggedOut:
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.authError = ""
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Println("[wa] Logged out (session revoked)")
		default:
			log.Printf("[wa] Event: unhandled type=%T", evt)
		}
	})

	return wa, nil
}

func dumpHistorySync(data *waHistorySync.HistorySync) {
	if data == nil || os.Getenv("WATAURI_DUMP_HISTORY_SYNC") != "1" {
		return
	}

	dir := os.Getenv("WATAURI_HISTORY_SYNC_DUMP_DIR")
	if dir == "" {
		dir = "history-sync-dumps"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("[wa] failed to create history sync dump dir %s: %v", dir, err)
		return
	}

	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	filename := fmt.Sprintf(
		"history-sync-%s-%s-chunk-%d.json",
		timestamp,
		data.GetSyncType().String(),
		data.GetChunkOrder(),
	)
	path := filepath.Join(dir, filename)

	payload, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
	}.Marshal(data)
	if err != nil {
		log.Printf("[wa] failed to marshal history sync dump: %v", err)
		return
	}

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		log.Printf("[wa] failed to write history sync dump %s: %v", path, err)
		return
	}
	log.Printf("[wa] wrote history sync dump: %s", path)
}

// / Stores a parsed Message event and stores it. The caller is responsible for upserting the chat before calling.
func (wa *WAManager) storeMessageEvent(evt *events.Message, source string) bool {

	text := evt.Message.GetConversation()

	if text == "" {
		if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
			text = ext.GetText()
		}
	}
	mediaType := ""
	if evt.Message.GetImageMessage() != nil {
		mediaType = "image"
	} else if evt.Message.GetVideoMessage() != nil {
		mediaType = "video"
	} else if evt.Message.GetAudioMessage() != nil {
		mediaType = "audio"
	} else if evt.Message.GetDocumentMessage() != nil {
		mediaType = "document"
	}
	if text == "" && mediaType == "" {
		log.Printf("[wa] Skipping message %s: empty text, no media (reaction/receipt)", evt.Info.ID)
		return false
	}

	status := "received"

	if evt.Info.IsFromMe {
		status = "sent"
	}
	log.Printf("[wa] Event: message id = %s chat = %s sender = %s sender_push = %s text = %q media = %s isFromMe = %v ",
		evt.Info.ID, evt.Info.Chat, evt.Info.Sender, evt.Info.PushName, text, mediaType, evt.Info.IsFromMe)

	chatJID := evt.Info.Chat.String()
	if evt.Info.Chat.Server != types.GroupServer {
		canonicalChatJID, err := wa.store.CanonicalDirectChatJID(chatJID)
		if err != nil {
			log.Printf("[wa] Failed to canonicalize message chat %s: %v", chatJID, err)
			return false
		}
		chatJID = canonicalChatJID
	}
	if source == "history" && evt.Info.PushName != "" && evt.Info.Sender.User != "" && evt.Info.Sender.Server != "" {
		if err := wa.store.UpsertContact(User{
			ID:       evt.Info.Sender.String(),
			PushName: evt.Info.PushName,
		}); err != nil {
			log.Printf("[wa] failed to upsert history message pushname %s: %v", evt.Info.Sender, err)
		}
	}
	ourMsg := Message{
		ID:            evt.Info.ID,
		ChatJID:       chatJID,
		SenderID:      evt.Info.Sender.String(),
		Text:          text,
		Timestamp:     evt.Info.Timestamp.Format(time.RFC3339),
		Status:        status,
		MediaType:     mediaType,
		IsFromMe:      evt.Info.IsFromMe,
		MentionedJIDs: extractMentionedJIDs(evt.Message),
	}
	if err := wa.store.InsertMessage(ourMsg, source == "live"); err != nil {
		log.Printf("[wa] Failed to store message %s: %v", evt.Info.ID, err)
		return false
	}
	if source == "live" {
		publishRealtime(RealtimeEvent{Type: "message", ChatID: chatJID, MessageIDs: []string{ourMsg.ID}})
	}
	return true

}

// extractMentionedJIDs reads the ContextInfo directly attached to a message
// payload. It deliberately does not walk into quoted messages, whose mentions
// belong to the quoted message rather than the current one.
func extractMentionedJIDs(message *waE2E.Message) []string {
	mentionedJIDs := []string{}
	if message == nil {
		return mentionedJIDs
	}

	messageReflect := message.ProtoReflect()
	fields := messageReflect.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if field.Kind() != protoreflect.MessageKind || field.IsList() || !messageReflect.Has(field) {
			continue
		}
		payload := messageReflect.Get(field).Message()
		contextField := payload.Descriptor().Fields().ByName("contextInfo")
		if contextField == nil || !payload.Has(contextField) {
			continue
		}
		contextInfo := payload.Get(contextField).Message()
		mentionField := contextInfo.Descriptor().Fields().ByName("mentionedJID")
		if mentionField == nil || !contextInfo.Has(mentionField) {
			continue
		}
		mentions := contextInfo.Get(mentionField).List()
		for j := 0; j < mentions.Len(); j++ {
			mentionedJIDs = append(mentionedJIDs, mentions.Get(j).String())
		}
	}
	return mentionedJIDs
}

func (wa *WAManager) StartPairing() {
	wa.mu.Lock()
	if wa.status != "unauthenticated" {
		wa.mu.Unlock()
		return
	}
	wa.status = "connecting"
	wa.authError = ""
	wa.qrCode = ""
	wa.pairingAttempt++
	attempt := wa.pairingAttempt
	wa.mu.Unlock()
	go wa.runPairingAttempt(attempt)
}

func (wa *WAManager) runPairingAttempt(attempt uint64) {
	getQRChannel := wa.getQRChannel
	connect := wa.connectForPairing
	if getQRChannel == nil {
		if wa.client == nil {
			wa.failPairingAttempt(attempt, "WhatsApp client is unavailable")
			return
		}
		getQRChannel = wa.client.GetQRChannel
	}
	if connect == nil {
		if wa.client == nil {
			wa.failPairingAttempt(attempt, "WhatsApp client is unavailable")
			return
		}
		connect = wa.client.Connect
	}

	ch, err := getQRChannel(context.Background())
	if err != nil {
		wa.failPairingAttempt(attempt, fmt.Sprintf("GetQRChannel failed: %v", err))
		return
	}

	qrTimeout := wa.pairingQRTimeout
	if qrTimeout <= 0 {
		qrTimeout = pairingQRTimeout
	}
	timer := time.NewTimer(qrTimeout)
	defer timer.Stop()
	timeout := timer.C
	connectResult := make(chan error, 1)
	go func() {
		connectResult <- connect()
	}()
	gotQR := false
	for {
		select {
		case err := <-connectResult:
			connectResult = nil
			if err != nil {
				wa.failPairingAttempt(attempt, fmt.Sprintf("Connect failed: %v", err))
				return
			}
		case item, ok := <-ch:
			if !ok {
				wa.failPairingAttempt(attempt, "QR channel closed before pairing succeeded")
				return
			}
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				png, err := qrcode.Encode(item.Code, qrcode.Medium, 256)
				if err != nil {
					wa.failPairingAttempt(attempt, fmt.Sprintf("QR encoding failed: %v", err))
					return
				}
				dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				wa.mu.Lock()
				if wa.pairingAttempt == attempt && wa.status == "connecting" {
					wa.qrCode = dataURL
				}
				wa.mu.Unlock()
				if !gotQR {
					gotQR = true
					timer.Stop()
					timeout = nil
				}
			case "success":
				if wa.completePairingAttempt(attempt) {
					go wa.SyncContacts(context.Background())
				}
				return
			case "timeout":
				wa.failPairingAttempt(attempt, "QR code expired before pairing succeeded")
				return
			case "err-scanned-without-multidevice":
				// whatsmeow emits this warning without closing the QR channel;
				// preserve the existing behavior and continue waiting for pairing.
				log.Println("[wa] Phone doesn't have multi-device enabled")
			default:
				if item.Event == whatsmeow.QRChannelEventError {
					wa.failPairingAttempt(attempt, fmt.Sprintf("pairing error: %v", item.Error))
				} else {
					wa.failPairingAttempt(attempt, fmt.Sprintf("pairing ended with event %q", item.Event))
				}
				return
			}
		case <-timeout:
			if !gotQR {
				wa.failPairingAttempt(attempt, "no QR code received before timeout")
				return
			}
		}
	}
}

func (wa *WAManager) failPairingAttempt(attempt uint64, reason string) {
	log.Printf("[wa] Pairing attempt %d failed: %s", attempt, reason)
	wa.mu.Lock()
	if wa.pairingAttempt == attempt && wa.status != "connected" && wa.status != "client_outdated" {
		wa.status = "unauthenticated"
		wa.qrCode = ""
	}
	wa.mu.Unlock()
}

func (wa *WAManager) completePairingAttempt(attempt uint64) bool {
	wa.mu.Lock()
	defer wa.mu.Unlock()
	if wa.pairingAttempt != attempt {
		return false
	}
	wa.status = "connected"
	wa.authError = ""
	wa.qrCode = ""
	return true
}

func (wa *WAManager) GetStatus() string {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	return wa.status
}

func (wa *WAManager) GetQR() string {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	return wa.qrCode
}

func (wa *WAManager) GetAuthError() string {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	return wa.authError
}

func (wa *WAManager) GetProfile() Profile {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	if wa.client == nil || wa.client.Store == nil {
		return Profile{}
	}
	profile := Profile{PushName: wa.client.Store.PushName}
	if wa.client.Store.ID != nil {
		profile.ID = wa.client.Store.ID.String()
	}
	return profile
}
func (wa *WAManager) SyncContacts(ctx context.Context) {
	start := time.Now()
	wa.mu.RLock()
	client := wa.client
	store := wa.store
	wa.mu.RUnlock()

	if client == nil || client.Store == nil || store == nil {
		log.Println("[wa] Skipping contact sync: client or store is nil")
		return
	}
	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		log.Printf("[wa] Failed to load contacts from whatsmeow store : %v", err)
		return
	}
	stored := 0
	skipped := 0

	for jid, info := range contacts {
		name := info.FullName
		if name == "" {
			name = info.FirstName
		}
		if name == "" {
			name = info.BusinessName
		}
		if err := store.UpsertContact(User{
			ID:       jid.String(),
			Name:     name,
			PushName: info.PushName,
		}); err != nil {
			log.Printf("[wa] Failed to upsert synced contact %s: %v", jid, err)
			skipped++
			continue

		}
		wa.fetchMissingAvatar(ctx, jid.ToNonAD())
		stored++
	}
	log.Printf("[wa] Contact sync complete: stored = %d, skipped = %d, total = %d (%v)", stored, skipped, len(contacts), time.Since(start))
	if stored > 0 {
		publishRealtime(RealtimeEvent{Type: "chat"})
	}
}

func groupParticipantRank(participant types.GroupParticipant) int {
	if participant.IsSuperAdmin {
		return 2
	}
	if participant.IsAdmin {
		return 1
	}
	return 0
}

func groupParticipantJID(participant types.GroupParticipant) types.JID {
	for _, candidate := range []types.JID{participant.PhoneNumber, participant.JID, participant.LID} {
		if normalized, err := normalizeParticipantJID(candidate.String()); err == nil {
			jid, _ := types.ParseJID(normalized)
			return jid
		}
	}
	return types.JID{}
}

func (wa *WAManager) persistGroupParticipant(chatJID string, participant types.GroupParticipant, snapshotRevision *int64) bool {
	wa.mu.RLock()
	store := wa.store
	wa.mu.RUnlock()
	if store == nil {
		return false
	}

	participantJID := groupParticipantJID(participant)
	if participantJID.User == "" || participantJID.Server == "" {
		return false
	}
	userID := participantJID.String()
	if err := store.UpsertContact(User{
		ID:       userID,
		PushName: participant.DisplayName,
	}); err != nil {
		log.Printf("[wa] failed to upsert group participant contact %s: %v", userID, err)
	}
	// Some snapshots carry the LID only in JID, not in the dedicated LID field.
	lid := participant.LID
	if lid.User == "" || lid.Server != types.HiddenUserServer {
		lid = participant.JID
	}
	phone, phoneErr := normalizeParticipantJID(participant.PhoneNumber.String())
	if lid.User != "" && lid.Server == types.HiddenUserServer && phoneErr == nil &&
		strings.HasSuffix(phone, "@"+types.DefaultUserServer) {
		if err := store.UpsertJIDMapping(lid.ToNonAD().String(), phone); err != nil {
			log.Printf("[wa] failed to upsert group participant jid mapping %s -> %s: %v", lid, phone, err)
		}
	}
	if err := store.writeParticipant(chatJID, userID, groupParticipantRank(participant), participantWrite{
		source: participantSnapshot, snapshotRevision: snapshotRevision,
	}); err != nil {
		log.Printf("[wa] failed to upsert group participant %s in %s: %v", userID, chatJID, err)
		return false
	}
	return true
}

func (wa *WAManager) SyncGroups(ctx context.Context) {
	wa.mu.RLock()
	client := wa.client
	store := wa.store
	wa.mu.RUnlock()
	if client == nil || store == nil {
		log.Println("[wa] Skipping group sync: client or store is nil")
		return
	}
	// A response already in flight must not undo a departure processed while
	// waiting for the network. Use a durable logical revision, not wall time.
	snapshotRevision, err := store.participantRevision()
	if err != nil {
		log.Printf("[wa] Failed to read participant revision: %v", err)
		return
	}
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		log.Printf("[wa] Failed to load joined groups: %v", err)
		return
	}
	groupsStored := 0
	participantsStored := 0
	groupsSkipped := 0

	for _, group := range groups {
		if group == nil || group.JID.User == "" || group.JID.Server == "" {
			groupsSkipped++
			continue
		}
		storedParticipants, err := wa.persistGroupInfoAtRevision(*group, &snapshotRevision)
		if err != nil {
			log.Printf("[wa] failed to upsert synced group %s: %v", group.JID, err)
			groupsSkipped++
			continue
		}
		groupsStored++
		participantsStored += storedParticipants
	}

	log.Printf("[wa] Group sync complete: groups=%d skipped=%d participants=%d total=%d", groupsStored, groupsSkipped, participantsStored, len(groups))
}

func (wa *WAManager) persistGroupInfo(group types.GroupInfo) (int, error) {
	// Unsolicited snapshots (JoinedGroup) have no request-start boundary and
	// cannot safely clear a departure. Explicit live joins can still restore it.
	return wa.persistGroupInfoAtRevision(group, nil)
}

func (wa *WAManager) persistGroupInfoAtRevision(group types.GroupInfo, snapshotRevision *int64) (int, error) {
	if group.JID.User == "" || group.JID.Server == "" {
		return 0, fmt.Errorf("invalid group JID %s", group.JID)
	}
	chatJID := group.JID.ToNonAD().String()
	chat := Chat{
		ID:             chatJID,
		IsGroup:        true,
		IsCommunity:    group.GroupParent.IsParent,
		IsCommunitySet: true,
	}
	if group.Name != "" {
		chat.Name = &group.Name
	}
	if err := wa.store.UpsertChat(chat); err != nil {
		return 0, err
	}
	go wa.fetchMissingAvatar(context.Background(), group.JID.ToNonAD())
	if err := wa.store.UpsertGroupMetadata(chatJID, groupMetadataUpdateFromGroupInfo(group)); err != nil {
		publishRealtime(RealtimeEvent{Type: "chat", ChatID: chatJID})
		return 0, err
	}

	participantsStored := 0
	for _, participant := range group.Participants {
		if wa.persistGroupParticipant(chatJID, participant, snapshotRevision) {
			participantsStored++
		}
	}
	publishRealtime(RealtimeEvent{Type: "chat", ChatID: chatJID})
	return participantsStored, nil
}

func (wa *WAManager) handleGroupInfoEvent(evt *events.GroupInfo) {
	if evt == nil || wa.store == nil || evt.JID.User == "" || evt.JID.Server != types.GroupServer {
		return
	}
	chatJID := evt.JID.ToNonAD().String()
	chat := Chat{ID: chatJID, IsGroup: true}
	if evt.Name != nil {
		chat.Name = &evt.Name.Name
	}
	patch := groupMetadataUpdateFromGroupInfoEvent(evt)
	if evt.Link != nil && evt.Link.Type == types.GroupLinkChangeTypeParent {
		chat.IsCommunity = true
		chat.IsCommunitySet = true
	}
	if evt.Unlink != nil && evt.Unlink.Type == types.GroupLinkChangeTypeParent {
		chat.IsCommunitySet = true
	}
	if err := wa.store.UpsertChat(chat); err != nil {
		log.Printf("[wa] failed to upsert group chat %s after metadata event: %v", chatJID, err)
		return
	}
	if err := wa.store.UpsertGroupMetadata(chatJID, patch); err != nil {
		log.Printf("[wa] failed to persist group metadata event for %s: %v", chatJID, err)
	}
	wa.persistLiveParticipants(evt)
	publishRealtime(RealtimeEvent{Type: "chat", ChatID: chatJID})

	if evt.Link != nil && evt.Link.Type == types.GroupLinkChangeTypeSub && evt.Link.Group.JID.User != "" {
		parentJID := chatJID
		childJID := evt.Link.Group.JID.ToNonAD().String()
		if err := wa.store.UpsertChat(Chat{ID: childJID, IsGroup: true}); err != nil {
			log.Printf("[wa] failed to create child group chat %s: %v", childJID, err)
			return
		}
		if err := wa.store.UpsertGroupMetadata(childJID, GroupMetadataPatch{
			ParentGroupJID:  &parentJID,
			LinkedParentJID: &parentJID,
		}); err != nil {
			log.Printf("[wa] failed to persist parent relationship for %s: %v", childJID, err)
		} else {
			publishRealtime(RealtimeEvent{Type: "chat", ChatID: childJID})
		}
	}
	if evt.Unlink != nil && evt.Unlink.Type == types.GroupLinkChangeTypeSub && evt.Unlink.Group.JID.User != "" {
		childJID := evt.Unlink.Group.JID.ToNonAD().String()
		cleared := ""
		if err := wa.store.UpsertGroupMetadata(childJID, GroupMetadataPatch{
			ParentGroupJID:  &cleared,
			LinkedParentJID: &cleared,
		}); err != nil {
			log.Printf("[wa] failed to clear parent relationship for %s: %v", childJID, err)
		} else {
			publishRealtime(RealtimeEvent{Type: "chat", ChatID: childJID})
		}
	}
}

func groupMetadataUpdateFromGroupInfo(group types.GroupInfo) GroupMetadataPatch {
	patch := GroupMetadataPatch{
		IsLocked:               boolPointer(group.GroupLocked.IsLocked),
		IsAnnouncement:         boolPointer(group.GroupAnnounce.IsAnnounce),
		IsEphemeral:            boolPointer(group.GroupEphemeral.IsEphemeral),
		DisappearingTimer:      intPointer(int(group.GroupEphemeral.DisappearingTimer)),
		IsJoinApprovalRequired: boolPointer(group.GroupMembershipApprovalMode.IsJoinApprovalRequired),
		IsDefaultSubgroup:      boolPointer(group.GroupIsDefaultSub.IsDefaultSubGroup),
		IsSuspended:            boolPointer(group.Suspended),
	}
	if group.OwnerJID.User != "" && group.OwnerJID.Server != "" {
		owner := group.OwnerJID.ToNonAD().String()
		patch.OwnerJID = &owner
	}
	if !group.GroupCreated.IsZero() {
		createdAt := group.GroupCreated.UTC().Format(time.RFC3339Nano)
		patch.CreatedAt = &createdAt
	}
	if group.GroupTopic.Topic != "" || group.GroupTopic.TopicDeleted {
		description := group.GroupTopic.Topic
		if group.GroupTopic.TopicDeleted {
			description = ""
		}
		patch.Description = &description
	}
	if group.GroupLinkedParent.LinkedParentJID.User != "" {
		linkedParent := group.GroupLinkedParent.LinkedParentJID.ToNonAD().String()
		patch.LinkedParentJID = &linkedParent
	}
	return patch
}

func groupMetadataUpdateFromHistoryConversation(conv *waHistorySync.Conversation) GroupMetadataPatch {
	var patch GroupMetadataPatch
	if conv == nil {
		return patch
	}
	if conv.Description != nil {
		patch.Description = conv.Description
	}
	if conv.CreatedBy != nil {
		patch.OwnerJID = conv.CreatedBy
		patch.OwnerJIDFallback = true
	}
	if conv.CreatedAt != nil && conv.GetCreatedAt() > 0 {
		createdAt := time.Unix(int64(conv.GetCreatedAt()), 0).UTC().Format(time.RFC3339)
		patch.CreatedAt = &createdAt
	}
	if conv.Locked != nil {
		patch.IsLocked = boolPointer(conv.GetLocked())
	}
	if conv.EphemeralExpiration != nil {
		timer := int(conv.GetEphemeralExpiration())
		patch.DisappearingTimer = &timer
		patch.IsEphemeral = boolPointer(timer > 0)
	}
	if conv.Suspended != nil {
		patch.IsSuspended = boolPointer(conv.GetSuspended())
	}
	if conv.Terminated != nil {
		patch.IsDeleted = boolPointer(conv.GetTerminated())
	}
	if conv.ParentGroupID != nil {
		patch.ParentGroupJID = conv.ParentGroupID
	}
	if conv.IsDefaultSubgroup != nil {
		patch.IsDefaultSubgroup = boolPointer(conv.GetIsDefaultSubgroup())
	}
	return patch
}

func groupMetadataUpdateFromGroupInfoEvent(evt *events.GroupInfo) GroupMetadataPatch {
	var patch GroupMetadataPatch
	if evt == nil {
		return patch
	}
	if evt.Topic != nil {
		description := evt.Topic.Topic
		if evt.Topic.TopicDeleted {
			description = ""
		}
		patch.Description = &description
	}
	if evt.Locked != nil {
		patch.IsLocked = boolPointer(evt.Locked.IsLocked)
	}
	if evt.Announce != nil {
		patch.IsAnnouncement = boolPointer(evt.Announce.IsAnnounce)
	}
	if evt.Ephemeral != nil {
		patch.IsEphemeral = boolPointer(evt.Ephemeral.IsEphemeral)
		timer := int(evt.Ephemeral.DisappearingTimer)
		patch.DisappearingTimer = &timer
	}
	if evt.MembershipApprovalMode != nil {
		patch.IsJoinApprovalRequired = boolPointer(evt.MembershipApprovalMode.IsJoinApprovalRequired)
	}
	if evt.Delete != nil {
		patch.IsDeleted = boolPointer(evt.Delete.Deleted)
	}
	if evt.Suspended {
		patch.IsSuspended = boolPointer(true)
	} else if evt.Unsuspended {
		patch.IsSuspended = boolPointer(false)
	}
	return patch
}

func boolPointer(value bool) *bool { return &value }

func intPointer(value int) *int { return &value }

func (wa *WAManager) SendText(ctx context.Context, chatID, text, clientSendID string) (Message, error) {
	to, err := types.ParseJID(chatID)
	if err != nil {
		return Message{}, fmt.Errorf("%w: %v", errInvalidChatID, err)
	}
	if to.User == "" || to.Server == "" {
		return Message{}, errInvalidChatID
	}
	wa.mu.RLock()
	client := wa.client
	wa.mu.RUnlock()
	if client == nil {
		return Message{}, errWAUnavailable
	}
	id := client.GenerateMessageID()
	senderID := ""
	if client.Store.ID != nil {
		senderID = client.Store.ID.String()
	}
	message := Message{
		ID:            string(id),
		ClientSendID:  clientSendID,
		ChatJID:       chatID,
		SenderID:      senderID,
		Text:          text,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Status:        "pending",
		IsFromMe:      true,
		MentionedJIDs: []string{},
	}
	if err := wa.store.InsertMessage(message, false); err != nil {
		return Message{}, fmt.Errorf("%w: %v", errPersistMessage, err)
	}
	publishRealtime(RealtimeEvent{Type: "message", ChatID: chatID, MessageIDs: []string{message.ID}, ClientSendID: clientSendID})
	if _, err := client.SendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(text)}, whatsmeow.SendRequestExtra{ID: id}); err != nil {
		return Message{}, fmt.Errorf("failed to send message: %w", err)
	}
	message.Status = "sent"
	if err := wa.store.UpdateMessageStatus([]string{message.ID}, message.Status); err != nil {
		return Message{}, fmt.Errorf("%w: %v", errPersistMessage, err)
	}
	publishRealtime(RealtimeEvent{Type: "receipt", ChatID: chatID, MessageIDs: []string{message.ID}})
	return message, nil
}

func (wa *WAManager) MarkRead(ctx context.Context, chatID string, sendReceipt bool, messageIDs []string) (int, error) {
	chatJID, err := types.ParseJID(chatID)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidChatID, err)
	}
	if chatJID.User == "" || chatJID.Server == "" {
		return 0, errInvalidChatID
	}

	wa.mu.RLock()
	client := wa.client
	store := wa.store
	wa.mu.RUnlock()

	if store == nil {
		return 0, errPersistMessage
	}
	if sendReceipt && client == nil {
		return 0, errWAUnavailable
	}
	messages, err := store.GetUnreadInboundMessages(chatJID.String(), messageIDs)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errPersistMessage, err)
	}

	filteredIDs := make([]string, 0, len(messages))
	bySender := map[string][]types.MessageID{}
	for _, message := range messages {
		filteredIDs = append(filteredIDs, message.ID)
		bySender[message.SenderID] = append(bySender[message.SenderID], types.MessageID(message.ID))

	}
	if sendReceipt && len(messages) > 0 {
		now := time.Now()
		for senderID, ids := range bySender {
			senderJID, err := types.ParseJID(senderID)
			if err != nil {
				return 0, fmt.Errorf("%w: invalid sender jid %q: %v", errInvalidChatID, senderID, err)
			}
			if err := client.MarkRead(ctx, ids, now, chatJID, senderJID); err != nil {
				return 0, err
			}
		}
	}
	unreadCount, err := store.MarkChatRead(chatJID.String(), filteredIDs)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errPersistMessage, err)
	}
	if len(filteredIDs) > 0 {
		publishRealtime(RealtimeEvent{Type: "receipt", ChatID: chatJID.String(), MessageIDs: filteredIDs})
		publishRealtime(RealtimeEvent{Type: "chat", ChatID: chatJID.String()})
	}
	return unreadCount, nil
}

func (wa *WAManager) Disconnect() { wa.client.Disconnect() }

func (wa *WAManager) Logout() error {
	err := wa.client.Logout(context.Background())
	wa.mu.Lock()
	wa.status = "unauthenticated"
	wa.qrCode = ""
	wa.mu.Unlock()
	return err
}

func (wa *WAManager) ResetSession() {
	wa.client.Disconnect()
	_ = wa.client.Store.Delete(context.Background())
	wa.client.Store.ID = nil
	wa.mu.Lock()
	wa.status = "unauthenticated"
	wa.qrCode = ""
	wa.mu.Unlock()
}
