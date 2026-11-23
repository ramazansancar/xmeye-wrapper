package cloud

// xmip.go: XMIP reliable-UDP transport (CPeerConnect veri katmanı) üstünde io.ReadWriteCloser.
// DVRIP bu akışın üstünde çalışır. Her gönderilen dilim (slice) sıra no + bayrak taşır;
// alıcı sıraya dizer (reorder) ve ACK yollar. Gönderici ACK gelene dek yeniden iletir.
//
// Paket biçimi (BuildXmipDataPacket/Ack):
//
//	Outer 12B: cmdDataPacket(4) + 0(2) + innerLen(2) + 0(4)
//	Inner 12B: "XMIP"(4) + seq(4) + count(1) + flags(1) + 0(2)   flags: DATA 0x02, first 0x04, last 0x08, ACK 0x01
//	payload  : @24
//
// Kaynak: cctv-backup XmCloudNatStream + XmipReliableStream (MIT). Alıcı ACK algoritması NetSdk uyumlu (bitmask).

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	maxSlicePayload = 1388 // 1400 MTU - 12 XMIP header
	xmipFlagACK     = 0x01
	xmipFlagData    = 0x02
	xmipFlagFirst   = 0x04
	xmipFlagLast    = 0x08
)

// Stream, bir Session üstünde XMIP reliable byte akışıdır (io.ReadWriteCloser).
type Stream struct {
	conn *net.UDPConn
	peer *net.UDPAddr

	mySession uint32
	myUUID    string

	// gönderme
	sendMu   sync.Mutex
	sendSeq  uint32
	pending  map[uint32]chan struct{} // ACK bekleyen seq'ler
	pendMu   sync.Mutex

	// alma
	recvMu      sync.Mutex
	nextRecvSeq uint32
	reorder     map[uint32][]byte
	ready       [][]byte
	readBuf     []byte // ready'den okunmayı bekleyen kalan
	dataCh      chan struct{}
	closed      bool
	remoteClose bool

	// bitmask ACK (NetSdk algoritması)
	hasBitmap  bool
	maxRecvSeq uint32
	recvBitmask uint16

	closeOnce sync.Once
	done      chan struct{}

	readTimeout time.Duration // 0 = sınırsız

	// teşhis sayaçları (debug)
	statMu     sync.Mutex
	rawPackets int            // recvLoop'un aldığı toplam UDP paketi
	cmdStats   map[uint32]int // cmd kodu -> adet
}

// Stats, teşhis için alınan ham UDP paket sayısı ve cmd dağılımını döndürür.
func (st *Stream) Stats() (rawPackets int, cmds map[uint32]int) {
	st.statMu.Lock()
	defer st.statMu.Unlock()
	cp := make(map[uint32]int, len(st.cmdStats))
	for k, v := range st.cmdStats {
		cp[k] = v
	}
	return st.rawPackets, cp
}

// SetReadTimeoutHint, Read için zaman aşımı belirler (0 = sınırsız bekle).
// Veri gelmezse Read, context.DeadlineExceeded benzeri bir hatayla döner.
func (st *Stream) SetReadTimeoutHint(d time.Duration) {
	st.recvMu.Lock()
	st.readTimeout = d
	st.recvMu.Unlock()
}

