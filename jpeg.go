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
	// kick the underlying reader into giving us more than just one byte because the
	// the naive multireader will just give us one byte from the first reader in its arguments
	if len(p) > 1 {
		if n, err = io.ReadAtLeast(teedReader, p, 2); err != nil {
			if errors.Is(err, io.EOF) {
				return
			} else if errors.Is(err, io.ErrUnexpectedEOF) {
				// safe to just deal with on the next pass through
				err = nil
			} else {
				err = &MalformedDataError{"unexpected error reading inside scan data", err}
				return
			}
		}
	} else {
		// if len(p) is only 1, we can still attempt to read because we'll still
		// peak the next byte and can act accordingly
		if n, err = teedReader.Read(p); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			err = &MalformedDataError{"unexpected error reading inside scan data", err}
			return
		}
	}

	// attempt to peek one more byte to ensure that the marker doesn't overlap
	// the bytes returned
	lastByte := make([]byte, 1)
	var lastByteN int
	if lastByteN, err = io.ReadFull(teedReader, lastByte); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// an EOF is ok here because we'll catch it normally on the next pass through
			err = nil
		} else {
			err = &MalformedDataError{"unexpected error reading inside scan data", err}
			return
		}
	}

	var markerIndex int
	if lastByteN == 1 {
		markerIndex = firstMarker(append(p[:n], lastByte[0]))
	} else {
		markerIndex = firstMarker(p[:n])
	}

	if markerIndex >= 0 {
		n = markerIndex
		if _, err = io.CopyN(ioutil.Discard, buf, int64(markerIndex)); err != nil {
			err = &MalformedDataError{"unexpected error reading inside scan data", err}
			return
		}

		// pop "unread" bytes back onto reader
		*sr.reader = io.MultiReader(buf, *sr.reader)
		if markerIndex == 0 {
			err = io.EOF
		}
		return
	}

	if lastByteN == 1 {
		// pop the last byte back onto the reader
		*sr.reader = io.MultiReader(bytes.NewReader(lastByte), *sr.reader)
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
	// https://github.com/dsoprea/go-jpeg-image-structure/blob/d40a386309d24fb714c60dcf1f5f88bff3ad9237/splitter.go#L219
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
		var segmentDataLength uint16
		if err = binary.Read(teedReader, binary.BigEndian, &segmentDataLength); err != nil {
			if !errors.Is(err, io.EOF) {
				err = &MalformedDataError{"can't parse jpeg data length", err}
			}
			return
		}

		if bytes.Equal(segmentMarker, []byte{0xff, 0xda}) {
			// SOS (start of scan) marker
			// https://stackoverflow.com/questions/26715684/parsing-jpeg-sos-marker
			jr.insideScan = true
		}

		// 2 byte segmentDataLength + segmentDataLength (doesn't count marker bytes)
		segmentLength = int64(2 + segmentDataLength)
	}

	r = io.LimitReader(io.MultiReader(segmentHeaderBuffer, jr.reader), segmentLength)

	isMetadata = jr.isMetadataType(segmentMarker)
	return
}
