package cloud

// live.go: canlı video akışı. İki-bağlantı modeli (cctv-backup):
//
//	A (kontrol): Connect + login(1000) → sessionID; OPMonitor Start(1410); periyodik keepalive.
//	B (video):   Connect (login YOK) + OPMonitor Claim(1413, aynı sessionID) → 1412 video akar.
//
//	B'den gelen akış: DVRIP paketleri (FF01, msgId 1412) → body = XM video frame (00 00 01 FX header).
//	XM header strip → ham H.264 NAL → onFrame callback.
//
// Gerçek cihazda doğrulandı: seri+user+pass → köyün kamera görüntüsü (H.264) decode edildi.

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ramazansancar/xmeye-wrapper/core-go/dvrip"
)

// LiveOptions, canlı akış ayarları.
type LiveOptions struct {
	User    string
	Pass    string
	Channel int
	Main    bool // true: ana akış (HD), false: alt (SD)
	Connect *Options
}

// LiveStream, bir kanalın canlı H.264 akışını yönetir (iki bağlantı + frame strip).
type LiveStream struct {
	ctrl  *Session // A
	ctrlS *Stream
	video *Session // B
	videoS *Stream

	sessionID uint32
	opt       LiveOptions
	cancel    context.CancelFunc
}

// StartLive, kanalın canlı akışını başlatır. onFrame her H.264 erişim birimi için çağrılır.
// Fonksiyon, akış bitene (ctx iptal / hata) dek bloklar; onFrame'i çağıran goroutine içinde.
func StartLive(ctx context.Context, serial string, opt LiveOptions, onFrame func(nal []byte)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// A: kontrol bağlantısı + login
	ctrl, err := Connect(ctx, serial, opt.Connect)
	if err != nil {
		return fmt.Errorf("kontrol bağlantısı: %w", err)
	}
	defer ctrl.Close()
	ctrlS := ctrl.NewStream()
	defer ctrlS.Close()

	loginPkt := dvrip.BuildLogin(opt.User, opt.Pass)
	if _, err := ctrlS.Write(loginPkt.Encode()); err != nil {
		return fmt.Errorf("login yazma: %w", err)
	}
	ctrlS.SetReadTimeoutHint(8 * time.Second)
	rb := make([]byte, 65536)
	n, err := ctrlS.Read(rb)
	if err != nil {
		return fmt.Errorf("login yanıtı: %w", err)
	}
	lp, _, err := dvrip.Decode(rb[:n])
	if err != nil {
		return fmt.Errorf("login yanıtı çözülemedi: %w", err)
	}
	sessionID := parseSessionIDFromBody(lp.Body)
	if sessionID == 0 {
		return fmt.Errorf("login başarısız / SessionID yok: %s", string(lp.Body))
	}

	// B: video bağlantısı (login YOK) + Claim. NAT bazen 2. eşzamanlı bağlantıyı geç veriyor → retry.
	var video *Session
	for attempt := 0; attempt < 3; attempt++ {
		video, err = Connect(ctx, serial, opt.Connect)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("video bağlantısı: %w", err)
	}
	defer video.Close()
	videoS := video.NewStream()
	defer videoS.Close()

	claim := dvrip.BuildOPMonitorClaim(sessionID, 1, opt.Channel, opt.Main)
	if _, err := videoS.Write(claim.Encode()); err != nil {
		return fmt.Errorf("claim yazma: %w", err)
	}
	videoS.SetReadTimeoutHint(4 * time.Second)
	if n, err := videoS.Read(rb); err == nil {
		_, _, _ = dvrip.Decode(rb[:n]) // 1414 Ret:100 (yoksay)
	}

	// A: Start (1410)
	start := dvrip.BuildOPMonitorStart(sessionID, 2, opt.Channel, opt.Main)
	if _, err := ctrlS.Write(start.Encode()); err != nil {
		return fmt.Errorf("start yazma: %w", err)
	}

	// A'da keepalive goroutine
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ka := dvrip.BuildJSONCommand(sessionID, 99, dvrip.MsgKeepAlive,
					map[string]any{"Name": "KeepAlive", "SessionID": dvrip.SessionHex(sessionID)})
				ctrlS.Write(ka.Encode())
			}
		}
	}()

	// B'den video oku → DVRIP parse → 1412 body → XM frame strip → H.264
	return readVideo(ctx, videoS, onFrame)
}

