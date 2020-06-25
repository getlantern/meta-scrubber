package metascrubber

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	pngimagestructure "github.com/dsoprea/go-png-image-structure"
)

var chunkTypesToScrub = []string{"eXIf", "tEXt", "iTXt", "zTXt"}

type pngSegmentReader struct {
	reader         io.Reader
	wroteSignature bool
}

func (pr *pngSegmentReader) isMetadataType(chunkType []byte) bool {
	for _, test := range chunkTypesToScrub {
		if bytes.Equal(chunkType, []byte(test)) {
			return true
		}
	}
	return false
}

func (pr *pngSegmentReader) nextSegment() (r io.Reader, isMetadata bool, err error) {
	if !pr.wroteSignature {
		r = io.LimitReader(pr.reader, int64(len(pngimagestructure.PngSignature)))
		pr.wroteSignature = true
		return
	}

	chunkHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(pr.reader, chunkHeaderBuffer)

	var chunkDataLength uint32
	if err = binary.Read(teedReader, binary.BigEndian, &chunkDataLength); err != nil {
		if !errors.Is(err, io.EOF) {
			err = &MalformedDataError{"can't parse png chunk data length", err}
		}
		return
	}

	chunkType := make([]byte, 4)
	if _, err = io.ReadFull(teedReader, chunkType); err != nil {
		err = &MalformedDataError{"can't parse png chunk type", err}
		return
	}

	// 4 byte chunkType Header + 4 byte chunkDataLength Header + chunkDataLength + 4 byte CRC length
	chunkLength := int64(12 + chunkDataLength)

	r = io.LimitReader(io.MultiReader(chunkHeaderBuffer, pr.reader), chunkLength)

	isMetadata = pr.isMetadataType(chunkType)
	return
}
