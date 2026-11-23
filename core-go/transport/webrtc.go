package transport

// webrtc.go — WebRTC transport (iskelet / tasarım).
//
// WebRTC en düşük gecikmeli yoldur (canlı izleme için ideal) ama kurulumu HLS/MSE'den
// karmaşıktır (ICE/SDP, RTP paketleme). İki gerçekçi yaklaşım:
//
//  1. go2rtc entegrasyonu (önerilen): go2rtc olgun bir medya sunucusudur; XMEye'nin
//     relay akışını (TS/H.264) alıp WebRTC/MSE/HLS'e çevirir. Köprü, go2rtc'yi gömülü
//     ya da yanında çalıştırıp akış kaynağını (bizim .ts proxy'si ya da ham H.264) besler.
//     Avantaj: WebRTC/ICE/TURN olgun; az kod. Dezavantaj: ekstra bileşen.
//
//  2. Saf Go (pion/webrtc): köprü H.264 NAL'leri RTP'ye paketleyip WebRTC track'e yazar.
//     Avantaj: tek binary, dış bileşen yok. Dezavantaj: pion bağımlılığı + H.264 paketleme kodu.
//
// Ortak ön koşul: ham H.264 (ya da H.265) elde etmek. 8088/.ts yolu zaten MPEG-TS taşır;
// TS demux → H.264 ES gerekir. 6611 DVRIP yolu çözülünce ham frame doğrudan gelir.
//
// Bu dosya şimdilik arayüzü tanımlar; implementasyon video kaynağı (token/AES) netleşince
// eklenecek. Karar (go2rtc vs pion) kaynak formatı belli olunca verilecek.

// VideoSource, transport-bağımsız ham video kaynağıdır (H.264/H.265 erişim birimleri).
// HLS/MSE .ts üzerinden çalışır; WebRTC ham ES ister, bu yüzden ayrı soyutlama.
type VideoSource interface {
	// NextAccessUnit, bir sonraki kodlanmış video erişim birimini (NAL grubu) döndürür.
	// Codec tespiti (H.264/H.265) ilk birimlerden yapılır.
	NextAccessUnit() (data []byte, keyframe bool, err error)
	// Codec, "h264" ya da "h265" döndürür (bilindiğinde).
	Codec() string
	Close() error
}

// WebRTCConfig, ileride pion ya da go2rtc köprüsü için ayarlar.
type WebRTCConfig struct {
	ICEServers []string // STUN/TURN (NAT arkası tarayıcılar için)
	UseGo2RTC  bool     // true: go2rtc köprüsü; false: saf pion (ileride)
}
