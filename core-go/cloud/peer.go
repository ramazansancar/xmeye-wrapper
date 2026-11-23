package cloud

// peer.go: CPeerConnect NAT delme state machine (P2P + proxy fallback).
//
//	State 0: WAN adres sorgusu (devNatServer'dan kendi dış IP:port)
//	State 1-2: PeerConnectReq gönder → PeerConnectAck (status) + PeerReverse (dest adresleri)
//	State 3: P2P punch — dest LAN + WAN hedeflerine HelloReq/Rsp
//	State 4-5: P2P tutmazsa proxy relay (ProxyReq → ProxyRsp → proxy üstünden Hello)
//
// Sonuç: bağlı peer'a UDP soketi (*Session). Kaynak: cctv-backup ConnectPeerAsync (MIT).

import (
	"context"
	"fmt"
	"net"
	"time"
)

// ConnectPeer, cihazın NAT sunucusuna bağlanıp P2P/proxy ile ulaşılabilir peer açar.
func ConnectPeer(ctx context.Context, serial string, devNatServer *net.UDPAddr, opts *Options) (*Session, error) {
	conn, err := net.ListenUDP("udp4", nil) // tüm peer'lardan alabilmek için bağlı değil
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			conn.Close()
		}
	}()

	timeout := opts.stepTimeout()
	myLanPort := uint16(conn.LocalAddr().(*net.UDPAddr).Port)
	myLanIPs := localIPv4List()
	myUUID := GenerateClientUUID()
	mySession := nextSession()

	rd := &reader{conn: conn}

	// State 0: WAN adresimizi öğren (3 deneme)
	myWanIP, myWanPort := "0.0.0.0", myLanPort
	wanReq := BuildWanAddrReq()
	for i := 0; i < 3; i++ {
		conn.WriteToUDP(wanReq, devNatServer)
		buf, _, err := rd.read(ctx, timeout)
		if err == nil {
			if ip, port, ok2 := ParseWanAddr(buf); ok2 {
				myWanIP, myWanPort = ip, port
				break
			}
		}
	}

	// State 1-2: PeerConnect → reverse info
	connReq := BuildPeerConnectReq(mySession, myUUID, serial, myWanIP, myWanPort, myLanIPs, myLanPort, opts != nil && opts.PreferProxy)
	var reverse *PeerReverseInfo
	var connectedPeer *net.UDPAddr
	deadline := time.Now().Add(maxDur(3*time.Second, timeout*2))
	conn.WriteToUDP(connReq, devNatServer)

	for reverse == nil && connectedPeer == nil {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("cloud: peer ters bilgi zaman aşımı (seri %s)", serial)
		}
		buf, from, err := rd.read(ctx, 600*time.Millisecond)
		if err != nil {
			conn.WriteToUDP(connReq, devNatServer) // yeniden gönder
			continue
		}
		switch cmdOf(buf) {
		case cmdPeerConnectAck:
			if status, ok2 := ParsePeerConnectAck(buf); ok2 && status != 1 {
				return nil, fmt.Errorf("cloud: peer connect reddedildi (status=%d)", status)
			}
		case cmdPeerConnectReq:
			if info, ok2 := ParsePeerReverse(buf, mySession); ok2 {
				reverse = info
			}
		case cmdHelloReq:
			// peer bize önce delmiş
			if len(buf) >= 112 {
				peerAttempt := uint16(buf[108]) | uint16(buf[109])<<8
				conn.WriteToUDP(BuildHelloRsp(mySession, myUUID, peerAttempt), from)
				connectedPeer = from
			}
		}
	}

	// State 3: P2P direct punch (PreferProxy ise atla → doğrudan proxy relay)
	preferProxy := opts != nil && opts.PreferProxy
	if connectedPeer == nil && reverse != nil && !preferProxy {
		targets := p2pTargets(reverse)
		if len(targets) > 0 {
			connectedPeer = handshakeTargets(ctx, rd, conn, targets, mySession, myUUID, timeout)
		}
	}

	// State 4-5: proxy fallback
	viaProxy := false
	if connectedPeer == nil && reverse != nil && reverse.ProxyPort > 0 {
		if proxyAddr := net.ParseIP(reverse.ProxyIP); proxyAddr != nil {
			proxyEp := &net.UDPAddr{IP: proxyAddr, Port: reverse.ProxyPort}
			proxyReq := BuildProxyReq(myUUID, mySession, serial, reverse.DstSessionID)
			registered := false
			for attempt := 0; attempt < 3 && !registered; attempt++ {
				conn.WriteToUDP(proxyReq, proxyEp)
				buf, from, err := rd.read(ctx, maxDur(500*time.Millisecond, timeout/2))
				if err != nil {
					continue
				}
				switch cmdOf(buf) {
				case cmdProxyRsp:
					registered = true
				case cmdHelloReq:
					if len(buf) >= 112 {
						peerAttempt := uint16(buf[108]) | uint16(buf[109])<<8
						conn.WriteToUDP(BuildHelloRsp(mySession, myUUID, peerAttempt), from)
						connectedPeer = from
						registered = true
					}
				case cmdHelloRsp, cmdHeartbeat:
					connectedPeer = from
					registered = true
				}
			}
			if connectedPeer == nil && registered {
				connectedPeer = handshakeTargets(ctx, rd, conn, []*net.UDPAddr{proxyEp}, mySession, myUUID, timeout)
			}
			if connectedPeer != nil && connectedPeer.IP.Equal(proxyEp.IP) && connectedPeer.Port == proxyEp.Port {
				viaProxy = true
			}
		}
	}

	if connectedPeer == nil {
		return nil, fmt.Errorf("cloud: P2P/proxy bağlantısı kurulamadı (seri %s)", serial)
	}

	ok = true
	return &Session{
		conn:      conn,
		peer:      connectedPeer,
		mySession: mySession,
		myUUID:    myUUID,
		viaProxy:  viaProxy,
	}, nil
}

