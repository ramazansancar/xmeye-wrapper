package dvrip

import (
	"bytes"
	"testing"
)

// Encode → Decode tur-gidiş eşitliği.
func TestRoundTrip(t *testing.T) {
	orig := Packet{
		SessionID:   0x0000084e, // pcap'te görülen gerçek session
		Sequence:    2043,
		TotalPacket: 1,
		CurPacket:   0,
		MsgID:       1412, // video frame (login2.pcap)
		Body:        []byte(`{"Name":"OPMonitor","SessionID":"0x084e"}`),
	}
	wire := orig.Encode()
	if len(wire) != HeaderLen+len(orig.Body) {
		t.Fatalf("encode uzunluğu yanlış: %d", len(wire))
	}
	got, consumed, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode hata: %v", err)
	}
	if consumed != len(wire) {
		t.Fatalf("consumed=%d, beklenen=%d", consumed, len(wire))
	}
	if got.SessionID != orig.SessionID || got.Sequence != orig.Sequence ||
		got.MsgID != orig.MsgID || !bytes.Equal(got.Body, orig.Body) {
		t.Fatalf("round-trip uyuşmadı:\n orig=%+v\n got =%+v", orig, got)
	}
}

// Gerçek tel baytları (login2.pcap, msgId=1412 başlığı) ile decode.
func TestDecodeRealHeader(t *testing.T) {
	// FF 01 00 00 | 4e 08 00 00 (sess=0x084e) | fb 07 00 00 (seq=2043) | 01 00 | 84 05 (msg=1412) | a8 00 00 00 (len=168)
	hdr := []byte{
		0xFF, 0x01, 0x00, 0x00,
		0x4e, 0x08, 0x00, 0x00,
		0xfb, 0x07, 0x00, 0x00,
		0x01, 0x00,
		0x84, 0x05,
		0xa8, 0x00, 0x00, 0x00,
	}
	body := make([]byte, 168)
	wire := append(hdr, body...)

	p, consumed, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode hata: %v", err)
	}
	if p.SessionID != 0x084e {
		t.Errorf("sessionID=%#x, beklenen 0x084e", p.SessionID)
	}
	if p.Sequence != 2043 {
		t.Errorf("sequence=%d, beklenen 2043", p.Sequence)
	}
	if p.MsgID != 1412 {
		t.Errorf("msgID=%d, beklenen 1412", p.MsgID)
	}
	if len(p.Body) != 168 {
		t.Errorf("body=%d, beklenen 168", len(p.Body))
	}
	if consumed != 188 {
		t.Errorf("consumed=%d, beklenen 188", consumed)
	}
}

// Kısa tampon → ErrShort (akış reassembly için kritik).
func TestShortBuffer(t *testing.T) {
	if _, _, err := Decode([]byte{0xFF, 0x01}); err != ErrShort {
		t.Fatalf("başlık kısa: ErrShort bekleniyordu, %v geldi", err)
	}
	// başlık tam, gövde eksik
	hdr := (&Packet{MsgID: 1, Body: make([]byte, 100)}).Encode()[:HeaderLen+10]
	if _, _, err := Decode(hdr); err != ErrShort {
		t.Fatalf("gövde kısa: ErrShort bekleniyordu, %v geldi", err)
	}
}

// Yanlış magic → ErrMagic.
func TestBadMagic(t *testing.T) {
	buf := make([]byte, HeaderLen)
	buf[0] = 0xAA
	if _, _, err := Decode(buf); err != ErrMagic {
		t.Fatalf("ErrMagic bekleniyordu, %v geldi", err)
	}
}
