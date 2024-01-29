package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-errors/errors"
)

// TestType represents the types of test supported by this framework
//go:generate jsonenums -type=TestType
type TestType int

const (
	// IOTest is a test that runs an executable with defined inputs send to stdin
	// and compares the output to some expected output
	IOTest TestType = iota
	// JUnitTest is a test that runs using the Java JUnit framework
	JUnitTest
	// xUnitTest uses the xUnit framework for F#
	xUnitTest
	// pytest
	PyTest
	// Matlab tests
	Matlab
)

type TestRunner interface {
	executeTest(execution Execution) TestResult
}

// get a runner for the given testType
func getRunner(testType TestType) TestRunner {
	switch testType {
	case IOTest:
		return IOTestRunner{}
	case JUnitTest:
		return JUnitTestRunner{}
	case xUnitTest:
		return XUnitTestRunner{}
	case PyTest:
		return PyTestRunner{}
	case Matlab:
		return MatlabTestRunner{}
	default:
		return TestRunnerNotFound{message: fmt.Sprintf("Test type not supported: %d", testType)}
	}
}

// maximum file size when reading user generated output
const maxFileSize = 1024 * 1024

func allEndLines(b []byte) bool {
	for _, c := range b {
		if c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}

func executeProgram(execution Execution, inFile string, paramFile string, outFile string, errFile string, dockerArgs []string, dockerImage string, command ...string) (err error) {
	testid := execution.ID + "-" + filepath.Base(inFile)
	runDir := execution.RunDir
	testDir := execution.TestDir

	timeout := execution.Config.Timeout
	if timeout == 0 {
		timeout = 10
	}
	maxMem := execution.Config.MaxMem
	if maxMem == 0 {
		maxMem = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer func() {
		err := exec.Command("docker", "stop", testid).Run()
		if err != nil {
			println("Could not stop", testid, err.Error())
		}
		cancel()
	}()
	// Docker command
	arguments, err := dockerArguments(execution.RunDir, testid)
	if err != nil {
		return fmt.Errorf("Could not get docker arguments: %s", err)
	}
	arguments = append(arguments, "-i")

	arguments = append(arguments, "-m", fmt.Sprintf("%dM", maxMem))

	arguments = append(arguments, dockerArgs...)

	arguments = append(arguments, dockerImage)

	// the java command
	arguments = append(arguments, command...)

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Dir = runDir

	paramFilePath := filepath.Join(testDir, paramFile)
	if finfo, err := os.Stat(paramFilePath); err == nil && !finfo.IsDir() {
		if paramBytes, err := ioutil.ReadFile(paramFilePath); err == nil {
			if debug {
				Debug.Printf("Found parameters: %v", strings.Split(strings.Trim(string(paramBytes), "\r\n"), " "))
			}
			arguments = append(arguments, strings.Split(strings.Trim(string(paramBytes), "\r\n"), " ")...)
		}
	}
	cmd.Args = arguments

	inFilePath := filepath.Join(testDir, inFile)
	if finfo, perr := os.Stat(inFilePath); perr == nil && !finfo.IsDir() {
		inFileHandle, err := os.Open(inFilePath)
		if err != nil {
			LogError("test", "Could not open test input file %s: %s", inFilePath, err)
			return err
		}
		defer inFileHandle.Close()
		cmd.Stdin = inFileHandle
	}

	outFilePath := filepath.Join(runDir, outFile)
	outFileHandle, err := os.Create(outFilePath)
	if err != nil {
		LogError("test", "Could not open test output file %s: %s", outFilePath, err)
		return
	}

	defer func() {
		cerr := outFileHandle.Close()
		if err == nil {
			err = cerr
			return
		}
	}()
	cmd.Stdout = LimitWriter(outFileHandle, maxFileSize)

	errFilePath := filepath.Join(runDir, errFile)
	errFileHandle, err := os.Create(errFilePath)
	if err != nil {
		LogError("test", "Could not open test error file %s: %s", errFilePath, err)
		return
	}

	defer func() {
		cerr := errFileHandle.Close()
		if err == nil {
			err = cerr
			return
		}
	}()
	cmd.Stderr = LimitWriter(errFileHandle, maxFileSize)

	if err = cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == -1 { // killed
					err = fmt.Errorf("Timeout")
				}
			}
		}

	}
	return
}

