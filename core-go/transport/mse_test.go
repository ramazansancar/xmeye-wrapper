package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// yol sonundaki "<n>.ts" parçasından n'i okur.
func fmtSscanfLast(path string, n *int) (int, error) {
	base := path[strings.LastIndex(path, "/")+1:]
	base = strings.TrimSuffix(base, ".ts")
	v, err := strconv.Atoi(base)
	if err != nil {
		return 0, err
	}
	*n = v
	return 1, nil
}

// Sahte relay: belirli segmentlere .ts döner, diğerlerine 404.
// SegmentStreamer'ın segment çekip WriteBinary'ye aktardığını doğrular.
func TestSegmentStreamer(t *testing.T) {
	// relay: /download/.../{n}.ts → n<=5 ise 200+"TS", yoksa 404
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// yol sonundaki Nts'yi al
		var n int
		_, _ = fmtSscanfLast(r.URL.Path, &n)
		if n >= 1 && n <= 5 {
			w.Write([]byte("TSDATA"))
			return
		}
		http.Error(w, "nf", http.StatusNotFound)
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	proxy := NewHLSProxy(mockSource{ref: StreamRef{
		RelayHost: host, Token: "t", Serial: "s", Session: "x",
	}})
	proxy.SegStep = 2

	var got [][]byte
	ss := &SegmentStreamer{
		Proxy:        proxy,
		Channel:      0,
		WriteBinary:  func(b []byte) error { got = append(got, append([]byte(nil), b...)); return nil },
		PollInterval: 5 * time.Millisecond,
		MaxMiss:      3,
	}
	// start 1 → 1,3,5 başarılı; 7 itibarıyla 404 → MaxMiss sonrası durur
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ss.Run(ctx, 1)

	if len(got) != 3 {
		t.Fatalf("beklenen 3 segment (1,3,5), gelen %d", len(got))
	}
	for _, g := range got {
		if string(g) != "TSDATA" {
			t.Errorf("segment içeriği yanlış: %q", g)
		}
	}
}
