// xmeye-wrapper köprü sunucu.
//
// XMEye bulut relay'inin HTTP/.ts yolunu tarayıcıya HLS olarak sunar.
// Token + relay + session cihaz login/signaling'den gelir (bkz. dvrip/cloud — devam ediyor);
// prototipte env/config'ten alınır. Token uzun ömürlü, bir kez elde edilir.
//
// Uç noktalar:
//
//	GET /                       -> web arayüz (çok kanal grid, hls.js)
//	GET /api/info               -> cihaz + kanal listesi (JSON)
//	GET /live/{ch}.m3u8         -> kanal HLS playlist
//	GET /live/seg/{ch}/{n}.ts   -> segment (relay proxy)
//
// Kullanım (env):
//
//	XMEYE_SERIAL=... XMEYE_USER=koy1 XMEYE_PASS=... XMEYE_NAME=tbdc \
//	XMEYE_RELAY=<host:port> XMEYE_TOKEN=<token> XMEYE_SESSION=<hex> \
//	go run ./server
//
// ya da config.json (çalışma dizininde).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ramazansancar/xmeye-wrapper/core-go/cloud"
	"github.com/ramazansancar/xmeye-wrapper/core-go/transport"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true }, // köprü lokal/güvenli ağda
	ReadBufferSize:  1024,
	WriteBufferSize: 262144,
}

// envSource, StreamRef'i ortamdan sağlar (prototip TokenSource; ileride cloud login).
type envSource struct{ ref transport.StreamRef }

func (e envSource) Stream(channel int) (transport.StreamRef, error) {
	if e.ref.Token == "" || e.ref.RelayHost == "" {
		return transport.StreamRef{}, fmt.Errorf("token/relay ayarlı değil (XMEYE_TOKEN, XMEYE_RELAY)")
	}
	return e.ref, nil
}

