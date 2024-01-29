package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Compiler types supported by the compiling service
// install enumer (go get github.com/campoy/jsonenums)
//go:generate jsonenums -type=Compiler
type Compiler int

const (
	// JavaCompiler uses the standard javac executable found in PATH
	JavaCompiler Compiler = iota
	// CCompiler uses the gcc compiler installed in PATH
	CCompiler
	// FsharpCompiler uses dotnet compile
	FsharpCompiler
	// Python compiler uses python3
	PythonCompiler
	// Matlab
	MatlabCompiler
)

// Can compile source code for a specific language
type CompilerProvider interface {
	compile(execution Execution) error
}

func compilerProvider(compiler Compiler) CompilerProvider {
	switch compiler {
	case JavaCompiler:
		return CompilerProviderJava{}
	case CCompiler:
		return CompilerProviderC{}
	case FsharpCompiler:
		return CompilerProviderFsharp{}
	case PythonCompiler:
		return CompilerProviderPython{}
	case MatlabCompiler:
		return CompilerProviderMatlab{}
	default:
		return CompilerProviderErr{compiler}
	}
}

type CompilerProviderErr struct {
	compiler Compiler
}

func (c CompilerProviderErr) compile(e Execution) error {
	return fmt.Errorf("Compiler not supported: %d", c.compiler)
}

func copyResources(execution Execution) error {
	if debug {
		Debug.Print("Copying resources")
	}
	err := copyFilesFromFolder(execution, resourcedir, true)
	if err != nil {
		return err
	}
	err = copyFilesFromFolder(execution, templateDir, false)
	if err != nil {
		return err
	}

	return nil
}
func copyFilesFromFolder(execution Execution, folderName string, overwriteUserFiles bool) error {
	absExecPath, err := filepath.Abs(execution.RunDir)
	if err != nil {
		LogError("compile", "Could not copy resource: %s", err)
		return err
	}
	absResourcePath, err := filepath.Abs(filepath.Join(execution.TestDir, folderName))
	if debug {
		Debug.Printf("Looking for resources in %s", absResourcePath)
	}
	if err != nil {
		LogError("compile", "Could not copy resource: %s", err)
		return err
	}
	return copyFiles(absExecPath, absResourcePath, overwriteUserFiles)
}

func copyFiles(destination string, source string, overwriteUserFiles bool) error {
	if stat, err := os.Stat(source); err == nil {
		if stat.IsDir() {
			if debug {
				Debug.Printf("Resource folder %s found", source)
			}
			files, err := ioutil.ReadDir(source)
			if err != nil {
				return err
			}

			if !fileExists(destination) {
				os.MkdirAll(destination, os.ModePerm)
			}

			for _, f := range files {
				if debug {
					Debug.Printf("Copying file %s", f.Name())
				}
				sourceChild := filepath.Join(source, f.Name())
				destinationChild := filepath.Join(destination, f.Name())
				copyFiles(destinationChild, sourceChild, overwriteUserFiles)
			}
		} else { // source is a single file
			if !overwriteUserFiles && fileExists(destination) {
				return nil
			}
			err = copyFile(source, destination)
			if err != nil {
				LogError("compile", "Could not copy resource: %s", err)
			}
		}
	}
	return nil
}

func fileExists(f string) bool {
	_, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func compileService() {
	for {
		execution := <-compileChannel

		if debug {
			Debug.Printf("Compiling: %+v\n", execution)
		}

		var err error

		copyResources(execution)

		if execution.AnalysisChan != nil {
			analysisChannel <- execution
		}
		if execution.ClocChan != nil {
			metricChannel <- execution
		}

		startTime := time.Now()
		err = compilerProvider(execution.Config.Compiler).compile(execution)
		duration := time.Since(startTime)
		if debug {
			Debug.Printf("Duration of compilation: %s", duration)
		}
		if err != nil {
			compileErrorCounter.Inc()
			execution.ResChan <- TestResult{
				ID:           execution.ID,
				Compiled:     false,
				CompileError: err.Error(),
			}
			continue
		}
		metricChannel <- execution
		testChannel <- execution
	}
}
