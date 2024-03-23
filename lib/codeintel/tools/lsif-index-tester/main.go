package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/conversion"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/validation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type projectResult struct {
	name         string
	usage        usageStats
	output       string
	bundleResult bundleResult
	suiteResult  testSuiteResult
}

type usageStats struct {
	// Memory usage in kilobytes by child process.
	memory int64
}

type passedTest struct {
	Name string
}

type failedTest struct {
	Name string
	Diff string
}

type bundleResult struct {
	Valid  bool
	Errors []string
}

type testFileResult struct {
	Name   string
	Passed []passedTest
	Failed []failedTest
}

type testSuiteResult struct {
	FileResults []testFileResult
}

var directory string
var raw_indexer string
var debug bool

// TODO: Do more monitoring of the process.
// var monitor bool

func main() {
	flag.StringVar(&directory, "dir", ".", "The directory to run the test harness over")
	flag.StringVar(&raw_indexer, "indexer", "", "The name of the indexer that you want to test")
	flag.BoolVar(&debug, "debug", false, "Enable debugging")
	flag.Parse()

	// Initialize log format and level
	if debug {
		os.Setenv("SRC_LOG_LEVEL", "debug")
	}
	if _, set := os.LookupEnv("SRC_LOG_FORMAT"); !set {
		// Unless a custom log format is set, initialize to dev-friendly output
		os.Setenv("SRC_LOG_FORMAT", "console")
		os.Setenv("SRC_DEVELOPMENT", "true")
	}
	liblog := log.Init(log.Resource{Name: "lsif-index-tester"})
	defer liblog.Sync()

	logger := log.Scoped(raw_indexer).With(log.String("directory", directory))

	if raw_indexer == "" {
		logger.Fatal("Indexer is required. Pass with --indexer")
	}

	logger.Info("Starting execution")

	indexer := strings.Split(raw_indexer, " ")
	if err := testDirectory(context.Background(), logger, indexer, directory); err != nil {
		logger.Fatal("Tests failed", log.Error(err))
		return
	}
	logger.Info("Tests passed")
}

func testDirectory(ctx context.Context, logger log.Logger, indexer []string, directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	type channelResult struct {
		name   string
		result projectResult
		err    error
	}

	resultChan := make(chan channelResult, len(files))
	var wg sync.WaitGroup

	for _, f := range files {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			projResult, err := testProject(ctx, logger, indexer, path.Join(directory, name), name)
			resultChan <- channelResult{
				name:   name,
				result: projResult,
				err:    err,
			}
		}(f.Name())

	}

	wg.Wait()
	close(resultChan)

	successful := true
	for res := range resultChan {
		fmt.Println("====================")
		if res.err != nil {
			successful = false

			logger.Warn("Failed to run test", log.String("name", res.name))
			fmt.Println(res.err)
			continue
		}

		if !res.result.bundleResult.Valid {
			successful = false

			fmt.Printf("%s bundle was found to be invalid:\n%s\n", res.name, res.result.bundleResult.Errors)
		}

		for _, fileResult := range res.result.suiteResult.FileResults {
			if len(fileResult.Failed) > 0 {
				successful = false

				for _, failed := range fileResult.Failed {
					fmt.Printf("Failed test: File: %s, Name: %s\nDiff: %s\n", fileResult.Name, failed.Name, failed.Diff)
				}
			}
		}
	}

	if !successful {
		return errors.Newf("'%s' Failed.", directory)
	}

	return nil
}

func testProject(ctx context.Context, logger log.Logger, indexer []string, project, name string) (projectResult, error) {
	output, err := setupProject(project)
	if err != nil {
		return projectResult{name: name, output: string(output)}, err
	}

	logger.Debug("... Completed setup project")
	result, err := runIndexer(ctx, logger.Scoped("run"), indexer, project, name)
	if err != nil {
		return projectResult{
			name:   name,
			output: result.output,
		}, err
	}

	usageData, _ := json.Marshal(result.usage)
	logger.Debug("... \t Resource usage", log.String("usage", string(usageData)))

	bundleResult, err := validateDump(project)
	if err != nil {
		return projectResult{}, err
	}
	logger.Debug("... Validated dump.lsif")

	bundle, err := readBundle(project)
	if err != nil {
		return projectResult{name: name}, err
	}
	logger.Debug("... Read bundle")

	testResult, err := validateTestCases(logger.Scoped("validate"), project, bundle)
	if err != nil {
		return projectResult{name: name}, err
	}

	return projectResult{
		name:         name,
		usage:        result.usage,
		output:       string(output),
		bundleResult: bundleResult,
		suiteResult:  testResult,
	}, nil
}

func setupProject(directory string) ([]byte, error) {
	cmd := exec.Command("./setup_indexer.sh")
	cmd.Dir = directory

	return cmd.CombinedOutput()
}

