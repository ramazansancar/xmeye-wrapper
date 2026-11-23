// Package cloud, XMEye/Xiongmai bulut (RPS/NAT) bağlantısını kurar: seri no ile cihazı
// bulut üzerinden bulur, NAT delme (P2P) ya da proxy relay ile ulaşılabilir bir akış açar.
// Bu akışın üstünde dvrip paketi çalışır (login, OPMonitor, video).
//
// Protokol kaynağı: gerçek cihaz trafiği (testdata/sig.pcap) + açık kaynak referans
// irlanfazryana23/cctv-backup (MIT) — XmCloudNatClient.cs. Paket formatları oradan birebir.
//
// packets.go: NAT/P2P sinyal paketlerinin ikili biçimi (build/parse). Hepsi little-endian.
// Komut (cmd) her paketin ilk 4 baytıdır. ASCII alanlar sabit boyutlu, null ile doldurulur.
package cloud

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// NAT protokol komut kodları (little-endian uint32, paket başı).
const (
	cmdMasterServerReq = 0x05F42014
	cmdMasterServerRsp = 0x05F52014
	cmdServerListReq   = 0x05F02012
	cmdServerListRsp   = 0x05F12012
	cmdQueryUUIDSvrReq = 0xB0002015
	cmdQueryUUIDSvrRsp = 0xB0012015
	cmdWanAddrReq      = 0x03E82012
	cmdWanAddrRsp      = 0x03E92012
	cmdPeerConnectReq  = 0x03EC2012
	cmdPeerConnectAck  = 0x03ED2012
	cmdHelloReq        = 0x07D02012
	cmdHelloRsp        = 0x07D12012
	cmdHeartbeat       = 0x07D22012
	cmdDataPacket      = 0x07D32012
	cmdCloseCon        = 0x07D42012
	cmdCloseConEx      = 0x07D62014
	cmdProxyReq        = 0x0BB92012
	cmdProxyRsp        = 0x0BBA2012
	cmdProxyFullRsp    = 0x0BBA2013
)

// Bilinen portlar.
const (
	portMasterDiscovery = 7999
	portDefaultNatSvr   = 8765
	portUUIDQuery       = 8777
)

// Varsayılan master discovery host'ları (biri cevap verene kadar denenir).
var defaultMasterHosts = []string{
	"secu100.net",
	"112.124.0.188",
	"54.84.132.236",
}

// DeviceNatServerInfo, bir seri no'nun kayıtlı olduğu NAT sunucusunu tanımlar (UUID query yanıtı).
type DeviceNatServerInfo struct {
	Status     int32
	UUID       string
	ServerIP   string
	ServerPort int
	NatType    uint32
	Family     uint32
}

// PeerReverseInfo, PeerConnect sonrası cihazın ulaşım bilgisi (WAN/LAN adresleri + proxy).
type PeerReverseInfo struct {
	DstSessionID uint32
	DstWanIP     string
	DstWanPort   uint16
	DstLanIPs    []string
	DstLanPort   uint16
	ProxyIP      string
	ProxyPort    int
}

// --- yardımcılar ---

// writeFixedASCII, s'yi dst'ye yazar; kalanı null (0) bırakır. s dst'den uzunsa kırpar.
func writeFixedASCII(dst []byte, s string) {
	for i := range dst {
		dst[i] = 0
	}
	n := copy(dst, []byte(s))
	_ = n
}

// readNullTerminatedASCII, ilk null'a kadar olan ASCII'yi döndürür.
func readNullTerminatedASCII(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// --- Master discovery (secu100.net:7999) ---

// BuildMasterServerDiscovery, master sunucu keşif paketini üretir (104 byte).
// macHex: istemci MAC (herhangi; sunucu yalnız formatı bekler).
func BuildMasterServerDiscovery(macHex string) []byte {
	pkt := make([]byte, 104)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdMasterServerReq)
	writeFixedASCII(pkt[4:104], macHex)
	return pkt
}