func main() {
	cfg, err := LoadConfig(envOr("XMEYE_CONFIG", "config.json"))
	if err != nil {
		log.Fatalf("config okunamadı: %v", err)
	}
	src := envSource{ref: transport.StreamRef{
		RelayHost: os.Getenv("XMEYE_RELAY"),
		Token:     os.Getenv("XMEYE_TOKEN"),
		Serial:    cfg.Device.Serial,
		Session:   os.Getenv("XMEYE_SESSION"),
	}}
	proxy := transport.NewHLSProxy(src)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, playerHTML)
	})

	// cihaz + kanal bilgisi (arayüz grid kurmak için)
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":     cfg.Device.Name,
			"serial":   cfg.Device.Serial,
			"channels": cfg.Device.Channels,
		})
	})

	// komut gonderme (PTZ, kalite, snapshot, kayit, playback, genel DVRIP cmdId).
	// Simdilik iskelet: komutu kaydeder, 6611 DVRIP istemci hazir olunca cihaza iletir.
	mux.HandleFunc("/api/cmd", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodPost {
			http.Error(w, "POST gerekli", http.StatusMethodNotAllowed)
			return
		}
		var cmd map[string]any
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			http.Error(w, "geçersiz JSON", http.StatusBadRequest)
			return
		}
		log.Printf("komut: %v", cmd)
		w.Header().Set("Content-Type", "application/json")
		// backend 6611 istemci henuz yok → kabul edildi ama islenmedi
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   false,
			"note": "komut alındı; cihaz bağlantısı (6611 DVRIP) henüz aktif değil",
			"cmd":  cmd,
		})
	})

	// kayit araliklari (timeline icin). Backend 6611 OPQueryRecord hazir olunca doldurulur.
	// Simdilik bos liste → timeline kayit bolgesiz calisir.
	mux.HandleFunc("/api/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"records": []any{}, // {s:ms, e:ms, alarm:bool}
		})
	})

	// canlı akış — iki-bağlantı cloud → H.264 NAL. İki taşıma:
	//   WS  (ANA):   GET /ws/live/{ch}?stream=0|1   → H.264 NAL'leri binary WS frame olarak
	//   REST (ALT):  GET /live/{ch}.h264            → chunked HTTP, ham H.264 akışı
	liveHandler := func(ch, stream int, write func([]byte) error, done <-chan struct{}) {
		opt := cloud.LiveOptions{
			User:    cfg.Device.Username,
			Pass:    cfg.Device.Password,
			Channel: ch,
			Main:    stream == 0,
			Connect: &cloud.Options{StepTimeout: 4 * time.Second},
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { <-done; cancel() }()
		log.Printf("canlı akış başlıyor: kanal=%d stream=%d user=%s", ch, stream, opt.User)
		frames := 0
		err := cloud.StartLive(ctx, cfg.Device.Serial, opt, func(nal []byte) {
			frames++
			if write(nal) != nil {
				cancel()
			}
		})
		log.Printf("canlı akış bitti: kanal=%d frames=%d err=%v", ch, frames, err)
	}

	// ANA: tek WebSocket, çoğullanmış çok kanal. İstemci JSON mesaj gönderir:
	//   {"type":"play","ch":N,"stream":0|1}   → o kanalı başlat (zaten açıksa yok say)
	//   {"type":"stop","ch":N}                → o kanalı durdur
	// Sunucu her video frame'ini binary gönderir: [ch:1 byte][H.264 NAL...].
	// Tek soket, tek NAT yönetimi → çok kanal verimli (mobil/web).
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		hub := newChannelHub(conn, cfg)
		defer hub.closeAll()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m struct {
				Type   string `json:"type"`
				Ch     int    `json:"ch"`
				Stream int    `json:"stream"`
			}
			if json.Unmarshal(msg, &m) != nil {
				continue
			}
			switch m.Type {
			case "play":
				hub.play(m.Ch, m.Stream)
			case "stop":
				hub.stop(m.Ch)
			}
		}
	})

	// ALT (geri uyum): kanal başına ayrı WebSocket. /ws/live/{ch}?stream=0|1
	mux.HandleFunc("/ws/live/", func(w http.ResponseWriter, r *http.Request) {
		ch, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/ws/live/"))
		stream := 0
		if r.URL.Query().Get("stream") == "1" {
			stream = 1
		}
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		done := make(chan struct{})
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					close(done)
					return
				}
			}
		}()
		liveHandler(ch, stream, func(nal []byte) error {
			return conn.WriteMessage(websocket.BinaryMessage, nal)
		}, done)
	})

	mux.HandleFunc("/live/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/live/")
		// REST canlı (alternatif): /live/{ch}.h264 → chunked ham H.264
		if strings.HasSuffix(path, ".h264") {
			ch, _ := strconv.Atoi(strings.TrimSuffix(path, ".h264"))
			stream := 0
			if r.URL.Query().Get("stream") == "1" {
				stream = 1
			}
			w.Header().Set("Content-Type", "video/h264")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, _ := w.(http.Flusher)
			done := r.Context().Done()
			liveHandler(ch, stream, func(nal []byte) error {
				if _, err := w.Write(nal); err != nil {
					return err
				}
				if flusher != nil {
					flusher.Flush()
				}
				return nil
			}, done)
			return
		}
		switch {
		case strings.HasSuffix(path, ".m3u8"):
			ch, _ := strconv.Atoi(strings.TrimSuffix(path, ".m3u8"))
			pl, err := proxy.Playlist(ch)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			fmt.Fprint(w, pl)
		case strings.HasPrefix(path, "seg/"):
			parts := strings.Split(strings.TrimSuffix(path, ".ts"), "/")
			if len(parts) != 3 {
				http.Error(w, "geçersiz segment yolu", http.StatusBadRequest)
				return
			}
			ch, _ := strconv.Atoi(parts[1])
			seg, _ := strconv.Atoi(parts[2])
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if err := proxy.Segment(ch, seg, w); err != nil {
				log.Printf("segment %d/%d: %v", ch, seg, err)
				http.Error(w, err.Error(), http.StatusBadGateway)
			}
		default:
			http.NotFound(w, r)
		}
	})

	log.Printf("xmeye-wrapper köprü: http://localhost%s  cihaz=%s kanal=%d",
		cfg.Addr, cfg.Device.Name, cfg.Device.Channels)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
