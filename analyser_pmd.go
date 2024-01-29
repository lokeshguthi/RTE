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

type PmdWarnings struct {
	XMLName      xml.Name       `xml:"pmd"`
	FileWarnings []FileWarnings `xml:"file"`
}

func pmdAnalysis(pmdFile string, timeout int, runid string, absRunDir string, maxMem int, runDir string, test string) []FileWarnings {
	err := runPmd(pmdFile, timeout, runid, absRunDir, maxMem, runDir)

	//parse result
	fileWarnings, parseErr := parsePmdResult(absRunDir)
	if parseErr != nil {
		//check err here because of exit codes
		if err != nil {
			fmt.Printf("Error in executing pmd analysis %s for test %s: %s\n", runid, test, err)
			return make([]FileWarnings, 0)
		}
		fmt.Printf("Error in parsing pmd analysis result %s for test %s: %s\n", runid, test, parseErr)
		return make([]FileWarnings, 0)
	}
	return fileWarnings
}

func runPmd(pmdFile string, timeout int, runid string, absRunDir string, maxMem int, runDir string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer func() {
		exec.Command("docker", "stop", runid).Run()
		cancel()
	}()
	arguments := make([]string, 0)
	// Docker command
	arguments = append(arguments, "docker", "run", "--name", runid, "-i", "--rm", "-v", absRunDir+":/code", "-m", fmt.Sprintf("%dM", maxMem))

	arguments = append(arguments, "-v", pmdFile+":/pmd/pmd.xml")

	arguments = append(arguments, *docker_image_pmd, "pmd", "-d", "/code", "-R", "/pmd/pmd.xml", "-f", "xml", "-shortnames", "-no-cache")

	cmd := exec.CommandContext(ctx, "docker")
	cmd.Dir = runDir
	cmd.Args = arguments

	outFilePath := filepath.Join(runDir, "analysis_pmd.xml")
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

func parsePmdResult(absRunDir string) ([]FileWarnings, error) {
	xmlFilePath := filepath.Join(absRunDir, "analysis_pmd.xml")
	xmlFile, err := os.Open(xmlFilePath)

	if err != nil {
		return nil, err
	}

	defer xmlFile.Close()

	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}

	var warnings PmdWarnings
	err = xml.Unmarshal(byteValue, &warnings)
	if err != nil {
		return nil, err
	}

	//replace newline in message
	for i := range warnings.FileWarnings {
		for j := range warnings.FileWarnings[i].Warnings {
			warnings.FileWarnings[i].Warnings[j].Message = strings.ReplaceAll(warnings.FileWarnings[i].Warnings[j].Message, "\n", "")
		}
	}

	return warnings.FileWarnings, nil
}