// ParseMasterServerDiscovery, keşif yanıtından aktif master IP'yi çıkarır.
func ParseMasterServerDiscovery(buf []byte) (serverIP string, ok bool) {
	if len(buf) < 24 {
		return "", false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != cmdMasterServerRsp {
		return "", false
	}
	serverIP = readNullTerminatedASCII(buf[4:24])
	return serverIP, serverIP != ""
}

// --- UUID (seri no) → NAT sunucu sorgusu ---

// BuildQueryDeviceNatServer, seri no için NAT sunucu sorgu paketini üretir.
func BuildQueryDeviceNatServer(serial string) []byte {
	sb := []byte(serial)
	pkt := make([]byte, 4+len(sb))
	binary.LittleEndian.PutUint32(pkt[0:4], cmdQueryUUIDSvrReq)
	copy(pkt[4:], sb)
	return pkt
}

// ParseQueryDeviceNatServer, sorgu yanıtını çözer (140+ byte).
func ParseQueryDeviceNatServer(buf []byte) (*DeviceNatServerInfo, bool) {
	if len(buf) < 140 {
		return nil, false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != cmdQueryUUIDSvrRsp {
		return nil, false
	}
	info := &DeviceNatServerInfo{
		Status:     int32(binary.LittleEndian.Uint32(buf[4:8])),
		UUID:       readNullTerminatedASCII(buf[8:108]),
		ServerIP:   readNullTerminatedASCII(buf[108:128]),
		ServerPort: int(binary.LittleEndian.Uint32(buf[128:132])),
		NatType:    binary.LittleEndian.Uint32(buf[132:136]),
		Family:     binary.LittleEndian.Uint32(buf[136:140]),
	}
	return info, true
}

// --- WAN adres sorgusu (kendi dış IP:port) ---

// BuildWanAddrReq, WAN adres sorgu paketini üretir (4 byte).
func BuildWanAddrReq() []byte {
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint32(pkt, cmdWanAddrReq)
	return pkt
}

// ParseWanAddr, WAN adres yanıtından dış IP:port'u çıkarır.
func ParseWanAddr(buf []byte) (wanIP string, wanPort uint16, ok bool) {
	if len(buf) < 24 {
		return "", 0, false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != cmdWanAddrRsp {
		return "", 0, false
	}
	wanIP = readNullTerminatedASCII(buf[4:20])
	wanPort = binary.LittleEndian.Uint16(buf[20:22])
	return wanIP, wanPort, wanIP != ""
}

// --- PeerConnect (NAT delme talebi) ---

// BuildPeerConnectReq, NAT delme/bağlantı talebini üretir (360 byte).
func BuildPeerConnectReq(mySession uint32, myUUID, destSerial, myWanIP string, myWanPort uint16, myLanIPs string, myLanPort uint16, skipP2P bool) []byte {
	pkt := make([]byte, 360)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdPeerConnectReq)
	binary.LittleEndian.PutUint32(pkt[4:8], mySession)
	writeFixedASCII(pkt[8:108], myUUID)
	writeFixedASCII(pkt[108:208], destSerial)
	writeFixedASCII(pkt[208:224], myWanIP)
	binary.LittleEndian.PutUint16(pkt[224:226], myWanPort)
	writeFixedASCII(pkt[226:326], myLanIPs)
	binary.LittleEndian.PutUint16(pkt[326:328], myLanPort)
	binary.LittleEndian.PutUint32(pkt[328:332], 0) // dstSession istek anında 0
	binary.LittleEndian.PutUint32(pkt[352:356], uint32(time.Now().UnixMilli()/1000))
	var sp uint32
	if skipP2P {
		sp = 1
	}
	binary.LittleEndian.PutUint32(pkt[356:360], sp)
	return pkt
}

// ParsePeerReverse, cihazdan dönen PeerConnect (ters bilgi) paketini çözer (360+ byte).
// expectMySession != 0 ise echo edilen session doğrulanır.
func ParsePeerReverse(buf []byte, expectMySession uint32) (*PeerReverseInfo, bool) {
	if len(buf) < 360 {
		return nil, false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != cmdPeerConnectReq {
		return nil, false
	}
	echoed := binary.LittleEndian.Uint32(buf[328:332])
	if expectMySession != 0 && echoed != expectMySession {
		return nil, false
	}
	lanRaw := readNullTerminatedASCII(buf[226:326])
	var lanIPs []string
	for _, p := range strings.FieldsFunc(lanRaw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	}) {
		if p = strings.TrimSpace(p); p != "" {
			lanIPs = append(lanIPs, p)
		}
	}
	info := &PeerReverseInfo{
		DstSessionID: binary.LittleEndian.Uint32(buf[4:8]),
		DstWanIP:     readNullTerminatedASCII(buf[208:224]),
		DstWanPort:   binary.LittleEndian.Uint16(buf[224:226]),
		DstLanIPs:    lanIPs,
		DstLanPort:   binary.LittleEndian.Uint16(buf[326:328]),
		ProxyIP:      readNullTerminatedASCII(buf[332:348]),
		ProxyPort:    int(binary.LittleEndian.Uint32(buf[348:352])),
	}
	return info, true
}

// ParsePeerConnectAck, PeerConnect onay paketinin durum kodunu döndürür.
func ParsePeerConnectAck(buf []byte) (status int32, ok bool) {
	if len(buf) < 12 {
		return 0, false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != cmdPeerConnectAck {
		return 0, false
	}
	return int32(binary.LittleEndian.Uint32(buf[8:12])), true
}

// --- Hello (P2P el sıkışma) ---

// BuildHelloReq, P2P hello isteği üretir (112 byte).
func BuildHelloReq(mySession uint32, myUUID string, attempt uint32) []byte {
	pkt := make([]byte, 112)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdHelloReq)
	binary.LittleEndian.PutUint32(pkt[4:8], mySession)
	writeFixedASCII(pkt[8:108], myUUID)
	binary.LittleEndian.PutUint32(pkt[108:112], attempt)
	return pkt
}

// BuildHelloRsp, P2P hello yanıtı üretir (112 byte).
func BuildHelloRsp(mySession uint32, myUUID string, peerAttempt uint16) []byte {
	pkt := make([]byte, 112)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdHelloRsp)
	binary.LittleEndian.PutUint32(pkt[4:8], mySession)
	writeFixedASCII(pkt[8:108], myUUID)
	binary.LittleEndian.PutUint16(pkt[108:110], peerAttempt)
	binary.LittleEndian.PutUint16(pkt[110:112], 2)
	return pkt
}

// --- Proxy (relay fallback) ---

// BuildProxyReq, proxy relay kayıt isteği üretir (212 byte).
func BuildProxyReq(myUUID string, mySession uint32, destSerial string, dstSession uint32) []byte {
	pkt := make([]byte, 212)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdProxyReq)
	writeFixedASCII(pkt[4:104], myUUID)
	binary.LittleEndian.PutUint32(pkt[104:108], mySession)
	writeFixedASCII(pkt[108:208], destSerial)
	binary.LittleEndian.PutUint32(pkt[208:212], dstSession)
	return pkt
}

// BuildHeartbeat, heartbeat paketi üretir (8 byte).
func BuildHeartbeat(seq uint16) []byte {
	pkt := make([]byte, 8)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdHeartbeat)
	binary.LittleEndian.PutUint16(pkt[4:6], seq)
	binary.LittleEndian.PutUint16(pkt[6:8], 2)
	return pkt
}

// cmdOf, bir paketin baştaki komut kodunu döndürür (0 = çok kısa).
func cmdOf(buf []byte) uint32 {
	if len(buf) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(buf[0:4])
}

// GenerateClientUUID, istemci için XMEye NAT UUID üretir (referans formatı).
func GenerateClientUUID() string {
	tick := (time.Now().UnixMilli() / 1) % 10000
	r1 := rand.Intn(10000)
	r2 := rand.Intn(10000)
	return fmt.Sprintf("!xmnatuuid-IEClient-%04d%04d%04d", tick, r1, r2)
}
