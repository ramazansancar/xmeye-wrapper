// xmframe — 6611 1412 body'lerindeki XM video frame header'ini strip eder, saf H.264 ES cikarir.
// XM frame: 00 00 01 FX marker. FC(I)=16 byte header, FA/FD/F8/F9=8 byte header, sonra payload.
// Kullanim: go run ./cmd/xmframe testdata/login3.pcap out.h264
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("kullanim: xmframe <pcap> <out.h264>")
		os.Exit(1)
	}
	b, _ := os.ReadFile(os.Args[1])
	lt := binary.LittleEndian.Uint32(b[20:24])
	type sg struct {
		seq uint32
		pl  []byte
	}
	fl := map[string][]sg{}
	off := 24
	for off+16 <= len(b) {
		cl := int(binary.LittleEndian.Uint32(b[off+8 : off+12]))
		off += 16
		if off+cl > len(b) {
			break
		}
		fr := b[off : off+cl]
		off += cl
		e := ipPayload(lt, fr)
		if e == nil || len(e) < 20 || e[9] != 6 {
			continue
		}
		ihl := int(e[0]&0x0f) * 4
		if len(e) < ihl+20 {
			continue
		}
		l4 := e[ihl:]
		sp := binary.BigEndian.Uint16(l4[0:2])
		dp := binary.BigEndian.Uint16(l4[2:4])
		do := int(l4[12]>>4) * 4
		if len(l4) < do {
			continue
		}
		pl := l4[do:]
		if len(pl) == 0 || (sp != 6611 && dp != 6611) {
			continue
		}
		k := fmt.Sprintf("%d->%d", sp, dp)
		fl[k] = append(fl[k], sg{binary.BigEndian.Uint32(l4[4:8]), pl})
	}

	var es []byte
	stat := map[byte]int{}
	for _, ss := range fl {
		if len(ss) < 50 {
			continue
		}
		sort.Slice(ss, func(i, j int) bool { return ss[i].seq < ss[j].seq })
		var buf []byte
		for _, s := range ss {
			buf = append(buf, s.pl...)
		}
		// 1412 body'leri topla (ES = XM frame'lerin ard arda akisi)
		var raw []byte
		for i := 0; i+20 <= len(buf); {
			if buf[i] == 0xFF && buf[i+1] == 0x01 {
				ml := binary.LittleEndian.Uint16(buf[i+14 : i+16])
				dl := int(binary.LittleEndian.Uint32(buf[i+16 : i+20]))
				if dl > 2000000 {
					i++
					continue
				}
				bs := i + 20
				be := bs + dl
				if be > len(buf) {
					be = len(buf)
				}
				if ml == 1412 {
					raw = append(raw, buf[bs:be]...)
				}
				if dl == 0 {
					i += 20
				} else {
					i = be
				}
			} else {
				i++
			}
		}
		es = append(es, stripXM(raw, stat)...)
		break
	}

	os.WriteFile(os.Args[2], es, 0644)
	fmt.Printf(">>> %d byte saf H.264 ES yazildi: %s\n", len(es), os.Args[2])
	fmt.Printf("frame tipleri: ")
	for m, c := range stat {
		fmt.Printf("%02X×%d  ", m, c)
	}
	fmt.Println()
}

// stripXM — XM frame header'larini cikarir, saf H.264 NAL akisini dondurur.
// Marker: 00 00 01 FX. FC(I)=16 byte, digerleri(FA/FD/F8/F9/FB/FE)=8 byte header.
func stripXM(raw []byte, stat map[byte]int) []byte {
	var out []byte
	i := 0
	for i+4 <= len(raw) {
		if raw[i] == 0 && raw[i+1] == 0 && raw[i+2] == 1 && raw[i+3] >= 0xF0 {
			marker := raw[i+3]
			stat[marker]++
			var hdr, dlen int
			switch marker {
			case 0xFC, 0xFE: // I-frame / video key: 16 byte header
				hdr = 16
				if i+16 <= len(raw) {
					dlen = int(binary.LittleEndian.Uint32(raw[i+12 : i+16]))
				}
			default: // FA/FD/F8/F9/FB: 8 byte header
				hdr = 8
				if i+8 <= len(raw) {
					dlen = int(binary.LittleEndian.Uint32(raw[i+4 : i+8]))
				}
			}
			payStart := i + hdr
			payEnd := payStart + dlen
			if dlen <= 0 || payEnd > len(raw) {
				// uzunluk bozuk → bir sonraki markere atla
				i += 4
				continue
			}
			// sadece video marker'lari ES'e ekle (F8=extra/SPS-PPS olabilir, F9=audio atla)
			if marker != 0xF9 {
				out = append(out, raw[payStart:payEnd]...)
			}
			i = payEnd
		} else {
			i++
		}
	}
	return out
}

func ipPayload(lt uint32, f []byte) []byte {
	switch lt {
	case 1:
		if len(f) < 14 || binary.BigEndian.Uint16(f[12:14]) != 0x0800 {
			return nil
		}
		return f[14:]
	case 113:
		if len(f) < 16 || binary.BigEndian.Uint16(f[14:16]) != 0x0800 {
			return nil
		}
		return f[16:]
	case 276:
		if len(f) < 20 || binary.BigEndian.Uint16(f[0:2]) != 0x0800 {
			return nil
		}
		return f[20:]
	default:
		return f
	}
}
