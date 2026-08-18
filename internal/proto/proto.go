package proto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Header struct {
	MessageType    string `json:"message_type,omitempty"`
	TransferID     string `json:"transfer_id,omitempty"`
	FileName       string `json:"file_name"`
	PartIndex      int    `json:"part_index"`
	TotalParts     int    `json:"total_parts"`
	PartSize       int64  `json:"part_size"`
	ChunkIndex     int64  `json:"chunk_index,omitempty"`
	Offset         int64  `json:"offset,omitempty"`
	TotalBytes     int64  `json:"total_bytes,omitempty"`
	Lab            bool   `json:"lab,omitempty"`
	LabPath        string `json:"lab_receive_path,omitempty"`
	ReceiveRelPath string `json:"receive_rel_path,omitempty"`
	PublishName    string `json:"publish_name,omitempty"`
	FolderResult   string `json:"folder_result,omitempty"`
	// ChunkSHA256 is the lowercase hex SHA-256 of the chunk payload that follows
	// this header. The receiver verifies it before acking "done". Empty means the
	// sender did not supply one (older clients); verification is then skipped.
	ChunkSHA256 string `json:"chunk_sha256,omitempty"`
	// Auth is an HMAC-SHA256 (hex) of the canonical header fields keyed by the
	// shared node secret. Empty when authentication is disabled.
	Auth string `json:"auth,omitempty"`
}

type ChunkAck struct {
	Type          string `json:"type"`
	TransferID    string `json:"transfer_id"`
	ChunkIndex    int64  `json:"chunk_index"`
	Status        string `json:"status"`
	BytesReceived int64  `json:"bytes_received"`
	Error         string `json:"error,omitempty"`
}

// authPayload builds the deterministic byte string that the HMAC covers. It
// includes every field a malicious peer could tamper with to redirect or
// corrupt a transfer (destination paths, sizes, offsets, payload hash). The
// Auth field itself is intentionally excluded.
func (h Header) authPayload() []byte {
	return []byte(fmt.Sprintf(
		"v1\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d\x00%d\x00%d\x00%t\x00%s\x00%s\x00%s\x00%s\x00%s",
		h.MessageType, h.TransferID, h.FileName, h.PartIndex, h.TotalParts,
		h.PartSize, h.ChunkIndex, h.Offset, h.TotalBytes, h.Lab,
		h.LabPath, h.ReceiveRelPath, h.PublishName, h.FolderResult, h.ChunkSHA256,
	))
}

// SignHeader sets h.Auth to the HMAC of the header keyed by secret. A nil/empty
// secret leaves the header unsigned (authentication disabled).
func SignHeader(h *Header, secret []byte) {
	if h == nil || len(secret) == 0 {
		return
	}
	h.Auth = ""
	mac := hmac.New(sha256.New, secret)
	mac.Write(h.authPayload())
	h.Auth = hex.EncodeToString(mac.Sum(nil))
}

// VerifyHeader reports whether h carries a valid HMAC for secret. When secret is
// empty, authentication is disabled and any header is accepted (backward compat).
// The comparison is constant time.
func VerifyHeader(h Header, secret []byte) bool {
	if len(secret) == 0 {
		return true
	}
	provided, err := hex.DecodeString(h.Auth)
	if err != nil || len(provided) == 0 {
		return false
	}
	h.Auth = ""
	mac := hmac.New(sha256.New, secret)
	mac.Write(h.authPayload())
	return hmac.Equal(mac.Sum(nil), provided)
}

func WriteHeader(w io.Writer, h Header) error {
	if h.MessageType == "" {
		h.MessageType = "chunk_start"
	}
	return writeFrame(w, h)
}

func ReadHeader(r io.Reader) (Header, error) {
	var h Header
	if err := readFrame(r, &h); err != nil {
		return Header{}, err
	}
	if h.MessageType == "" {
		h.MessageType = "chunk_start"
	}
	return h, nil
}

// ValidateHeader validates the structural chunk invariants shared by all
// receivers. Authentication and digest requirements remain receiver policy.
func ValidateHeader(h Header) error {
	if h.MessageType != "" && h.MessageType != "chunk_start" {
		return fmt.Errorf("invalid message type")
	}
	if len(h.TransferID) == 0 || len(h.TransferID) > 128 || strings.IndexFunc(h.TransferID, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	}) >= 0 {
		return fmt.Errorf("invalid transfer_id")
	}
	if strings.TrimSpace(h.FileName) == "" {
		return fmt.Errorf("invalid file_name")
	}
	if h.TotalParts < 1 || h.TotalParts > 1_000_000 || h.ChunkIndex < 0 || h.ChunkIndex >= int64(h.TotalParts) || h.PartIndex != int(h.ChunkIndex)+1 {
		return fmt.Errorf("invalid chunk position")
	}
	if h.TotalBytes < 0 || h.Offset < 0 || h.PartSize < 0 || h.PartSize > h.TotalBytes || h.Offset > h.TotalBytes-h.PartSize {
		return fmt.Errorf("invalid chunk bounds")
	}
	if h.ChunkSHA256 != "" {
		if len(h.ChunkSHA256) != sha256.Size*2 {
			return fmt.Errorf("invalid chunk_sha256")
		}
		if _, err := hex.DecodeString(h.ChunkSHA256); err != nil {
			return fmt.Errorf("invalid chunk_sha256")
		}
	}
	return nil
}

func WriteChunkAck(w io.Writer, ack ChunkAck) error {
	if ack.Type == "" {
		ack.Type = "chunk_ack"
	}
	return writeFrame(w, ack)
}

func ReadChunkAck(r io.Reader) (ChunkAck, error) {
	var ack ChunkAck
	if err := readFrame(r, &ack); err != nil {
		return ChunkAck{}, err
	}
	if ack.Type == "" {
		ack.Type = "chunk_ack"
	}
	return ack, nil
}

func writeFrame(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > 64*1024 {
		return fmt.Errorf("frame too large")
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func readFrame(r io.Reader, v any) error {
	var n uint32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return err
	}
	if n == 0 || n > 64*1024 {
		return fmt.Errorf("invalid frame length")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}
