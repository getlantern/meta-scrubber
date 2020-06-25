package metascrubber

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var headerBytes []byte = []byte{0xff, 0xd8}

type jpegSegmentReader struct {
	reader io.Reader
}

func (jr *jpegSegmentReader) isMetadataType(segmentMarker []byte) bool {
	// APP segments are of form 0xff, 0xe[0-f]
	// COM segments are of form 0xff, 0xfe
	return segmentMarker[0] == 0xff &&
		((segmentMarker[1] >= 0xe0 && segmentMarker[1] <= 0xef) || segmentMarker[1] == 0xfe)
}

func (jr *jpegSegmentReader) nextSegment() (r io.Reader, isMetadata bool, err error) {
	segmentHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(jr.reader, segmentHeaderBuffer)

	segmentMarker := make([]byte, 2)
	if _, err = io.ReadFull(teedReader, segmentMarker); err != nil {
		if !errors.Is(err, io.EOF) {
			err = &MalformedDataError{"can't parse jpeg segment marker", err}
		}
		return
	}

	// segment marker logic from:
	// https://dev.exiv2.org/projects/exiv2/wiki/The_Metadata_in_JPEG_files
	var segmentLength int64

	if bytes.Equal(segmentMarker, []byte{0xff, 0xd8}) || bytes.Equal(segmentMarker, []byte{0xff, 0xd9}) {
		// SOI (start of image) and EOI (end of image) markers respectively
		segmentLength = 2
	} else if segmentMarker[0] == 0xff && segmentMarker[1] >= 0xd0 && segmentMarker[1] <= 0xd7 {
		// RST marker
		segmentLength = 2
	} else if bytes.Equal(segmentMarker, []byte{0xff, 0xdd}) {
		// DRI marker
		segmentLength = 4
	} else {
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
		segmentLength = int64(2 + segmentDataLength)
	}

	r = io.LimitReader(io.MultiReader(segmentHeaderBuffer, jr.reader), segmentLength)

	isMetadata = jr.isMetadataType(segmentMarker)
	return
}
