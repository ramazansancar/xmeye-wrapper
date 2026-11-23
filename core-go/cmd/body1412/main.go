// body1412 — 6611 msgId=1412 body'lerini hex + entropi + ilk/son byte desenleriyle dok.
// AES mi (yuksek entropi, rastgele) yoksa yapisal mi (dusuk entropi, tekrar, H.264 start code) ayirt etmek icin.
// Kullanim: go run ./cmd/body1412 testdata/login2.pcap
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("kullanim: body1412 <dosya.pcap>")
		os.Exit(1)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("hata:", err)
		os.Exit(1)
	}
	linkType := binary.LittleEndian.Uint32(b[20:24])
	// akis birlestir (6611 giden video)
	type seg struct {
		seq uint32
		pl  []byte
	}
	flows := map[string][]seg{}
	off := 24
	for off+16 <= len(b) {
		capLen := int(binary.LittleEndian.Uint32(b[off+8 : off+12]))
		off += 16
		if off+capLen > len(b) {
			break
		}
		frame := b[off : off+capLen]
		off += capLen
		eth := ipPayload(linkType, frame)
		if eth == nil || len(eth) < 20 || eth[9] != 6 {
			continue
		}
		ihl := int(eth[0]&0x0f) * 4
		if len(eth) < ihl+20 {
			continue
		}
		src := fmt.Sprintf("%d.%d.%d.%d", eth[12], eth[13], eth[14], eth[15])
		l4 := eth[ihl:]
		sp := binary.BigEndian.Uint16(l4[0:2])
		dp := binary.BigEndian.Uint16(l4[2:4])
		doff := int(l4[12]>>4) * 4
		if len(l4) < doff {
			continue
		}
		pl := l4[doff:]
		if len(pl) == 0 {
			continue
		}
		if sp != 6611 && dp != 6611 {
			continue
		}
		k := fmt.Sprintf("%s:%d->:%d", src, sp, dp)
		flows[k] = append(flows[k], seg{binary.BigEndian.Uint32(l4[4:8]), pl})
	}

	var outFile string
	if len(os.Args) >= 3 {
		outFile = os.Args[2] // 3. arg: 1412 body'leri bu .h264 dosyasina yaz
	}
	var es []byte
	for k, segs := range flows {
		sort.Slice(segs, func(i, j int) bool { return segs[i].seq < segs[j].seq })
		var buf []byte
		for _, s := range segs {
			buf = append(buf, s.pl...)
		}
		fmt.Printf("\n===== akis %s (%d byte) =====\n", k, len(buf))
		es = append(es, extract1412(buf)...)
		dump1412(buf)
	}
	if outFile != "" && len(es) > 0 {
		if err := os.WriteFile(outFile, es, 0644); err != nil {
			fmt.Println("yazma hata:", err)
		} else {
			fmt.Printf("\n>>> %d byte H.264 ES yazildi: %s\n", len(es), outFile)
		}
	}
}

// extract1412 — tum 1412 body'lerini sirayla birlestirir (ham H.264 ES).
func extract1412(buf []byte) []byte {
	var out []byte
	for i := 0; i+20 <= len(buf); {
		if buf[i] == 0xFF && buf[i+1] == 0x01 {
			msgID := binary.LittleEndian.Uint16(buf[i+14 : i+16])
			dataLen := binary.LittleEndian.Uint32(buf[i+16 : i+20])
			if dataLen > 2000000 {
				i++
				continue
			}
			bodyStart := i + 20
			bodyEnd := bodyStart + int(dataLen)
			if bodyEnd > len(buf) {
				bodyEnd = len(buf)
			}
			if msgID == 1412 {
				out = append(out, buf[bodyStart:bodyEnd]...)
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
	return out
}

func dump1412(buf []byte) {
	count := 0
	lenHist := map[uint32]int{}
	for i := 0; i+20 <= len(buf); {
		if buf[i] == 0xFF && buf[i+1] == 0x01 {
			msgID := binary.LittleEndian.Uint16(buf[i+14 : i+16])
			dataLen := binary.LittleEndian.Uint32(buf[i+16 : i+20])
			if dataLen > 2000000 {
				i++
				continue
			}
			bodyStart := i + 20
			bodyEnd := bodyStart + int(dataLen)
			if bodyEnd > len(buf) {
				bodyEnd = len(buf)
			}
			body := buf[bodyStart:bodyEnd]
			if msgID == 1412 {
				lenHist[dataLen]++
				// ilk 12 farkli 1412 paketini detayli dok
				if count < 12 {
					fmt.Printf("\n[1412 #%d] len=%d entropi=%.3f\n", count, dataLen, entropy(body))
					fmt.Printf("  hex(ilk48): %x\n", head(body, 48))
					if len(body) > 48 {
						fmt.Printf("  hex(son16): %x\n", body[len(body)-16:])
					}
					// H.264/H.265 start code var mi (00 00 00 01 / 00 00 01)
					if findStart(body) {
						fmt.Printf("  >>> H.264/5 START CODE (00 00 01) BULUNDU — body SIFRESIZ video!\n")
					}
				}
				count++
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
	fmt.Printf("\n-- 1412 toplam=%d, uzunluk dagilimi: ", count)
	var ks []uint32
	for l := range lenHist {
		ks = append(ks, l)
	}
	sort.Slice(ks, func(i, j int) bool { return lenHist[ks[i]] > lenHist[ks[j]] })
	for i, l := range ks {
		if i >= 8 {
			break
		}
		fmt.Printf("len%d×%d  ", l, lenHist[l])
	}
	fmt.Println()
}

// Shannon entropi (byte bazli, 0-8 bit). >7.5 = AES/sikistirilmis (rastgele); <6 = yapisal/metin.
func entropy(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	var freq [256]int
	for _, c := range b {
		freq[c]++
	}
	var e float64
	n := float64(len(b))
	for _, f := range freq {
		if f == 0 {
			continue
		}
		p := float64(f) / n
		e -= p * math.Log2(p)
	}
	return e
}

func findStart(b []byte) bool {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			return true
		}
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 0 && b[i+3] == 1 {
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

func ipPayload(linkType uint32, frame []byte) []byte {
	switch linkType {
	case 1:
		if len(frame) < 14 || binary.BigEndian.Uint16(frame[12:14]) != 0x0800 {
			return nil
		}
		return frame[14:]
	case 113:
		if len(frame) < 16 || binary.BigEndian.Uint16(frame[14:16]) != 0x0800 {
			return nil
		}
		return frame[16:]
	case 276:
		if len(frame) < 20 || binary.BigEndian.Uint16(frame[0:2]) != 0x0800 {
			return nil
		}
		return frame[20:]
	case 101:
		return frame
	default:
		return frame
	}
}
