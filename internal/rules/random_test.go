package rules

import "testing"

func TestLooksRandom(t *testing.T) {
	for _, s := range []string{
		"Xk9vQ2mLp7RtZ4wB",                 // base62
		"a3f8c9e1b2d4f6a8c0e2b4d6f8a1c3e5", // hex
		"dGhpcyBpcyBhIHNlY3JldCB2YWx1ZQ==", // base64
		"7Fh2_kL9-pQ4xZ8m",                 // with separators
		"72f988bf-86f1-41af-91ab-2d7cd011db47",
	} {
		if !looksRandom(s) {
			t.Errorf("%s should look random", s)
		}
	}
	for _, s := range []string{
		"Xk9vQ2mLp7RtZ4w", // 15 characters
		"AuthenticationService2",
		"PasswordManager1",
		"my-service-name-2",
		"configuration_value",
		"Version1.2.3-beta",
		"2024-01-15T10:30:00",
		"ab12ab12ab12ab12",
		"0123456789012345",
		"qwhxkzmvbtrplnfdgs", // random, but letters only
	} {
		if looksRandom(s) {
			t.Errorf("%s should not look random", s)
		}
	}
}
