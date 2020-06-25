package metascrubber

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadBytes(t *testing.T, name string) []byte {
	path := filepath.Join("testdata", name)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func TestScrubbingEmptyReader(t *testing.T) {
	emptyReader := bytes.NewReader([]byte{})
	emptyScrubber, err := GetScrubber(emptyReader)
	require.NoError(t, err)
	actuallyRead, err := ioutil.ReadAll(emptyScrubber)
	require.NoError(t, err)
	assert.Empty(t, actuallyRead)
}

func TestScrubbingShortReader(t *testing.T) {
	shortBytes := []byte{0xbe, 0xef, 0xfe, 0xed}
	shortReader := bytes.NewReader(shortBytes)
	shortScrubber, err := GetScrubber(shortReader)
	require.NoError(t, err)
	actuallyRead, err := ioutil.ReadAll(shortScrubber)
	require.NoError(t, err)
	assert.Equal(t, shortBytes, actuallyRead)
}

func TestImageValidity(t *testing.T) {
	var files []string
	var err error

	root := "testdata"
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		if ext == ".jpeg" || ext == ".jpg" || ext == ".png" {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	for _, file := range files {
		t.Logf("decoding %v", file)
		inputImage, err := os.Open(file)
		require.NoError(t, err)
		defer inputImage.Close()
		_, _, err = image.Decode(inputImage)
		if err == nil {
			inputImage, err := os.Open(file)
			defer inputImage.Close()
			require.NoError(t, err)
			scrubberReader, err := GetScrubber(inputImage)
			require.NoError(t, err)
			_, _, err = image.Decode(scrubberReader)
			require.NoError(t, err)
		}
	}
}

func compareImages(t *testing.T, originalFilename string, expectedFilename string) {
	originalImageBytes := loadBytes(t, originalFilename)
	expectedImageBytes := loadBytes(t, expectedFilename)

	imageReader := bytes.NewReader(originalImageBytes)
	imageScrubber, err := GetScrubber(imageReader)
	require.NoError(t, err)
	actuallyRead, err := ioutil.ReadAll(imageScrubber)
	require.NoError(t, err)
	assert.Equal(t, expectedImageBytes, actuallyRead)
}

func TestScrubbingPngWithoutExif(t *testing.T) {
	compareImages(t, "kitten-without-meta.png", "kitten-without-meta.png")
}

func TestScrubbingPngWithExif(t *testing.T) {
	compareImages(t, "kitten-with-exif-description.png", "kitten-without-meta.png")
}

func TestScrubbingPngWithTextAuthor(t *testing.T) {
	compareImages(t, "kitten-with-text-author.png", "kitten-without-meta.png")
}

func TestScrubbingPngWithXMPManufacturer(t *testing.T) {
	compareImages(t, "kitten-with-xmp-manufacturer.png", "kitten-without-meta.png")
}

func TestScrubbingJpegWithoutExif(t *testing.T) {
	compareImages(t, "kitten-without-meta.jpeg", "kitten-without-meta.jpeg")
}

func TestScrubbingJpegWithXmpDescription(t *testing.T) {
	compareImages(t, "kitten-with-xmp-description.jpeg", "kitten-without-meta.jpeg")
}

func TestScrubbingJpegWithExifMake(t *testing.T) {
	compareImages(t, "kitten-with-exif-make.jpeg", "kitten-without-meta.jpeg")
}
