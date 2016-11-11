package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/atola-technology/seghash/external/github.com/cheggaaa/pb"
)

const csvDelimiter = ','

type hashContainer struct {
	h    hash.Hash
	name string
}

type readRange struct {
	start  int64
	length int64
}

type segmentChunk struct {
	data             []byte
	isLast           bool
	baseSegmentStart int64
}

type segment struct {
	start  int64
	length int64
	hash   []byte
	err    error
}

func (seg segment) ToStringSlice() []string {
	startLba, endLba := bytesToSectors(seg.start, seg.length)
	values := make([]string, 3)
	values[0] = fmt.Sprintf("%x", seg.hash)
	values[1] = fmt.Sprintf("%d", startLba)
	values[2] = fmt.Sprintf("%d", endLba)
	return values
}

func (seg segment) ToCsvString(delimiter rune) string {
	return strings.Join(seg.ToStringSlice(), string(delimiter))
}

type inputFile interface {
	io.Reader
	io.Seeker
	io.Closer
	Name() string
	Stat() (os.FileInfo, error)
}

type outputFile interface {
	io.Writer
	io.Closer
	Name() string
}

func fatal(a interface{}) {
	fmt.Print(os.Args[0], ": error: ", a, "\n")
	os.Exit(255)
}

func fatalf(s string, a ...interface{}) {
	fatal(fmt.Sprintf(s, a...))
}

func lnfatal(a interface{}) {
	fmt.Print("\n", os.Args[0], ": error: ", a, "\n")
	os.Exit(255)
}

func lnfatalf(s string, a ...interface{}) {
	lnfatal(fmt.Sprintf(s, a...))
}

func checkErr(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

func lnCheckErr(err error) {
	if err != nil {
		lnfatalf("%v", err)
	}
}

func askForConfirmation(question string) bool {
	fmt.Printf("%s (y/n): ", question)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		fatal("expected confirmation")
	}
	response = strings.ToLower(response)
	if response == "y" || response == "yes" {
		return true
	} else if response == "n" || response == "no" {
		return false
	} else {
		fmt.Println("Please type y/n")
		return askForConfirmation(question)
	}
}

func getProgress(showProgress bool, fileForProgress inputFile) (progress func(int64), finishProgress func(string)) {
	if showProgress {
		progress, finishProgress = createProgress(fileSize(fileForProgress))
	} else {
		// Empty function not to show the progress
		progress = func(int64) {}
		finishProgress = func(str string) {}
	}
	return
}

func createProgress(total int64) (progress func(int64), finishProgress func(string)) {
	bar := pb.New64(total)
	bar.SetMaxWidth(120)
	bar.SetRefreshRate(100 * time.Millisecond)
	bar.SetUnits(pb.U_BYTES)
	started := false
	return func(n int64) {
			if !started {
				started = true
				bar.Start()
			}
			bar.Add64(n)
		}, func(finishStr string) {
			if bar.Get() != bar.Total {
				finishStr = "Progress percentage is different from 100 due to overlapping or lack of segments in hashes file (different data size was expected to be processed).\n" + finishStr
			}

			bar.FinishPrint(finishStr)

		}
}

func fileSize(input inputFile) int64 {
	fi, err := input.Stat()
	checkErr(err)
	return fi.Size()
}

func getHashContainerByHash(hashBytes []byte) hashContainer {
	switch len(hashBytes) {
	case md5.Size:
		return hashContainer{h: md5.New(), name: md5Name}
	case sha1.Size:
		return hashContainer{h: sha1.New(), name: sha1Name}
	case sha256.Size224:
		return hashContainer{h: sha256.New224(), name: sha224Name}
	case sha256.Size:
		return hashContainer{h: sha256.New(), name: sha256Name}
	case sha512.Size384:
		return hashContainer{h: sha512.New384(), name: sha384Name}
	case sha512.Size:
		return hashContainer{h: sha512.New(), name: sha512Name}
	default:
		fatalf("unknown hash type with digest length %d.", len(hashBytes))
	}

	return hashContainer{}
}

func getHashContainersByNames(hashNames []string) []hashContainer {
	hashContainers := make([]hashContainer, len(hashNames))
	for i, hashName := range hashNames {
		switch hashName {
		case md5Name:
			hashContainers[i] = hashContainer{h: md5.New(), name: md5Name}
		case sha1Name:
			hashContainers[i] = hashContainer{h: sha1.New(), name: sha1Name}
		case sha224Name:
			hashContainers[i] = hashContainer{h: sha256.New224(), name: sha224Name}
		case sha256Name:
			hashContainers[i] = hashContainer{h: sha256.New(), name: sha256Name}
		case sha384Name:
			hashContainers[i] = hashContainer{h: sha512.New384(), name: sha384Name}
		case sha512Name:
			hashContainers[i] = hashContainer{h: sha512.New(), name: sha512Name}
		default:
			fatalf("unknown hash type %s", hashName)
		}
	}

	return hashContainers
}

func bytesToSectors(start, length int64) (startLba, endLba int64) {
	if start != 0 && start%sectorSize != 0 {
		lnfatalf("segment start is not sector aligned. start=%v", start)
	}
	startLba = int64(math.Ceil(float64(start) / float64(sectorSize)))
	endLba = startLba + int64(math.Ceil(float64(length)/float64(sectorSize))) - 1
	return
}

func sectorToBytes(startLba, endLba int64) (startOffset, length int64) {
	startOffset = startLba * sectorSize
	length = (endLba - startLba + 1) * sectorSize
	return
}

func readSegmentLine(csvReader *csv.Reader) (startLba, endLba int64, hash []byte, err error) {
	record, err := csvReader.Read()
	if err != nil {
		return
	}

	hash, hashErr := hex.DecodeString(record[0])
	startLba, startErr := strconv.ParseInt(record[1], 10, 64)
	endLba, endErr := strconv.ParseInt(record[2], 10, 64)

	if hashErr != nil || startErr != nil || endErr != nil || endLba < startLba {
		err = errors.New(strings.Join(record, string(csvDelimiter)))
	}

	return
}

func writeSegmentLine(csvWriter *csv.Writer, seg segment) {
	err := csvWriter.Write(seg.ToStringSlice())
	lnCheckErr(err)
	csvWriter.Flush()
	err = csvWriter.Error()
	lnCheckErr(err)
}

func writeDiffLine(csvWriter *csv.Writer, startLba, endLba int64) {
	values := make([]string, 2)
	values[0] = fmt.Sprintf("%d", startLba)
	values[1] = fmt.Sprintf("%d", endLba)

	err := csvWriter.Write(values)
	lnCheckErr(err)
	csvWriter.Flush()
	err = csvWriter.Error()
	lnCheckErr(err)
}

func writeErrorLine(csvWriter *csv.Writer, errorString string) {
	values := make([]string, 1)
	values[0] = errorString

	err := csvWriter.Write(values)
	lnCheckErr(err)
	csvWriter.Flush()
	err = csvWriter.Error()
	lnCheckErr(err)
}

func createCsvReader(f io.Reader) *csv.Reader {
	csvReader := csv.NewReader(f)
	csvReader.Comma = csvDelimiter
	csvReader.FieldsPerRecord = 3

	return csvReader
}

func createCsvWriter(f io.Writer) *csv.Writer {
	csvWriter := csv.NewWriter(f)
	csvWriter.Comma = csvDelimiter
	csvWriter.UseCRLF = runtime.GOOS == "windows"

	return csvWriter
}
