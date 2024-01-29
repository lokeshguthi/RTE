# Remote Test Executor

The Remote Test Executor (RTE) is a program for execution of tests on a remote server.
The test cases are defined on the server and hand-ins (for example of students) can be submitted using a REST interface.
The result of the test execution is returned as JSON in machine-readable format.

## Prerequisites for Running RTE

- Docker
- The user executing RTE needs to have the right to execute the `docker` command (without sudo)
- the `openjdk:8` and `openjdk:8-jre-slim` Docker images (pull first)
- a `softech-git.informatik.uni-kl.de:5050/stats/rte-go/cdev` image (needs to be created/pulled on the server)

## The Server Process

The RTE process accepts command-line flags and environment variables for configuration.

- `-host <hostname/ip>` The address to listen on
- `-port <port>` The port to listen for test requests
- `-metricson <port>` The port to export Prometheus metrics on under the address `/metrics`
- `-basedir <path>` The base folder of the server; location of the test definitions and execution results
- `-debug` Turn debug logging on

By default, the REST-interface is not protected and can be accessed without providing user credentials.
This interface can be protected using an API-key by setting the `RTE_API_KEY` environment variable.
After starting RTE with the environment variable set, the key has to be provided in every request to the REST interface
using the `ApiKey` header field.

The base folder contains:

- the `tests` folder with the test case definitions
- the `runs` folder with the working directories of the test executions
- the `junitrunner.jar` file; the JUnit test executor with report generation

The working directories of test executions are generated as UUIDv4 identifiers and contain the uploaded files of the test,
the results of the compilation and additional outputs of the test execution.
The run folders are not cleaned after test execution and can be used to identify bugs and problems in the test execution.

## Test Case Definition

Test cases are defined by adding a folder under the `tests` directory in the base folder of the server.
Test cases are configured using a configuration in JSON format in a `config.json` file.
RTE supports two sorts of test cases: Input/Output tests and JUnit tests (only for Java test cases).
Depending on the type of test, different configuration parameters have to be given.

### IO Tests

A basic configuration of an IO test looks like this:

```json
{
  "TestType": "IOTest",
  "Compiler": "<compilertype>",
  "MainIs": "<classname>",
  "Timeout": <in seconds>,
  "MaxMem": <in MB>
}
```

Two types of compilers are supported at the moment: `JavaCompiler` and `CCompiler`.
In case the test case is a Java test, the name of the main-class has to be given as `<classname>`.
By the default, all test cases are executed with default resource restrictions of **100 MB RAM** and **100 MB swap** memory and a timeout of **10 seconds**.
These restrictions can be overwritten using the `Timeout` and `MaxMem` configuration parameters and have to be given as integer values.

The different tests are specified using a triple of `<testname>.in.txt`, `<testname>.param.txt>` and `<testname>.out.txt` files.
The content of the `.in.txt` file is piped to the stdin of the program.
The stdout of the program is piped to a result file and compared (byte-by-byte) with the content of the `.out.txt` file.
If the content matches (ignoring additional new-line characters in the program output), the test is successful.
Otherwise the test failed.
If the corresponding `.param.txt` file for an out file exists, the content of the param file is given as parameters to the executed program.
The tailing new-line characters are removed.
The expected format of a param file is a single line of text including all parameters.
The `.in.txt` and `.param.txt` files can be omitted if not needed.


### JUnit Tests

JUnit tests are only support for Java test cases.
A sample configuration looks like this:

```json
{
  "TestType": "JUnitTest",
  "Compiler": "JavaCompiler",
  "JUnitTest": "<testname>",
  "Timeout": <in seconds>,
  "MaxMem": <in MB>
}
```

All JUnit tests of the test case have to implemented in a single Java class.
The name of this class has to be given as `<testname>`.
The resource restrictions are the same as for IO tests and can be adapted in the same way.
The timeout is a hard limit of the execution time of the complete JUnit test.
If this hard limit triggers, the complete execution will be killed and no output is produced.
In order to avoid this, make sure that all tests contain a timeout such that the total sum of these timeouts is less than the hard limit specified using the `Timeout` parameter.






### Building on the server

- `git clone` this repository on the server
- Build the docker images in the docker folder using the respective Makefiles (`make` or `make build`).