// NewStream, bağlı bir Session'ı XMIP reliable akışa sarar ve alıcı döngüsünü başlatır.
func (s *Session) NewStream() *Stream {
	st := &Stream{
		conn:      s.conn,
		peer:      s.peer,
		mySession: s.mySession,
		myUUID:    s.myUUID,
		pending:   make(map[uint32]chan struct{}),
		reorder:   make(map[uint32][]byte),
		dataCh:    make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	go st.recvLoop()
	return st
}

var errClosed = errors.New("cloud: akış kapalı")

// Write, veriyi dilimlere bölüp güvenilir gönderir (her dilim ACK'lenene dek yeniden iletilir).
func (st *Stream) Write(p []byte) (int, error) {
	st.sendMu.Lock()
	defer st.sendMu.Unlock()
	if st.isClosed() {
		return 0, errClosed
	}
	total := len(p)
	for off := 0; off < total; {
		end := off + maxSlicePayload
		if end > total {
			end = total
		}
		slice := p[off:end]
		first := off == 0
		last := end == total
		seq := st.sendSeq
		st.sendSeq++
		if err := st.sendReliable(seq, slice, first, last); err != nil {
			return off, err
		}
		off = end
	}
	return total, nil
}

// sendReliable, bir dilimi gönderir ve ACK gelene dek (zaman aşımıyla) yeniden iletir.
func (st *Stream) sendReliable(seq uint32, payload []byte, first, last bool) error {
	ackCh := make(chan struct{}, 1)
	st.pendMu.Lock()
	st.pending[seq] = ackCh
	st.pendMu.Unlock()
	defer func() {
		st.pendMu.Lock()
		delete(st.pending, seq)
		st.pendMu.Unlock()
	}()

	pkt := buildXmipData(seq, payload, first, last)
	for attempt := 0; attempt < 12; attempt++ {
		if st.isClosed() {
			return errClosed
		}
		if _, err := st.conn.WriteToUDP(pkt, st.peer); err != nil {
			return err
		}
		select {
		case <-ackCh:
			return nil
		case <-time.After(300 * time.Millisecond):
			// yeniden ilet
		case <-st.done:
			return errClosed
		}
	}
	return errors.New("cloud: XMIP dilimi ACK alınamadı")
}

// Read, sıraya dizilmiş veriyi döndürür.
func (st *Stream) Read(p []byte) (int, error) {
	for {
		st.recvMu.Lock()
		if len(st.readBuf) > 0 {
			n := copy(p, st.readBuf)
			st.readBuf = st.readBuf[n:]
			st.recvMu.Unlock()
			return n, nil
		}
		if len(st.ready) > 0 {
			st.readBuf = st.ready[0]
			st.ready = st.ready[1:]
			st.recvMu.Unlock()
			continue
		}
		closed := st.closed || st.remoteClose
		to := st.readTimeout
		st.recvMu.Unlock()
		if closed {
			return 0, io.EOF
		}
		if to > 0 {
			select {
			case <-st.dataCh:
			case <-st.done:
				return 0, io.EOF
			case <-time.After(to):
				return 0, errReadTimeout
			}
		} else {
			select {
			case <-st.dataCh:
			case <-st.done:
				return 0, io.EOF
			}
		}
	}
}

var errReadTimeout = errors.New("cloud: okuma zaman aşımı")

// recvLoop, soketten okur; XMIP veri/ACK/kontrol paketlerini işler.
func (st *Stream) recvLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-st.done:
			return
		default:
		}
		st.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, from, err := st.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			st.signalClose(true)
			return
		}
		_ = from
		st.handlePacket(buf[:n])
	}
}

func (st *Stream) handlePacket(buf []byte) {
	cmd := cmdOf(buf)
	st.statMu.Lock()
	st.rawPackets++
	if st.cmdStats == nil {
		st.cmdStats = make(map[uint32]int)
	}
	st.cmdStats[cmd]++
	st.statMu.Unlock()
	switch cmd {
	case cmdHelloReq:
		if len(buf) >= 112 {
			peerAttempt := binary.LittleEndian.Uint16(buf[108:110])
			st.conn.WriteToUDP(BuildHelloRsp(st.mySession, st.myUUID, peerAttempt), st.peer)
		}
	case cmdCloseCon, cmdCloseConEx:
		st.signalClose(true)
	case cmdDataPacket:
		st.handleData(buf)
	}
}

