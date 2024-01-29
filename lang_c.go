package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

type CompilerProviderC struct{}

func (c CompilerProviderC) compile(execution Execution) error {
	files, err := ioutil.ReadDir(execution.RunDir)
	if err != nil {
		return fmt.Errorf("Test not found: %s", err)
	}
	arguments, err := dockerArguments(execution.RunDir, execution.ID)
	if err != nil {
		return fmt.Errorf("Could not get docker arguments: %s", err)
	}

	// Docker command
	arguments = append(arguments, *docker_image_c) // program name first
	// TODO add restrictions (e.g. memory, cpu, ...)

	// javac in Docker container
	arguments = append(arguments, "clang", "-Wall", "-Werror", "-fsanitize=address", "-fsanitize=undefined", "-g")

	arguments = append(arguments)
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".c") {
			arguments = append(arguments, name)
		}
	}

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

func executeC(execution Execution, inFile string, paramFile string, outFile string, errFile string) (err error) {
	dockerArgs := []string{"-e", "ASAN_OPTIONS=detect_leaks=1"}
	// 'stdbuf -oL' disables buffering, so that all output ends up in the output file, even if there is an error
	return executeProgram(execution, inFile, paramFile, outFile, errFile, dockerArgs, *docker_image_c, "stdbuf", "-o0", "./a.out")
}
