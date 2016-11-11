package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
)

func verify(args *verifyArgs, showProgress bool) int {
	_, _, firstHash, err := readSegmentLine(createCsvReader(args.segmentHashesInput))
	checkErr(err)

	progress, finishProgress := getProgress(showProgress, args.input)

	hcontainer := getHashContainerByHash(firstHash)
	args.segmentHashesInput.Seek(0, 0)

	readRanges, fileSegments := readHashesFromFile(args.segmentHashesInput, fileSize(args.input))
	segmentChunks := readFile(args.input, bufferSize, 1, readRanges, progress)

	calculatedSegments := calculateHash(hcontainer, segmentChunks[0])

	diffs, diffsFname, errors := verifySegments(args.createOutputFile, calculatedSegments, fileSegments)

	finishStr := fmt.Sprintf("Segment hashes verified. \nInput data file: %s. Input hashes file: %s. \nNumber of different segments: %d. ",
		args.input.Name(), args.segmentHashesInput.Name(), diffs)
	if diffs > 0 {
		finishStr += fmt.Sprintf("Different segments written to %s.", diffsFname)
	}
	if errors > 0 {
		finishStr += fmt.Sprintf("\nErrors during verify: %d.", errors)
	}
	finishProgress(finishStr)
	return diffs
}

func readHashesFromFile(hashes io.Reader, dataSize int64) (<-chan readRange, <-chan segment) {
	rangeChan := make(chan readRange)
	segmentChan := make(chan segment)

	_, dataEndSector := bytesToSectors(0, dataSize)

	csvReader := createCsvReader(hashes)
	go func() {
		defer close(rangeChan)
		defer close(segmentChan)

		for line := 1; ; line++ {
			startLba, endLba, hash, err := readSegmentLine(csvReader)
			if err == io.EOF {
				break
			}
			if endLba > dataEndSector {
				err = fmt.Errorf("segment with range (%d, %d) exceeds input file range", startLba, endLba)
			}
			start, length := sectorToBytes(startLba, endLba)
			segment := segment{start: start, length: length, hash: hash}

			if err != nil {
				segment.err = fmt.Errorf("Error in line %d: %s", line, err.Error())
				segmentChan <- segment
				continue
			}

			rangeChan <- readRange{start: segment.start, length: segment.length}
			segmentChan <- segment
		}
	}()

	return rangeChan, segmentChan
}

func verifySegments(createOutput func() outputFile, calculatorChan, fileChan <-chan segment) (diffs int, diffsFname string, errors int) {
	var csvWriter *csv.Writer
	for fileSegment := range fileChan {
		if fileSegment.err != nil {
			if csvWriter == nil {
				outFile := createOutput()
				defer outFile.Close()
				diffsFname = outFile.Name()
				csvWriter = createCsvWriter(outFile)
			}
			errors++
			writeErrorLine(csvWriter, fileSegment.err.Error())
			continue
		}

		calculatedSegment := <-calculatorChan

		if fileSegment.start != calculatedSegment.start {
			lnfatalf("Internal error: fileSegment start(%d) is different from calculatedSegment start(%d)", fileSegment.start, calculatedSegment.start)
		}

		if !bytes.Equal(fileSegment.hash, calculatedSegment.hash) {
			if csvWriter == nil {
				outFile := createOutput()
				defer outFile.Close()
				diffsFname = outFile.Name()
				csvWriter = createCsvWriter(outFile)
			}
			diffs++
			startLba, endLba := bytesToSectors(fileSegment.start, fileSegment.length)
			writeDiffLine(csvWriter, startLba, endLba)
		}
	}

	return
}
