package handler

import "testing"

func TestValidateTMDbProxyURLUsesReverseProxySemantics(t *testing.T) {
	valid := map[string]string{
		"":                               "",
		" https://example.com/tmdbapi/ ": "https://example.com/tmdbapi",
		"http://192.168.1.10:8080":       "http://192.168.1.10:8080",
	}
	for input, expected := range valid {
		actual, err := validateTMDbProxyURL(input)
		if err != nil || actual != expected {
			t.Fatalf("validateTMDbProxyURL(%q) = %q, %v; want %q", input, actual, err, expected)
		}
	}
	for _, input := range []string{"socks5://127.0.0.1:1080", "ftp://example.com", "http:///missing"} {
		if _, err := validateTMDbProxyURL(input); err == nil {
			t.Fatalf("validateTMDbProxyURL(%q) should fail", input)
		}
	}
}
