// Scrubber provides a streaming metadata remover
package metascrubber

import (
	"bytes"
	"io"
	"net/http"

	"github.com/getlantern/meta-scrubber/jpeg"
	"github.com/getlantern/meta-scrubber/png"
)

func GetScrubber(reader io.Reader) (io.Reader, error) {
	// net/http.sniffLen = 512
	contentTypeHead := make([]byte, 512)
	n, err := io.ReadFull(reader, contentTypeHead)

	if err != nil {
		if err == io.EOF {
			// TODO: what's the best way to return an empty reader
			return bytes.NewReader([]byte{}), nil
		} else if err == io.ErrUnexpectedEOF {
			return bytes.NewReader(contentTypeHead[:n]), nil
		} else {
			return nil, err
		}
	}

	mimeType := http.DetectContentType(contentTypeHead)

	// Reset the reader to contain the whole stream again
	reader = io.MultiReader(bytes.NewReader(contentTypeHead), reader)

	if mimeType == "image/png" {
		return png.NewPngScrubber(reader), nil
	} else if mimeType == "image/jpeg" {
		return jpeg.NewJpegScrubber(reader), nil
	} else {
		return reader, nil
	}
}