func compareFileContent(expectedFile, outFile string, execution Execution) (expectedResult string, testOk bool, err error) {
	config := execution.Config
	if config.CompareTool == "" {
		return compareFileContentExactMatch(expectedFile, outFile)
	}

	var tool string
	if filepath.IsAbs(*tools_folder) {
		tool = filepath.Join(*tools_folder, config.CompareTool)
	} else {
		tool = filepath.Join(testdataDir, *tools_folder, config.CompareTool)
	}

	args := make([]string, 0)
	args = append(args, config.CompareToolArgs...)
	args = append(args, expectedFile, outFile)

	cmd := exec.Command(tool, args...)
	output, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return string(output), false, nil
		}
		return "", false, err
	}
	return "", true, nil
}

func compareFileContentExactMatch(expectedFile, outFile string) (expectedResult string, testOk bool, err error) {
	expected, err := readFile(expectedFile)
	if err != nil {
		return "", false, err
	}
	output, err := readFile(outFile)
	if err != nil {
		return "", false, err
	}

	outputLen := len(output)
	expectedLen := len(expected)
	if outputLen != expectedLen {
		if outputLen > expectedLen && allEndLines(output[expectedLen:]) {
			outputLen = expectedLen
		} else if expectedLen > outputLen && allEndLines(expected[outputLen:]) {
			// all right, shorter len already
		} else {
			return string(expected), false, nil
		}
	}

	for i := 0; i < outputLen; i++ {
		if output[i] != expected[i] {
			return string(expected), false, nil
		}
	}

	return "", true, nil
}

type IOTestRunner struct {
}

func (t IOTestRunner) executeTest(execution Execution) TestResult {
	testDir := execution.getTestDir()
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		return internalErrorResult(execution, "Could not read test folder")
	}

	numTests := 0
	numFailed := 0
	tests := make([]Test, 0)
	startTime := time.Now()
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".out.txt") {
			fileBaseName := f.Name()[:len(f.Name())-8]

			test := Test{
				Name:   fileBaseName,
				Output: "",
			}

			inFileName := fileBaseName + ".in.txt"
			inFile := filepath.Join(testDir, inFileName)
			paramFileName := fileBaseName + ".param.txt"
			paramFile := filepath.Join(testDir, paramFileName)
			outFileName := fileBaseName + ".out.txt"
			outFile := filepath.Join(execution.RunDir, outFileName)
			errFileName := fileBaseName + ".err.txt"
			errFile := filepath.Join(execution.RunDir, errFileName)
			expectedFile := filepath.Join(testDir, outFileName)

			numTests++
			var execErr error

			switch execution.Config.Compiler {
			case JavaCompiler:
				execErr = executeJava(execution, inFileName, paramFileName, outFileName, errFileName)
			case CCompiler:
				execErr = executeC(execution, inFileName, paramFileName, outFileName, errFileName)
			case PythonCompiler:
				execErr = executePython(execution, inFileName, paramFileName, outFileName, errFileName)
			default:
				LogError("test", "Execution not supported for compiler %s", _CompilerValueToName[execution.Config.Compiler])
				execErr = fmt.Errorf("execution not supported for compiler %s", _CompilerValueToName[execution.Config.Compiler])
			}

			// read in file
			inFileContent, err := readFile(inFile)
			if err != nil {
				inFileContent = []byte("No input")
			}

			parameters := ""
			if finfo, err := os.Stat(paramFile); err == nil && !finfo.IsDir() {
				if paramBytes, err := ioutil.ReadFile(paramFile); err == nil {
					parameters = " with parameters '" + string(paramBytes) + "'"
				}
			}

			test.Error = fmt.Sprintf("Error for the following input%s:\n%s", parameters, string(inFileContent))

			// read out file
			outFileContent, err := readFile(outFile)
			if err != nil {
				outFileContent = []byte("")
			}
			test.Output += string(outFileContent)

			// read expectedFile
			expectedFileContent, err := readFile(expectedFile)
			if err != nil {
				expectedFileContent = []byte("")
			}
			test.Expected += string(expectedFileContent)

			if execErr != nil {
				test.Success = false

				// read err file
				errFileContent, err := readFile(errFile)
				if err != nil {
					errFileContent = []byte("")
				}
				test.Output += fmt.Sprintf("\n\n\n%s\n%s\n", execErr.Error(), string(errFileContent))
				tests = append(tests, test)
				if debug {
					Debug.Println(execErr)
				}
				numFailed++
				continue
			}

			expectedResult, resultOk, execErr := compareFileContent(expectedFile, outFile, execution)
			if execErr != nil {
				test.Success = false
				test.Output += fmt.Sprintf("\n\n\nError comparing results:\n%s\n", execErr.Error())
				numFailed++
				tests = append(tests, test)
				continue
			}
			if !resultOk {
				test.Success = false

				// get expected result from error
				test.Expected = expectedResult

				tests = append(tests, test)
				numFailed++
				continue
			}

			if errContent, err := readFile(errFile); err == nil {
				test.Output += string(errContent)
			}

			test.Success = true
			test.Error = ""
			tests = append(tests, test)
		}
	}
	duration := time.Since(startTime)
	testExecutionTimeHistogram.Observe(duration.Seconds())
	if debug {
		Debug.Printf("Duration of IO test execution: %s", duration)
	}

	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		Tests:         tests,
		TestsExecuted: numTests,
		TestsFailed:   numFailed}
}

