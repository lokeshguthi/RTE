# User Guide

This is a short reference guide for users who want to create test cases for Exlaim.

## Overview

Test cases are defined in a Git repository (typically in one of the following groups):

- https://softech-git.informatik.uni-kl.de/exclaim-tests/
- https://pl-git.informatik.uni-kl.de/exclaim-tests/

The name of the repository is equal to the exclaim-id of the exercise.
Gitlab is set up to automatically synchronize the contents of the repository with the test server (see https://softech-git.informatik.uni-kl.de/exclaim-tests/documentation).

The repository content is structured as follows:

 - One folder per exercise sheet (folder name equals id of exercise sheet in exclaim)
    - One folder per task (folder name equals id of task in exclaim)
        - `config.json`: only mandatory file, configures the test case, options are given below
        - Files used by the testcase)
        - `resources` resources are additional files, which are added before compilation and overwrite user files
        - `template` template files are like resource files, but do not overwrite user files
        - `_solution` contains solution for the task

## config.json

The configuration defines the language to use, type of test and additional settings.

```json
{
	"Compiler": 'JavaCompiler' | 'CCompiler' | 'FsharpCompiler' | 'PythonCompiler' | 'MatlabCompiler',
	"TestType": 'IOTest' | 'JUnitTest' | 'xUnitTest' | 'PyTest' | 'Matlab',
	"MainIs": string, 
	"Timeout": int,
	"MaxMem": int,
	"AnalysisTimeout": int,
	"AnalysisMaxMem": int,
	"CompareTool": string,  
	"CompareToolArgs": string[],
	"RequiredFiles": string[],
	"AllowedFiles": string[],
	"UploadsDirectory": string
}
```
 
- `Compiler`: Compiler/Language
- `TestType`: See test types below
- `MainIs`: 
    Main class for Java, main function in Matlab, main file for Python
- `Timeout`: Timeout in seconds
- `MaxMem`: Maximum allowed memory usage in MB    
- `AnalysisTimeout`, `AnalysisMaxMem`: Limits for static analysis  
- `CompareTool`: Special script to use for comparing actual and expected output in IO-tests 
- `CompareToolArgs`: Additional arguments for the compare tool. 
    The compare tool takes these arguments first followed by the file containing the expected output. The actual input is given via standard in.
- `RequiredFiles`: List of files that must be included in upload.
- `AllowedFiles`: Regular expressions describing allowed files (each uploaded file must match one of these).
- `UploadsDirectory`: Moves uploaded files into this subdirectory.


## IO-tests

Example configuration:

```json
{
  "Compiler": "JavaCompiler",
  "TestType": "IOTest",
  "MainIs": "Range",
  "Timeout": 5,
  "MaxMem": 100,
  "CompareTool": "wildcard_compare.py"
}

```

The different tests are specified using a triple of `<testname>.in.txt`, `<testname>.param.txt>` and `<testname>.out.txt` files.
The content of the `.in.txt` file is piped to the stdin of the program.
The stdout of the program is piped to a result file and compared (byte-by-byte) with the content of the `.out.txt` file.
If the content matches (ignoring additional new-line characters in the program output), the test is successful.
Otherwise the test failed.
If the corresponding `.param.txt` file for an out file exists, the content of the param file is given as parameters to the executed program.
The tailing new-line characters are removed.
The expected format of a param file is a single line of text including all parameters.
The `.in.txt` and `.param.txt` files can be omitted if not needed.

## Junit Tests

```json
{
	"Compiler": "JavaCompiler",
	"TestType": "JUnitTest",
	"UploadsDirectory": "pic"
}
```

Runs all JUnit Tests found in provided and uploaded files.

It is a good idea to add timeouts to tests, so that Exclaim can show a useful error message in case of infinite loops:

```java
import org.junit.Rule;
import org.junit.rules.Timeout;

public class MyTests {

    @Rule
    public Timeout globalTimeout = Timeout.seconds(3);
    
    // ...
}
```


## Static analysis

If a configuration for CheckStyle or PMD (`checkstyle.xml` or `pmd.xml`) is present, the respective tool is executed on the source code and results are presented as warnings to students.