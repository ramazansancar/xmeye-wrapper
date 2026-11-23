// Package dvrip, Xiongmai/XMEye DVRIP (Sofia) protokolünün paket katmanıdır.
//
// Paket biçimi (20 byte başlık + gövde), little-endian:
//
//	offset  boyut  alan
//	0       1      head     = 0xFF (sabit)
//	1       1      version  = 0x01
//	2       1      reserved1
//	3       1      reserved2
//	4       4      sessionID    (login'de atanır)
//	8       4      sequence     (artan)
//	12      1      totalPacket  (çok parçalı mesajlarda toplam)
//	13      1      curPacket    (çok parçalı mesajlarda sıra)
//	14      2      msgID        (mesaj türü; EDEV_JSON_ID ile eşleşir)
//	16      4      dataLen      (gövde uzunluğu)
//	20      ...    body         (JSON ya da ikili; dataLen kadar)
//
// Doğrulama kaynağı: core-go/testdata/*.pcap (gerçek cihaz trafiği, msgId 1412 vb.).
package dvrip

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// HeaderLen, DVRIP sabit başlık uzunluğu.
const HeaderLen = 20

// Magic baytları.
const (
	magicHead    = 0xFF
	magicVersion = 0x01
)

// Packet, tek bir DVRIP mesajıdır.
type Packet struct {
	SessionID   uint32
	Sequence    uint32
	TotalPacket uint8
	CurPacket   uint8
	MsgID       uint16
	Body        []byte
}

var (
	// ErrShort, tampon tam bir başlık/gövde için kısa.
	ErrShort = errors.New("dvrip: tampon çok kısa")
	// ErrMagic, başlık 0xFF 0x01 ile başlamıyor.
	ErrMagic = errors.New("dvrip: geçersiz magic (FF 01 değil)")
)

// Encode, paketi tele yazılacak bayt dizisine çevirir.
func (p *Packet) Encode() []byte {
	buf := make([]byte, HeaderLen+len(p.Body))
	buf[0] = magicHead
	buf[1] = magicVersion
	// 2,3 reserved = 0
	binary.LittleEndian.PutUint32(buf[4:8], p.SessionID)
	binary.LittleEndian.PutUint32(buf[8:12], p.Sequence)
	buf[12] = p.TotalPacket
	buf[13] = p.CurPacket
	binary.LittleEndian.PutUint16(buf[14:16], p.MsgID)
	binary.LittleEndian.PutUint32(buf[16:20], uint32(len(p.Body)))
	copy(buf[HeaderLen:], p.Body)
	return buf
}

// Decode, tamponun başındaki tek paketi çözer; tükettiği bayt sayısını döndürür.
// Gövde tamponda henüz tam değilse ErrShort döner (daha fazla veri beklenir).
func Decode(buf []byte) (p Packet, consumed int, err error) {
	if len(buf) < HeaderLen {
		return Packet{}, 0, ErrShort
	}
	if buf[0] != magicHead || buf[1] != magicVersion {
		return Packet{}, 0, ErrMagic
	}
	dataLen := binary.LittleEndian.Uint32(buf[16:20])
	// makul sınır (yanlış hizalama / bozuk veri koruması)
	if dataLen > 8<<20 { // 8 MiB
		return Packet{}, 0, fmt.Errorf("dvrip: gövde uzunluğu saçma: %d", dataLen)
	}
	total := HeaderLen + int(dataLen)
	if len(buf) < total {
		return Packet{}, 0, ErrShort
	}
	p = Packet{
		SessionID:   binary.LittleEndian.Uint32(buf[4:8]),
		Sequence:    binary.LittleEndian.Uint32(buf[8:12]),
		TotalPacket: buf[12],
		CurPacket:   buf[13],
		MsgID:       binary.LittleEndian.Uint16(buf[14:16]),
	}
	if dataLen > 0 {
		p.Body = make([]byte, dataLen)
		copy(p.Body, buf[HeaderLen:total])
	}
	return p, total, nil
}

// String, hata ayıklama için kısa özet.
func (p Packet) String() string {
	return fmt.Sprintf("DVRIP{sess=%08x seq=%d msg=%d total=%d cur=%d len=%d}",
		p.SessionID, p.Sequence, p.MsgID, p.TotalPacket, p.CurPacket, len(p.Body))
}
