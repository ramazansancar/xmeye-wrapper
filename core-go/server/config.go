package main

import (
	"encoding/json"
	"os"
	"strconv"
)

// Config, köprü sunucunun çalışma ayarları.
// JSON dosyadan (config.json) ya da ortam değişkenlerinden okunur.
// Bir cihaz + kimlik + kanal bilgisi; çok cihaz ileride Devices listesi olur.
type Config struct {
	Addr    string   `json:"addr"`    // dinleme adresi, örn ":8080"
	Device  Device   `json:"device"`  // izlenecek cihaz
	Relays  []string `json:"relays"`  // bilinen relay host:port listesi (opsiyonel; token'dan da gelir)
}

// Device, tek bir XMEye cihazı.
type Device struct {
	Name     string `json:"name"`     // görünen ad (örn "tbdc")
	Serial   string `json:"serial"`   // seri no (Cloud ID)
	Username string `json:"username"` // kullanıcı adı
	Password string `json:"password"` // parola (Sofia-hash'lenecek)
	Channels int    `json:"channels"` // kanal sayısı (örn 16)
}

// LoadConfig, config.json varsa okur; yoksa ortam değişkenlerinden kurar.
func LoadConfig(path string) (*Config, error) {
	c := &Config{Addr: envOr("XMEYE_ADDR", ":8080")}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	// ortam değişkenleri config.json'u ezer (Docker/hızlı kullanım)
	if v := os.Getenv("XMEYE_SERIAL"); v != "" {
		c.Device.Serial = v
	}
	if v := os.Getenv("XMEYE_USER"); v != "" {
		c.Device.Username = v
	}
	if v := os.Getenv("XMEYE_PASS"); v != "" {
		c.Device.Password = v
	}
	if v := os.Getenv("XMEYE_NAME"); v != "" {
		c.Device.Name = v
	}
	if v := os.Getenv("XMEYE_CHANNELS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Device.Channels = n
		}
	}
	if c.Device.Channels == 0 {
		c.Device.Channels = 1
	}
	if c.Device.Name == "" {
		c.Device.Name = c.Device.Serial
	}
	return c, nil
}
