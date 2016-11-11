package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	minSegmentSize       = 2 * 1024 * 1024
	maxHashesToCalculate = 2
	sectorSize           = 512
)

func distinct(names []string) []string {
	distinctHashNamesMap := make(map[string]int)
	for _, name := range names {
		distinctHashNamesMap[name] = 0
	}
	distinctHashNames := make([]string, len(distinctHashNamesMap))
	i := 0
	for name := range distinctHashNamesMap {
		distinctHashNames[i] = name
		i++
	}
	return distinctHashNames
}

func contains(values []string, value string) bool {
	for _, val := range values {
		if val == value {
			return true
		}
	}
	return false
}

func checkHashNames(hashNames []string) {
	if len(hashNames) > maxHashesToCalculate {
		fatal("cannot calculate more than two hashes at once.")
	}
	validHashNames := []string{md5Name, sha1Name, sha224Name, sha256Name, sha384Name, sha512Name}
	for i, hash := range hashNames {
		hashNames[i] = strings.ToLower(hash)
		if !contains(validHashNames, hashNames[i]) {
			fatal(fmt.Sprintf("hashtype value must be one of %s, but got '%s' ", strings.Join(validHashNames, ", "), hash))
		}
	}
}

func checkSegmentSize(segmentSize int64) {
	if segmentSize < minSegmentSize {
		fatal("segment size is less then 2M")
	}
	if (segmentSize % int64(sectorSize)) != 0 {
		fatal("segment size is not a multiple of 512.")
	}
}

func checkDirPathExistence(_path string) {
	dirPath := filepath.Dir(_path)
	_, err := os.Stat(dirPath)
	checkErr(err)
}

func checkFileCreation(_path string) {
	f, err := os.Create(_path)
	checkErr(err)
	f.Close()
	os.Remove(f.Name())
}

func fileIsNonEmptyFile(f inputFile, argName, dirErrorStr, emptyErrorStr string) {
	fi, err := f.Stat()
	checkErr(err)
	if fi.IsDir() {
		fatal(argName + " is a directory, " + dirErrorStr)
	}
	if fi.Size() <= 0 {
		fatal(argName + " is empty, " + emptyErrorStr)
	}
}

func fileHasRightStructure(f inputFile, errorStr string) {
	testData := make([]byte, 1024)
	n, _ := f.Read(testData)
	testString := string(testData[:n])
	if strings.Count(testString, string(csvDelimiter)) < 2 {
		fatal(errorStr)
	}
	f.Seek(0, 0)
}

func checkForensicFileExtensions(f inputFile) {
	if hasForensicContainerFileExtensions(f) {
		question := fmt.Sprintf("File %s will be interpreted as a raw file. Do you want to continue?", filepath.Base(f.Name()))
		if askForConfirmation(question) {
			return
		}

		os.Exit(0)
	}
}

func hasForensicContainerFileExtensions(f inputFile) bool {
	extension := filepath.Ext(f.Name())
	if len(extension) < 3 {
		return false
	}

	// remove '.' from extension and convert to Upper case
	extension = strings.ToUpper(extension[1:])

	if strings.HasPrefix(extension, "EX") ||
		strings.HasPrefix(extension, "LX") {

		number, err := strconv.Atoi(extension[2:])
		if err == nil && number > 0 && number < 100 {
			return true
		}
	} else if strings.HasPrefix(extension, "E") ||
		strings.HasPrefix(extension, "S") ||
		strings.HasPrefix(extension, "L") {

		number, err := strconv.Atoi(extension[1:])
		if err == nil && number > 0 && number < 100 {
			return true
		}
	} else if extension == "AFF" ||
		extension == "AFM" ||
		extension == "AFD" ||
		extension == "AD1" ||
		extension == "MFS01" ||
		extension == "AFF4" {

		return true
	}

	return false
}

func checkDiffFileExtension(verifyDiffOutputFname string) string {
	extension := strings.ToLower(filepath.Ext(verifyDiffOutputFname))
	if extension != ".csv" {
		return verifyDiffOutputFname + ".csv"
	}

	return verifyDiffOutputFname
}

func filenameWithoutExtension(input inputFile) string {
	extension := filepath.Ext(input.Name())
	return input.Name()[0 : len(input.Name())-len(extension)]
}
