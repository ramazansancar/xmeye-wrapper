// pcapanalyze — XMEye bulut trafiğini (pcap) analiz eder: TCP akışlarını ayıklar,
// DVRIP (FF 01) paket başlıklarını çözümler. Bağımlılık yok (pure Go), linktype any + ethernet.
//
// Kullanım: go run ./cmd/pcapanalyze testdata/xmeye.pcap
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// --- pcap global header (24 byte) + record header (16 byte) ---
type pcapRec struct {
	ts     uint32
	tsUsec uint32
	data   []byte
}

func readPcap(path string) (linkType uint32, recs []pcapRec, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	if len(b) < 24 {
		return 0, nil, fmt.Errorf("pcap çok kısa")
	}
	magic := binary.LittleEndian.Uint32(b[0:4])
	if magic != 0xa1b2c3d4 {
		return 0, nil, fmt.Errorf("beklenmeyen pcap magic: %08x (sadece LE classic pcap)", magic)
	}
	linkType = binary.LittleEndian.Uint32(b[20:24])
	off := 24
	for off+16 <= len(b) {
		tsSec := binary.LittleEndian.Uint32(b[off : off+4])
		tsUsec := binary.LittleEndian.Uint32(b[off+4 : off+8])
		capLen := binary.LittleEndian.Uint32(b[off+8 : off+12])
		off += 16
		if off+int(capLen) > len(b) {
			break
		}
		recs = append(recs, pcapRec{ts: tsSec, tsUsec: tsUsec, data: b[off : off+int(capLen)]})
		off += int(capLen)
	}
	return linkType, recs, nil
}

// Linux "cooked" (SLL, linktype 113) veya Ethernet (1) içinden IPv4 payload'ı çıkar.
func ipv4Payload(linkType uint32, frame []byte) (proto byte, src, dst [4]byte, l4 []byte, ok bool) {
	var eth []byte
	switch linkType {
	case 1: // Ethernet
		if len(frame) < 14 {
			return
		}
		etherType := binary.BigEndian.Uint16(frame[12:14])
		if etherType != 0x0800 {
			return
		}
		eth = frame[14:]
	case 113: // Linux SLL v1 (16 byte header, ethertype @14)
		if len(frame) < 16 {
			return
		}
		if binary.BigEndian.Uint16(frame[14:16]) != 0x0800 {
			return
		}
		eth = frame[16:]
	case 276: // Linux SLL v2 (20 byte header, ethertype @0 BE)
		if len(frame) < 20 {
			return
		}
		if binary.BigEndian.Uint16(frame[0:2]) != 0x0800 {
			return
		}
		eth = frame[20:]
	case 101: // raw IP
		eth = frame
	default:
		// "any" (linktype 113) çoğu; denenmeyen için ham dene
		eth = frame
	}
	if len(eth) < 20 {
		return
	}
	ihl := int(eth[0]&0x0f) * 4
	if ihl < 20 || len(eth) < ihl {
		return
	}
	proto = eth[9]
	copy(src[:], eth[12:16])
	copy(dst[:], eth[16:20])
	totalLen := int(binary.BigEndian.Uint16(eth[2:4]))
	if totalLen > len(eth) || totalLen < ihl {
		totalLen = len(eth)
	}
	l4 = eth[ihl:totalLen]
	ok = true
	return
}

type tcpSeg struct {
	srcPort, dstPort uint16
	seq              uint32
	payload          []byte
}

func parseTCP(l4 []byte) (tcpSeg, bool) {
	if len(l4) < 20 {
		return tcpSeg{}, false
	}
	dataOff := int(l4[12]>>4) * 4
	if dataOff < 20 || len(l4) < dataOff {
		return tcpSeg{}, false
	}
	return tcpSeg{
		srcPort: binary.BigEndian.Uint16(l4[0:2]),
		dstPort: binary.BigEndian.Uint16(l4[2:4]),
		seq:     binary.BigEndian.Uint32(l4[4:8]),
		payload: l4[dataOff:],
	}, true
}

