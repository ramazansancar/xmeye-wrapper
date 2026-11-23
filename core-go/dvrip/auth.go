package dvrip

import "crypto/md5"

// SofiaHash, Xiongmai/Sofia'nın parola özet algoritması ("compressed MD5").
// SEC Consult tarafından tersine mühendislikle çözülmüş, public algoritma:
//   1. ham parolanın MD5'i alınır (16 byte)
//   2. ardışık ikişer byte toplanır, 0x3E ile mod'lanır
//   3. sonuç [0-9A-Za-z] alfabesinden 8 karakterlik dizeye eşlenir
//
// Boş parola → "tlJwpbo6". Bu, DVRIP login JSON'undaki "PassWord" alanında kullanılır.
func SofiaHash(password string) string {
	const alphabet = "0123456789" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz"
	sum := md5.Sum([]byte(password)) // 16 byte
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		n := int(sum[2*i]) + int(sum[2*i+1])
		out[i] = alphabet[n%len(alphabet)]
	}
	return string(out)
}
