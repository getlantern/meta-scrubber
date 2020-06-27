package metascrubber

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

var headerBytes []byte = []byte{0xff, 0xd8}

// scanReader reads until the scan terminating bytes are encountered
type scanReader struct {
	reader *io.Reader
}

func firstMarker(p []byte) int {
	markerIndex := -1
	// Look for an 0xff followed by anything other than 0x00 or [0xd0-0xd7] (those are RST markers)
	for i := 0x01; i <= 0xff; i++ {
		if i >= 0xd0 && i <= 0xd7 {
			continue
		}
		markerIndex = bytes.Index(p, []byte{0xff, byte(i)})
		if markerIndex >= 0 {
			return markerIndex
		}
	}
	return markerIndex
}

func (sr *scanReader) Read(p []byte) (n int, err error) {
	buf := new(bytes.Buffer)
	teedReader := io.TeeReader(*sr.reader, buf)
	if n, err = teedReader.Read(p); err != nil {
		if errors.Is(err, io.EOF) {
			return
		}
		err = &MalformedDataError{"unexpected error reading inside scan data", err}
		return
	}

	// TODO:
	// - what happens when a 2 byte marker overlaps the end of the buffer???

	markerIndex := firstMarker(p[:n])

	if markerIndex >= 0 {
		n = markerIndex
		if _, err = io.CopyN(ioutil.Discard, buf, int64(markerIndex)); err != nil {
			return
			// TODO: error handling??
		}
		*sr.reader = io.MultiReader(buf, *sr.reader)
		if markerIndex == 0 {
			err = io.EOF
		}
		return
	}

	return
}

type jpegSegmentReader struct {
	reader     io.Reader
	insideScan bool
	endOfImage bool
}

func (jr *jpegSegmentReader) isMetadataType(segmentMarker []byte) bool {
	// APP segments are of form 0xff, 0xe[0-f]
	// more information about them at https://exiftool.org/TagNames/JPEG.html
	// APP14 and APP15 *seem* to be required to actually decode image data
	// COM segments are of form 0xff, 0xfe
	return segmentMarker[0] == 0xff &&
		((segmentMarker[1] >= 0xe0 && segmentMarker[1] <= 0xed) || segmentMarker[1] == 0xfe)
}

func (jr *jpegSegmentReader) nextSegment() (r io.Reader, isMetadata bool, err error) {
	if jr.endOfImage {
		return nil, false, io.EOF
	}

	if jr.insideScan {
		jr.insideScan = false
		return &scanReader{&jr.reader}, false, nil
	}

	segmentHeaderBuffer := new(bytes.Buffer)
	teedReader := io.TeeReader(jr.reader, segmentHeaderBuffer)

	segmentMarker := make([]byte, 2)
	if _, err = io.ReadFull(teedReader, segmentMarker); err != nil {
		if !errors.Is(err, io.EOF) {
			err = &MalformedDataError{"can't parse jpeg segment marker", err}
			return
		}
	}

	// segment marker logic from:
	// https://dev.exiv2.org/projects/exiv2/wiki/The_Metadata_in_JPEG_files
	var segmentLength int64

	if bytes.Equal(segmentMarker, []byte{0xff, 0xd8}) {
		// SOI (start of image) marker
		segmentLength = 2
	} else if bytes.Equal(segmentMarker, []byte{0xff, 0xd9}) {
		// EOI (end of image) marker
		segmentLength = 2
		jr.endOfImage = true
	} else if segmentMarker[0] == 0xff && segmentMarker[1] >= 0xd0 && segmentMarker[1] <= 0xd7 {
		// RST marker
		segmentLength = 2
	} else {
		// TODO: double check potentially static-length markers????
		// https://github.com/dsoprea/go-jpeg-image-structure/blob/d40a386309d24fb714c60dcf1f5f88bff3ad9237/splitter.go#L219
		var segmentDataLength uint16
		if err = binary.Read(teedReader, binary.BigEndian, &segmentDataLength); err != nil {
			if !errors.Is(err, io.EOF) {
				err = &MalformedDataError{"can't parse jpeg data length", err}
			}
			return
		}

		if bytes.Equal(segmentMarker, []byte{0xff, 0xda}) {
			// SOS (start of scan) marker
			jr.insideScan = true
		}

		// 2 byte segmentDataLength + segmentDataLength (doesn't count marker bytes)
		segmentLength = int64(2 + segmentDataLength)
	}

	r = io.LimitReader(io.MultiReader(segmentHeaderBuffer, jr.reader), segmentLength)

	isMetadata = jr.isMetadataType(segmentMarker)
	return
}
