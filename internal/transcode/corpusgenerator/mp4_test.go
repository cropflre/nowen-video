package corpusgenerator

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestContainsMP4BoxFindsNestedEditList(t *testing.T) {
	content := mp4Box("ftyp", []byte("isom"))
	content = append(content, mp4Box("moov", mp4Box("trak", mp4Box("edts", mp4Box("elst", []byte{0, 0, 0, 0}))))...)
	path := filepath.Join(t.TempDir(), "edit-list.mp4")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := containsMP4Box(path, "edts")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("nested edit-list box was not found")
	}
}

func TestContainsMP4BoxRejectsAbsentEditList(t *testing.T) {
	content := mp4Box("moov", mp4Box("trak", mp4Box("mdia", nil)))
	path := filepath.Join(t.TempDir(), "no-edit-list.mp4")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := containsMP4Box(path, "edts")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("unexpected edit-list box")
	}
}

func TestContainsMP4BoxRejectsInvalidSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.mp4")
	content := make([]byte, 8)
	binary.BigEndian.PutUint32(content[:4], 32)
	copy(content[4:8], "moov")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := containsMP4Box(path, "edts")
	if err == nil {
		t.Fatal("expected invalid box size failure")
	}
}

func mp4Box(boxType string, payload []byte) []byte {
	if len(boxType) != 4 {
		panic("MP4 box type must contain four bytes")
	}
	buffer := bytes.NewBuffer(make([]byte, 0, len(payload)+8))
	if err := binary.Write(buffer, binary.BigEndian, uint32(len(payload)+8)); err != nil {
		panic(err)
	}
	buffer.WriteString(boxType)
	buffer.Write(payload)
	return buffer.Bytes()
}

func TestMP4ParserDoesNotDependOnHostEndianness(t *testing.T) {
	if runtime.GOARCH == "wasm" {
		t.Skip("filesystem semantics are not relevant on wasm")
	}
	content := mp4Box("moov", mp4Box("trak", mp4Box("edts", nil)))
	path := filepath.Join(t.TempDir(), "portable.mp4")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := containsMP4Box(path, "edts")
	if err != nil || !found {
		t.Fatalf("portable box parse failed: found=%v err=%v", found, err)
	}
}
