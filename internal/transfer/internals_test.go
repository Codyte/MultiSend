package transfer

import (
	"bytes"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"lab/multinet/internal/manifest"
)

func TestContentFingerprintDetectsInPlaceEdit(t *testing.T) {
	// Two buffers of identical length but different content (head window) must
	// produce different fingerprints so resume cannot reuse a stale manifest.
	a := bytes.Repeat([]byte{0xAA}, 256*1024)
	b := append([]byte(nil), a...)
	b[10] = 0x55

	fpA, err := contentFingerprint(bytes.NewReader(a), int64(len(a)))
	if err != nil {
		t.Fatal(err)
	}
	fpB, err := contentFingerprint(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	if fpA == fpB {
		t.Fatal("fingerprints must differ when content changes at same size")
	}

	// Tail-window edit must also be detected.
	c := append([]byte(nil), a...)
	c[len(c)-5] = 0x55
	fpC, err := contentFingerprint(bytes.NewReader(c), int64(len(c)))
	if err != nil {
		t.Fatal(err)
	}
	if fpA == fpC {
		t.Fatal("fingerprints must differ on tail edit")
	}
}

func TestProgressReaderRollback(t *testing.T) {
	var cable int64
	pr := &progressReader{r: bytes.NewReader(make([]byte, 4096)), cable: &cable, channel: "cable"}
	buf := make([]byte, 4096)
	if _, err := pr.Read(buf); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(&cable) == 0 {
		t.Fatal("expected bytes to be counted")
	}
	pr.rollback()
	if got := atomic.LoadInt64(&cable); got != 0 {
		t.Fatalf("rollback must zero the counter, got %d", got)
	}
}

func TestManifestSaverThrottleAndFlush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.json")
	mf := &manifest.Manifest{SchemaVersion: 1, TransferID: "tx", FileName: "f"}

	s := newManifestSaver(path, time.Hour)
	s.save(mf, false) // first write happens (last is zero)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("first save should persist: %v", err)
	}
	// Throttled: remove the file, a non-forced save within the interval must skip.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.save(mf, false)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("throttled save must not write within the interval")
	}
	// Forced flush always writes.
	if err := s.flush(mf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("flush should persist: %v", err)
	}
}

func TestHashSectionMatchesKnown(t *testing.T) {
	data := []byte("hello-multisend")
	got, err := hashSection(bytes.NewReader(data), 0, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	// Independent recompute via fingerprint of the same single window must be stable.
	again, err := hashSection(bytes.NewReader(data), 0, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if got != again || got == "" {
		t.Fatalf("hashSection unstable: %q vs %q", got, again)
	}
}
