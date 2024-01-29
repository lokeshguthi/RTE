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
	"strings"
	"syscall"
	"time"
)

type CheckstyleWarnings struct {
	XMLName      xml.Name                 `xml:"checkstyle"`
	FileWarnings []CheckstyleFileWarnings `xml:"file"`
}

type CheckstyleFileWarnings struct {
	XMLName  xml.Name            `xml:"file"`
	File     string              `xml:"name,attr"`
	Warnings []CheckstyleWarning `xml:"error"`
}

type CheckstyleWarning struct {
	XMLName  xml.Name `xml:"error"`
	Rule     string   `xml:"source,attr"`
	Line     int      `xml:"line,attr"`
	Severity string   `xml:"severity,attr"`
	Message  string   `xml:"message,attr"`
}

func checkstyleAnalysis(pmdFile string, timeout int, runid string, absRunDir string, maxMem int, runDir string, test string) []FileWarnings {
	err := runCheckstyle(pmdFile, timeout, runid, absRunDir, maxMem, runDir)
	if err != nil {
		fmt.Printf("Error in executing checkstyle analysis %s for test %s: %s\n", runid, test, err)
		return make([]FileWarnings, 0)
	}

	//parse result
	fileWarnings, err := parseCheckstyleResult(absRunDir)
	if err != nil {
		fmt.Printf("Error in parsing checkstyle analysis result %s for test %s: %s\n", runid, test, err)
		return make([]FileWarnings, 0)
	}
	return fileWarnings
}

func runCheckstyle(checkstyleFile string, timeout int, runid string, absRunDir string, maxMem int, runDir string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer func() {
		exec.Command("docker", "stop", runid).Run()
		cancel()
	}()
	arguments := make([]string, 0)
	// Docker command
	arguments = append(arguments, "docker", "run", "--name", runid, "-i", "--rm", "-v", absRunDir+":/code", "-m", fmt.Sprintf("%dM", maxMem))

	arguments = append(arguments, "-v", checkstyleFile+":/checkstyle/checkstyle.xml")

	arguments = append(arguments, *docker_image_checkstyle, "-c", "/checkstyle/checkstyle.xml", "-f", "xml", "/code")

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Dir = runDir
	cmd.Args = arguments

	outFilePath := filepath.Join(runDir, "analysis_checkstyle.xml")
	outFileHandle, err := os.Create(outFilePath)
	if err != nil {
		err = fmt.Errorf("could not open analysis output file %s: %s", outFilePath, err)
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

func parseCheckstyleResult(absRunDir string) ([]FileWarnings, error) {
	xmlFilePath := filepath.Join(absRunDir, "analysis_checkstyle.xml")
	xmlFile, err := os.Open(xmlFilePath)

	if err != nil {
		return nil, err
	}

	defer xmlFile.Close()

	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}

	var warnings CheckstyleWarnings
	err = xml.Unmarshal(byteValue, &warnings)
	if err != nil {
		return nil, err
	}

	return convertWarnings(warnings.FileWarnings), nil
}

func convertWarnings(checkstyleFileWarnings []CheckstyleFileWarnings) []FileWarnings {
	fileWarnings := make([]FileWarnings, 0)
	for _, checkstyleFileWarning := range checkstyleFileWarnings {
		fileWarning := FileWarnings{
			File:     checkstyleFileWarning.File,
			Warnings: make([]Warning, 0),
		}
		for _, checkstyleWarning := range checkstyleFileWarning.Warnings {
			warning := Warning{
				RuleSet:   "checkstyle",
				BeginLine: checkstyleWarning.Line,
				Message:   checkstyleWarning.Message,
			}
			//remove package name
			split := strings.Split(checkstyleWarning.Rule, ".")
			warning.Rule = split[len(split)-1]

			switch checkstyleWarning.Severity {
			case "ignore":
				warning.Priority = 5
			case "info":
				warning.Priority = 4
			case "warning":
				warning.Priority = 3
			case "error":
				warning.Priority = 1
			default:
				warning.Priority = 3
			}
			fileWarning.Warnings = append(fileWarning.Warnings, warning)
		}
		fileWarnings = append(fileWarnings, fileWarning)
	}
	return fileWarnings
}
