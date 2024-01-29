package main

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/go-errors/errors"
)

func metric(execution Execution) []ClocResult {
	runid := execution.ID + "-analysis"
	runDir := execution.RunDir

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
		return nil
	}

	wg := &sync.WaitGroup{}
	clocResult := make(chan []ClocResult)

	wg.Add(1)
	go func() {
		defer wg.Done()
		//run cloc analysis
		clocResult <- clocMetric(timeout, runid+"-cloc", absRunDir, maxMem, runDir, execution.Test, execution.Config)
	}()

	go func() {
		wg.Wait()
	}()
	//get result of the analysis
	v := <-clocResult
	return v

}

func metricService() {
	for {
		handleMetricRequest()

	}
}

func handleMetricRequest() {
	execution := <-metricChannel
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Error in metric execution: %+v\n", execution)
			fmt.Println("Recovered from error", err)
			fmt.Println(errors.Wrap(err, 2).ErrorStack())
		}
	}()
	fmt.Printf("Executing metric: %+v\n", execution)
	execution.ClocChan <- metric(execution)
}
