package proto

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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
}

type ChunkAck struct {
	Type          string `json:"type"`
	TransferID    string `json:"transfer_id"`
	ChunkIndex    int64  `json:"chunk_index"`
	Status        string `json:"status"`
	BytesReceived int64  `json:"bytes_received"`
	Error         string `json:"error,omitempty"`
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
	if h.PartIndex < 1 || h.TotalParts < 1 || h.PartIndex > h.TotalParts || h.PartSize < 0 {
		return Header{}, fmt.Errorf("invalid header fields")
	}
	if h.MessageType == "" {
		h.MessageType = "chunk_start"
	}
	return h, nil
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
