package dvrip

import "encoding/json"

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