func ipStr(a [4]byte) string { return fmt.Sprintf("%d.%d.%d.%d", a[0], a[1], a[2], a[3]) }

// akış = yön bazlı (src:port -> dst:port). seq'e göre sıralı payload birleştir.
type flowKey struct {
	src, dst         [4]byte
	srcPort, dstPort uint16
}

type flow struct {
	segs  []tcpSeg
	bytes int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("kullanım: pcapanalyze <dosya.pcap>")
		os.Exit(1)
	}
	linkType, recs, err := readPcap(os.Args[1])
	if err != nil {
		fmt.Println("hata:", err)
		os.Exit(1)
	}
	fmt.Printf("pcap: %d paket, linktype=%d\n\n", len(recs), linkType)

	flows := map[flowKey]*flow{}
	portHits := map[uint16]int{}

	for _, r := range recs {
		proto, src, dst, l4, ok := ipv4Payload(linkType, r.data)
		if !ok || proto != 6 { // TCP
			continue
		}
		seg, ok := parseTCP(l4)
		if !ok {
			continue
		}
		portHits[seg.srcPort]++
		portHits[seg.dstPort]++
		if len(seg.payload) == 0 {
			continue
		}
		k := flowKey{src, dst, seg.srcPort, seg.dstPort}
		f := flows[k]
		if f == nil {
			f = &flow{}
			flows[k] = f
		}
		f.segs = append(f.segs, seg)
		f.bytes += len(seg.payload)
	}

	// ilgilendiğimiz portlar
	fmt.Println("== port istatistiği (XMEye ilgili) ==")
	for _, p := range []uint16{12132, 8088, 34567, 8000} {
		if portHits[p] > 0 {
			fmt.Printf("  port %d: %d segment\n", p, portHits[p])
		}
	}

	// akışları boyuta göre sırala
	type kv struct {
		k flowKey
		f *flow
	}
	var list []kv
	for k, f := range flows {
		list = append(list, kv{k, f})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].f.bytes > list[j].f.bytes })

	fmt.Println("\n== en büyük 12 akış ==")
	for i, e := range list {
		if i >= 12 {
			break
		}
		fmt.Printf("  %s:%d -> %s:%d  %d byte, %d seg\n",
			ipStr(e.k.src), e.k.srcPort, ipStr(e.k.dst), e.k.dstPort, e.f.bytes, len(e.f.segs))
	}

	// HTTP istek/yanıt header'ları (8088 relay HTTP tabanlı çıktı)
	fmt.Println("\n== HTTP istek satırları (giden, :8088) ==")
	httpSeen := map[string]int{}
	for _, e := range list {
		if e.k.dstPort != 8088 {
			continue
		}
		sort.Slice(e.f.segs, func(i, j int) bool { return e.f.segs[i].seq < e.f.segs[j].seq })
		var buf []byte
		for _, s := range e.f.segs {
			buf = append(buf, s.payload...)
		}
		// ilk satır (GET/POST ... HTTP/1.1) + Host + ilgili header'lar
		line := firstLine(buf)
		if line != "" {
			httpSeen[line]++
			if httpSeen[line] <= 2 {
				fmt.Printf("  %s:%d -> %s:%d\n    %s\n    %s\n", ipStr(e.k.src), e.k.srcPort, ipStr(e.k.dst), e.k.dstPort, line, hdrLines(buf))
			}
		}
	}

	// 12132 + 8088 akışlarında DVRIP FF01 ara (handshake + relay)
	fmt.Println("\n== 12132/8088 akışlarında DVRIP (FF 01) başlıkları ==")
	shown := 0
	for _, e := range list {
		relayPorts := map[uint16]bool{12132: true, 8088: true, 6611: true, 6510: true, 8000: true}
		if !relayPorts[e.k.dstPort] && !relayPorts[e.k.srcPort] {
			continue
		}
		if shown >= 6 { // en büyük birkaç akış yeter
			break
		}
		shown++
		// seq sırala + birleştir
		sort.Slice(e.f.segs, func(i, j int) bool { return e.f.segs[i].seq < e.f.segs[j].seq })
		var buf []byte
		for _, s := range e.f.segs {
			buf = append(buf, s.payload...)
		}
		dir := fmt.Sprintf("%s:%d->%s:%d", ipStr(e.k.src), e.k.srcPort, ipStr(e.k.dst), e.k.dstPort)
		scanDVRIP(dir, buf)
	}
}

