package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
)

func TestStartPairingFailuresRestoreUnauthenticatedState(t *testing.T) {
	tests := []struct {
		name         string
		getQRChannel func() (<-chan whatsmeow.QRChannelItem, error)
		connect      func() error
		qrTimeout    time.Duration
		blockConnect bool
	}{
		{
			name: "QR channel setup error",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				return nil, errors.New("setup failed")
			},
		},
		{
			name: "connect error",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				return make(chan whatsmeow.QRChannelItem), nil
			},
			connect: func() error { return errors.New("connect failed") },
		},
		{
			name: "channel closes before success",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				ch := make(chan whatsmeow.QRChannelItem)
				close(ch)
				return ch, nil
			},
		},
		{
			name: "QR channel error",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				ch := make(chan whatsmeow.QRChannelItem, 1)
				ch <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventError, Error: errors.New("pairing failed")}
				return ch, nil
			},
		},
		{
			name: "terminal QR event",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				ch := make(chan whatsmeow.QRChannelItem, 1)
				ch <- whatsmeow.QRChannelItem{Event: "err-client-outdated"}
				return ch, nil
			},
		},
		{
			name: "no QR before timeout despite Connect hanging",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				return make(chan whatsmeow.QRChannelItem), nil
			},
			blockConnect: true,
			qrTimeout:    10 * time.Millisecond,
		},
		{
			name: "WhatsApp QR timeout event",
			getQRChannel: func() (<-chan whatsmeow.QRChannelItem, error) {
				ch := make(chan whatsmeow.QRChannelItem, 1)
				ch <- whatsmeow.QRChannelItem{Event: "timeout"}
				return ch, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connectRelease := make(chan struct{})
			if tt.blockConnect {
				defer close(connectRelease)
			}
			wa := &WAManager{
				status:           "unauthenticated",
				qrCode:           "stale-qr",
				pairingAttempt:   1,
				pairingQRTimeout: tt.qrTimeout,
				getQRChannel: func(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
					return tt.getQRChannel()
				},
				connectForPairing: tt.connect,
			}
			if tt.blockConnect {
				wa.connectForPairing = func() error {
					<-connectRelease
					return nil
				}
			}
			if wa.connectForPairing == nil {
				wa.connectForPairing = func() error { return nil }
			}

			wa.mu.Lock()
			wa.status = "connecting"
			wa.mu.Unlock()
			wa.runPairingAttempt(1)
			if got := wa.GetStatus(); got != "unauthenticated" {
				t.Fatalf("status = %q, want unauthenticated", got)
			}
			if got := wa.GetQR(); got != "" {
				t.Fatalf("QR = %q, want cleared", got)
			}
		})
	}
}

func TestStartPairingSuccessAndRetryAfterFailure(t *testing.T) {
	var starts atomic.Int32
	wa := &WAManager{status: "unauthenticated"}
	wa.pairingQRTimeout = time.Second
	wa.getQRChannel = func(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
		if starts.Add(1) == 1 {
			ch := make(chan whatsmeow.QRChannelItem)
			close(ch)
			return ch, nil
		}
		ch := make(chan whatsmeow.QRChannelItem, 1)
		ch <- whatsmeow.QRChannelItem{Event: "success"}
		return ch, nil
	}
	wa.connectForPairing = func() error { return nil }

	wa.pairingAttempt = 1
	wa.status = "connecting"
	wa.runPairingAttempt(1)
	if got := wa.GetStatus(); got != "unauthenticated" {
		t.Fatalf("after first attempt status = %q, want unauthenticated", got)
	}
	wa.StartPairing()
	waitForPairingStatus(t, wa, "connected")
	if got := starts.Load(); got != 2 {
		t.Fatalf("pairing starts = %d, want 2", got)
	}
}

func TestOldPairingFailureDoesNotOverwriteCurrentAttempt(t *testing.T) {
	wa := &WAManager{status: "connecting", pairingAttempt: 2}
	wa.failPairingAttempt(1, "stale failure")
	if got := wa.GetStatus(); got != "connecting" {
		t.Fatalf("status = %q, want connecting", got)
	}
}

func TestPairingQRTimeoutOnlyAppliesUntilFirstQR(t *testing.T) {
	ch := make(chan whatsmeow.QRChannelItem, 2)
	ch <- whatsmeow.QRChannelItem{Event: "err-scanned-without-multidevice"}
	ch <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "pairing-code"}
	wa := &WAManager{
		status:           "unauthenticated",
		pairingQRTimeout: 10 * time.Millisecond,
		getQRChannel: func(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
			return ch, nil
		},
		connectForPairing: func() error { return nil },
	}

	wa.StartPairing()
	waitForQR(t, wa)
	time.Sleep(20 * time.Millisecond)
	if got := wa.GetStatus(); got != "connecting" {
		t.Fatalf("status after receiving QR = %q, want connecting", got)
	}
	ch <- whatsmeow.QRChannelItem{Event: "success"}
	waitForPairingStatus(t, wa, "connected")
}

func waitForQR(t *testing.T, wa *WAManager) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if wa.GetQR() != "" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for QR")
}

func waitForPairingStatus(t *testing.T, wa *WAManager, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if wa.GetStatus() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("status = %q, want %q", wa.GetStatus(), want)
}