func (st *Stream) handleData(buf []byte) {
	if len(buf) < 12 {
		return
	}
	innerLen := int(binary.LittleEndian.Uint16(buf[6:8]))
	if innerLen == 0 || 12+innerLen > len(buf) {
		return
	}
	inner := buf[12 : 12+innerLen]
	if len(inner) < 12 || inner[0] != 'X' || inner[1] != 'M' || inner[2] != 'I' || inner[3] != 'P' {
		// ham (XMIP değil) veri — doğrudan sıraya
		st.enqueue(buf[12 : 12+innerLen])
		return
	}
	seq := binary.LittleEndian.Uint32(inner[4:8])
	count := inner[8]
	flags := inner[9]

	if innerLen == 12 && flags&xmipFlagACK != 0 {
		// peer'dan ACK
		ackCount := count
		if ackCount < 1 {
			ackCount = 1
		}
		st.pendMu.Lock()
		for i := uint32(0); i < uint32(ackCount); i++ {
			if ch, ok := st.pending[seq+i]; ok {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
		st.pendMu.Unlock()
		return
	}
	if flags&xmipFlagData == 0 {
		return
	}
	payload := make([]byte, innerLen-12)
	copy(payload, inner[12:])
	st.acceptSlice(seq, payload)
}

// acceptSlice, gelen veri dilimini reorder buffer'a koyar, sırayı ilerletir, ACK yollar.
func (st *Stream) acceptSlice(seq uint32, payload []byte) {
	released := false
	var ackSeq uint32
	var ackCount byte

	st.recvMu.Lock()
	if seq < st.nextRecvSeq {
		ackSeq, ackCount = seq, 1 // eski/duplike → yine ACK (gönderici dursun)
	} else {
		if _, exists := st.reorder[seq]; !exists {
			st.reorder[seq] = payload
			for {
				nx, ok := st.reorder[st.nextRecvSeq]
				if !ok {
					break
				}
				delete(st.reorder, st.nextRecvSeq)
				st.ready = append(st.ready, nx)
				st.nextRecvSeq++
				released = true
			}
		}
		// bitmask ACK (NetSdk uyumlu)
		if !st.hasBitmap {
			st.hasBitmap = true
			st.maxRecvSeq = seq
			st.recvBitmask = 1
			ackSeq, ackCount = seq, 1
		} else if seq > st.maxRecvSeq {
			diff := seq - st.maxRecvSeq
			if diff >= 32 {
				st.recvBitmask = 0
			} else {
				st.recvBitmask = (st.recvBitmask << diff) & 0xFFFF
			}
			st.recvBitmask |= 1
			st.maxRecvSeq = seq
			cl := 0
			for ; cl < 16; cl++ {
				pattern := uint16(0xFFFF) >> cl
				if st.recvBitmask&pattern == pattern {
					break
				}
			}
			ackCount = byte(16 - cl)
			ackSeq = seq - uint32(ackCount-1)
		} else {
			diff := st.maxRecvSeq - seq
			if diff < 16 {
				st.recvBitmask |= uint16(1) << diff
			}
			ackSeq, ackCount = seq, 1
		}
	}
	st.recvMu.Unlock()

	ack := buildXmipAck(ackSeq, ackCount)
	st.conn.WriteToUDP(ack, st.peer)

	if released {
		st.signalData()
	}
}

func (st *Stream) enqueue(b []byte) {
	cp := make([]byte, len(b))
	copy(cp, b)
	st.recvMu.Lock()
	st.ready = append(st.ready, cp)
	st.recvMu.Unlock()
	st.signalData()
}

func (st *Stream) signalData() {
	select {
	case st.dataCh <- struct{}{}:
	default:
	}
}

func (st *Stream) signalClose(remote bool) {
	st.recvMu.Lock()
	if remote {
		st.remoteClose = true
	}
	st.recvMu.Unlock()
	st.signalData()
}

func (st *Stream) isClosed() bool {
	st.recvMu.Lock()
	defer st.recvMu.Unlock()
	return st.closed
}

// Close, akışı kapatır.
func (st *Stream) Close() error {
	st.closeOnce.Do(func() {
		st.recvMu.Lock()
		st.closed = true
		st.recvMu.Unlock()
		close(st.done)
	})
	return nil
}

// --- paket yardımcıları ---

func buildXmipData(seq uint32, payload []byte, first, last bool) []byte {
	innerLen := 12 + len(payload)
	pkt := make([]byte, 12+innerLen)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdDataPacket)
	binary.LittleEndian.PutUint16(pkt[4:6], 0)
	binary.LittleEndian.PutUint16(pkt[6:8], uint16(innerLen))
	binary.LittleEndian.PutUint32(pkt[8:12], 0)
	pkt[12], pkt[13], pkt[14], pkt[15] = 'X', 'M', 'I', 'P'
	binary.LittleEndian.PutUint32(pkt[16:20], seq)
	pkt[20] = 1
	flags := byte(xmipFlagData)
	if first {
		flags |= xmipFlagFirst
	}
	if last {
		flags |= xmipFlagLast
	}
	pkt[21] = flags
	copy(pkt[24:], payload)
	return pkt
}

func buildXmipAck(seq uint32, count byte) []byte {
	pkt := make([]byte, 24)
	binary.LittleEndian.PutUint32(pkt[0:4], cmdDataPacket)
	binary.LittleEndian.PutUint16(pkt[6:8], 12)
	pkt[12], pkt[13], pkt[14], pkt[15] = 'X', 'M', 'I', 'P'
	binary.LittleEndian.PutUint32(pkt[16:20], seq)
	pkt[20] = count
	pkt[21] = xmipFlagACK
	return pkt
}
