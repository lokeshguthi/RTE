package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type Result struct {
	XMLName xml.Name    `xml:"results"`
	Result  ClocResults `xml:"files"`
}
type ClocResults struct {
	XMLName     xml.Name     `xml:"files"`
	ClocResults []ClocResult `xml:"file"`
}

type ClocResult struct {
	XMLName  xml.Name `xml:"file" json:"-"`
	Name     string   `xml:"name,attr" json:"name,omitempty"`
	Comments string   `xml:"comment,attr" json:"comments_number,omitempty"`
	LOC      string   `xml:"code,attr" json:"loc_number,omitempty"`
}

func clocMetric(timeout int, runid string, absRunDir string, maxMem int, runDir string, test string, testConfig TestConfig) []ClocResult {
	testFile := testConfig.AllowedFiles[0] //Pick single filename in config AllowedFiles
	//run cloc analysis
	err := runCloc(timeout, runid, absRunDir, maxMem, runDir, testFile)
	if err != nil {
		fmt.Printf("Error in executing Cloc metric %s for test %s: %s\n", runid, test, err)
		return nil
	}

	//parse result
	result := parseClocResult(absRunDir, testConfig)

	if err != nil {
		fmt.Printf("Error in parsing cloc metric result %s for test %s: %s\n", runid, test, err)
		return nil
	}
	return result
}

func runCloc(timeout int, runid string, absRunDir string, maxMem int, runDir string, testFile string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer func() {
		exec.Command("docker", "stop", runid).Run()
		cancel()
	}()

	arguments := make([]string, 0)

	// Docker command
	arguments = append(arguments, "docker", "run", "--name", runid, "-i", "--rm", "-v", absRunDir+":/code", "-m", fmt.Sprintf("%dM", maxMem))

	arguments = append(arguments, *docker_image_cloc, "--quiet", "--xml", "/code/"+testFile, "exclude-dir=bin,obj,TestResults", "--by-file")
	cmd := exec.CommandContext(ctx, "docker")
	cmd.Dir = runDir
	cmd.Args = arguments

	//push xml file in respective runs folder
	outFilePath := filepath.Join(runDir, "metric_cloc.xml")
	outFileHandle, err := os.Create(outFilePath)
	if err != nil {
		err = fmt.Errorf("could not open metric output file %s: %s", outFilePath, err)
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
	errBuffer := new(bytes.Buffer)
	cmd.Stderr = errBuffer

	//execute docker
	if err = cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == -1 { // killed
					err = fmt.Errorf("timeout")
				}
			}
		}
		if errBuffer.Len() > 0 {
			err = fmt.Errorf(errBuffer.String())
		}
		return
	}
	return
}

func parseClocResult(absRunDir string, testConfig TestConfig) []ClocResult {
	//parse the result, i.e. parse the xml file

	time.Sleep(time.Second * 10)

	var clocTest Result
	//parse the XML file till there is no error in parsing
	for {
		xmlFilePath := filepath.Join(absRunDir, "metric_cloc.xml")
		xmlFile, err := os.Open(xmlFilePath)

		if err != nil {
			return nil
		}

		byteValue, err := ioutil.ReadAll(xmlFile)
		if err != nil {
			return nil
		}

		err = xml.Unmarshal(byteValue, &clocTest)
		if err != nil {

		} else {
			defer xmlFile.Close()
			break
		}
		xmlFile.Close()

	}

	return clocTest.Result.ClocResults
}
