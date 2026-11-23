package cloud

import (
	"encoding/binary"
	"testing"
)

// XMIP veri paketi doğru biçimde kurulmalı (outer + inner header + payload).
func TestBuildXmipData(t *testing.T) {
	payload := []byte("hello-dvrip")
	pkt := buildXmipData(7, payload, true, true)

	if cmdOf(pkt) != cmdDataPacket {
		t.Errorf("outer cmd: %08x", cmdOf(pkt))
	}
	innerLen := binary.LittleEndian.Uint16(pkt[6:8])
	if int(innerLen) != 12+len(payload) {
		t.Errorf("innerLen: %d, beklenen %d", innerLen, 12+len(payload))
	}
	if string(pkt[12:16]) != "XMIP" {
		t.Errorf("magic: %q", pkt[12:16])
	}
	if seq := binary.LittleEndian.Uint32(pkt[16:20]); seq != 7 {
		t.Errorf("seq: %d", seq)
	}
	flags := pkt[21]
	if flags&xmipFlagData == 0 || flags&xmipFlagFirst == 0 || flags&xmipFlagLast == 0 {
		t.Errorf("flags: %02x (DATA+first+last bekleniyor)", flags)
	}
	if string(pkt[24:]) != string(payload) {
		t.Errorf("payload: %q", pkt[24:])
	}
}

// XMIP ACK paketi doğru biçimde kurulmalı (24 byte, ACK biti).
func TestBuildXmipAck(t *testing.T) {
	pkt := buildXmipAck(42, 3)
	if len(pkt) != 24 {
		t.Fatalf("ack boyut: %d", len(pkt))
	}
	if string(pkt[12:16]) != "XMIP" {
		t.Errorf("magic: %q", pkt[12:16])
	}
	if seq := binary.LittleEndian.Uint32(pkt[16:20]); seq != 42 {
		t.Errorf("seq: %d", seq)
	}
	if pkt[20] != 3 {
		t.Errorf("count: %d", pkt[20])
	}
	if pkt[21]&xmipFlagACK == 0 {
		t.Errorf("ACK biti yok: %02x", pkt[21])
	}
	if il := binary.LittleEndian.Uint16(pkt[6:8]); il != 12 {
		t.Errorf("ack innerLen: %d", il)
	}
}

// Write bir payload'ı maxSlicePayload'dan büyükse birden çok dilime böler.
// (gerçek ağ yok; sadece dilimleme mantığı — sendReliable ACK beklediği için
//  burada sadece dilim sayısını hesaplamayı doğruluyoruz, I/O test ileride mock ile.)
func TestSliceCount(t *testing.T) {
	// 3000 byte → maxSlicePayload(1388) ile 3 dilim (1388+1388+224)
	total := 3000
	slices := (total + maxSlicePayload - 1) / maxSlicePayload
	if slices != 3 {
		t.Errorf("beklenen 3 dilim, hesap %d", slices)
	}
}
