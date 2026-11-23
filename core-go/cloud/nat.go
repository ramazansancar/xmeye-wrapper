package cloud

// nat.go: bulut NAT bağlantı akışı.
//
//	Connect(serial):
//	  1. DiscoverMaster       → aktif master IP (secu100.net:7999)
//	  2. QueryDeviceNatServer → seri no'nun NAT sunucusu (master:8777)
//	  3. ConnectPeer          → WAN sorgu → PeerConnect → P2P punch → Proxy fallback
//	     → *Session (bağlı peer'a UDP; üstünde XMIP reliable stream kurulur)
//
// Kaynak: cctv-backup/XmCloudNatClient.cs (MIT). Akış ve zaman aşımı mantığı oradan.

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Options, bağlantı ayarları (test/override için).
type Options struct {
	MasterHosts  []string      // boşsa defaultMasterHosts
	StepTimeout  time.Duration // adım zaman aşımı (varsayılan 4s)
	MasterPort   int           // varsayılan 7999
	UUIDPort     int           // varsayılan 8777
	PreferProxy  bool          // true: P2P'yi atla, doğrudan proxy (RPS cihazları için hızlı)
}

func (o *Options) stepTimeout() time.Duration {
	if o != nil && o.StepTimeout > 0 {
		return o.StepTimeout
	}
	return 4 * time.Second
}

func (o *Options) masterHosts() []string {
	if o != nil && len(o.MasterHosts) > 0 {
		return o.MasterHosts
	}
	return defaultMasterHosts
}

func (o *Options) masterPort() int {
	if o != nil && o.MasterPort > 0 {
		return o.MasterPort
	}
	return portMasterDiscovery
}

func (o *Options) uuidPort() int {
	if o != nil && o.UUIDPort > 0 {
		return o.UUIDPort
	}
	return portUUIDQuery
}

// Session, kurulmuş bulut bağlantısıdır: bağlı peer'a UDP soketi + oturum kimliği.
// XMIP reliable stream (xmip.go) bunun üstünde çalışır.
type Session struct {
	conn       *net.UDPConn
	peer       *net.UDPAddr
	mySession  uint32
	myUUID     string
	viaProxy   bool
}

// Peer döndürür (teşhis).
func (s *Session) Peer() *net.UDPAddr { return s.peer }

// ViaProxy, bağlantı proxy relay üzerinden mi (true) yoksa P2P direkt mi (false).
func (s *Session) ViaProxy() bool { return s.viaProxy }

// Close, UDP soketini kapatır.
func (s *Session) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Connect, seri no ile bulut NAT bağlantısını kurar.
func Connect(ctx context.Context, serial string, opts *Options) (*Session, error) {
	serial = trimSpace(serial)
	if serial == "" {
		return nil, fmt.Errorf("cloud: seri no gerekli")
	}

	// 1. master discovery
	masterIP, err := DiscoverMaster(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("cloud: master bulunamadı: %w", err)
	}

	// 2. seri no → NAT sunucusu
	info, err := QueryDeviceNatServer(ctx, masterIP, opts.uuidPort(), serial, opts)
	if err != nil {
		return nil, fmt.Errorf("cloud: NAT sunucu sorgusu: %w", err)
	}
	if info.Status != 1 || info.ServerIP == "" {
		return nil, fmt.Errorf("cloud: cihaz çevrimdışı ya da bulunamadı (status=%d)", info.Status)
	}

	// 3. peer bağlantı
	port := info.ServerPort
	if port <= 0 {
		port = portDefaultNatSvr
	}
	devSvr := &net.UDPAddr{IP: net.ParseIP(info.ServerIP), Port: port}
	return ConnectPeer(ctx, serial, devSvr, opts)
}

// DiscoverMaster, aktif master sunucu IP'sini bulur.
func DiscoverMaster(ctx context.Context, opts *Options) (string, error) {
	timeout := opts.stepTimeout()
	var candidates []net.IP
	for _, host := range opts.masterHosts() {
		if ip := net.ParseIP(host); ip != nil {
			candidates = append(candidates, ip)
			continue
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
		if err == nil {
			candidates = append(candidates, ips...)
		}
	}
	for _, ip := range candidates {
		ep := &net.UDPAddr{IP: ip, Port: opts.masterPort()}
		got, err := queryMaster(ctx, ep, timeout)
		if err == nil && net.ParseIP(got) != nil {
			return got, nil
		}
	}
	if len(candidates) > 0 {
		return candidates[0].String(), nil // keşif cevapsız → ilk adayı kullan
	}
	return "", fmt.Errorf("master host çözülemedi")
}

func queryMaster(ctx context.Context, ep *net.UDPAddr, timeout time.Duration) (string, error) {
	conn, err := net.DialUDP("udp4", nil, ep)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	req := BuildMasterServerDiscovery("1234567890ab")
	if _, err := conn.Write(req); err != nil {
		return "", err
	}
	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	if ip, ok := ParseMasterServerDiscovery(buf[:n]); ok {
		return ip, nil
	}
	return "", fmt.Errorf("master yanıtı çözülemedi")
}

// QueryDeviceNatServer, seri no'nun kayıtlı olduğu NAT sunucusunu sorgular (2 deneme).
func QueryDeviceNatServer(ctx context.Context, masterIP string, port int, serial string, opts *Options) (*DeviceNatServerInfo, error) {
	timeout := opts.stepTimeout()
	ep := &net.UDPAddr{IP: net.ParseIP(masterIP), Port: port}
	conn, err := net.DialUDP("udp4", nil, ep)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	req := BuildQueryDeviceNatServer(serial)
	buf := make([]byte, 2048)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := conn.Write(req); err != nil {
			return nil, err
		}
		conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}
		if info, ok := ParseQueryDeviceNatServer(buf[:n]); ok {
			return info, nil
		}
	}
	return nil, fmt.Errorf("NAT sunucu sorgusu zaman aşımı (seri %s)", serial)
}

// trimSpace, baş/son boşlukları atar (strings bağımlılığı paket içinde zaten var).
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// localIPv4List, yerel IPv4 adreslerini virgülle birleştirir (PeerConnect LAN alanı için).
func localIPv4List() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "0.0.0.0"
	}
	var ips []string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if v4 := ipnet.IP.To4(); v4 != nil {
				ips = append(ips, v4.String())
			}
		}
	}
	if len(ips) == 0 {
		return "0.0.0.0"
	}
	out := ips[0]
	for _, ip := range ips[1:] {
		out += "," + ip
	}
	return out
}