func runIndexer(ctx context.Context, logger log.Logger, indexer []string, directory, name string) (projectResult, error) {
	command := indexer[0]
	args := indexer[1:]

	logger.Debug("... Generating dump.lsif")
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = directory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return projectResult{}, err
	}

	sysUsage := cmd.ProcessState.SysUsage()
	mem, _ := MaxMemoryInKB(sysUsage)

	return projectResult{
		name:   name,
		usage:  usageStats{memory: mem},
		output: string(output),
	}, err
}

// Returns the bundle result. Only errors when the bundle doesn't exist or is
// unreadable. Otherwise, we send errors back in bundleResult so that we can
// run the tests even with invalid bundles.
func validateDump(directory string) (bundleResult, error) {
	dumpFile, err := os.Open(filepath.Join(directory, "dump.lsif"))
	if err != nil {
		return bundleResult{}, err
	}

	ctx := validation.NewValidationContext()
	validator := &validation.Validator{Context: ctx}

	if err := validator.Validate(dumpFile); err != nil {
		return bundleResult{}, err
	}

	if len(ctx.Errors) > 0 {
		errs := make([]string, len(ctx.Errors)+1)
		errs[0] = fmt.Sprintf("Detected %d errors", len(ctx.Errors))
		for i, err := range ctx.Errors {
			errs[i+1] = fmt.Sprintf("%d. %s", i, err)
		}

		return bundleResult{Valid: false, Errors: errs}, nil
	}

	return bundleResult{Valid: true}, nil
}

func validateTestCases(logger log.Logger, projectRoot string, bundle *precise.GroupedBundleDataMaps) (testSuiteResult, error) {
	testFiles, err := os.ReadDir(filepath.Join(projectRoot, "lsif_tests"))
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("No lsif test directory exists here", log.String("directory", projectRoot))
			return testSuiteResult{}, nil
		}

		return testSuiteResult{}, err
	}

	fileResults := []testFileResult{}
	for _, file := range testFiles {
		if testFileExtension := filepath.Ext(file.Name()); testFileExtension != ".json" {
			continue
		}

		testFileName := filepath.Join(projectRoot, "lsif_tests", file.Name())
		fileResult, err := runOneTestFile(logger, projectRoot, testFileName, bundle)
		if err != nil {
			logger.Fatal("Had an error while we do the test file", log.String("file", testFileName), log.Error(err))
		}

		fileResults = append(fileResults, fileResult)
	}

	return testSuiteResult{FileResults: fileResults}, nil
}

func runOneTestFile(logger log.Logger, projectRoot, file string, bundle *precise.GroupedBundleDataMaps) (testFileResult, error) {
	doc, err := os.ReadFile(file)
	if err != nil {
		return testFileResult{}, errors.Wrap(err, "Failed to read file")
	}

	var testCase LsifTest
	if err := json.Unmarshal(doc, &testCase); err != nil {
		return testFileResult{}, errors.Wrap(err, "Malformed JSON")
	}

	fileResult := testFileResult{Name: file}

	for _, definitionTest := range testCase.Definitions {
		if err := runOneDefinitionRequest(logger, projectRoot, bundle, definitionTest, &fileResult); err != nil {
			return fileResult, err
		}
	}

	for _, referencesTest := range testCase.References {
		if err := runOneReferencesRequest(projectRoot, bundle, referencesTest, &fileResult); err != nil {
			return fileResult, err
		}
	}

	return fileResult, nil
}

// Stable sort for references so that we can compare much more easily.
// Without this, it's a bit annoying to get the diffs.
func sortReferences(references []Location) {
	sort.SliceStable(references, func(i, j int) bool {
		left := references[i]
		right := references[j]

		if left.URI > right.URI {
			return false
		} else if left.URI < right.URI {
			return true
		}

		cmpRange := sortRange(left.Range, right.Range)
		if cmpRange != 0 {
			return cmpRange > 0
		}

		return i < j
	})
}

func sortRange(left, right Range) int {
	start := sortPosition(left.Start, right.Start)
	if start != 0 {
		return start
	}

	end := sortPosition(left.End, right.End)
	if end != 0 {
		return end
	}

	return 0
}
func sortPosition(left, right Position) int {
	if left.Line > right.Line {
		return -1
	} else if left.Line < right.Line {
		return 1
	}

	if left.Character > right.Character {
		return -1
	} else if left.Character < right.Character {
		return 1
	}

	return 0
}

