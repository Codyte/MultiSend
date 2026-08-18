package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/proto"
)

// fakeReceiver mimics the agent receiver: it verifies the HMAC and the per-chunk
// SHA-256 before acking "done", exactly like cmd/multisend-agent/handleConn.
type fakeReceiver struct {
	secret    []byte
	mu        sync.Mutex
	got       map[int64][]byte
	rejectBad bool
}

func (fr *fakeReceiver) serve(t *testing.T, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go fr.handle(conn)
	}
}

func (fr *fakeReceiver) handle(conn net.Conn) {
	defer conn.Close()
	h, err := proto.ReadHeader(conn)
	if err != nil {
		return
	}
	if !proto.VerifyHeader(h, fr.secret) {
		_ = proto.WriteChunkAck(conn, proto.ChunkAck{TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "unauthorized"})
		return
	}
	hasher := sha256.New()
	buf := make([]byte, h.PartSize)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	hasher.Write(buf)
	if h.ChunkSHA256 != "" && hex.EncodeToString(hasher.Sum(nil)) != h.ChunkSHA256 {
		_ = proto.WriteChunkAck(conn, proto.ChunkAck{TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "hash_mismatch"})
		return
	}
	fr.mu.Lock()
	fr.got[h.ChunkIndex] = buf
	fr.mu.Unlock()
	_ = proto.WriteChunkAck(conn, proto.ChunkAck{TransferID: h.TransferID, ChunkIndex: h.ChunkIndex, Status: "done", BytesReceived: h.PartSize})
}

func writeTempFile(t *testing.T, size int) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "payload.bin")
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSendAuthenticatedAndVerified(t *testing.T) {
	secret := []byte("e2e-shared-secret")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fr := &fakeReceiver{secret: secret, got: map[int64][]byte{}}
	go fr.serve(t, ln)

	// Isolate manifest output so the test does not touch the user profile.
	t.Setenv("USERPROFILE", t.TempDir())
	src := writeTempFile(t, 300*1024)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = SendFileSplitWithResultCtx(ctx, src, PeerTarget{Address: ln.Addr().String()}, []string{"127.0.0.1"}, nil, SendOptions{
		ChunkSize:  64 * 1024,
		AuthSecret: secret,
	})
	if err != nil {
		t.Fatalf("send must succeed with matching secret: %v", err)
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.got) == 0 {
		t.Fatal("receiver got no chunks")
	}
}

func TestSendRejectedWithWrongSecret(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fr := &fakeReceiver{secret: []byte("server-secret"), got: map[int64][]byte{}}
	go fr.serve(t, ln)

	t.Setenv("USERPROFILE", t.TempDir())
	src := writeTempFile(t, 128*1024)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = SendFileSplitWithResultCtx(ctx, src, PeerTarget{Address: ln.Addr().String()}, []string{"127.0.0.1"}, nil, SendOptions{
		ChunkSize:  64 * 1024,
		AuthSecret: []byte("client-mismatch"),
	})
	if err == nil {
		t.Fatal("send must fail when the receiver rejects the HMAC")
	}
}
