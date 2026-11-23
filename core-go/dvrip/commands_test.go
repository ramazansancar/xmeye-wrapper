package dvrip

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Login paketi: doğru msgId, çözülebilir JSON, Sofia-hash parola, null sonlandırıcı.
func TestBuildLogin(t *testing.T) {
	p := BuildLogin("koy1", "admin123.")
	if p.MsgID != MsgLoginReq {
		t.Fatalf("msgId=%d, beklenen %d", p.MsgID, MsgLoginReq)
	}
	// gövde \n\x00 ile bitmeli
	if n := len(p.Body); n < 2 || p.Body[n-1] != 0x00 || p.Body[n-2] != 0x0a {
		t.Fatalf("gövde \\n\\x00 ile bitmiyor: %x", p.Body)
	}
	// JSON kısmı (sonlandırıcısız) çözülebilmeli
	jsonPart := bytes.TrimRight(p.Body, "\x00")
	jsonPart = bytes.TrimRight(jsonPart, "\n")
	var req LoginReq
	if err := json.Unmarshal(jsonPart, &req); err != nil {
		t.Fatalf("login JSON çözülemedi: %v (%s)", err, jsonPart)
	}
	if req.UserName != "koy1" {
		t.Errorf("UserName=%q", req.UserName)
	}
	if req.EncryptType != "MD5" || req.LoginType != "DVRIP-Web" {
		t.Errorf("Encrypt/LoginType yanlış: %+v", req)
	}
	if req.PassWord != SofiaHash("admin123.") {
		t.Errorf("parola hash eşleşmedi: %q != %q", req.PassWord, SofiaHash("admin123."))
	}
	// encode→decode tur-gidiş
	wire := p.Encode()
	got, _, err := Decode(wire)
	if err != nil {
		t.Fatalf("login encode/decode: %v", err)
	}
	if !bytes.Equal(got.Body, p.Body) {
		t.Fatalf("login round-trip gövde uyuşmadı")
	}
}

// Genel JSON komut: sessionID/seq/msgId korunur, gövde çözülebilir.
func TestBuildJSONCommand(t *testing.T) {
	payload := map[string]any{"Name": "OPMonitor", "Channel": 0}
	p := BuildJSONCommand(0x084e, 5, MsgOPMonitorReq, payload)
	if p.SessionID != 0x084e || p.Sequence != 5 || p.MsgID != MsgOPMonitorReq {
		t.Fatalf("başlık alanları yanlış: %+v", p)
	}
	jsonPart := bytes.TrimRight(bytes.TrimRight(p.Body, "\x00"), "\n")
	var m map[string]any
	if err := json.Unmarshal(jsonPart, &m); err != nil {
		t.Fatalf("komut JSON çözülemedi: %v", err)
	}
	if m["Name"] != "OPMonitor" {
		t.Errorf("Name=%v", m["Name"])
	}
}
