package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattn/go-zglob"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	uuid "github.com/satori/go.uuid"
)

var maxMemory = int64(5242880) // 5 MB

var baseDir = "."
var testrunDir = "runs"
var testdataDir = "tests"

// Resources are additional files, which are added before compilation and overwrite user files
var resourcedir = "resources"

// Template files are like resource files, but do not overwrite user files
var templateDir = "template"
var libDir = "lib"
var apiKey string = "etywsNj7W2GnszdpewwzZqndMypv9wcq"

type RteResult struct {
	TestResult   TestResult     `json:"test_result"`
	FileWarnings []FileWarnings `json:"file_warnings,omitempty"`
	ClocResults  []ClocResult   `json:"cloc_result"`
}

type Test struct {
	Name     string `json:"name"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Expected string `json:"expected,omitempty"`
	Output   string `json:"output,omitempty"`
}

// TestResult represents the result of executing a test on some input
type TestResult struct {
	ID            string   `json:"id"`
	Compiled      bool     `json:"compiled"`
	CompileError  string   `json:"compile_error,omitempty"`
	InternalError string   `json:"internal_error,omitempty"`
	Tests         []Test   `json:"tests"`
	TestsExecuted int      `json:"tests_executed"`
	TestsFailed   int      `json:"tests_failed"`
	MissingFiles  []string `json:"missing_files"`
	IllegalFiles  []string `json:"illegal_files"`
}

// Execution represents an execution of a test as it is channeled through the system
type Execution struct {
	ID           string
	RunDir       string
	TestDir      string
	Test         string
	Config       TestConfig
	ResChan      chan TestResult
	AnalysisChan chan []FileWarnings
	ClocChan     chan []ClocResult
}

// TestConfig represents the configuration of a test (for JSON marchalling)
type TestConfig struct {
	Compiler         Compiler
	TestType         TestType
	MainIs           string   `json:",omitempty"`
	Timeout          int      `json:",omitempty"`
	MaxMem           int      `json:",omitempty"`
	AnalysisTimeout  int      `json:",omitempty"`
	AnalysisMaxMem   int      `json:",omitempty"`
	CompareTool      string   `json:",omitempty"`
	CompareToolArgs  []string `json:",omitempty"`
	RequiredFiles    []string `json:",omitempty"`
	AllowedFiles     []string `json:",omitempty"`
	UploadsDirectory string   `json:",omitempty"`
}

type FileWarnings struct {
	XMLName  xml.Name  `xml:"file" json:"-"`
	File     string    `xml:"name,attr" json:"file"`
	Warnings []Warning `xml:"violation" json:"warnings"`
}

type Warning struct {
	XMLName   xml.Name `xml:"violation" json:"-"`
	Rule      string   `xml:"rule,attr" json:"rule,omitempty"`
	RuleSet   string   `xml:"ruleset,attr" json:"rule_set,omitempty"`
	BeginLine int      `xml:"beginline,attr" json:"begin_line,omitempty"`
	InfoUrl   string   `xml:"externalInfoUrl,attr" json:"info_url,omitempty"`
	Priority  int      `xml:"priority,attr" json:"priority,omitempty"`
	Message   string   `xml:",chardata" json:"message,omitempty"`
}

var compileChannel = make(chan Execution)
var testChannel = make(chan Execution)
var analysisChannel = make(chan Execution)
var metricChannel = make(chan Execution)

func (exec *Execution) getTestDir() string {
	return filepath.Join(testdataDir, filepath.Clean(exec.Test))
}

func returnRteResult(w http.ResponseWriter, res *RteResult) {
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

func returnTestResult(w http.ResponseWriter, res *TestResult) {
	enc := json.NewEncoder(w)
	rteResult := RteResult{
		TestResult: *res,
	}
	enc.Encode(rteResult)
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string

	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)

	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))

	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}

	// Return the request as a string
	return strings.Join(request, "\n")
}

func FormValueFlexible(r *http.Request, key string) string {
	if r.Form == nil {
		r.ParseMultipartForm(maxMemory)
	}
	if vs := r.Form[key]; len(vs) > 0 {
		return vs[0]
	}
	if vs := r.MultipartForm.File[key]; vs != nil {
		file, err := vs[0].Open()
		if err != nil {
			fmt.Printf("FormValueFlexible error %v\n", err)
			return ""
		}
		byteContainer, err := ioutil.ReadAll(file)
		return string(byteContainer)
	}

	return ""
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		LogError("upload", "Rejected POST request from %s", r.RemoteAddr)
		return
	}

	accessCounter.Inc()
	println("handleTest ######################################################")
	println(formatRequest(r))
	testid := uuid.NewV4()

	if apiKey != "" {
		sentApiKey := r.Header.Get("ApiKey")
		if sentApiKey != apiKey {
			w.WriteHeader(http.StatusForbidden)
			LogError("upload", "Invalid or missing ApiKey")
			return
		}
	}

	r.ParseMultipartForm(maxMemory)
	testref := filepath.Clean(FormValueFlexible(r, "test"))
	if testref == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Parameter 'test' required!")
		LogError("upload", "Missing parameter 'test'")
		return
	}
	testdir := filepath.Join(testdataDir, testref)
	if stat, err := os.Stat(testdir); err != nil || !stat.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Test not found!\n")
		LogError("upload", "Test not found: %s", testref)
		return
	}

	configfile, err := os.Open(filepath.Join(testdir, "config.json"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Config for test not found: %s\n", testdir)
		LogError("upload", "Test is missing config file: %s", testref)
		return
	}
	defer configfile.Close()
	dec := json.NewDecoder(configfile)
	var testConfig TestConfig
	err = dec.Decode(&testConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error reading test configuration: %s (%s)\n", testdir, err)
		LogError("upload", "Error in test configuration; %s (%s)", testdir, err)
		return
	}

	rundir := filepath.Join(testrunDir, testid.String())
	err = os.MkdirAll(rundir, 0777)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		LogError("upload", "Could not create test folder: %s", rundir)
		return
	}
	uploadFolder := filepath.Join(rundir, testConfig.UploadsDirectory)
	err = os.MkdirAll(uploadFolder, os.ModePerm)

	numfilesStr := FormValueFlexible(r, "numfiles")

	if len(numfilesStr) > 0 {

		numfiles, err := strconv.Atoi(numfilesStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			LogError("upload", "Error parsing number of files: %s", err)
			return
		}

		//check for required and allowed files
		if len(testConfig.AllowedFiles) > 0 || len(testConfig.RequiredFiles) > 0 {
			//create list of file names
			files := make([]string, numfiles)
			for i := 0; i < numfiles; i++ {
				_, header, err := r.FormFile(fmt.Sprintf("file%d", i))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					LogError("upload", "Error reading file from request: %s", err)
					return
				}
				files[i] = header.Filename
			}

			//check if all required files are uploaded
			if len(testConfig.RequiredFiles) > 0 {
				var missingFiles []string
				for _, required := range testConfig.RequiredFiles {
					missing := true
					for _, file := range files {
						if required == file {
							missing = false
							break
						}
					}
					if missing {
						missingFiles = append(missingFiles, required)
					}
				}
				if len(missingFiles) > 0 {
					res := TestResult{
						ID:           testid.String(),
						Compiled:     false,
						MissingFiles: missingFiles,
					}
					returnTestResult(w, &res)
					return
				}
			}
			//check if all files match a regexp
			if len(testConfig.AllowedFiles) > 0 {
				var regex []*regexp.Regexp
				var illegalFiles []string
				for _, allowed := range testConfig.AllowedFiles {
					r, err := regexp.Compile(allowed)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(err.Error()))
						LogError("matching", "Error parsing regular expression %s in test %s", allowed, testref)
						return
					}
					regex = append(regex, r)
				}
				for _, file := range files {
					matches := false
					for _, r := range regex {
						if r.MatchString(file) {
							matches = true
							break
						}
					}
					if !matches {
						illegalFiles = append(illegalFiles, file)
						fmt.Printf("File %s not allowed for test %s\n", file, testref)
					}
				}
				if len(illegalFiles) > 0 {
					res := TestResult{
						ID:           testid.String(),
						Compiled:     false,
						IllegalFiles: illegalFiles,
					}
					returnTestResult(w, &res)
					return
				}
			}
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			LogError("upload", "Could not create upload folder %s: %s", uploadFolder, err)
			return
		}

		// copy files into run directory
		for i := 0; i < numfiles; i++ {
			file, header, err := r.FormFile(fmt.Sprintf("file%d", i))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				LogError("upload", "Error reading file from request: %s", err)
				return
			}
			filename := header.Filename
			relfilename := filepath.Join(uploadFolder, filename)
			f, err := os.OpenFile(relfilename, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				LogError("upload", "Could not open target file for writing: %s", relfilename)
				return
			}
			_, err = io.Copy(f, file)
			f.Close()
			file.Close()
		}
	} else {
		// copy code to test dir:
		code := FormValueFlexible(r, "code")
		filename := FormValueFlexible(r, "filename")

		relfilename := filepath.Join(uploadFolder, filename) //this is the uploadfile
		f, err := os.OpenFile(relfilename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			LogError("upload", "Could not open target file for writing: %s", relfilename)
			return
		}
		f.WriteString(code)
		f.Close()

	}

	rteResult := RteResult{}

	// send test execution into the pipeline
	resChan := make(chan TestResult)
	clocResultChannel := make(chan []ClocResult)
	execution := Execution{
		ID:       testid.String(),
		RunDir:   rundir,
		TestDir:  testdir,
		Test:     testref,
		Config:   testConfig,
		ResChan:  resChan,
		ClocChan: clocResultChannel,
	}

	//run static analysis if rule file exists
	analysisResultChannel := make(chan []FileWarnings)
	if fileExists(filepath.Join(testdir, "pmd.xml")) || fileExists(filepath.Join(testdir, "checkstyle.xml")) {
		execution.AnalysisChan = analysisResultChannel
	} else {
		close(analysisResultChannel)
	}

	compileChannel <- execution
	rteResult.FileWarnings = <-analysisResultChannel
	rteResult.TestResult = <-resChan
	defer func() {
		rteResult.ClocResults = <-clocResultChannel
		returnRteResult(w, &rteResult)
	}()
	returnRteResult(w, &rteResult)
}

type ListResult struct {
	Success bool
	Tests   []string
}

func handleListTests(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusNotFound)
		LogError("listing", "Rejected POST request to list from %s", r.RemoteAddr)
		return
	}

	if apiKey != "" {
		sentApiKey := r.Header.Get("ApiKey")
		if sentApiKey != apiKey {
			w.WriteHeader(http.StatusForbidden)
			LogError("listing", "Invalid or missing ApiKey")
			return
		}
	}

	configFiles, err := zglob.GlobFollowSymlinks(testdataDir + "/**/config.json") // using this instead of the builtin Glob which does not support '**'
	if err != nil {
		handleError(w, err)
		LogError("listing", "Could not find tests: %s", err)
		return
	}
	fileNames := make([]string, 0, len(configFiles))
	for _, file := range configFiles {
		test, err := filepath.Rel(testdataDir, filepath.Dir(file))
		if err != nil {
			handleError(w, err)
			LogError("listing", "Could not create relative path for config file %s", file)
			return
		}
		fileNames = append(fileNames, test)
	}
	res := ListResult{
		Success: true,
		Tests:   fileNames,
	}
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

func handleError(w http.ResponseWriter, err error) {
	res := ListResult{
		Success: false,
		Tests:   []string{},
	}
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

var (
	hostname                = flag.String("host", "", "hostname the program should bind to")
	port                    = flag.Int("port", 8080, "port the program should listen on")
	metricsAddress          = flag.String("metricson", ":3003", "port the program should expose Prometheus metrics on")
	baseDirFlag             = flag.String("basedir", ".", "the base directory (tests, runs, ...)")
	debugFlag               = flag.Bool("debug", false, "Turn on debug logging")
	testSolutionFlag        = flag.Bool("testSolution", false, "Test the solutions stored in the test directory.")
	testSolutionTestname    = flag.String("testName", "", "Testname of a specific solution to test. Use with 'testSolution'.")
	contextPath             = flag.String("contextPath", "", "A prefix that is used for all URLs on the server.")
	docker_image_python     = flag.String("docker_image_python", "softech-git.informatik.uni-kl.de:5050/stats/rte-go/pydev", "Image to use for Python tests.")
	docker_image_matlab     = flag.String("docker_image_matlab", "matlab", "Image to use for Matlab tests.")
	docker_image_fsharp     = flag.String("docker_image_fsharp", "softech-git.informatik.uni-kl.de:5050/stats/rte-go/fsharpdev", "Image to use for F# tests.")
	docker_image_java       = flag.String("docker_image_java", "openjdk:12", "Image to use for Java tests.")
	docker_image_c          = flag.String("docker_image_c", "softech-git.informatik.uni-kl.de:5050/stats/rte-go/cdev", "Image to use for C tests.")
	docker_image_checkstyle = flag.String("docker_image_checkstyle", "softech-git.informatik.uni-kl.de:5050/stats/rte-go/checkstyle", "Docker image for checkstyle analysis")
	docker_image_cloc       = flag.String("docker_image_cloc", "aldanial/cloc", "Docker image for cloc analysis")
	docker_image_pmd        = flag.String("docker_image_pmd", "softech-git.informatik.uni-kl.de:5050/stats/rte-go/pmd", "Docker image for PMD analysis")
	testdata_folder         = flag.String("testdata_folder", "tests", "Folder where tests are stored. If this is not an absolute path it is interpreted relative to the basedir.")
	testrun_folder          = flag.String("testrun_folder", "runs", "Folder where individual test runs are stored. If this is not an absolute path it is interpreted relative to the basedir.")
	tools_folder            = flag.String("tools_folder", "_tools", "Folder where individual test runs are stored. If this is not an absolute path it is interpreted relative to the testdata_folder.")
	clean_testruns          = flag.Bool("clean_testruns", false, "Remove test run folders after executing tests.")
)

var debug = false

func main() {
	InitLoggers(os.Stdout, os.Stdout)
	InitMonitoring()

	flag.Parse()
	debug = *debugFlag

	apiKey = os.Getenv("RTE_API_KEY")
	if debug {
		Debug.Printf("Using API key: %s", apiKey)
	}

	absBaseDir, err := filepath.Abs(*baseDirFlag)
	if err != nil {
		panic(err)
	}
	baseDir = absBaseDir
	if filepath.IsAbs(*testrun_folder) {
		testrunDir = *testrun_folder
	} else {
		testrunDir = filepath.Join(absBaseDir, *testrun_folder)
	}
	if filepath.IsAbs(*testdata_folder) {
		testdataDir = *testdata_folder
	} else {
		testdataDir = filepath.Join(absBaseDir, *testdata_folder)
	}
	println("Setting testdataDir to ", testdataDir)

	if *testSolutionFlag {
		err := testSolutions()
		if err != nil {
			panic(err)
		}
		return
	}

	Info.Printf("Remote Test Executor starting up...\n")

	// start 10 compile services
	for i := 0; i < 10; i++ {
		go compileService()
	}
	// start 15 testing services
	for i := 0; i < 10; i++ {
		go testService()
	}
	// start 15 analysis services
	for i := 0; i < 10; i++ {
		go analysisService()
	}
	//start 10 metric services
	for i := 0; i < 10; i++ {
		go metricService()
	}

	if debug {
		Debug.Println("Registering /test hook")
	}
	http.HandleFunc(*contextPath+"/test", handleTest)

	if debug {
		Debug.Println("Registering /listtests hook")
	}
	http.HandleFunc(*contextPath+"/listtests", handleListTests)

	Info.Println("done")

	Info.Printf("Exposing metrics on '%s'\n", *metricsAddress)
	go func() {
		LogError("startup", "Error trying to bind metrics port: %s\n", http.ListenAndServe(*metricsAddress, promhttp.Handler()))
	}()

	Info.Printf("Listening on '%s:%d'\n", *hostname, *port)
	LogError("startup", "Error trying to bind: %s\n", http.ListenAndServe(fmt.Sprintf("%s:%d", *hostname, *port), nil))
}
