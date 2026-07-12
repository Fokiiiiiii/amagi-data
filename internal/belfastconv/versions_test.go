package belfastconv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteVersionsJSONUsesBelfastOrderAndCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global", "versions.json")
	versions := map[string]string{
		"CN": "9.6.667",
		"EN": "9.2.557",
		"JP": "9.2.821",
		"KR": "8.4.137",
		"TW": "8.4.272",
	}
	if err := writeVersionsJSON(path, versions); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\r\n    \"TW\":\"8.4.272\",\r\n    \"KR\":\"8.4.137\",\r\n    \"CN\":\"9.6.667\",\r\n    \"JP\":\"9.2.821\",\r\n    \"EN\":\"9.2.557\"\r\n}\r\n"
	if string(got) != want {
		t.Fatalf("versions JSON mismatch:\n got %q\nwant %q", got, want)
	}
}
