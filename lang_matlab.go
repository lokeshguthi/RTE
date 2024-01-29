package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type CompilerProviderMatlab struct{}

func (c CompilerProviderMatlab) compile(execution Execution) error {
	return nil
}

//func executeMatlab(execution Execution, inFile string, paramFile string, outFile string, errFile string) (err error) {
//
//	files, err := ioutil.ReadDir(execution.RunDir)
//	if err != nil {
//		return fmt.Errorf("Internal Error: Could not read Test files")
//	}
//	mainFile := ""
//	if len(execution.Config.MainIs) > 0 {
//		mainFile = execution.Config.MainIs
//	} else {
//		for _, f := range files {
//			name := f.Name()
//			if strings.HasSuffix(name, ".py") {
//				mainFile = name
//			}
//		}
//		if len(mainFile) == 0 {
//			return fmt.Errorf("Keine Matlab Datei gefunden!")
//		}
//	}
//
//
//	absMainFile, err := filepath.Abs(filepath.Join(execution.RunDir, mainFile))
//	if err != nil {
//		return fmt.Errorf("Internal Error: Could not create absolute path of main file")
//	}
//
//	if finfo, err := os.Stat(absMainFile); err != nil || finfo.IsDir() {
//		return fmt.Errorf("Could not find %s (rename your program accordingly and try again)", mainFile)
//	}
//	maxMem := execution.Config.MaxMem
//	if maxMem == 0 {
//		maxMem = 100
//	}
//
//	arguments := make([]string, 0)
//
//
//	return executeProgram(execution, inFile, paramFile, outFile, errFile, arguments, *docker_image_matlab, "Matlab3", mainFile)
//}

func executeMatlabTest(execution Execution) TestResult {
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

	// execute in Matlab environment
	arguments = append(arguments, *docker_image_matlab)

	// call Pytest runner
	arguments = append(arguments, "/usr/local/MATLAB/R2018b/bin/matlab", "-nodisplay", "-sd", "/code", "-r", "disp("+execution.Config.MainIs+");exit")

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Args = arguments
	cmd.Dir = absRunDir
	// Test-Funktion aufrufen:
	//cmd.Stdin = strings.NewReader("disp("+execution.Config.MainIs + ");exit")

	outLogFile := filepath.Join(absRunDir, "matlab.out.log")
	outFileHandle, err := os.Create(outLogFile)
	if err != nil {
		return internalErrorResult(execution, "Could not create JUnit log file in run directory")
	}
	defer outFileHandle.Close()

	errLogFile := filepath.Join(absRunDir, "matlab.err.log")
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
				errorMsg := "Failed with exit code " + string(status.ExitStatus())
				if status.ExitStatus() == -1 {
					errorMsg = "Timeout"
				}
				message := appendOutput(outFileHandle, errFileHandle, outLogFile, errLogFile, errorMsg)

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

	stderr, _ := readFileToString(errLogFile)

	message := appendOutput(outFileHandle, errFileHandle, outLogFile, errLogFile, "")

	testsExecuted := 1

	lines := strings.Split(message, "\n")

	testsFailed := 1
	if strings.Contains(lines[len(lines)-3], "1") {
		testsFailed = 0
	}

	if len(stderr) > 0 {
		testsFailed = 1
	}

	tests := []Test{
		{
			Name:    execution.Config.MainIs,
			Success: testsFailed == 0,
			Error:   extractMessage(message),
		},
	}

	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		TestsExecuted: testsExecuted,
		TestsFailed:   testsFailed,
		Tests:         tests,
	}
}

func extractMessage(s string) string {
	sep := "###############"
	first := strings.Index(s, sep)
	last := strings.LastIndex(s, sep)
	if first >= 0 && last > first {
		return s[first+len(sep) : last]
	}
	return s
}

type MatlabTestRunner struct {
}

func (t MatlabTestRunner) executeTest(execution Execution) TestResult {
	// JUnit test class should be available, execute test
	return executeMatlabTest(execution)
}
