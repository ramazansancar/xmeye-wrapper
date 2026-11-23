package dvrip

import (
	"encoding/json"
	"fmt"
)

// DVRIP mesaj ID'leri (tel protokolü). Standart Sofia + cihazdan doğrulananlar.
// Kaynak: python-dvr/dvrip + core-go/testdata pcap (1412 video doğrulandı).
const (
	MsgLoginReq  = 1000 // LOGIN_REQ2  — JSON login isteği
	MsgLoginResp = 1001 // LOGIN_RSP   — login yanıtı (SessionID döner)
	MsgKeepAlive = 1006 // KEEPALIVE_REQ
	MsgKeepResp  = 1007 // KEEPALIVE_RSP

	MsgOPMonitorClaim = 1413 // OPMonitor "Claim" (akış talebi)
	MsgOPMonitorData  = 1412 // OPMonitor veri (video frame) — pcap'te doğrulandı
	MsgOPMonitorReq   = 1410 // OPMonitor başlat/durdur

	MsgOPPlayBack = 1420 // OPPlayBack (geçmiş kayıt)
	MsgOPTalk     = 1430 // OPTalk (ses)
	MsgSysTime    = 1452 // SYSTEM_TIME
)

// LoginReq, DVRIP login JSON gövdesi (Sofia standart alanları).
type LoginReq struct {
	EncryptType string `json:"EncryptType"` // "MD5"
	LoginType   string `json:"LoginType"`   // "DVRIP-Web"
	PassWord    string `json:"PassWord"`    // SofiaHash(parola)
	UserName    string `json:"UserName"`    // kullanıcı adı
}

// BuildLogin, kullanıcı/parola için login paketini üretir.
// Parola Sofia-hash'lenir; gövde JSON + sonlandırıcı null bayt (Sofia JSON gövdeleri \x00 ile biter).
func BuildLogin(user, password string) *Packet {
	req := LoginReq{
		EncryptType: "MD5",
		LoginType:   "DVRIP-Web",
		PassWord:    SofiaHash(password),
		UserName:    user,
	}
	body, _ := json.Marshal(req)
	body = append(body, 0x0a, 0x00) // Sofia: \n + null sonlandırıcı
	return &Packet{
		MsgID: MsgLoginReq,
		Body:  body,
	}
}

// SessionHex, sessionID'yi DVRIP JSON'da beklenen "0x........" biçimine çevirir.
func SessionHex(sessionID uint32) string {
	return fmt.Sprintf("0x%08X", sessionID)
}

// BuildOPMonitorClaim, canlı akış başlatma (OPMonitor Claim) paketi üretir.
// channel: 0-tabanlı kanal; main true=ana akış (HD), false=alt akış (SD).
// pcap'teki biçim: {"Name":"OPMonitor","OPMonitor":{"Action":"Claim","Action1":"Start",
//   "Parameter":{"Channel":n,"CombinMode":"NONE","StreamType":"Main|Extra1","TransMode":"TCP"}},"SessionID":"0x..."}
func BuildOPMonitorClaim(sessionID uint32, seq uint32, channel int, main bool) *Packet {
	streamType := "Main"
	if !main {
		streamType = "Extra1"
	}
	payload := map[string]any{
		"Name": "OPMonitor",
		"OPMonitor": map[string]any{
			"Action": "Claim",
			"Parameter": map[string]any{
				"Channel":    channel,
				"CombinMode": "NONE",
				"StreamType": streamType,
				"TransMode":  "TCP",
			},
		},
		// pcap'te Claim gövdesinde DHParameter.RandomStrA bulunuyordu (DH anahtar değişimi alanı).
		"DHParameter": map[string]any{"RandomStrA": "1JJu"},
		"SessionID":   SessionHex(sessionID),
	}
	p := BuildJSONCommand(sessionID, seq, MsgOPMonitorClaim, payload)
	// DVRIP header offset 12 = kanal (NetSdk header[12]=channel). TotalPacket alanı bu offset.
	p.TotalPacket = uint8(channel)
	return p
}

// BuildOPMonitorStart, canlı akışı ASIL başlatan 1410 (OPMonitorReq) paketini üretir.
// pcap sırası: 1413 Claim → 1414 Ret:100 → 1410 Start → 1412 video. channel header offset 12'ye yazılır.
// NOT: pcap'te 1410 body base64/ikili (muhtemelen data-stream auth) — burada JSON denenir; gerekirse
// gerçek body formatı (native) çözülecek. StreamType header + JSON ile deneme.
func BuildOPMonitorStart(sessionID uint32, seq uint32, channel int, main bool) *Packet {
	streamType := "Main"
	if !main {
		streamType = "Extra1"
	}
	payload := map[string]any{
		"Name": "OPMonitor",
		"OPMonitor": map[string]any{
			"Action": "Start",
			"Parameter": map[string]any{
				"Channel":    channel,
				"CombinMode": "NONE",
				"StreamType": streamType,
				"TransMode":  "TCP",
			},
		},
		"SessionID": SessionHex(sessionID),
	}
	p := BuildJSONCommand(sessionID, seq, MsgOPMonitorReq, payload)
	p.TotalPacket = uint8(channel)
	return p
}

// BuildJSONCommand, verilen msgId + JSON gövdeli genel komut paketi üretir.
// sessionID login sonrası alınır; Sofia gövde sonuna \n\x00 ekler.
func BuildJSONCommand(sessionID uint32, seq uint32, msgID uint16, payload any) *Packet {
	body, _ := json.Marshal(payload)
	body = append(body, 0x0a, 0x00)
	return &Packet{
		SessionID: sessionID,
		Sequence:  seq,
		MsgID:     msgID,
		Body:      body,
	}
}
