package db

import "testing"

func BenchmarkKey(b *testing.B) {
	secret, salt := "secret", []byte("abcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefga")
	for n := 0; n < b.N; n++ {
		key, h := Key(secret, salt)
		if (len(key) == 0) || (len(h) == 0) {
			b.Error("unexpected error")
		}
	}
}
