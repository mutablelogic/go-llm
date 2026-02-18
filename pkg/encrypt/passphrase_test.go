package encrypt

import "testing"

func BenchmarkDeriveKey(b *testing.B) {
	salt, _ := GenerateSalt()
	for i := 0; i < b.N; i++ {
		DeriveKey("test-passphrase", salt)
	}
}
