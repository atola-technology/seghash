package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"
)

const (
	inputFilename         = "test_input"
	verifyOutputFilename  = "test_diffs"
	predefinedDataLength  = 20000000
	predefinedPattern     = "testPatternLongEnough\n"
	predefinedSegmentSize = 3 * 1024 * 1024
	predefinedDataMd5     = "dd772048b3e0245de296896af49876c2"
)

//Test data obtained by:
// $ yes testPatternLongEnough | head -c 20000000 > testfile
// segmentsize: 3*1024*1024=3145728
//
// test data md5 sum: dd772048b3e0245de296896af49876c2
var predefinedHashesMd5 = []string{
	"e2696f6b63017a35e01d422500d615b9",
	"044057803de9f1026ed7e03e4227e443",
	"5da7a30fb5f6292ef02fa18231c99ad5",
	"0c0ff74d8dae87c22324c449d8b2a865",
	"98d3bd327e79370326524c2b59f8e595",
	"0f0e7ba9cb0f0c4ce26f965c772beb4c",
	"4a28f57ec8eb09f016994f658681d4bc",
}

var predefinedHashesSha1 = []string{
	"567ac896f2f19d0c31d6b8c803069ce530b012f2",
	"4ccbfcaa005c3c508a6b95769edba60759892141",
	"bd5ff8607faccedb1d952da5632283874bcd2410",
	"1eaf2f20a84782d6272f50e0c3dd346a85fd3d7b",
	"f944b7ea3f473f5c51225a2a6418bdc5c041fe75",
	"c8f35045375a6c36dd893e2f5172d6637294c2c8",
	"49b2539a5df31aa609302acef9834540d7784a55",
}

func createPredefinedData(fs afero.Fs, t *testing.T) {
	f, _ := fs.Create(inputFilename)
	patternBuf := []byte(predefinedPattern)
	written := 0

	for written < predefinedDataLength {
		var n int
		if written+len(patternBuf) > predefinedDataLength {
			n, _ = f.Write(patternBuf[:predefinedDataLength-written])
		} else {
			n, _ = f.Write(patternBuf)
		}
		written += n
	}
	f.Close()

	//check data
	f, _ = fs.Open(inputFilename)
	fi, _ := f.Stat()
	if fi.Size() != predefinedDataLength {
		t.Fatalf("Incorrect test data. Expected length: %d, actual length: %d", predefinedDataLength, fi.Size())
	}
	hasher := md5.New()
	io.Copy(hasher, f)
	sum := hasher.Sum(nil)
	if hex.EncodeToString(sum) != predefinedDataMd5 {
		t.Fatalf("Incorrect test data. Expected md5: %s, actual md5: %s", predefinedDataMd5, hex.EncodeToString(sum))
	}
	f.Close()
}

func memfs() afero.Fs {
	return afero.NewMemMapFs()
}

func TestPredefined(t *testing.T) {
	fmt.Printf("Test with predefined data: ")
	fs := memfs()
	createPredefinedData(fs, t)

	input, _ := fs.Open(inputFilename)
	calcArgs := &calcArgs{segmentSize: predefinedSegmentSize, input: input, hashNames: []string{md5Name, sha1Name}, createOutputFile: func(name string) outputFile {
		out, _ := fs.Create(name)
		return out
	}}
	outNames := calc(calcArgs, false)
	input.Close()

	for _, outname := range outNames {
		hashes, _ := fs.Open(outname)
		reader := createCsvReader(hashes)
		lines, _ := reader.ReadAll()
		if len(lines) != len(predefinedHashesMd5) {
			t.Fatalf("Different segment count. Expected: %d, actual: %d", len(predefinedHashesMd5), len(lines))
		}

		if strings.Contains(outname, md5Name) {
			for i, line := range lines {
				//hash goes first
				if line[0] != predefinedHashesMd5[i] {
					t.Errorf("Different segment %d. Expected %s, computed: %s", i, predefinedHashesMd5[i], line[0])
				}
			}
		} else {
			for i, line := range lines {
				//hash goes first
				if line[0] != predefinedHashesSha1[i] {
					t.Errorf("Different segment %d. Expected %s, computed: %s", i, predefinedHashesSha1[i], line[0])
				}
			}

		}
		hashes.Close()
	}
	fmt.Println("OK")
}

func TestSelf(t *testing.T) {
	fmt.Printf("Test calculate-verify: ")
	fs := memfs()
	inputBuf := make([]byte, 20*1024*1024)
	input, _ := fs.Create(inputFilename)
	rand.Read(inputBuf)
	input.Write(inputBuf)
	input.Close()

	input, _ = fs.Open(inputFilename)
	calcArgs := &calcArgs{segmentSize: 2 * 1024 * 1024, input: input, hashNames: []string{md5Name, sha1Name}, createOutputFile: func(name string) outputFile {
		out, _ := fs.Create(name)
		return out
	}}
	outNames := calc(calcArgs, false)
	input.Close()

	for _, outname := range outNames {
		input, _ = fs.Open(inputFilename)
		inputHashes, _ := fs.Open(outname)
		verifyArgs := &verifyArgs{input: input, segmentHashesInput: inputHashes, createOutputFile: func() outputFile {
			f, _ := fs.Create(verifyOutputFilename)
			return f
		}}
		diffsCount := verify(verifyArgs, false)
		if diffsCount > 0 {
			t.Error()
		}
		input.Close()
	}
	fmt.Println("OK")
}

func speedTest(hashName string) {
	fmt.Printf("Speed (%s): ", hashName)
	mbs := 300
	benchFs := memfs()
	inputBuf := make([]byte, mbs*1024*1024)
	benchInput, _ := benchFs.Create(inputFilename)
	rand.Read(inputBuf)
	benchInput.Write(inputBuf)
	benchCalcArgs := &calcArgs{segmentSize: 2 * 1024 * 1024, input: benchInput, hashNames: []string{hashName}, createOutputFile: func(name string) outputFile {
		out, _ := benchFs.Create(name)
		return out
	}}

	defer benchInput.Close()
	totalNs := int64(0)
	skipTest := 2
	iterations := 10
	for i := 0; i < iterations; i++ {
		start := time.Now()
		benchCalcArgs.input.Seek(0, 0)
		calc(benchCalcArgs, false)
		duration := time.Now().Sub(start)
		if i >= skipTest {
			totalNs += duration.Nanoseconds()
		}
	}
	speedMbs := (int64(mbs) * int64(iterations-skipTest) * int64(time.Second)) / totalNs
	fmt.Printf("%d MiB/s\n", speedMbs)
}

func TestSpeedMd5(t *testing.T) {
	speedTest(md5Name)

}

func TestSpeedSha1(t *testing.T) {
	speedTest(sha1Name)
}