func runOneReferencesRequest(projectRoot string, bundle *precise.GroupedBundleDataMaps, testCase ReferencesTest, fileResult *testFileResult) error {
	request := testCase.Request

	filePath := request.TextDocument
	line := request.Position.Line
	character := request.Position.Character

	results, err := precise.Query(bundle, filePath, line, character)
	if err != nil {
		return err
	}

	// TODO: We need to add support for not including the declaration from the context.
	//       I don't know of any way to do that currently, so it would require changes to Query or similar.
	if !request.Context.IncludeDeclaration {
		return errors.New("'context.IncludeDeclaration = false' configuration is not currently supported")
	}

	// At this point we can have multiple references, but can handle only one _set_ of references.
	if len(results) > 1 {
		return errors.New("Had too many results")
	}

	// Short circuit for expected empty but didn't get empty
	if len(testCase.Response) == 0 {
		if len(results[0].References) != 0 {
			fileResult.Failed = append(fileResult.Failed, failedTest{
				Name: testCase.Name,
				Diff: cmp.Diff(testCase.Response, results),
			})
		} else {
			fileResult.Passed = append(fileResult.Passed, passedTest{
				Name: testCase.Name,
			})
		}

		return nil
	}

	// Expected results but didn't get any
	if len(results) == 0 {
		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: "Found no results\n" + cmp.Diff(testCase.Response, results),
		})

		return nil
	}

	preciseReferences := results[0].References

	actualReferences := make([]Location, len(preciseReferences))
	for index, ref := range preciseReferences {
		actualReferences[index] = transformLocationToResponse(ref)
	}

	expectedReferences := []Location(testCase.Response)

	sortReferences(actualReferences)
	sortReferences(expectedReferences)

	if !cmp.Equal(actualReferences, expectedReferences) {
		diff := ""
		for index, actual := range actualReferences {
			if len(expectedReferences) <= index {
				diff += fmt.Sprintf("Missing Reference:\n%+v", actual)
				continue
			}

			expected := expectedReferences[index]
			if actual == expected {
				continue
			}

			thisDiff, err := getLocationDiff(projectRoot, expected, actual)
			if err != nil {
				return err
			}

			diff += cmp.Diff(actual, expected)
			diff += "\n" + thisDiff
		}

		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: diff,
		})
	} else {
		fileResult.Passed = append(fileResult.Passed, passedTest{
			Name: testCase.Name,
		})
	}

	return nil
}

func runOneDefinitionRequest(logger log.Logger, projectRoot string, bundle *precise.GroupedBundleDataMaps, testCase DefinitionTest, fileResult *testFileResult) error {
	request := testCase.Request

	docPath := request.TextDocument
	line := request.Position.Line
	character := request.Position.Character

	results, err := precise.Query(bundle, docPath, line, character)
	if err != nil {
		return err
	}

	// TODO: We probably can have more than one result and have that make sense...
	//       should allow testing that at some point
	if len(results) > 1 {
		return errors.New("Had too many results")
	}

	// Expected results but didn't get any
	if len(results) == 0 {
		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: "Found no results\n" + cmp.Diff(testCase.Response, results),
		})

		return nil
	}

	definitions := results[0].Definitions
	definitionsData, _ := json.Marshal(definitions)
	definitionsField := log.String("definitions", string(definitionsData))

	if len(definitions) > 1 {
		logger.Fatal("Had too many definitions", definitionsField)
	} else if len(definitions) == 0 {
		logger.Fatal("Found no definitions", definitionsField)
	}

	response := transformLocationToResponse(definitions[0])
	if diff := cmp.Diff(response, testCase.Response); diff != "" {
		thisDiff, err := getLocationDiff(projectRoot, testCase.Response, response)
		if err != nil {
			return err
		}

		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: diff + "\n" + thisDiff,
		})
	} else {
		fileResult.Passed = append(fileResult.Passed, passedTest{
			Name: testCase.Name,
		})
	}

	return nil
}

func transformLocationToResponse(location precise.LocationData) Location {
	return Location{
		URI: "file://" + location.URI,
		Range: Range{
			Start: Position{
				Line:      location.StartLine,
				Character: location.StartCharacter,
			},
			End: Position{
				Line:      location.EndLine,
				Character: location.EndCharacter,
			},
		},
	}

}
func readBundle(root string) (*precise.GroupedBundleDataMaps, error) {
	bundle, err := conversion.CorrelateLocalGitRelative(context.Background(), path.Join(root, "dump.lsif"), root)
	if err != nil {
		return nil, err
	}

	return precise.GroupedBundleDataChansToMaps(bundle), nil
}

var filesToContents = make(map[string]string)

func getFileContents(projectRoot, uri string) (string, error) {
	contents, ok := filesToContents[uri]
	if !ok {
		// ok, read the file
		fileName := strings.Replace(uri, "file://", "", 1)
		byteContents, err := os.ReadFile(path.Join(projectRoot, fileName))
		if err != nil {
			return "", err
		}

		contents = string(byteContents)
		filesToContents[uri] = contents
	}

	return contents, nil
}

func getLocationDiff(projectRoot string, expected, actual Location) (string, error) {

	contents, err := getFileContents(projectRoot, actual.URI)
	if err != nil {
		return "", err
	}

	diff, err := DrawLocations(contents, expected, actual, 2)
	if err != nil {
		return "", errors.Wrap(err, "Unable to draw the pretty diff")
	}

	return diff, nil
}
