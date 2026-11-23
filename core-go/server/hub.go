package main

// hub.go: tek WebSocket üstünde çok kanal video çoğullama.
// İstemci "play"/"stop" mesajlarıyla kanal açar/kapatır; her kanalın H.264 frame'leri
// tek sokete [ch:1 byte][H.264...] biçiminde yazılır. WS yazımı tek mutex ile korunur.

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ramazansancar/xmeye-wrapper/core-go/cloud"
)

type channelHub struct {
	conn *websocket.Conn
	cfg  *Config

	mu       sync.Mutex // WS yazımı + chans haritası
	chans    map[int]context.CancelFunc
}

func newChannelHub(conn *websocket.Conn, cfg *Config) *channelHub {
	return &channelHub{conn: conn, cfg: cfg, chans: make(map[int]context.CancelFunc)}
}

// play, kanalı başlatır (zaten açıksa yok sayar).
func (h *channelHub) play(ch, stream int) {
	h.mu.Lock()
	if _, exists := h.chans[ch]; exists {
		h.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.chans[ch] = cancel
	h.mu.Unlock()

	go func() {
		opt := cloud.LiveOptions{
			User:    h.cfg.Device.Username,
			Pass:    h.cfg.Device.Password,
			Channel: ch,
			Main:    stream == 0,
			Connect: &cloud.Options{StepTimeout: 4 * time.Second},
		}
		log.Printf("[hub] play kanal=%d stream=%d başlıyor", ch, stream)
		// NAT rate-limit / geçici hata → backoff ile tekrar dene (istemci stop'a kadar).
		backoff := 2 * time.Second
		for ctx.Err() == nil {
			frames, bytes := 0, 0
			err := cloud.StartLive(ctx, h.cfg.Device.Serial, opt, func(nal []byte) {
				frames++
				bytes += len(nal)
				buf := make([]byte, 1+len(nal)) // [ch:1 byte][H.264...]
				buf[0] = byte(ch)
				copy(buf[1:], nal)
				h.mu.Lock()
				werr := h.conn.WriteMessage(websocket.BinaryMessage, buf)
				h.mu.Unlock()
				if werr != nil {
					cancel()
				}
			})
			if ctx.Err() != nil {
				break
			}
			log.Printf("[hub] kanal=%d akış bitti frames=%d bytes=%d err=%v → %v sonra tekrar", ch, frames, bytes, err, backoff)
			if frames > 0 {
				backoff = 2 * time.Second // akış vardıysa backoff sıfırla
			}
			select {
			case <-ctx.Done():
			case <-time.After(backoff):
			}
			if backoff < 15*time.Second {
				backoff += 3 * time.Second // NAT yorgunsa gittikçe uzat
			}
		}
		h.mu.Lock()
		delete(h.chans, ch)
		h.mu.Unlock()
	}()
}

// stop, kanalı durdurur.
func (h *channelHub) stop(ch int) {
	h.mu.Lock()
	if cancel, ok := h.chans[ch]; ok {
		cancel()
		delete(h.chans, ch)
	}
	h.mu.Unlock()
}

// closeAll, tüm kanalları durdurur (WS kapanınca).
func (h *channelHub) closeAll() {
	h.mu.Lock()
	for _, cancel := range h.chans {
		cancel()
	}
	h.chans = make(map[int]context.CancelFunc)
	h.mu.Unlock()
}
