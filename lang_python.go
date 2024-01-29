package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	xmlquery "github.com/antchfx/xquery/xml"
)

type CompilerProviderPython struct{}

func (c CompilerProviderPython) compile(execution Execution) error {
	absLibPath, err := filepath.Abs(filepath.Join(execution.TestDir, libDir))
	if err != nil {
		return fmt.Errorf("Internal Error: Could not create absolute path of lib folder")
	}

	files, err := ioutil.ReadDir(execution.RunDir)
	if err != nil {
		return fmt.Errorf("Test not found: %s", err)
	}

	// Docker command
	arguments, err := dockerArguments(execution.RunDir, execution.ID)
	if err != nil {
		return fmt.Errorf("Could not get docker arguments: %s", err)
	}

	if stat, err := os.Stat(absLibPath); err == nil && stat.IsDir() {
		arguments = append(arguments, "-v", absLibPath+":/libs:ro")
	}

	arguments = append(arguments, *docker_image_python)

	// run python compile in Docker container
	arguments = append(arguments, "python3", "-m", "py_compile")
	// add python files from current directory
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".py") {
			arguments = append(arguments, name)
		}
	}

	// TODO timeout?
	if debug {
		Debug.Printf("args = %v\n", arguments)
	}
	cmd := exec.Command("docker")
	cmd.Args = arguments
	cmd.Dir = execution.RunDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error compiling:\n%s", string(out))
	}
	return nil
}

func executePython(execution Execution, inFile string, paramFile string, outFile string, errFile string) (err error) {

	files, err := ioutil.ReadDir(execution.RunDir)
	if err != nil {
		return fmt.Errorf("Internal Error: Could not read Test files")
	}
	mainFile := ""
	if len(execution.Config.MainIs) > 0 {
		mainFile = execution.Config.MainIs
	} else {
		for _, f := range files {
			name := f.Name()
			if strings.HasSuffix(name, ".py") {
				mainFile = name
			}
		}
		if len(mainFile) == 0 {
			return fmt.Errorf("Keine Python Datei gefunden!")
		}
	}

	absMainFile, err := filepath.Abs(filepath.Join(execution.RunDir, mainFile))
	if err != nil {
		return fmt.Errorf("Internal Error: Could not create absolute path of main file")
	}

	if finfo, err := os.Stat(absMainFile); err != nil || finfo.IsDir() {
		return fmt.Errorf("Could not find %s (rename your program accordingly and try again)", mainFile)
	}
	maxMem := execution.Config.MaxMem
	if maxMem == 0 {
		maxMem = 100
	}

	arguments := make([]string, 0)

	return executeProgram(execution, inFile, paramFile, outFile, errFile, arguments, *docker_image_python, "python3", mainFile)
}

func executePytest(execution Execution) TestResult {
	testid := execution.ID
	absRunDir, err := filepath.Abs(execution.RunDir)
	if err != nil {
		return internalErrorResult(execution, "Could not make path of run dir absolute")
	}

	timeout := execution.Config.Timeout
	if timeout == 0 {
		timeout = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer func() {
		exec.Command("docker", "stop", testid).Run()
		cancel()
	}()

	// Docker command
	arguments, err := dockerArguments(execution.RunDir, testid)
	if err != nil {
		return internalErrorResult(execution, fmt.Sprintf("Could not get docker arguments: %s", err))
	}

	// execute in Python environment
	arguments = append(arguments, *docker_image_python)

	// call Pytest runner
	arguments = append(arguments, "python3", "-m", "pytest", "-o", "junit_family=xunit1", "-v", "--junitxml=./test-result.xml", "--doctest-glob='*.md'", "--doctest-modules")

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Args = arguments
	cmd.Dir = absRunDir

	outLogFile := filepath.Join(absRunDir, "junit.out.log")
	outFileHandle, err := os.Create(outLogFile)
	if err != nil {
		return internalErrorResult(execution, "Could not create JUnit log file in run directory")
	}
	defer outFileHandle.Close()

	errLogFile := filepath.Join(absRunDir, "junit.err.log")
	errFileHandle, err := os.Create(errLogFile)
	if err != nil {
		return internalErrorResult(execution, "Could not create JUnit error log file in run directory")
	}
	defer errFileHandle.Close()

	cmd.Stdout = LimitWriter(outFileHandle, maxFileSize)
	cmd.Stderr = LimitWriter(errFileHandle, maxFileSize)
	startTime := time.Now()
	err = cmd.Run()
	// executing JUnit might result in exit code 1 because of failed tests
	cancel()
	duration := time.Since(startTime)
	testExecutionTimeHistogram.Observe(duration.Seconds())
	if debug {
		Debug.Printf("Duration of JUnit test execution: %s", duration)
	}
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == -1 { // killed
					message := appendOutput(outFileHandle, errFileHandle, outLogFile, errLogFile, "Timeout")
					return TestResult{
						ID:            execution.ID,
						Compiled:      true,
						TestsExecuted: 1,
						TestsFailed:   1,
						Tests: []Test{
							{
								Name:    "Testfälle",
								Success: false,
								Error:   message,
							},
						},
					}
				}
			}
		}

	}

	message := appendOutput(outFileHandle, errFileHandle, outLogFile, errLogFile, "")

	reportFile, err := os.Open(filepath.Join(absRunDir, "test-result.xml"))
	if err != nil {
		message := appendOutput(outFileHandle, errFileHandle, outLogFile, errLogFile, "Could not find result of unit test execution")
		return internalErrorResult(execution, message)
	}
	// Parse XML document.
	doc, err := xmlquery.Parse(reportFile)
	if err != nil {
		message := "Could not parse result of XUnit execution.\n\n\n" + message
		return internalErrorResult(execution, message)
	}

	tests := make([]Test, 0)
	testsExecuted := 0
	testsFailed := 0
	for _, n := range xmlquery.Find(doc, "//testcase") {
		testsExecuted += 1
		testName := n.SelectAttr("name")

		errorMessage := ""
		for _, failure := range xmlquery.Find(n, "//failure") {
			errorMessage += failure.SelectAttr("message") + "\n\n" + failure.InnerText() + "\n\n"
		}
		for _, failure := range xmlquery.Find(n, "//error") {
			errorMessage += "\n\n" + failure.InnerText() + "\n\n"
		}

		success := len(errorMessage) == 0

		test := Test{
			Name:    testName,
			Success: success,
		}
		if !success {
			testsFailed += 1
			test.Error = errorMessage
		}
		tests = append(tests, test)
	}

	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		TestsExecuted: testsExecuted,
		TestsFailed:   testsFailed,
		Tests:         tests,
	}
}

type PyTestRunner struct {
}

func (t PyTestRunner) executeTest(execution Execution) TestResult {
	// JUnit test class should be available, execute test
	return executePytest(execution)
}