// p2pTargets, reverse bilgisinden denenecek LAN+WAN hedeflerini çıkarır.
func p2pTargets(r *PeerReverseInfo) []*net.UDPAddr {
	var t []*net.UDPAddr
	if r.DstLanPort > 0 {
		for _, lan := range r.DstLanIPs {
			if ip := net.ParseIP(lan); ip != nil {
				t = append(t, &net.UDPAddr{IP: ip, Port: int(r.DstLanPort)})
			}
		}
	}
	if r.DstWanPort > 0 {
		if ip := net.ParseIP(r.DstWanIP); ip != nil {
			wan := &net.UDPAddr{IP: ip, Port: int(r.DstWanPort)}
			dup := false
			for _, x := range t {
				if x.IP.Equal(wan.IP) && x.Port == wan.Port {
					dup = true
					break
				}
			}
			if !dup {
				t = append(t, wan)
			}
		}
	}
	return t
}

// handshakeTargets, hedeflere HelloReq gönderip ilk yanıt vereni bağlı peer olarak döndürür.
func handshakeTargets(ctx context.Context, rd *reader, conn *net.UDPConn, targets []*net.UDPAddr, mySession uint32, myUUID string, timeout time.Duration) *net.UDPAddr {
	for attempt := uint32(0); attempt < 4; attempt++ {
		hello := BuildHelloReq(mySession, myUUID, attempt)
		for _, t := range targets {
			conn.WriteToUDP(hello, t)
		}
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			buf, from, err := rd.read(ctx, 300*time.Millisecond)
			if err != nil {
				continue
			}
			switch cmdOf(buf) {
			case cmdHelloRsp, cmdHeartbeat:
				return from
			case cmdHelloReq:
				if len(buf) >= 112 {
					peerAttempt := uint16(buf[108]) | uint16(buf[109])<<8
					conn.WriteToUDP(BuildHelloRsp(mySession, myUUID, peerAttempt), from)
					return from
				}
			}
		}
	}
	return nil
}

// reader, UDP okumayı context + timeout ile sarar.
type reader struct {
	conn *net.UDPConn
	buf  [4096]byte
}

func (r *reader) read(ctx context.Context, timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}
	r.conn.SetReadDeadline(time.Now().Add(timeout))
	n, from, err := r.conn.ReadFromUDP(r.buf[:])
	if err != nil {
		return nil, nil, err
	}
	out := make([]byte, n)
	copy(out, r.buf[:n])
	return out, from, nil
}

func maxDur(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

var sessionCounter uint32 = 0x1000

func nextSession() uint32 {
	sessionCounter++
	return sessionCounter
}
