package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// The canonical SHA-256 of "abc" — core matches on this digest to
	// de-duplicate, so the algorithm and encoding both have to be exact.
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	got, err := fileChecksum(path)
	if err != nil {
		t.Fatalf("fileChecksum: %v", err)
	}
	if got != want {
		t.Errorf("fileChecksum = %q, want %q", got, want)
	}
	if len(got) != 64 {
		t.Errorf("digest is %d chars, want the 64 core's regex requires", len(got))
	}
}

func TestFileChecksumMissingFile(t *testing.T) {
	if _, err := fileChecksum(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("fileChecksum on a missing file = nil error, want an error")
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		// The limit users will actually see quoted back at them.
		{50 * 1024 * 1024, "50.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
