package main

import (
	"context"
	"encoding/base64"
	"log"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	wastore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type WAManager struct {
	client *whatsmeow.Client
	status string // "unauthenticated" | "connecting" | "connected"
	qrCode string // base64 PNG data URL of the current QR
	store  *UserDataStore
	mu     sync.RWMutex
}

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
		case *events.QR:
			png, _ := qrcode.Encode(v.Codes[0], qrcode.Medium, 256)
			dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
			wa.mu.Lock()
			wa.qrCode = dataURL
			wa.status = "connecting"
			wa.mu.Unlock()
			log.Println("[wa] New QR Code generated")
		case *events.Connected:
			wa.mu.Lock()
			wa.status = "connected"
			wa.mu.Unlock()
			log.Println("[wa] Connected to Whatsapp")
		case *events.Disconnected:
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.mu.Unlock()
			log.Println("[wa] Disconnected from WhatsApp")
		case *events.Message:
			if wa.store == nil {
				log.Println("[wa] Skipping message: store is nil")
				break
			}

			chatJID := v.Info.Chat.String()
			placeholder := Chat{
				ID: chatJID,
			}
			if err := wa.store.UpsertChat(placeholder); err != nil {
				log.Printf("[wa] Failed to upsert placeholder chat %s: %v", chatJID, err)
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
				}
			}
		case *events.Presence:
			log.Printf("[wa] Event: presence from=%s unavailable=%v lastSeen=%v", v.From, v.Unavailable, v.LastSeen)
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

			conversationsSeen := len(data.GetConversations())
			conversationsStored := 0
			conversationsSkipped := 0
			messagesSeen := 0
			messagesStored := 0
			messagesSkipped := 0

			log.Printf("[wa] Event: historySync type=%v chunk=%d progress=%d conversations=%d statusMessages=%d pushnames=%d inlineContacts=%d",
				data.GetSyncType(), data.GetChunkOrder(), data.GetProgress(), conversationsSeen, len(data.GetStatusV3Messages()), len(data.GetPushnames()), len(data.GetInlineContacts()))

			for _, conv := range data.GetConversations() {
				chatJID, err := types.ParseJID(conv.GetID())
				if err != nil {
					log.Printf("[wa] history sync invalid chat jid %q : %v", conv.GetID(), err)
					conversationsSkipped++
					continue
				}

				name := conv.GetName()
				if name == "" {
					name = conv.GetDisplayName()
				}
				chat := Chat{
					ID:          chatJID.String(),
					UnreadCount: int(conv.GetUnreadCount()),
					IsGroup:     chatJID.Server == "g.us",
					IsArchived:  conv.GetArchived(),
					IsCommunity: conv.GetIsParentGroup(),
				}

				if name != "" {
					chat.Name = &name
				}
				if err := wa.store.UpsertChat(chat); err != nil {
					log.Printf("[wa] failed to upsert history chat %s: %v", chatJID, err)
					conversationsSkipped++
					continue
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
			log.Printf("[wa] historySync done type=%v chunk=%d progress=%d conversationsSeen=%d conversationsStored=%d conversationsSkipped=%d messagesSeen=%d messagesStored=%d messagesSkipped=%d",
				data.GetSyncType(), data.GetChunkOrder(), data.GetProgress(), conversationsSeen, conversationsStored, conversationsSkipped, messagesSeen, messagesStored, messagesSkipped)
		case *events.PushName:
			log.Printf("[wa] Event: pushName jid=%s old=%q new=%q", v.JID, v.OldPushName, v.NewPushName)
			if wa.store != nil {
				wa.store.UpsertContact(User{
					ID:   v.JID.String(),
					Name: v.NewPushName,
				})
			}
		case *events.LoggedOut:
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Println("[wa] Logged out (session revoked)")
		default:
			log.Printf("[wa] Event: unhandled type=%T", evt)
		}
	})

	return wa, nil
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
	log.Printf("[wa] Event: message id = %s chat = %s sender = %s text = %q media = %s isFromMe = %v ",
		evt.Info.ID, evt.Info.Chat, evt.Info.Sender, text, mediaType, evt.Info.IsFromMe)

	chatJID := evt.Info.Chat.String()

	ourMsg := Message{
		ID:        evt.Info.ID,
		ChatJID:   chatJID,
		SenderID:  evt.Info.Sender.String(),
		Text:      text,
		Timestamp: evt.Info.Timestamp.Format(time.RFC3339),
		Status:    status,
		MediaType: mediaType,
		IsFromMe:  evt.Info.IsFromMe,
	}
	if err := wa.store.InsertMessage(ourMsg); err != nil {
		log.Printf("[wa] Failed to store message %s: %v", evt.Info.ID, err)
		return false
	}
	log.Printf("[wa] Stored message %s in chat %s", evt.Info.ID, chatJID)
	return true

}

func (wa *WAManager) StartPairing() {
	wa.mu.Lock()
	if wa.status != "unauthenticated" {
		wa.mu.Unlock()
		return
	}
	wa.status = "connecting"
	wa.qrCode = ""
	wa.mu.Unlock()
	go func() {
		ch, err := wa.client.GetQRChannel(context.Background())
		if err != nil {
			log.Println("[wa] GetQRChannel err: ", err)
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.mu.Unlock()
			return
		}
		if err := wa.client.Connect(); err != nil {
			log.Println("[wa] Connect error:", err)
		}
		for item := range ch {
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				png, _ := qrcode.Encode(item.Code, qrcode.Medium, 256)
				dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				wa.mu.Lock()
				wa.qrCode = dataURL
				wa.mu.Unlock()
			case "success":
				wa.mu.Lock()
				wa.status = "connected"
				wa.mu.Unlock()
			case "timeout":
				wa.mu.Lock()
				wa.status = "unauthenticated"
				wa.qrCode = ""
				wa.mu.Unlock()
			case "err-scanned-without-multidevice":
				log.Println("[wa] Phone doesn't have multidevice enabled")
			case whatsmeow.QRChannelEventError:
				log.Println("[wa] Pairing error:", item.Error)
			}
		}
	}()
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
