package metascrubber

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	jpegimagestructure "github.com/dsoprea/go-jpeg-image-structure"
)

var headerBytes []byte = []byte{0xff, 0xd8}

type jpegSegmentReader struct {
	reader      io.Reader
	wroteHeader bool
}

func (jr *jpegSegmentReader) isMetadataType(segmentMarker []byte) bool {
	// TODO: consider other APP types
	return segmentMarker[0] == 0xff &&
		(segmentMarker[1] == jpegimagestructure.MARKER_APP1 || segmentMarker[1] == jpegimagestructure.MARKER_APP0)
}

func (jr *jpegSegmentReader) nextSegment() (r io.Reader, isMetadata bool, err error) {
	if !jr.wroteHeader {
		r = io.LimitReader(jr.reader, int64(len(headerBytes)))
		jr.wroteHeader = true
		return
	}

	segmentHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(jr.reader, segmentHeaderBuffer)

	segmentMarker := make([]byte, 2)
	if _, err = io.ReadFull(teedReader, segmentMarker); err != nil {
		if !errors.Is(err, io.EOF) {
			err = &MalformedDataError{"can't parse jpeg segment marker", err}
		}
		return
	}

	// TODO: apparently there are some number of static-length markers????
	// https://github.com/dsoprea/go-jpeg-image-structure/blob/d40a386309d24fb714c60dcf1f5f88bff3ad9237/splitter.go#L219
	var segmentDataLength uint16
	if err = binary.Read(teedReader, binary.BigEndian, &segmentDataLength); err != nil {
		if !errors.Is(err, io.EOF) {
			err = &MalformedDataError{"can't parse jpeg data length", err}
		}
		return
	}

	// 2 byte segmentDataLength + segmentDataLength (doesn't count marker bytes)
	segmentLength := int64(2 + segmentDataLength)

	r = io.LimitReader(io.MultiReader(segmentHeaderBuffer, jr.reader), segmentLength)

	isMetadata = jr.isMetadataType(segmentMarker)
	return
}