func internalErrorResult(execution Execution, msg string) TestResult {
	LogError("test", fmt.Sprintf("%s (Test: %s)", msg, execution.Test))
	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		InternalError: msg,
	}
}

type RunnerResult struct {
	Name           string  `json:"name"`
	Success        bool    `json:"success"`
	Error          string  `json:"error"`
	TestClass      string  `json:"testClass"`
	MethodName     string  `json:"methodName"`
	StackTrace     string  `json:"stackTrace"`
	Stdout         string  `json:"stdout"`
	Stderr         string  `json:"stderr"`
	ExpectedResult *string `json:"expectedResult"`
	ActualResult   *string `json:"actualResult"`
}

type RunnerReport struct {
	NumTests    int            `json:"numTests"`
	FailedTests int            `json:"failedTests"`
	Time        float32        `json:"time"`
	Results     []RunnerResult `json:"results"`
}

/* appends the output from the logfile to the error message */
func appendOutput(outFileHandle *os.File, errFileHandle *os.File, outLogFile string, errLogFile string, message string) string {
	outFileHandle.Close()
	errFileHandle.Close()
	stdout, _ := readFileToString(outLogFile)
	stderr, _ := readFileToString(errLogFile)
	if len(stdout) > 0 {
		message += "\n\nOutput:\n" + string(stdout)
	}
	if len(stderr) > 0 {
		message += "\n\nError Output:\n" + string(stderr)
	}
	return message
}

/** reads a file into a string. If the file is too long it skips some part in the middle. */
func readFileToString(filename string) (string, error) {
	res, err := readFile(filename)
	return string(res), err
}

func readFile(filename string) ([]byte, error) {
	res, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if len(res) > maxFileSize {
		return append(res[:maxFileSize/4],
			append([]byte("\n... [Output too long, skipping "+strconv.Itoa(len(res)-maxFileSize/2)+" bytes] ... \n"),
				res[len(res)-maxFileSize/4:]...)...), nil
	}
	return res, nil
}

func testService() {
	for {
		handleTestRequest()
	}
}

func handleTestRequest() {
	execution := <-testChannel
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Error in test execution: %+v\n", execution)
			fmt.Println("Recovered from error", err)
			fmt.Println(errors.Wrap(err, 2).ErrorStack())
		}
	}()
	fmt.Printf("Executing test: %+v\n", execution)
	testResult := getRunner(execution.Config.TestType).executeTest(execution)
	testCount.WithLabelValues(execution.Test).Add(float64(testResult.TestsExecuted))
	testFailCount.WithLabelValues(execution.Test).Add(float64(testResult.TestsFailed))
	execution.ResChan <- testResult
	if *clean_testruns {
		err := os.RemoveAll(execution.RunDir)
		if err != nil {
			fmt.Printf("Could not delete test directory %s: %s", execution.TestDir, err)
		}
	}
}

type TestRunnerNotFound struct {
	message string
}

func (t TestRunnerNotFound) executeTest(execution Execution) TestResult {
	return internalErrorResult(execution, t.message)
}
