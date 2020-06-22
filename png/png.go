// png handles scrubbing png files of metadata
package png

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"

	pngimagestructure "github.com/dsoprea/go-png-image-structure"
)

type PngScrubber struct {
	reader         io.Reader
	chunkData      io.LimitedReader
	wroteSignature bool
}

var chunkTypesToScrub = []string{"eXIf", "tEXt", "iTXt", "zTXt"}

func NewPngScrubber(reader io.Reader) *PngScrubber {
	return &PngScrubber{
		reader: reader,
	}
}

func isMetadataType(chunkType []byte) bool {
	// TODO: strip other chunk types??
	for _, test := range chunkTypesToScrub {
		if bytes.Equal(chunkType, []byte(test)) {
			return true
		}
	}
	return false
}

func (self *PngScrubber) Read(buf []byte) (n int, err error) {
	var n2 int

	if !self.wroteSignature {
		n, err = io.ReadFull(self.reader, buf[:len(pngimagestructure.PngSignature)])
		if err != nil {
			return
		}
		self.wroteSignature = true
	}

	n2, err = self.chunkData.Read(buf[n:])
	n += n2
	if err != io.EOF {
		return
	}

	chunkHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(self.reader, chunkHeaderBuffer)

	reset := func() {
		self.reader = io.MultiReader(chunkHeaderBuffer, self.reader)
	}

	var chunkDataLength uint32
	if err = binary.Read(teedReader, binary.BigEndian, &chunkDataLength); err != nil {
		reset()
		return
	}

	chunkType := make([]byte, 4)
	if _, err = io.ReadFull(teedReader, chunkType); err != nil {
		reset()
		return
	}

	// 4 byte chunkType Header + 4 byte chunkDataLength Header + chunkDataLength + 4 byte CRC length
	chunkLength := int64(12 + chunkDataLength)

	chunkReader := io.MultiReader(chunkHeaderBuffer, self.reader)

	if isMetadataType(chunkType) {
		_, err = io.CopyN(ioutil.Discard, chunkReader, chunkLength)
		// TODO: this is actually a more nuanced situation than I had thought
		//       we need to be guaranteed that this will advance the reader to the next chunk
	} else {
		self.chunkData = io.LimitedReader{
			R: chunkReader,
			N: chunkLength,
		}

		n2, err = self.chunkData.Read(buf[n:])
		n += n2
	}
	return
}