// readVideo, B stream'inden DVRIP paketlerini ayıklar, 1412 body'lerini XM-strip eder, H.264 verir.
func readVideo(ctx context.Context, s *Stream, onFrame func(nal []byte)) error {
	s.SetReadTimeoutHint(3 * time.Second)
	rbuf := make([]byte, 262144)
	var acc []byte
	var frameBuf []byte // XM frame ES biriktir (paketler arası bölünmüş frame'ler için)

	// flush: biriken XM frame'leri H.264'e çevirip gönder. XM I-frame (FC) sınırında kesmek güvenli.
	flush := func() {
		if len(frameBuf) == 0 {
			return
		}
		nal := stripXMFrames(frameBuf)
		if len(nal) > 0 {
			onFrame(nal)
		}
		frameBuf = frameBuf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return ctx.Err()
		default:
		}
		n, err := s.Read(rbuf)
		if err != nil {
			continue // timeout → tekrar dene
		}
		acc = append(acc, rbuf[:n]...)
		// DVRIP paketlerini ayıkla
		for len(acc) >= dvrip.HeaderLen {
			if acc[0] != 0xFF || acc[1] != 0x01 {
				acc = acc[1:] // hizalama
				continue
			}
			dataLen := int(binary.LittleEndian.Uint32(acc[16:20]))
			if dataLen > 2_000_000 {
				acc = acc[1:]
				continue
			}
			total := dvrip.HeaderLen + dataLen
			if len(acc) < total {
				break // gövde henüz tam değil
			}
			msgID := binary.LittleEndian.Uint16(acc[14:16])
			body := acc[dvrip.HeaderLen:total]
			if msgID == dvrip.MsgOPMonitorData || msgID == 1422 { // 1412 canlı, 1422 playback
				// XM I-frame (FC) marker = yeni GOP başı. Önceki biriken frame'leri burada flush et
				// (bölünmüş XM frame'ler birleşmiş olur), sonra yeni frame'i biriktir.
				if len(body) >= 4 && body[0] == 0 && body[1] == 0 && body[2] == 1 && body[3] == 0xFC {
					flush()
				}
				frameBuf = append(frameBuf, body...)
				if len(frameBuf) > 262144 { // emniyet: aşırı birikmeyi önle
					flush()
				}
			}
			acc = acc[total:]
		}
	}
}

// stripXMFrames, XM video frame header'larını (00 00 01 FX) çıkarır, ham H.264 NAL akışını döndürür.
func stripXMFrames(raw []byte) []byte {
	var out []byte
	i := 0
	for i+4 <= len(raw) {
		if raw[i] == 0 && raw[i+1] == 0 && raw[i+2] == 1 && raw[i+3] >= 0xF0 {
			marker := raw[i+3]
			var hdr, dlen int
			switch marker {
			case 0xFC, 0xFE: // I-frame / config: 16 byte header
				hdr = 16
				if i+16 <= len(raw) {
					dlen = int(binary.LittleEndian.Uint32(raw[i+12 : i+16]))
				}
			default: // P/FA/F8/F9: 8 byte header
				hdr = 8
				if i+8 <= len(raw) {
					dlen = int(binary.LittleEndian.Uint32(raw[i+4 : i+8]))
				}
			}
			payStart := i + hdr
			payEnd := payStart + dlen
			if dlen <= 0 || payEnd > len(raw) {
				i += 4
				continue
			}
			if marker != 0xF9 { // F9 = audio, atla
				out = append(out, raw[payStart:payEnd]...)
			}
			i = payEnd
		} else {
			i++
		}
	}
	return out
}

// parseSessionIDFromBody, login yanıtındaki "SessionID":"0x...." değerini uint32'e çevirir.
func parseSessionIDFromBody(body []byte) uint32 {
	s := string(body)
	i := indexOfStr(s, "\"SessionID\"")
	if i < 0 {
		return 0
	}
	j := indexOfStr(s[i:], "0x")
	if j < 0 {
		return 0
	}
	start := i + j + 2
	var val uint32
	for k := start; k < len(s); k++ {
		c := s[k]
		var d uint32
		switch {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint32(c-'A') + 10
		default:
			return val
		}
		val = val*16 + d
	}
	return val
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
