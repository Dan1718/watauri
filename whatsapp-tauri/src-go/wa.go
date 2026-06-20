package main

import (
	"context"
	"encoding/base64"
	"log"
	"sync"

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
	mu     sync.RWMutex
}

func newWAManager() (*WAManager, error) {
	storeContainer, err := sqlstore.New(context.Background(), "sqlite3", "file:wa-session.db?_foreign_keys=on", nil)
	if err != nil {
		return nil, err
	}

	device, err := storeContainer.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}
	client := whatsmeow.NewClient(device, nil)
	wa := &WAManager{client: client, status: "unauthenticated"}
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
	wa.status = "connecting"
	go func() {
		if err := wa.client.Connect(); err != nil {
			log.Println("[wa] Connect error:", err)
			wa.mu.Lock()
			wa.status = "unauthenticated"
			wa.mu.Unlock()
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
