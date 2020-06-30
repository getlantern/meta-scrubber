# meta-scrubber
[![Documentation](https://godoc.org/github.com/getlantern/meta-scrubber?status.svg)](http://pkg.go.dev/github.com/getlantern/meta-scrubber?tab=doc)
[![CircleCI](https://circleci.com/gh/getlantern/meta-scrubber.svg?style=svg)](https://circleci.com/gh/getlantern/meta-scrubber)

meta-scrubber provides a streaming metadata remover

It is a WORK IN PROGRESS and currently provides ZERO guarantees and VERY limited file format support.

## cli usage
```
$ go build ./cmd/meta-scrubber
$ ./meta-scrubber input-file.png output-file.png
```

## development
metascrubber will run tests on any `.jpg` or `.png` images in `testdata`.
There's a large (currently ~1GB) corpus for testing at https://meta-scrubber-test-corpus.s3.us-west-1.amazonaws.com/exif-image-corpus.tar.gz
To download and test:
```
$ git clone https://github.com/getlantern/meta-scrubber.git
$ cd meta-scrubber
$ curl https://meta-scrubber-test-corpus.s3.us-west-1.amazonaws.com/exif-image-corpus.tar.gz | tar -C ./testdata/exif-image-corpus -xzv
$ go test . -v
```
