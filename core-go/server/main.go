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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ramazansancar/xmeye-wrapper/core-go/transport"
)

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

	mux.HandleFunc("/live/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/live/")
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
