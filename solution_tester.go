package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mattn/go-zglob"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

func testSolutions() error {
	println("Test solutions " + testdataDir)
	configFiles, err := zglob.GlobFollowSymlinks(testdataDir + "/**/config.json")
	if err != nil {
		return err
	}
	if *testSolutionTestname != "" {
		solutionFolder := filepath.Join(testdataDir, *testSolutionTestname, "_solution")
		if _, err := os.Stat(solutionFolder); os.IsNotExist(err) {
			return fmt.Errorf("Could not find solution folder in %s", solutionFolder)
		}
		files, err := ioutil.ReadDir(solutionFolder)
		if err != nil {
			return err
		}
		err = sendFiles(*testSolutionTestname, solutionFolder, files)
		if err != nil {
			return err
		}
	} else {
		for _, configFile := range configFiles {
			folder := filepath.Dir(configFile)
			solutionFolder := filepath.Join(folder, "_solution")
			if _, err := os.Stat(solutionFolder); os.IsNotExist(err) {
				continue
			}

			//files := make([]string,0)

			testName, err := filepath.Rel(testdataDir, folder)
			if err != nil {
				return err
			}
			fmt.Println("Testing folder", solutionFolder)

			files, err := ioutil.ReadDir(solutionFolder)
			if err != nil {
				return err
			}
			err = sendFiles(testName, solutionFolder, files)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func sendFiles(testName string, solutionFolder string, files []os.FileInfo) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	bodyWriter.WriteField("test", testName)
	bodyWriter.WriteField("numfiles", strconv.Itoa(len(files)))

	for i, file := range files {
		filename := filepath.Join(solutionFolder, file.Name())

		// this step is very important
		fileWriter, err := bodyWriter.CreateFormFile("file"+strconv.Itoa(i), file.Name())
		if err != nil {
			fmt.Println("error writing to buffer")
			return err
		}

		// open file handle
		fh, err := os.Open(filename)
		if err != nil {
			fmt.Println("error opening file " + filename)
			return err
		}
		defer fh.Close()

		//iocopy
		_, err = io.Copy(fileWriter, fh)
		if err != nil {
			return err
		}
	}

	contentType := bodyWriter.FormDataContentType()

	bodyWriter.Close()

	targetUrl := "http://" + *hostname + ":" + strconv.Itoa(*port) + "/test"
	resp, err := http.Post(targetUrl, contentType, bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Request failed: %s", resp.Status)
	}

	rteResult := RteResult{}
	err = json.Unmarshal(resp_body, &rteResult)
	if err != nil {
		return err
	}
	for _, w := range rteResult.FileWarnings {
		if len(w.Warnings) > 0 {
			fmt.Printf("\n\nWarnings in %s:\n", w.File)
			for _, w2 := range w.Warnings {
				fmt.Printf("Warning in line %d: %s - %s\n%s\n", w2.BeginLine, w2.RuleSet, w2.Rule, w2.Message)
			}
		}
	}
	testResult := rteResult.TestResult
	if !testResult.Compiled {
		println("Compilation problem:")
		println(testResult.CompileError)
		return nil
	}
	if len(testResult.InternalError) > 0 {
		println("Internal error:")
		println(testResult.InternalError)
	}
	fmt.Printf("passed %d / %d tests\n", testResult.TestsExecuted-testResult.TestsFailed, testResult.TestsExecuted)
	if testResult.TestsFailed == 0 {
		return nil
	}
	for _, test := range testResult.Tests {
		if !test.Success {
			println("TEST " + test.Name)
			println(test.Error)
			if len(test.Output) > 0 {
				println("\n----Output:\n")
				println(test.Output)
			}
			if len(test.Expected) > 0 {
				println("\n----Expected:\n")
				println(test.Expected)
			}
			println("\n\n\n")
		}
	}
	return nil

}
