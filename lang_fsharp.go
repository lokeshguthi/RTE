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

	xmlquery "github.com/antchfx/xquery/xml"
)

type CompilerProviderFsharp struct{}

func (c CompilerProviderFsharp) compile(execution Execution) error {
	files, err := ioutil.ReadDir(execution.RunDir)
	if err != nil {
		return fmt.Errorf("Test not found: %s", err)
	}

	// collect f# files:
	fsharpFiles := make([]string, 0)
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".fs") {
			fsharpFiles = append(fsharpFiles, name)
		}
	}

	arguments, err := dockerArguments(execution.RunDir, execution.ID)
	if err != nil {
		return fmt.Errorf("Could not get docker arguments: %s", err)
	}

	arguments = append(arguments, *docker_image_fsharp)

	// dotnet in Docker container
	arguments = append(arguments, "dotnet", "build")

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

type XUnitTestRunner struct {
}

func (t XUnitTestRunner) executeTest(execution Execution) TestResult {
	testDir := execution.getTestDir()

	// copy test files into project:
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		return TestResult{
			ID:           execution.ID,
			Compiled:     false,
			CompileError: fmt.Sprintf("Could not read test dir\n%s", err),
		}
	}

	// copy f# files:
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".fs") {
			source := filepath.Join(testDir, f.Name())
			target := filepath.Join(execution.RunDir, filepath.Base(f.Name()))
			err = copyFile(source, target)
			if err != nil {
				return TestResult{
					ID:           execution.ID,
					Compiled:     false,
					CompileError: fmt.Sprintf("Could not copy file %s\n%s", name, err),
				}
			}
		}
	}

	compileError := CompilerProviderFsharp{}.compile(execution)
	if compileError != nil {
		junitIncompatibilityCount.WithLabelValues(execution.Test).Inc()
		return TestResult{
			ID:           execution.ID,
			Compiled:     false,
			CompileError: fmt.Sprintf("Error compiling test cases  (maybe wrong name of submitted class)\n%s", compileError.Error()),
		}
	}
	return executeXUnit(execution)
}

func executeXUnit(execution Execution) TestResult {
	testid := execution.ID
	absRunDir, err := filepath.Abs(execution.RunDir)
	if err != nil {
		return internalErrorResult(execution, "Could not make path of run dir absolute")
	}

	timeout := execution.Config.Timeout
	if timeout == 0 {
		timeout = 30
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

	// execute in Java environment
	arguments = append(arguments, *docker_image_fsharp)

	// call JUnit runner
	arguments = append(arguments, "dotnet", "test", "--blame", "-p:ParallelizeTestCollections=false", "--logger", "trx;LogFileName=Results.trx")

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Args = arguments
	cmd.Dir = absRunDir

	outLogFile := filepath.Join(absRunDir, "xunit.out.log")
	outFileHandle, err := os.Create(outLogFile)
	if err != nil {
		return internalErrorResult(execution, "Could not create XUnit log file in run directory")
	}
	defer outFileHandle.Close()

	errLogFile := filepath.Join(absRunDir, "xunit.err.log")
	errFileHandle, err := os.Create(errLogFile)
	if err != nil {
		return internalErrorResult(execution, "Could not create XUnit error log file in run directory")
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
		Debug.Printf("Duration of XUnit test execution: %s", duration)
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

	if strings.Contains(message, "while executing following test") {
		// handle case where "--blame" detects the problematic test case
		lines := strings.Split(message, "\n")
		testName := "Test execution"
		for i := 0; i+1 < len(lines); i++ {
			if strings.Contains(lines[i], "while executing following test") {
				i++
				testName = lines[i]
			}
		}

		if strings.Contains(message, "Reason: Process is terminating due to StackOverflowException") {
			message = "Die Tests konnten nicht erfolgreich beendet werden.\n" +
				"Der Fehler 'StackOverflowException' deutet darauf hin, dass der Abbruchfall der Rekursion nicht erreicht wird.\n" +
				"\n\n\n" + message
		}

		return TestResult{
			ID:            execution.ID,
			Compiled:      true,
			TestsExecuted: 1,
			TestsFailed:   1,
			Tests: []Test{
				{
					Name:    testName,
					Success: false,
					Error:   message,
				},
			},
		}
	}

	reportFile, err := os.Open(filepath.Join(absRunDir, "TestResults", "Results.trx"))
	if err != nil {
		message = "Could not open Results.trx file.\n\n" + message
		return internalErrorResult(execution, message)
	}
	// Parse XML document.
	doc, err := xmlquery.Parse(reportFile)
	if err != nil {
		message := "Could not parse result of XUnit execution.\n\n\n" + message
		return internalErrorResult(execution, message)
	}

	tests := make([]Test, 0)
	for _, n := range xmlquery.Find(doc, "//UnitTestResult") {
		test := Test{
			Name:    n.SelectAttr("testName"),
			Success: n.SelectAttr("outcome") == "Passed",
		}
		if !test.Success {
			test.Error = ""
			output := xmlquery.FindOne(n, "//Output/ErrorInfo/Message")
			if output != nil {
				test.Error += output.InnerText()
			}
			stacktrace := xmlquery.FindOne(n, "//Output/ErrorInfo/StackTrace")
			if stacktrace != nil {
				test.Error += "\n\n" + stacktrace.InnerText()
			}
		}
		tests = append(tests, test)
	}

	testSummary := xmlquery.FindOne(doc, "//ResultSummary/Counters")
	testsExecuted, err := strconv.Atoi(testSummary.SelectAttr("total"))
	if err != nil {
		return internalErrorResult(execution, "Could not read number of total tests.")
	}
	failedTests, err := strconv.Atoi(testSummary.SelectAttr("failed"))
	if err != nil {
		return internalErrorResult(execution, "Could not read number of failed tests.")
	}

	for _, n := range xmlquery.Find(doc, "//RunInfo") {
		if n.SelectAttr("outcome") == "Error" {
			test := Test{
				Name:    "RunInfo",
				Success: false,
				Error:   n.InnerText(),
			}
			tests = append(tests, test)
			failedTests++
			testsExecuted++
		}
	}

	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		TestsExecuted: testsExecuted,
		TestsFailed:   failedTests,
		Tests:         tests,
	}

}
