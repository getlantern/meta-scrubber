// Package metascrubber provides a streaming metadata remover
package metascrubber

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

// GetScrubber returns a reader which has metadata removed from its contents
func GetScrubber(reader io.Reader) (io.Reader, error) {
	// net/http.sniffLen = 512
	contentTypeHead := make([]byte, 512)
	n, err := io.ReadFull(reader, contentTypeHead)

	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return bytes.NewReader(contentTypeHead[:n]), nil
		}
		return nil, err
	}

	mimeType := http.DetectContentType(contentTypeHead)

	// Reset the reader to contain the whole stream again
	reader = io.MultiReader(bytes.NewReader(contentTypeHead), reader)

	if mimeType == "image/png" {
		return &metaScrubber{
			sr: &pngSegmentReader{
				reader: reader,
			},
			segmentData: bytes.NewReader([]byte{}),
		}, nil
	} else if mimeType == "image/jpeg" {
		return &metaScrubber{
			sr: &jpegSegmentReader{
				reader: reader,
			},
			segmentData: bytes.NewReader([]byte{}),
		}, nil
	}
	return reader, nil
}

// MalformedDataError represents an error parsing the underlying file format
// This could be an upstream reader issue, so consider checking it if needed
type MalformedDataError struct {
	Message string
	Err     error
}

func (e *MalformedDataError) Unwrap() error { return e.Err }
func (e *MalformedDataError) Error() string { return e.Message + ": " + e.Err.Error() }

type segmentReader interface {
	// Returns io.EOF when no more segments exist.
	// If isMetadata, r will be an empty reader
	// The returned reader MUST be exhausted/read until EOF for further calls to nextSegment to be valid
	nextSegment() (r io.Reader, isMetadata bool, err error)
}

type metaScrubber struct {
	sr          segmentReader
	segmentData io.Reader
}

func (ms *metaScrubber) Read(p []byte) (n int, err error) {
	var (
		m      int
		isMeta bool
	)

	n, err = ms.segmentData.Read(p)
	if (err != nil && !errors.Is(err, io.EOF)) || n >= len(p) {
		return
	}

	for ms.segmentData, isMeta, err = ms.sr.nextSegment(); err == nil; ms.segmentData, isMeta, err = ms.sr.nextSegment() {
		if isMeta {
			continue
		}
		// Need to keep calling read until EOF, n >= len(p), or other error
		for err == nil {
			m, err = ms.segmentData.Read(p[n:])
			n += m
			if (err != nil && !errors.Is(err, io.EOF)) || n >= len(p) {
				return
			}
		}
	}
	return
}
