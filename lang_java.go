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

type CompilerProviderJava struct{}

// download from https://mvnrepository.com/artifact/org.junit.platform/junit-platform-console-standalone/1.5.0-M1
const junitStandaloneJar = "junit-platform-console-standalone-1.5.0-M1.jar"

func (c CompilerProviderJava) compile(execution Execution) error {
	absLibPath, err := filepath.Abs(filepath.Join(execution.TestDir, libDir))
	if err != nil {
		return fmt.Errorf("Internal Error: Could not create absolute path of lib folder")
	}

	libraries := make([]string, 2)
	libraries[0] = "."
	libraries[1] = "/jars/" + junitStandaloneJar

	// Docker command
	arguments, err := dockerArguments(execution.RunDir, execution.ID)
	if err != nil {
		return fmt.Errorf("Could not get docker arguments: %s", err)
	}
	arguments = append(arguments, "-v", filepath.Join(baseDir, junitStandaloneJar)+":/jars/"+junitStandaloneJar+":ro") // program name first
	// TODO add restrictions (e.g. memory, cpu, ...)

	if stat, err := os.Stat(absLibPath); err == nil && stat.IsDir() {
		arguments = append(arguments, "-v", absLibPath+":/libs:ro")
		libraries = append(libraries, "/libs/*")
	}

	arguments = append(arguments, *docker_image_java)

	// javac in Docker container
	arguments = append(arguments, "javac", "-d", ".", "-cp", strings.Join(libraries, ":"))

	arguments = append(arguments, "-encoding", "utf-8")

	javaFiles, err := collectFilesWithExtension(execution.RunDir, ".java")

	if err != nil {
		return fmt.Errorf("Could not find java files: %s", err)
	}
	arguments = append(arguments, javaFiles...)

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

func collectFilesWithExtension(base string, extension string) ([]string, error) {
	result := []string{}
	err := filepath.Walk(base, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == extension {
			Rel, err := filepath.Rel(base, path)
			if err != nil {
				return fmt.Errorf("Could not get relative path: %s", err)
			}
			result = append(result, Rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func executeJava(execution Execution, inFile string, paramFile string, outFile string, errFile string) (err error) {
	absLibPath, err := filepath.Abs(filepath.Join(execution.TestDir, libDir))
	if err != nil {
		return fmt.Errorf("Internal Error: Could not create absolute path of lib folder")
	}

	absMainFile, err := filepath.Abs(filepath.Join(execution.RunDir, execution.Config.MainIs+".class"))
	if err != nil {
		return fmt.Errorf("Internal Error: Could not create absolute path of main file")
	}

	if finfo, err := os.Stat(absMainFile); err != nil || finfo.IsDir() {
		return fmt.Errorf("Could not find %s (rename your program accordingly and try again)", execution.Config.MainIs+".java")
	}
	maxMem := execution.Config.MaxMem
	if maxMem == 0 {
		maxMem = 100
	}

	arguments := make([]string, 0)
	libraries := make([]string, 0)

	libraries = append(libraries, ".")
	arguments = append(arguments, "-v", filepath.Join(baseDir, junitStandaloneJar)+":/jars/"+junitStandaloneJar+":ro")

	if stat, err := os.Stat(absLibPath); err == nil && stat.IsDir() {
		arguments = append(arguments, "-v", absLibPath+":/libs:ro")
		libraries = append(libraries, "/libs/*")
	}

	return executeProgram(execution, inFile, paramFile, outFile, errFile, arguments, *docker_image_java, "java", "-cp", strings.Join(libraries, ":"), fmt.Sprintf("-Xmx%dm", maxMem), execution.Config.MainIs)
}

func executeJUnit(execution Execution) TestResult {
	testid := execution.ID
	absRunDir, err := filepath.Abs(execution.RunDir)
	if err != nil {
		return internalErrorResult(execution, "Could not make path of run dir absolute")
	}
	absLibPath, err := filepath.Abs(filepath.Join(execution.TestDir, libDir))
	if err != nil {
		return internalErrorResult(execution, "Internal Error: Could not create absolute path of lib folder")
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

	libraries := make([]string, 1)
	libraries[0] = "/jars/" + junitStandaloneJar

	// Docker command
	arguments, err := dockerArguments(execution.RunDir, testid)
	if err != nil {
		return internalErrorResult(execution, fmt.Sprintf("Could not get docker arguments: %s", err))
	}
	arguments = append(arguments, "-v", filepath.Join(baseDir, junitStandaloneJar)+":/jars/"+junitStandaloneJar+":ro")

	if stat, err := os.Stat(absLibPath); err == nil && stat.IsDir() {
		arguments = append(arguments, "-v", absLibPath+":/libs:ro")
		libraries = append(libraries, "/libs/*")
	}

	// execute in Java environment
	arguments = append(arguments, *docker_image_java)

	// call JUnit runner
	libraries = append(libraries, ".")
	arguments = append(arguments, "java", "-jar", "/jars/"+junitStandaloneJar, "-cp", ".", "--scan-classpath=.", "--reports-dir=reports", "--config=junit.platform.output.capture.stderr=true", "--config=junit.platform.output.capture.stdout=true")

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

	tests := make([]Test, 0)
	failureCount := 0

	reportFile, err := os.Open(filepath.Join(absRunDir, "reports", "TEST-junit-jupiter.xml"))
	// Parse XML document.
	doc, err := xmlquery.Parse(reportFile)
	if err != nil {
		message := "Could not parse result of JUnit execution.\n\n\n" + message
		return internalErrorResult(execution, message)
	}
	parseTestResults(doc, &failureCount, &tests)

	reportFile2, err := os.Open(filepath.Join(absRunDir, "reports", "TEST-junit-vintage.xml"))
	// Parse XML document.
	doc2, err := xmlquery.Parse(reportFile2)
	if err != nil {
		message := "Could not parse result of JUnit execution.\n\n\n" + message
		return internalErrorResult(execution, message)
	}
	parseTestResults(doc2, &failureCount, &tests)

	return TestResult{
		ID:            execution.ID,
		Compiled:      true,
		TestsExecuted: len(tests),
		TestsFailed:   failureCount,
		Tests:         tests,
	}
}

func parseTestResults(doc *xmlquery.Node, failureCount *int, tests *[]Test) {
	for _, n := range xmlquery.Find(doc, "//testcase") {
		failures := xmlquery.Find(n, "/failure")
		errors := xmlquery.Find(n, "/error")
		test := Test{
			Name:    n.SelectAttr("classname") + "." + n.SelectAttr("name"),
			Success: len(failures) == 0 && len(errors) == 0,
		}
		if !test.Success {
			*failureCount++
			test.Error = ""
			for _, o := range errors {
				test.Error += o.InnerText()
			}
			for _, o := range failures {
				test.Error += o.InnerText()
			}
			first := true
			for _, o := range xmlquery.Find(n, "/system-out") {
				if !first {
					test.Error += o.InnerText()
				}
				first = false
			}
			for _, o := range xmlquery.Find(n, "/system-err") {
				test.Error += o.InnerText()
			}
		}
		*tests = append(*tests, test)
	}
}

type JUnitTestRunner struct {
}

func (t JUnitTestRunner) executeTest(execution Execution) TestResult {
	testDir := execution.getTestDir()

	// copy over all Java files:
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		return internalErrorResult(execution, "Could not read test dir")
	}

	for _, f := range files {
		if debug {
			Debug.Printf("Copying file %s", f.Name())
		}
		sourceChild := filepath.Join(testDir, f.Name())
		destinationChild := filepath.Join(execution.RunDir, f.Name())
		copyFile(sourceChild, destinationChild)
	}

	// Compile with tests:
	compileError := CompilerProviderJava{}.compile(execution)
	if compileError != nil {
		junitIncompatibilityCount.WithLabelValues(execution.Test).Inc()
		return TestResult{
			ID:           execution.ID,
			Compiled:     false,
			CompileError: fmt.Sprintf("Error compiling test cases  (maybe wrong name of submitted class)\n%s", compileError.Error()),
		}
	}

	// remove Java files
	files, err = ioutil.ReadDir(execution.RunDir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".java") {
			_ = os.Remove(f.Name())
		}
	}

	// JUnit test class should be available, execute test
	return executeJUnit(execution)
}
