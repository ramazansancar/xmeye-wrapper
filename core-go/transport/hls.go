// Package transport, köprü sunucunun tarayıcıya video verme katmanıdır.
//
// hls.go: XMEye bulut relay'inin HTTP/.ts yolunu (port 8088) tarayıcıya HLS olarak sunar.
// Cihazın .ts segmentleri zaten MPEG-TS; biz bir .m3u8 playlist üretip segmentleri
// relay'den çekip (proxy) tarayıcıya iletiriz. Şifre çözme gerekmez.
//
// Token modeli: /download/<token>/<serial>/<session>/<seg>.ts — token + session + relay IP
// login/signaling'den gelir (bkz. cloud paketi). Burada TokenSource arayüzü ile soyutlanır.
package transport

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// StreamRef, tek bir kanal akışının relay'deki konumunu tanımlar.
// Bu bilgiler bulut signaling/login'den elde edilir (henüz tam otomatik değil).
type StreamRef struct {
	RelayHost string // örn "101.44.32.246:8088"
	Token     string // /download/<TOKEN> kısmı (md5 imzası dahil, URL-encoded)
	Serial    string // cihaz seri no
	Session   string // oturum hex
}

// SegmentURL, verilen segment numarası için tam .ts URL'sini kurar.
func (r StreamRef) SegmentURL(seg int) string {
	return fmt.Sprintf("http://%s/download/%s/%s/%s/%d.ts",
		r.RelayHost, r.Token, r.Serial, r.Session, seg)
}

// TokenSource, taze StreamRef sağlar. Gerçek kaynak bulut login'i olacak;
// prototipte elle doldurulmuş sabit bir kaynak da olabilir.
type TokenSource interface {
	// Stream, kanal için güncel relay referansını döndürür (token süreliyse yeniler).
	Stream(channel int) (StreamRef, error)
}

// HLSProxy, bir kanalı HLS playlist + segment proxy olarak sunar.
//
// XMEye relay'i .m3u8 KULLANMAZ; istemci segment numarasını kendi artırır
// (gözlem: ana akış 3,5,7,...; artış +2). Biz tarayıcıya standart HLS playlist
// üretip bu numaraları segment proxy'ye yönlendiririz. SegStep = numara artışı.
type HLSProxy struct {
	Source  TokenSource
	Client  *http.Client
	Window  int     // playlist'te tutulacak segment sayısı
	SegDur  float64 // segment süresi (sn), playlist #EXTINF için
	SegStep int     // ardışık segment no farkı (gözlemlenen: 2)
	mu      sync.Mutex
	nextSeg int // çekilecek sonraki segment no
}

// NewHLSProxy, varsayılanlarla bir proxy oluşturur.
func NewHLSProxy(src TokenSource) *HLSProxy {
	return &HLSProxy{
		Source:  src,
		Client:  &http.Client{Timeout: 10 * time.Second},
		Window:  6,
		SegDur:  1.0,
		SegStep: 2,
		nextSeg: 1,
	}
}

// Playlist, kanal için canlı .m3u8 üretir (kayan pencere).
func (p *HLSProxy) Playlist(channel int) (string, error) {
	ref, err := p.Source.Stream(channel)
	if err != nil {
		return "", err
	}
	_ = ref
	p.mu.Lock()
	start := p.nextSeg
	p.mu.Unlock()

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", int(p.SegDur)+1)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", start)
	step := p.SegStep
	if step <= 0 {
		step = 1
	}
	for i := 0; i < p.Window; i++ {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n", p.SegDur)
		fmt.Fprintf(&b, "seg/%d/%d.ts\n", channel, start+i*step)
	}
	return b.String(), nil
}

// Segment, tek bir .ts'yi relay'den çekip w'ye yazar (proxy).
func (p *HLSProxy) Segment(channel, seg int, w io.Writer) error {
	ref, err := p.Source.Stream(channel)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("GET", ref.SegmentURL(seg), nil)
	req.Header.Set("User-Agent", "Android APP") // relay bu UA'yı bekliyor (pcap'ten)
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("segment %d: relay HTTP %d (token süresi dolmuş olabilir)", seg, resp.StatusCode)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
