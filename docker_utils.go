package main

import (
	"fmt"
	"path/filepath"
)

func dockerArguments(runDir string, executionId string) ([]string, error) {
	absExecPath, err := filepath.Abs(runDir)
	if err != nil {
		return nil, fmt.Errorf("Internal Error: Could not create absolute path of test folder")
	}

	arguments := make([]string, 0)
	arguments = append(arguments, "docker", "run", "--name", executionId, "--rm", "-v", absExecPath+":/code", "--workdir", "/code")

	return arguments, nil

}
