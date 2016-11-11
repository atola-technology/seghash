package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/atola-technology/seghash/external/github.com/alecthomas/kingpin"
	"github.com/atola-technology/seghash/external/github.com/alecthomas/units"
)

const (
	segmenthashHelp = `Calculates segment hashes of specified image file or verifies an image file against existing segmented hash file.
Example: seghash calc inputfile.img md5`

	// calc command constants
	calcHelp = `Calculates segment hashes of an image file and puts resulting hashes in Hashes-<inputfile>-<hashtype>.csv.
If file already exists it is overwritten.`
	calcSegmentSizeHelp = `Desired size of a single segment in bytes. Minimum 2M. Must be multiple of 512.
May have a case-insensitive multiplier suffix: M (1024*1024), G (1024*1024*1024), and T. Example: -s 2G`
	calcInputHelp           = "Input file to calculate segment hashes over."
	calcOutputPrefixHelp    = "Specify prefix to replace default 'Hashes-<inputfile>' prefix."
	calcHashtypesHelpFormat = "Hash type. At most two hashtypes can be specified. Valid hashtypes are %s."

	// verify command constants
	verifyHelp = `Verify existing input file against existing csv file with segment hashes and write diffs to file Diffs-<hashfile>.csv if found.
Process exit code is set to 255 if any errors are encountered. Otherwise, it equals to the amount of found different segments.
If the number of mismatches is over 254, exit code remains 254 anyway.`
	verifyDiffOutputHelp = "Alternative file name for diff file."
	verifyInputHelp      = "Input file to verify segment hashes over."
	verifyHashesFileHelp = "Existing csv files with segment hashes."
)

type calcArgs struct {
	segmentSize      int64
	input            inputFile
	hashNames        []string
	createOutputFile func(name string) outputFile
	delimiter        rune
}

type verifyArgs struct {
	input              inputFile
	createOutputFile   func() outputFile
	segmentHashesInput inputFile
}

type strictBytesValue int64

var metricUnitMap = units.MakeUnitMap("B", "B", 1000)
var base2UnitMap = units.MakeUnitMap("", "", 1024)

func (sb *strictBytesValue) Set(value string) error {
	var i int64
	var err error
	v := strings.ToUpper(value)
	matched, err := regexp.MatchString("\\d+", v)
	if matched && err != nil {
		i, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
	} else {
		i, err = units.ParseUnit(v, base2UnitMap)
		if err != nil {
			i, err = units.ParseUnit(v, metricUnitMap)
			if err != nil {
				return err
			}
		}
	}
	*sb = strictBytesValue(i)
	return nil
}

func (sb *strictBytesValue) String() string {
	return ""
}

func strictBytes(s kingpin.Settings) (target *int64) {
	target = new(int64)
	s.SetValue((*strictBytesValue)(target))
	return
}

func getCalcHashtypesHelpString() string {
	return fmt.Sprintf(calcHashtypesHelpFormat, strings.Join([]string{md5Name, sha1Name, sha224Name, sha256Name, sha384Name, sha512Name}, ", "))
}

func parseArgs() (*calcArgs, *verifyArgs) {
	app := kingpin.New("seghash", segmenthashHelp)
	app.Version(version)
	app.VersionFlag.Short('v')
	app.HelpFlag.Short('h')

	calc := app.Command("calc", calcHelp)
	calcSegmentSize := strictBytes(calc.Flag("segmentsize", calcSegmentSizeHelp).Short('s').Default("4G"))
	calcOutputPrefix := calc.Flag("opref", calcOutputPrefixHelp).Short('o').String()
	calcInput := calc.Arg("inputfile", calcInputHelp).Required().File()
	calcHashNames := calc.Arg("hashtype", getCalcHashtypesHelpString()).Required().Strings()

	verify := app.Command("verify", verifyHelp)
	verifyDiffOutputFname := verify.Flag("diffname", verifyDiffOutputHelp).Short('d').String()
	verifyInput := verify.Arg("inputfile", verifyInputHelp).Required().File()
	verifyHashesFile := verify.Arg("hashfile", verifyHashesFileHelp).Required().File()

	cmd, err := app.Parse(os.Args[1:])
	if err != nil {
		fatalf("%s, try --help", err)
	}
	switch cmd {

	case calc.FullCommand():
		checkHashNames(*calcHashNames)
		checkSegmentSize(*calcSegmentSize)

		if calcOutputPrefix == nil || *calcOutputPrefix == "" {
			*calcOutputPrefix = "Hashes-" + filepath.Base((*calcInput).Name())
		} else {
			checkDirPathExistence(*calcOutputPrefix)
		}

		fileIsNonEmptyFile(
			*calcInput,
			"<inputfile>",
			"cannot calculate segment hashes over directories.",
			"cannot calculate segment hashes over empty files.")

		checkForensicFileExtensions(*calcInput)

		return &calcArgs{
			segmentSize: *calcSegmentSize,
			hashNames:   distinct(*calcHashNames),
			input:       *calcInput,
			createOutputFile: func(name string) outputFile {
				f, err := os.Create(*calcOutputPrefix + "-" + name)
				lnCheckErr(err)
				return f
			},
		}, nil

	case verify.FullCommand():
		if verifyDiffOutputFname == nil || *verifyDiffOutputFname == "" {
			*verifyDiffOutputFname = "Diffs-" + filepath.Base(filenameWithoutExtension(*verifyHashesFile))
		} else {
			checkFileCreation(*verifyDiffOutputFname)
		}

		*verifyDiffOutputFname = checkDiffFileExtension(*verifyDiffOutputFname)

		fileIsNonEmptyFile(
			*verifyInput,
			"<inputfile>",
			"cannot verify segment hashes against directories.",
			"cannot verify segment hashes against empty files.")

		checkForensicFileExtensions(*verifyInput)

		fileIsNonEmptyFile(
			*verifyHashesFile,
			"<hashfile>",
			"cannot verify segment hashes against directories.",
			"cannot verify segment hashes against empty files.")

		fileHasRightStructure(*verifyHashesFile, "file with segment hashes is invalid")

		return nil, &verifyArgs{
			input: *verifyInput,
			createOutputFile: func() outputFile {
				f, err := os.Create(*verifyDiffOutputFname)
				lnCheckErr(err)
				return f
			},
			segmentHashesInput: *verifyHashesFile,
		}
	}

	return nil, nil
}

func finalizeArgs(calcArgs *calcArgs, verifyArgs *verifyArgs) {
	if calcArgs != nil {
		calcArgs.input.Close()
	} else if verifyArgs != nil {
		verifyArgs.input.Close()
		verifyArgs.segmentHashesInput.Close()
	}
}
