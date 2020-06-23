// jpeg handles scrubbing jpeg files of metadata
package jpeg

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"

	jpegimagestructure "github.com/dsoprea/go-jpeg-image-structure"
)

var headerBytes []byte = []byte{0xff, 0xd8}

type JpegScrubber struct {
	reader      io.Reader
	segmentData io.LimitedReader
	wroteHeader bool
}

func NewJpegScrubber(reader io.Reader) *JpegScrubber {
	return &JpegScrubber{
		reader: reader,
	}
}

func isMetadataType(segmentMarker []byte) bool {
	// TODO: consider other APP types
	return segmentMarker[0] == 0xff &&
		(segmentMarker[1] == jpegimagestructure.MARKER_APP1 || segmentMarker[1] == jpegimagestructure.MARKER_APP0)
}

func (self *JpegScrubber) Read(buf []byte) (n int, err error) {
	var n2 int

	if !self.wroteHeader {
		n, err = io.ReadFull(self.reader, buf[:len(headerBytes)])
		if err != nil {
			return
		}
		self.wroteHeader = true
	}

	n2, err = self.segmentData.Read(buf[n:])
	n += n2
	if err != io.EOF {
		return
	}

	segmentHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(self.reader, segmentHeaderBuffer)

	reset := func() {
		self.reader = io.MultiReader(segmentHeaderBuffer, self.reader)
	}

	segmentMarker := make([]byte, 2)
	if _, err = io.ReadFull(teedReader, segmentMarker); err != nil {
		reset()
		return
	}

	// TODO: apparently there are some number of static-length markers????
	// https://github.com/dsoprea/go-jpeg-image-structure/blob/d40a386309d24fb714c60dcf1f5f88bff3ad9237/splitter.go#L219
	var segmentDataLength uint16
	if err = binary.Read(teedReader, binary.BigEndian, &segmentDataLength); err != nil {
		reset()
		return
	}

	// 2 byte segmentDataLength + segmentDataLength (doesn't count marker bytes)
	segmentLength := int64(2 + segmentDataLength)

	segmentReader := io.MultiReader(segmentHeaderBuffer, self.reader)

	if isMetadataType(segmentMarker) {
		if _, err = io.CopyN(ioutil.Discard, segmentReader, segmentLength); err != nil {
			// TODO: some kind of reset here?
			return
		}
	} else {
		self.segmentData = io.LimitedReader{
			R: segmentReader,
			N: segmentLength,
		}

		n2, err = self.segmentData.Read(buf[n:])
		n += n2
	}
	return
}
