package corpusgenerator

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var mp4ContainerBoxes = map[string]bool{
	"moov": true,
	"trak": true,
	"mdia": true,
	"minf": true,
	"stbl": true,
	"edts": true,
}

func containsMP4Box(path, target string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	return scanMP4Boxes(file, 0, info.Size(), target)
}

func scanMP4Boxes(reader io.ReaderAt, start, end int64, target string) (bool, error) {
	position := start
	for position+8 <= end {
		header := make([]byte, 16)
		if _, err := reader.ReadAt(header[:8], position); err != nil {
			return false, err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		boxType := string(header[4:8])
		headerSize := int64(8)
		switch size {
		case 0:
			size = end - position
		case 1:
			if _, err := reader.ReadAt(header[8:16], position+8); err != nil {
				return false, err
			}
			size = int64(binary.BigEndian.Uint64(header[8:16]))
			headerSize = 16
		}
		if size < headerSize || position+size > end {
			return false, fmt.Errorf("invalid MP4 box %q at %d with size %d", boxType, position, size)
		}
		if boxType == target {
			return true, nil
		}
		if mp4ContainerBoxes[boxType] {
			found, err := scanMP4Boxes(reader, position+headerSize, position+size, target)
			if err != nil || found {
				return found, err
			}
		}
		position += size
	}
	return false, nil
}
