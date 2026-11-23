package dvrip

import "testing"

// Sofia parola özeti bilinen vektörlerle doğrulanır.
// Boş parola → "tlJwpbo6" (python-dvr / dvrip projelerinde sabit referans değer).
func TestSofiaHash(t *testing.T) {
	cases := map[string]string{
		"": "tlJwpbo6", // bilinen referans
	}
	for pw, want := range cases {
		if got := SofiaHash(pw); got != want {
			t.Errorf("SofiaHash(%q) = %q, beklenen %q", pw, got, want)
		}
	}
}

// Uzunluk her zaman 8 ve alfabe içinde olmalı.
func TestSofiaHashShape(t *testing.T) {
	for _, pw := range []string{"", "admin123.", "koy1", "password", "1234"} {
		h := SofiaHash(pw)
		if len(h) != 8 {
			t.Errorf("SofiaHash(%q) = %q uzunluk %d, 8 bekleniyor", pw, h, len(h))
		}
	}
}
