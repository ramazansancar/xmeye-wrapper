package transport

// mse.go: MPEG-TS segmentlerini WebSocket üzerinden tarayıcıya akıtır.
// Tarayıcı tarafı mpegts.js ile MSE'ye besler (ham .ts → remux gerekmez).
// HLS'e göre daha düşük gecikme; relay .ts'leri doğrudan iletilir.
//
// Akış: istemci WS açar → sunucu ardışık segmentleri (SegStep adımıyla) relay'den
// çekip WS'e yazar → mpegts.js oynatır. Tek yönlü (sunucu→istemci) ikili mesajlar.

import (
	"bytes"
	"context"
	"fmt"
	"time"
)

// SegmentStreamer, bir kanalın segmentlerini sırayla çekip bir yazıcıya (WS) aktarır.
// Transport-bağımsız: WS kütüphanesi main'de bağlanır, buraya WriteFunc verilir.
type SegmentStreamer struct {
	Proxy   *HLSProxy
	Channel int
	// WriteBinary, bir segmentin ham baytlarını istemciye yazar (ör. WS binary mesaj).
	WriteBinary func([]byte) error
	// PollInterval, segment 404 olduğunda (henüz hazır değil) tekrar deneme aralığı.
	PollInterval time.Duration
	// MaxMiss, üst üste bu kadar 404 sonrası vazgeç (akış bitti say).
	MaxMiss int
}

// Run, context iptal edilene ya da çok fazla ardışık 404 olana dek segment akıtır.
func (s *SegmentStreamer) Run(ctx context.Context, startSeg int) error {
	if s.PollInterval == 0 {
		s.PollInterval = 200 * time.Millisecond
	}
	if s.MaxMiss == 0 {
		s.MaxMiss = 20
	}
	step := s.Proxy.SegStep
	if step <= 0 {
		step = 1
	}
	seg := startSeg
	miss := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var buf bytes.Buffer
		err := s.Proxy.Segment(s.Channel, seg, &buf)
		if err != nil {
			// büyük olasılıkla 404 (segment henüz yok ya da geçti): kısa bekle, tekrar dene
			miss++
			if miss >= s.MaxMiss {
				return fmt.Errorf("segment akışı durdu (%d ardışık hata): %w", miss, err)
			}
			time.Sleep(s.PollInterval)
			continue
		}
		miss = 0
		if err := s.WriteBinary(buf.Bytes()); err != nil {
			return err // istemci kapandı
		}
		seg += step
	}
}
