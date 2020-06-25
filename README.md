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
metascrubber uses [exif-samples](https://github.com/ianare/exif-samples) as a image test corpus.
In order to run tests on that corpus, you'll need to initialize the submodule:
```
$ git clone https://github.com/getlantern/meta-scrubber.git
$ cd meta-scrubber
$ git submodule update --init
$ go test ./...
```
