package metascrubber

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	corpusURL = "https://meta-scrubber-test-corpus.s3.us-west-1.amazonaws.com/exif-image-corpus.tar.gz"

	// Permissions for the corpus directory itself, as well as permissions for directories within
	// the corpus, before umask.
	corpusDirPerm = 0777
)

var (
	// These paths should agree with those specified in the CircleCI config.
	corpusETagFile = filepath.Join("testdata", "corpus.etag")
	corpusDir      = filepath.Join("testdata", "corpus")
)

func init() {
	testing.Init()
	flag.Parse()
	if testing.Short() {
		return
	}
	if err := updateCorpus(os.Stderr); err != nil {
		panic(fmt.Sprintf("failed to update test corpus: %v", err))
	}
}

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

func TestPngCorpusValidity(t *testing.T) {
	checkImageValidity(t, "png")
}

func TestJpgCorpusValidity(t *testing.T) {
	checkImageValidity(t, "jpg")
}

func checkImageValidity(t *testing.T, imageType string) {
	var files []string
	var err error

	root := "testdata"
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == corpusDir && testing.Short() {
			return filepath.SkipDir
		}
		ext := filepath.Ext(path)
		if imageType == "png" && ext == ".png" {
			files = append(files, path)
		} else if imageType == "jpg" && (ext == ".jpg" || ext == ".jpeg") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	for _, file := range files {
		inputImage, err := os.Open(file)
		require.NoError(t, err)
		_, _, err = image.Decode(inputImage)
		require.NoErrorf(t, err, "could not decode %s before scrubbing, image is probably bad test file", file)
		inputImage.Close()
		t.Logf("decoding %v", file)
		inputImage, err = os.Open(file)
		require.NoError(t, err)
		scrubberReader, err := GetScrubber(inputImage)
		require.NoError(t, err)
		_, _, err = image.Decode(scrubberReader)
		assert.NoErrorf(t, err, "could not decode %s after scrubbing", file)
		inputImage.Close()
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
	assert.Equalf(t, expectedImageBytes, actuallyRead, "expected %v to match %v after scrubbing", originalFilename, expectedFilename)
}

func TestScrubbing(t *testing.T) {
	images := []struct {
		originalFile string
		expectedFile string
	}{
		{"kitten-without-meta.png", "kitten-without-meta.png"},
		{"kitten-with-exif-description.png", "kitten-without-meta.png"},
		{"kitten-with-text-author.png", "kitten-without-meta.png"},
		{"kitten-with-xmp-manufacturer.png", "kitten-without-meta.png"},
		{"kitten-without-meta.jpeg", "kitten-without-meta.jpeg"},
		{"kitten-with-xmp-description.jpeg", "kitten-without-meta.jpeg"},
		{"kitten-with-exif-make.jpeg", "kitten-without-meta.jpeg"},
		{"kitten-with-exif-make.jpeg", "kitten-without-meta.jpeg"},
	}

	for _, test := range images {
		compareImages(t, test.originalFile, test.expectedFile)
	}
}

func updateCorpus(logger io.Writer) error {
	currentETag, err := ioutil.ReadFile(corpusETagFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read current ETag file: %w", err)
	}
	headResp, err := http.Head(corpusURL)
	if err != nil {
		return fmt.Errorf("HEAD failed: %w", err)
	}
	defer headResp.Body.Close()
	if headResp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response to HEAD request: %s", headResp.Status)
	}
	etag := headResp.Header.Get("ETag")
	if etag == "" {
		return errors.New("no ETAG in HEAD response")
	}
	if etag == strings.TrimSpace(string(currentETag)) {
		return nil
	}

	fmt.Fprintln(logger, "ETag changed; downloading new test corpus")
	fmt.Fprintf(logger, "Current ETag: < %s >\n", string(currentETag))
	fmt.Fprintf(logger, "New ETag: < %s >\n", etag)
	getResp, err := http.Get(corpusURL)
	if err != nil {
		return fmt.Errorf("GET failed: %w", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response to GET request: %s", getResp.Status)
	}
	gzr, err := gzip.NewReader(getResp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for response body: %w", err)
	}

	if err := os.RemoveAll(corpusDir); err != nil {
		return fmt.Errorf("failed to remove old corpus: %w", err)
	}
	if err := os.Mkdir(corpusDir, corpusDirPerm); err != nil {
		return fmt.Errorf("failed to create new corpus directory: %w", err)
	}

	var (
		tr  = tar.NewReader(gzr)
		hdr *tar.Header
	)
	for hdr, err = tr.Next(); err == nil; hdr, err = tr.Next() {
		switch hdr.Typeflag {
		case tar.TypeReg:
			err = func() error {
				f, err := os.Create(filepath.Join(corpusDir, hdr.Name))
				if err != nil {
					return fmt.Errorf("failed to create file: %w", err)
				}
				defer f.Close()
				if _, err := io.Copy(f, tr); err != nil {
					return fmt.Errorf("failed to write file: %w", err)
				}
				return nil
			}()
			if err != nil {
				return fmt.Errorf("failed to extract %s: %w", hdr.Name, err)
			}
		case tar.TypeDir:
			if err := os.Mkdir(filepath.Join(corpusDir, hdr.Name), corpusDirPerm); err != nil {
				return fmt.Errorf("failed to extract directory %s: mkdir failed: %w", hdr.Name, err)
			}
		default:
			fmt.Fprintf(logger, "Skipping %s: unsupported file type %d", hdr.Name, hdr.Typeflag)
		}
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("error while extracting tar archive %w", err)
	}
	fmt.Fprintln(logger, "Successfully updated test corpus")
	if err := ioutil.WriteFile(corpusETagFile, []byte(etag), 0644); err != nil {
		fmt.Fprintln(logger, "Failed to update corpus ETag file: %w", err)
	}
	return nil
}
