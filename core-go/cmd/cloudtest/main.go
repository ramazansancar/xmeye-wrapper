// cloudtest — gerçek XMEye bulutuna (secu100.net) bağlanmayı dener, her adımı raporlar.
// Protokol doğrulama aracı (entegrasyondan önce). Gizli: seri no argümandan/env'den, loglanmaz.
//
// Kullanım: go run ./cmd/cloudtest <seri-no>   (ya da XMEYE_SERIAL env)
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/ramazansancar/xmeye-wrapper/core-go/cloud"
	"github.com/ramazansancar/xmeye-wrapper/core-go/dvrip"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func main() {
	serial := ""
	if len(os.Args) > 1 {
		serial = os.Args[1]
	} else {
		serial = os.Getenv("XMEYE_SERIAL")
	}
	if serial == "" {
		fmt.Println("kullanım: cloudtest <seri-no>  (ya da XMEYE_SERIAL env)")
		os.Exit(1)
	}
	masked := serial
	if len(masked) > 6 {
		masked = masked[:4] + "…" + masked[len(masked)-2:]
	}
	fmt.Printf("== bulut bağlantı testi (seri %s) ==\n", masked)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opts := &cloud.Options{StepTimeout: 4 * time.Second}

	// Adım 1: master discovery
	fmt.Print("1) master discovery (secu100.net:7999) ... ")
	t0 := time.Now()
	masterIP, err := cloud.DiscoverMaster(ctx, opts)
	if err != nil {
		fmt.Printf("HATA: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK → master=%s (%v)\n", masterIP, time.Since(t0).Round(time.Millisecond))

	// Adım 2: UUID query (seri → NAT sunucu)
	fmt.Print("2) UUID query (master:8777) ... ")
	t0 = time.Now()
	info, err := cloud.QueryDeviceNatServer(ctx, masterIP, 8777, serial, opts)
	if err != nil {
		fmt.Printf("HATA: %v\n", err)
		fmt.Println("   (master doğru ama seri no bu master'da kayıtlı olmayabilir; farklı master/port denenebilir)")
		os.Exit(1)
	}
	fmt.Printf("OK → status=%d natServer=%s:%d natType=%d (%v)\n",
		info.Status, info.ServerIP, info.ServerPort, info.NatType, time.Since(t0).Round(time.Millisecond))
	if info.Status != 1 {
		fmt.Println("   status != 1 → cihaz çevrimdışı ya da bulunamadı.")
		os.Exit(1)
	}

	// Adım 3: peer bağlantı (P2P + proxy)
	fmt.Print("3) CPeerConnect (WAN → PeerConnect → P2P/proxy) ... ")
	t0 = time.Now()
	sess, err := cloud.Connect(ctx, serial, opts)
	if err != nil {
		fmt.Printf("HATA: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()
	via := "P2P (direct)"
	if sess.ViaProxy() {
		via = "PROXY (relay)"
	}
	fmt.Printf("OK → peer=%s via %s (%v)\n", sess.Peer(), via, time.Since(t0).Round(time.Millisecond))

	// Adım 4: XMIP stream aç
	fmt.Print("4) XMIP stream aç ... ")
	stream := sess.NewStream()
	defer stream.Close()
	fmt.Println("OK → io.ReadWriteCloser hazır")

	// Adım 5: DVRIP login (user/pass env'den varsa). Yanıt 1001: SessionID + DataUseAES.
	user := os.Getenv("XMEYE_USER")
	pass := os.Getenv("XMEYE_PASS")
	if user == "" {
		fmt.Println("\n(XMEYE_USER/PASS yok → login denenmedi. Bağlantı zinciri çalışıyor.)")
		return
	}
	fmt.Printf("5) DVRIP login (user=%s) ... ", user)
	loginPkt := dvrip.BuildLogin(user, pass)
	if _, err := stream.Write(loginPkt.Encode()); err != nil {
		fmt.Printf("HATA (yazma): %v\n", err)
		os.Exit(1)
	}
	// yanıt oku (1001)
	stream.SetReadTimeoutHint(6 * time.Second)
	resp := make([]byte, 65536)
	n, err := stream.Read(resp)
	if err != nil {
		fmt.Printf("HATA (okuma): %v\n", err)
		os.Exit(1)
	}
	pkt, _, derr := dvrip.Decode(resp[:n])
	if derr != nil {
		fmt.Printf("yanıt çözülemedi (%d byte): %x\n", n, resp[:min(n, 64)])
		os.Exit(1)
	}
	fmt.Printf("OK → msgId=%d (len=%d)\n", pkt.MsgID, len(pkt.Body))
	fmt.Printf("   yanıt gövdesi: %s\n", string(pkt.Body))

	// login yanıtından SessionID çıkar
	sessionID := parseSessionID(pkt.Body)
	if sessionID == 0 {
		fmt.Println("   SessionID çözülemedi → OPMonitor denenmedi.")
		return
	}

	ch := 0
	// İKİ BAĞLANTI modeli (cctv-backup): A=kontrol (login'li), B=claim (login YOK, aynı sessionID).
	// Claim B'den, Start A'dan → video B'den gelir. relayToken GEREKMEZ.
	fmt.Printf("6) CLAIM bağlantısı (2. NAT stream, login yok, sess=0x%X) ... ", sessionID)
	vsess, err := cloud.Connect(ctx, serial, opts)
	if err != nil {
		fmt.Printf("HATA: %v\n", err)
		os.Exit(1)
	}
	defer vsess.Close()
	vstream := vsess.NewStream() // B = video/claim
	defer vstream.Close()
	fmt.Println("OK")

	// B: OPMonitor Claim (kanal, login session'ı taşır)
	fmt.Printf("7a) Claim (B bağlantısı, kanal %d) ... ", ch)
	claim := dvrip.BuildOPMonitorClaim(sessionID, 1, ch, true)
	if _, err := vstream.Write(claim.Encode()); err != nil {
		fmt.Printf("HATA: %v\n", err)
		os.Exit(1)
	}
	vstream.SetReadTimeoutHint(4 * time.Second)
	cb := make([]byte, 8192)
	if n, err := vstream.Read(cb); err == nil {
		if cp, _, e := dvrip.Decode(cb[:n]); e == nil {
			fmt.Printf("yanıt msg=%d %s\n", cp.MsgID, string(cp.Body))
		} else {
			fmt.Println("(yanıt çözülemedi)")
		}
	} else {
		fmt.Println("(yanıt yok)")
	}

	// A: OPMonitor Start (1410, kontrol bağlantısı, channel header[12])
	fmt.Printf("7b) Start (A kontrol bağlantısı, 1410, kanal %d) ... ", ch)
	start := dvrip.BuildOPMonitorStart(sessionID, 2, ch, true)
	if _, err := stream.Write(start.Encode()); err != nil {
		fmt.Printf("HATA: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("gönderildi")

	// B'den video oku (A'da keepalive)
	fmt.Println("8) B bağlantısından video bekleniyor...")
	var dumpFile *os.File
	if dp := os.Getenv("XMEYE_DUMP"); dp != "" {
		dumpFile, _ = os.Create(dp)
		if dumpFile != nil {
			defer dumpFile.Close()
		}
	}
	vstream.SetReadTimeoutHint(2 * time.Second)
	var got1412, gotOther, totalBytes int
	deadline := time.Now().Add(12 * time.Second)
	rbuf := make([]byte, 262144)
	var acc []byte
	lastKa := time.Now()
	for time.Now().Before(deadline) {
		// A'da keepalive (kontrol bağlantısı canlı kalsın)
		if time.Since(lastKa) > 5*time.Second {
			ka := dvrip.BuildJSONCommand(sessionID, 99, dvrip.MsgKeepAlive, map[string]any{"Name": "KeepAlive", "SessionID": dvrip.SessionHex(sessionID)})
			stream.Write(ka.Encode())
			lastKa = time.Now()
		}
		n, err := vstream.Read(rbuf)
		if err != nil {
			continue
		}
		// TEŞHİS: tüm gelen ham veriyi dosyaya biriktir (sonra ffmpeg ile çöz)
		if dumpFile != nil {
			dumpFile.Write(rbuf[:n])
		}
		acc = append(acc, rbuf[:n]...)
		// akıştan DVRIP paketlerini ayıkla
		for len(acc) >= dvrip.HeaderLen {
			p, consumed, derr := dvrip.Decode(acc)
			if derr != nil {
				acc = acc[1:] // hizalama kaymış, 1 ilerle
				continue
			}
			acc = acc[consumed:]
			totalBytes += len(p.Body)
			if p.MsgID == dvrip.MsgOPMonitorData {
				got1412++
			} else {
				gotOther++
				if gotOther <= 5 {
					fmt.Printf("   [diğer paket] msgId=%d len=%d body=%s\n", p.MsgID, len(p.Body), string(p.Body))
				}
			}
		}
	}
	fmt.Printf("   → 1412 video paketi: %d, diğer: %d, toplam body: %d byte\n", got1412, gotOther, totalBytes)
	// teşhis: ham UDP seviyesinde ne geldi (XMIP işlemeden önce)
	raw, cmds := vstream.Stats()
	fmt.Printf("   [TEŞHİS] ham UDP paketi: %d, cmd dağılımı:", raw)
	for c, n := range cmds {
		fmt.Printf(" %08x×%d", c, n)
	}
	fmt.Println()
	if got1412 > 0 {
		fmt.Println("\n✓✓✓ VIDEO GELDI! Bulut → login → OPMonitor → 1412 H.264 çalışıyor. Kalan: frame strip + HLS.")
	} else {
		fmt.Println("\n~ Login OK ama 1412 gelmedi (OPMonitor formatı/kanal ayarı kontrol edilmeli).")
	}
}

// parseSessionID, login yanıtındaki "SessionID":"0x...." değerini uint32'e çevirir.
func parseSessionID(body []byte) uint32 {
	s := string(body)
	key := "\"SessionID\""
	i := indexOf(s, key)
	if i < 0 {
		return 0
	}
	// 0x... bul
	j := indexOf(s[i:], "0x")
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

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
