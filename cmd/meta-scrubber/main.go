// Command meta-scrubber is a CLI for scrubbing metadata from files
package main

import (
	"io"
	"log"
	"os"

	metascrubber "github.com/getlantern/meta-scrubber"
	cli "github.com/jawher/mow.cli"
)

func main() {
	app := cli.App("meta-scrubber", "Remove metadata from files")

	filenameInput := app.StringArg("FILE_INPUT", "", "File to remove metadata from")
	filenameOutput := app.StringArg("FILE_OUTPUT", "", "Location to write new contents")

	app.Action = func() {
		log.Printf("Removing metadata from %v\n", *filenameInput)
		inputFile, err := os.Open(*filenameInput)
		if err != nil {
			log.Fatalf("error opening file for reading: %v", err)
		}
		outputFile, err := os.Create(*filenameOutput)
		if err != nil {
			log.Fatalf("error opening file for writing: %v", err)
		}
		scrubberReader, err := metascrubber.GetScrubber(inputFile)
		if err != nil {
			log.Fatalf("error getting scrubber : %v", err)
		}

		if n, err := io.Copy(outputFile, scrubberReader); err != nil {
			log.Fatalf("error writing scrubbed : %v", err)
		} else {
			log.Printf("wrote %d bytes to %v", n, *filenameOutput)
		}
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("error removing metadata: %v", err)
	}
}
