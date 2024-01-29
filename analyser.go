package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-errors/errors"
)

func analyse(execution Execution) (fileWarnings []FileWarnings) {
	runid := execution.ID + "-analysis"
	runDir := execution.RunDir
	testDir := execution.TestDir

	timeout := execution.Config.AnalysisTimeout
	if timeout == 0 {
		timeout = 20
	}
	maxMem := execution.Config.AnalysisMaxMem
	if maxMem == 0 {
		maxMem = 100
	}

	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	result := make(chan []FileWarnings)

	pmdFile := filepath.Join(testDir, "pmd.xml")
	if fileExists(pmdFile) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result <- pmdAnalysis(pmdFile, timeout, runid+"-pmd", absRunDir, maxMem, runDir, execution.Test)
		}()
	}

	checkstyleFile := filepath.Join(testDir, "checkstyle.xml")
	if fileExists(checkstyleFile) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result <- checkstyleAnalysis(checkstyleFile, timeout, runid+"-checkstyle", absRunDir, maxMem, runDir, execution.Test)
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	fileWarnings = make([]FileWarnings, 0)
	for res := range result {
		fileWarnings = append(fileWarnings, res...)
	}

	// fix filenames of moved files
	for i, fw := range fileWarnings {
		if strings.HasPrefix(fw.File, execution.Config.UploadsDirectory) {
			fw.File, _ = filepath.Rel(execution.Config.UploadsDirectory, fw.File)
		}
		fileWarnings[i] = fw
	}

	return
}

func analysisService() {
	for {
		handleAnalysisRequest()
	}
}

func handleAnalysisRequest() {
	execution := <-analysisChannel
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Error in analysis execution: %+v\n", execution)
			fmt.Println("Recovered from error", err)
			fmt.Println(errors.Wrap(err, 2).ErrorStack())
		}
	}()
	fmt.Printf("Executing analysis: %+v\n", execution)
	execution.AnalysisChan <- analyse(execution)
}
