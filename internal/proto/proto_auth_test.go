package proto

import "testing"

func sampleHeader() Header {
	return Header{
		MessageType: "chunk_start",
		TransferID:  "tx-1",
		FileName:    "file.bin",
		PartIndex:   1,
		TotalParts:  4,
		PartSize:    1024,
		ChunkIndex:  0,
		Offset:      0,
		TotalBytes:  4096,
		ChunkSHA256: "deadbeef",
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := []byte("shared-secret")
	h := sampleHeader()
	SignHeader(&h, secret)
	if h.Auth == "" {
		t.Fatal("expected Auth to be set after signing")
	}
	if !VerifyHeader(h, secret) {
		t.Fatal("valid signature must verify")
	}
}

func TestVerifyRejectsTamper(t *testing.T) {
	secret := []byte("shared-secret")
	h := sampleHeader()
	SignHeader(&h, secret)

	tampered := h
	tampered.ReceiveRelPath = "../escape"
	if VerifyHeader(tampered, secret) {
		t.Fatal("tampered destination path must fail verification")
	}

	tampered = h
	tampered.ChunkSHA256 = "00000000"
	if VerifyHeader(tampered, secret) {
		t.Fatal("tampered payload hash must fail verification")
	}

	if VerifyHeader(h, []byte("wrong-secret")) {
		t.Fatal("wrong secret must fail verification")
	}
}

func TestVerifyDisabledWhenNoSecret(t *testing.T) {
	h := sampleHeader()
	// No signing; empty secret means auth disabled and any header is accepted.
	if !VerifyHeader(h, nil) {
		t.Fatal("empty secret should accept unsigned headers")
	}
	// Signing with empty secret is a no-op.
	SignHeader(&h, nil)
	if h.Auth != "" {
		t.Fatal("signing with empty secret must not set Auth")
	}
}

func TestVerifyRejectsMissingAuthWhenRequired(t *testing.T) {
	secret := []byte("shared-secret")
	h := sampleHeader() // unsigned
	if VerifyHeader(h, secret) {
		t.Fatal("unsigned header must be rejected when a secret is configured")
	}
}
