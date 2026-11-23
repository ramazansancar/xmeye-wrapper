package transport

import (
	"strings"
	"testing"
)

// Sabit referans döndüren mock kaynak (gerçek relay/login gerektirmez).
type mockSource struct{ ref StreamRef }

func (m mockSource) Stream(channel int) (StreamRef, error) { return m.ref, nil }

func TestSegmentURL(t *testing.T) {
	ref := StreamRef{
		RelayHost: "101.44.32.246:8088",
		Token:     "ABC.def123",
		Serial:    "22e40425f2fc5906shc6",
		Session:   "a7680fd7session",
	}
	got := ref.SegmentURL(7)
	want := "http://101.44.32.246:8088/download/ABC.def123/22e40425f2fc5906shc6/a7680fd7session/7.ts"
	if got != want {
		t.Fatalf("SegmentURL:\n got  %s\n want %s", got, want)
	}
}

func TestPlaylist(t *testing.T) {
	p := NewHLSProxy(mockSource{ref: StreamRef{RelayHost: "h", Token: "t", Serial: "s", Session: "x"}})
	pl, err := p.Playlist(0)
	if err != nil {
		t.Fatalf("playlist hata: %v", err)
	}
	// temel HLS yapısı (nextSeg=1, SegStep=2 → 1,3,5,...)
	for _, must := range []string{"#EXTM3U", "#EXT-X-VERSION:3", "#EXT-X-TARGETDURATION", "#EXT-X-MEDIA-SEQUENCE:1", "#EXTINF:", "seg/0/1.ts", "seg/0/3.ts"} {
		if !strings.Contains(pl, must) {
			t.Errorf("playlist %q içermiyor:\n%s", must, pl)
		}
	}
	// pencere kadar segment satırı
	if n := strings.Count(pl, ".ts"); n != p.Window {
		t.Errorf("segment sayısı %d, beklenen %d", n, p.Window)
	}
}