// DVRIP paket: XM genel kafa 20 byte: [0xFF ver 0 0][0..3 session][0..3 seq?][... msgId(2) len(4)]
// Resmî Sofia kafa: byte0=0xFF, byte1=version(0x01), 2-3 reserved, 4-7 sessionId(LE),
// 8-11 seq, 12 total/cur pkt, 13 cur pkt, 14-15 msgId(LE), 16-19 dataLen(LE), sonra body.
func scanDVRIP(dir string, buf []byte) {
	n := 0
	for i := 0; i+20 <= len(buf); {
		if buf[i] == 0xFF && buf[i+1] == 0x01 {
			sess := binary.LittleEndian.Uint32(buf[i+4 : i+8])
			seq := binary.LittleEndian.Uint32(buf[i+8 : i+12])
			msgID := binary.LittleEndian.Uint16(buf[i+14 : i+16])
			dataLen := binary.LittleEndian.Uint32(buf[i+16 : i+20])
			if dataLen > 2_000_000 { // saçma uzunluk → yanlış hizalama, 1 ilerle
				i++
				continue
			}
			bodyStart := i + 20
			bodyEnd := bodyStart + int(dataLen)
			if bodyEnd > len(buf) {
				bodyEnd = len(buf)
			}
			body := buf[bodyStart:bodyEnd]
			preview := ascii(body, 80)
			fmt.Printf("  [%s] sess=%08x seq=%d msgId=%d len=%d body=%q\n", dir, sess, seq, msgID, dataLen, preview)
			n++
			if n > 40 {
				fmt.Println("  ... (kesildi)")
				return
			}
			if dataLen == 0 {
				i += 20
			} else {
				i = bodyEnd
			}
		} else {
			i++
		}
	}
	if n == 0 {
		fmt.Printf("  [%s] FF01 yok (%d byte). İlk 64 byte:\n    hex: %x\n    asc: %q\n", dir, len(buf), head(buf, 64), ascii(buf, 64))
	}
}

func firstLine(b []byte) string {
	// yalnız yazdırılabilir başlıyorsa (HTTP metin)
	if len(b) < 4 {
		return ""
	}
	m := string(b[:min(4, len(b))])
	if m[:3] != "GET" && m[:3] != "POS" && m[:3] != "PUT" && m[:3] != "HEA" && m[:4] != "HTTP" {
		return ""
	}
	for i := 0; i < len(b)-1 && i < 512; i++ {
		if b[i] == '\r' && b[i+1] == '\n' {
			return string(b[:i])
		}
	}
	return ""
}

// Host, X-*, Content-Type gibi ilginç header'ları tek satırda topla
func hdrLines(b []byte) string {
	s := string(b[:min(1024, len(b))])
	out := ""
	start := 0
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\r' && s[i+1] == '\n' {
			ln := s[start:i]
			start = i + 2
			if ln == "" {
				break
			}
			low := toLower(ln)
			if has(low, "host:") || has(low, "x-") || has(low, "content-type") || has(low, "authorization") || has(low, "user-agent") || has(low, "range") {
				out += ln + "  |  "
			}
		}
	}
	return out
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
func has(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func head(b []byte, max int) []byte {
	if len(b) > max {
		return b[:max]
	}
	return b
}

func ascii(b []byte, max int) string {
	if len(b) > max {
		b = b[:max]
	}
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 32 && c < 127 {
			out[i] = c
		} else {
			out[i] = '.'
		}
	}
	return string(out)
}
