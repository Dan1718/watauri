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
	"go.mau.fi/whatsmeow/store/sqlstore"
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
	storeContainer, err := sqlstore.New(context.Background(), "sqlite3", "file:wa-session.db?_foreign_keys=on", nil)
	if err != nil {
		return nil, err
	}

	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}
	client := whatsmeow.NewClient(device, nil)
	wa := &WAManager{client: client, status: "unauthenticated", store: store}
	if device.ID != nil {
		wa.status = "connected"
		go func() {
			if err := client.Connect(); err != nil {
				log.Println("[wa] Auto-connect error:", err)
			}
		}()
	}
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			wa.mu.Lock()
			wa.status = "connected"
			wa.mu.Unlock()
			log.Println("[wa] Connected to Whatsapp")
		case *events.Message:
			if wa.store == nil {
				break
			}
			text := v.Message.GetConversation()
			if text == "" {
				if ext := v.Message.GetExtendedTextMessage(); ext != nil {
					text = ext.GetText()
				}
			}
			status := "received"
			if v.Info.IsFromMe {
				status = "sent"
			}
			mediaType := ""
			if v.Message.GetImageMessage() != nil {
				mediaType = "image"
			} else if v.Message.GetVideoMessage() != nil {
				mediaType = "video"
			} else if v.Message.GetAudioMessage() != nil {
				mediaType = "audio"
			} else if v.Message.GetDocumentMessage() != nil {
				mediaType = "document"
			}
			ourMsg := Message{
				ID:        v.Info.ID,
				ChatJID:   v.Info.Chat.String(),
				SenderID:  v.Info.Sender.String(),
				Text:      text,
				Timestamp: v.Info.Timestamp.Format(time.RFC3339),
				Status:    status,
				MediaType: mediaType,
				IsFromMe:  v.Info.IsFromMe,
			}
			if err := wa.store.InsertMessage(ourMsg); err != nil {
				log.Println("[wa] Failed to store message:", err)
			}
		case *events.LoggedOut:
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.qrCode = ""
			wa.mu.Unlock()
			log.Println("[wa] logged out")
		}
	})

	return wa, nil
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
