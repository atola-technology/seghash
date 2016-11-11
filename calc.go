package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

func calc(args *calcArgs, showProgress bool) []string {
	if len(args.hashNames) > 2 {
		fatal("cannot calculate more than two hashes simultaneously")
	}
	hashContainers := getHashContainersByNames(args.hashNames)

	progress, finishProgress := getProgress(showProgress, args.input)

	readRanges := produceReadRanges(args.segmentSize, fileSize(args.input))
	segmentChunks := readFile(args.input, bufferSize, len(hashContainers), readRanges, progress)

	outputFilenames := make([]string, len(args.hashNames))
	wg := sync.WaitGroup{}
	wg.Add(len(hashContainers))
	for i, segmentChunk := range segmentChunks {
		calculatedHashes := calculateHash(hashContainers[i], segmentChunk)
		out := args.createOutputFile(fmt.Sprintf("%s.csv", hashContainers[i].name))
		writeFile(out, calculatedHashes, &wg)
		outputFilenames[i] = out.Name()
	}
	wg.Wait()
	finishProgress(fmt.Sprintf("Segment hashes calculated. \nInput file: %s. Output file(s): %s", args.input.Name(), strings.Join(outputFilenames, ", ")))
	return outputFilenames
}

func produceReadRanges(segmentSize, fileSize int64) <-chan readRange {
	out := make(chan readRange)
	go func() {
		defer close(out)

		for producedBytes := int64(0); producedBytes < fileSize; producedBytes += segmentSize {
			if fileSize - producedBytes < segmentSize {
				out <- readRange{start: producedBytes, length: fileSize - producedBytes}
			} else {
				out <- readRange{start: producedBytes, length: segmentSize}
			}
		}
	}()

	return out
}

func writeFile(output io.Writer, in <-chan segment, wg *sync.WaitGroup) {
	csvWriter := createCsvWriter(output)
	go func() {
		defer wg.Done()

		for segment := range in {
			writeSegmentLine(csvWriter, segment)
		}
	}()
}
