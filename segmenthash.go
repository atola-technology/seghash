package main

import (
	"hash"
	"io"
	"os"
)

const (
	version    = "0.1.10"
	md5Name    = "md5"
	sha1Name   = "sha1"
	sha224Name = "sha224"
	sha256Name = "sha256"
	sha384Name = "sha384"
	sha512Name = "sha512"
	bufferSize = 2 * 1024 * 1024
)

func finalizeSegment(h hash.Hash, seg segment, out chan<- segment) segment {
	seg.hash = h.Sum(nil)
	out <- seg
	h.Reset()
	return segment{}
}

func calculateHash(hc hashContainer, in <-chan segmentChunk) <-chan segment {
	out := make(chan segment)

	go func() {
		defer close(out)
		var currentSegment segment
		for chunk := range in {
			_, err := hc.h.Write(chunk.data)
			lnCheckErr(err)

			// Setting rigth start offset for new segment
			if currentSegment.length == 0 {
				currentSegment.start = chunk.baseSegmentStart
			}

			currentSegment.length += int64(len(chunk.data))

			if chunk.isLast {
				currentSegment = finalizeSegment(hc.h, currentSegment, out)
			}
		}
		if currentSegment.length > 0 {
			finalizeSegment(hc.h, currentSegment, out)
		}
	}()

	return out
}

func readFile(input io.ReadSeeker, bufSize int64, consumersCount int, in <-chan readRange, progress func(n int64)) []chan segmentChunk {
	buffers := make([][]byte, 2)
	buffers[0] = make([]byte, bufSize)
	buffers[1] = make([]byte, bufSize)
	curBuffer := 0

	out := make([]chan segmentChunk, consumersCount)
	for i := range out {
		out[i] = make(chan segmentChunk)
	}

	go func() {
		defer func() {
			for i := range out {
				close(out[i])
			}
		}()

		var err error
		currentReadOffset := int64(0)
		for readRange := range in {
			if readRange.start != currentReadOffset {
				_, err = input.Seek(readRange.start, 0)
				lnCheckErr(err)
			}

			for left := readRange.length; left > 0; {
				bufferToRead := buffers[curBuffer]
				if left < bufSize {
					bufferToRead = buffers[curBuffer][:left]
				}
				n, err := input.Read(bufferToRead)
				if n == 0 {
					break
				}

				// Check whether last part was read
				if n < len(bufferToRead) {
					left = int64(n)
				}

				left -= int64(n)
				progress(int64(n))

				for i := range out {
					out[i] <- segmentChunk{data: bufferToRead[:n], isLast: left <= 0, baseSegmentStart: readRange.start}
				}

				// Switch buffer
				curBuffer = curBuffer ^ 1

				if err != nil {
					break
				}
			}
			if err != nil && err != io.EOF {
				lnfatal(err)
			}

			currentReadOffset = readRange.start + readRange.length
		}
	}()

	return out
}

func main() {
	calcArgs, verifyArgs := parseArgs()
	defer finalizeArgs(calcArgs, verifyArgs)

	if calcArgs != nil {
		calc(calcArgs, true)
	} else if verifyArgs != nil {
		diffs := verify(verifyArgs, true)
		if diffs > 254 {
			diffs = 254
		}
		os.Exit(diffs)
	} else {
		fatal("invalid command arguments")
	}
}
