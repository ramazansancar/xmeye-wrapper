package cloud

import (
	"encoding/binary"
	"testing"
)

// Paket boyutları referanstaki (cctv-backup) sabit uzunluklarla eşleşmeli.
func TestPacketSizes(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"MasterDiscovery", len(BuildMasterServerDiscovery("1234567890ab")), 104},
		{"WanAddrReq", len(BuildWanAddrReq()), 4},
		{"PeerConnectReq", len(BuildPeerConnectReq(1, "u", "s", "1.2.3.4", 5, "10.0.0.1", 6, false)), 360},
		{"HelloReq", len(BuildHelloReq(1, "u", 0)), 112},
		{"HelloRsp", len(BuildHelloRsp(1, "u", 0)), 112},
		{"ProxyReq", len(BuildProxyReq("u", 1, "s", 2)), 212},
		{"Heartbeat", len(BuildHeartbeat(0)), 8},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s boyut: %d, beklenen %d", c.name, c.got, c.want)
		}
	}
}

// İlk 4 bayt (komut kodu) doğru olmalı.
func TestPacketCommands(t *testing.T) {
	cases := []struct {
		name string
		pkt  []byte
		want uint32
	}{
		{"MasterDiscovery", BuildMasterServerDiscovery("x"), cmdMasterServerReq},
		{"QueryNatSvr", BuildQueryDeviceNatServer("22e40425"), cmdQueryUUIDSvrReq},
		{"WanAddrReq", BuildWanAddrReq(), cmdWanAddrReq},
		{"PeerConnectReq", BuildPeerConnectReq(1, "u", "s", "1.2.3.4", 5, "10.0.0.1", 6, false), cmdPeerConnectReq},
		{"HelloReq", BuildHelloReq(1, "u", 0), cmdHelloReq},
		{"ProxyReq", BuildProxyReq("u", 1, "s", 2), cmdProxyReq},
		{"Heartbeat", BuildHeartbeat(0), cmdHeartbeat},
	}
	for _, c := range cases {
		if got := cmdOf(c.pkt); got != c.want {
			t.Errorf("%s cmd: %08x, beklenen %08x", c.name, got, c.want)
		}
	}
}

// PeerConnectReq alanları doğru offset'lere yazılmalı (round-trip: yaz → ParsePeerReverse ile oku).
// ParsePeerReverse req formatını da çözer (echo session @328 doğrular).
func TestPeerConnectRoundTrip(t *testing.T) {
	mySession := uint32(0x1234)
	pkt := BuildPeerConnectReq(mySession, "myuuid", "destserial", "9.8.7.6", 1111, "192.168.1.5", 2222, false)

	// ters bilgi parse için: req'de dstSession@328 = mySession olarak set edilmeli ki echo doğrulansın.
	// Build istek anında @328'i 0 yazar (dstSession). ParsePeerReverse echo'yu @328'den okur.
	// Bu yüzden round-trip testinde expect=0 geçip alanları kontrol ederiz.
	info, ok := ParsePeerReverse(pkt, 0)
	if !ok {
		t.Fatal("ParsePeerReverse başarısız")
	}
	// istek paketinde WAN alanı @208 bizim myWanIP; ters pakette dstWanIP olarak okunur
	if info.DstWanIP != "9.8.7.6" {
		t.Errorf("WanIP: %q", info.DstWanIP)
	}
	if info.DstWanPort != 1111 {
		t.Errorf("WanPort: %d", info.DstWanPort)
	}
	if info.DstLanPort != 2222 {
		t.Errorf("LanPort: %d", info.DstLanPort)
	}
	if len(info.DstLanIPs) != 1 || info.DstLanIPs[0] != "192.168.1.5" {
		t.Errorf("LanIPs: %v", info.DstLanIPs)
	}
}

// Query NAT yanıtı parse: elle bir yanıt kur (status=1, serverIp, port), çöz.
func TestParseQueryDeviceNatServer(t *testing.T) {
	buf := make([]byte, 140)
	binary.LittleEndian.PutUint32(buf[0:4], cmdQueryUUIDSvrRsp)
	binary.LittleEndian.PutUint32(buf[4:8], 1) // status
	copy(buf[8:108], "22e40425f2fc5906shc6")
	copy(buf[108:128], "101.44.42.121")
	binary.LittleEndian.PutUint32(buf[128:132], 8765)
	binary.LittleEndian.PutUint32(buf[132:136], 3) // natType

	info, ok := ParseQueryDeviceNatServer(buf)
	if !ok {
		t.Fatal("parse başarısız")
	}
	if info.Status != 1 {
		t.Errorf("status: %d", info.Status)
	}
	if info.ServerIP != "101.44.42.121" {
		t.Errorf("serverIP: %q", info.ServerIP)
	}
	if info.ServerPort != 8765 {
		t.Errorf("serverPort: %d", info.ServerPort)
	}
	if info.UUID != "22e40425f2fc5906shc6" {
		t.Errorf("uuid: %q", info.UUID)
	}
}

// WAN adres yanıtı round-trip.
func TestParseWanAddr(t *testing.T) {
	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf[0:4], cmdWanAddrRsp)
	copy(buf[4:20], "85.34.12.7")
	binary.LittleEndian.PutUint16(buf[20:22], 40001)

	ip, port, ok := ParseWanAddr(buf)
	if !ok {
		t.Fatal("parse başarısız")
	}
	if ip != "85.34.12.7" || port != 40001 {
		t.Errorf("wan: %s:%d", ip, port)
	}
}

func TestGenerateClientUUID(t *testing.T) {
	u := GenerateClientUUID()
	if len(u) < 20 || u[:20] != "!xmnatuuid-IEClient-" {
		t.Errorf("uuid format: %q", u)
	}
}
