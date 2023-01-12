package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
)

// Output to store test results from specReport
type Output struct {
	State string
	Name  string
	Type  string
	Time  float64
}

// GoTestJSONRowData to store logs in JSON format
type GoTestJSONRowData struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

// ProcessedTestdata to store formatted data
type ProcessedTestdata struct {
	TotalTestTime    string
	TestDate         string
	FailedTests      int
	PassedTests      int
	SkippedTests     int
	TestSummary      []TestOverview
	TestSuiteSummary map[string]TestSuiteDetails
}

// TestSuiteDetails to store test suite details
type TestSuiteDetails struct {
	TestSuiteName string
	ElapsedTime   float64
	Status        string
	FailedTests   int
	PassedTests   int
	SkippedTests  int
}

// TestDetails to store test case details
type TestDetails struct {
	TestSuiteName string
	TestCaseName  string
	ElapsedTime   float64
	Status        string
}

// TestOverview to store structured test case details per test suite
type TestOverview struct {
	TestSuiteName string
	TestCases     []TestDetails
}

func main() {
	rootCmd := initCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var fileName string
var testSuiteName string
var OS string
var htmlReport string
var bucketName string

func initCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "go-test-html-report",
		Short: "go-test-html-report generates a html report of go-test logs",
		RunE: func(cmd *cobra.Command, args []string) (e error) {
			file, _ := cmd.Flags().GetString("file")
			testData := make([]GoTestJSONRowData, 0)
			if file != "" {
				testData = ReadLogsFromFile(file)
			} else {
				log.Println("Log file not passed")
				os.Exit(1)
			}
			OS = strings.Split(strings.Split(file, "_")[1], ".")[0]
			processedTestdata := ProcessTestData(testData)
			GenerateHTMLReport(processedTestdata.TotalTestTime,
				processedTestdata.TestDate,
				processedTestdata.TestSummary,
				processedTestdata.TestSuiteSummary,
			)
			log.Println("Report Generated")
			LaunchHTML()
			// upload test result
			UploadReport(htmlReport)
			// upload result result for all OS
			UploadReport("launchresults.html")
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVarP(
		&fileName,
		"file",
		"f",
		"",
		"set the file of the go test json logs")
	return rootCmd
}

// ReadLogsFromFile the logs generated from the test run script
func ReadLogsFromFile(fileName string) []GoTestJSONRowData {
	file, err := os.Open(fileName)
	if err != nil {
		log.Println("error opening file: ", err)
		os.Exit(1)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Println("error closing file: ", err)
			os.Exit(1)
		}
	}()

	scanner := bufio.NewScanner(file)
	rowData := make([]GoTestJSONRowData, 0)

	for scanner.Scan() {
		row := GoTestJSONRowData{}

		// unmarshall each line to GoTestJSONRowData
		err := json.Unmarshal([]byte(scanner.Text()), &row)
		if err != nil {
			log.Println("error to unmarshall test logs: ", err)
			os.Exit(1)
		}
		rowData = append(rowData, row)
	}

	if err := scanner.Err(); err != nil {
		log.Println("error with file scanner: ", err)
		os.Exit(1)
	}
	return rowData
}

// ProcessTestData reads json formatted data and stores into respective structures
func ProcessTestData(rowData []GoTestJSONRowData) ProcessedTestdata {
	testDetails := map[string]TestDetails{}
	testSuiteDetails := map[string]TestSuiteDetails{}
	passedTests := 0
	failedTests := 0
	skippedTests := 0
	// Loop through logs
	for _, r := range rowData {
		if r.Test != "" {
			testSuiteName = r.Test
			var jsonMap Output

			// Extract valid data from the logs
			if strings.Contains(r.Output, "k3s test") || strings.Contains(r.Output, "rke2 test") {
				output2 := strings.LastIndex(r.Output, "}")
				output2 = output2 + 1
				json.Unmarshal([]byte(strings.TrimSpace(r.Output[:output2])), &jsonMap)
			}

			// Check if there is a valid test case
			if len(jsonMap.Name) > 1 {
				if jsonMap.State == "failed" || jsonMap.State == "passed" || jsonMap.State == "skipped" {
					testDetails[r.Test+jsonMap.Name] = TestDetails{
						TestSuiteName: r.Test,
						TestCaseName:  jsonMap.Name,
						ElapsedTime:   jsonMap.Time / (1000 * 1000 * 1000 * 60),
						Status:        jsonMap.State,
					}
				}

				if jsonMap.State == "failed" {
					failedTests = failedTests + 1
				} else if jsonMap.State == "passed" {
					passedTests = passedTests + 1
				} else if jsonMap.State == "skipped" {
					skippedTests = skippedTests + 1
				}
			}
		} else {
			if r.Action == "fail" || r.Action == "pass" || r.Action == "skip" {
				testSuiteDetails[testSuiteName] = TestSuiteDetails{
					TestSuiteName: testSuiteName,
					ElapsedTime:   r.Elapsed / 60,
					Status:        r.Action,
					FailedTests:   failedTests,
					PassedTests:   passedTests,
					SkippedTests:  skippedTests,
				}
				passedTests = 0
				failedTests = 0
				skippedTests = 0

			}
		}
	}

	testSummary := make([]TestOverview, 0)
	for key := range testSuiteDetails {
		testCases := make([]TestDetails, 0)
		for _, t2 := range testDetails {
			if t2.TestSuiteName == key {
				testCases = append(testCases, t2)
			}
		}
		// testSummary holds testSuiteName and testCases
		testSummary = append(testSummary, TestOverview{
			TestSuiteName: key,
			TestCases:     testCases,
		})
	}
	// determine total test time
	totalTestTime := ""
	if rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() < 60 {
		totalTestTime = fmt.Sprintf("%f s", rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds())
	} else {
		min := int(math.Trunc(rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() / 60))
		seconds := int(math.Trunc((rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Minutes() - float64(min)) * 60))
		totalTestTime = fmt.Sprintf("%dm:%ds", min, seconds)
	}

	// determine test date
	testDate := rowData[0].Time.Format(time.RFC850)

	return ProcessedTestdata{
		TotalTestTime:    totalTestTime,
		TestDate:         testDate,
		FailedTests:      failedTests,
		PassedTests:      passedTests,
		SkippedTests:     skippedTests,
		TestSummary:      testSummary,
		TestSuiteSummary: testSuiteDetails,
	}
}

// GenerateHTMLReport generates report in the form rke2_<OS>_results.html
func GenerateHTMLReport(totalTestTime, testDate string, testSummary []TestOverview, testSuiteDetails map[string]TestSuiteDetails) {
	totalPassedTests := 0
	totalFailedTests := 0
	totalSkippedTests := 0
	templates := make([]template.HTML, 0)
	for _, testSuite := range testSuiteDetails {
		totalPassedTests = totalPassedTests + testSuite.PassedTests
		totalFailedTests = totalFailedTests + testSuite.FailedTests
		totalSkippedTests = totalSkippedTests + testSuite.SkippedTests
		// display testSuiteName
		htmlString := template.HTML("<div type=\"button\" class=\"collapsible\">\n")
		packageInfoTemplateString := template.HTML("")

		packageInfoTemplateString = "<div>{{.testsuiteName}}</div>" + "\n" + "<div>Run Time: {{.elapsedTime}}m</div> " + "\n"
		packageInfoTemplate, err := template.New("packageInfoTemplate").Parse(string(packageInfoTemplateString))
		if err != nil {
			log.Println("error parsing package info template", err)
			os.Exit(1)
		}

		var processedPackageTemplate bytes.Buffer
		err = packageInfoTemplate.Execute(&processedPackageTemplate, map[string]string{
			"testsuiteName": testSuite.TestSuiteName + "_" + OS,
			"elapsedTime":   fmt.Sprintf("%.2f", testSuite.ElapsedTime),
		})
		if err != nil {
			log.Println("error applying package info template: ", err)
			os.Exit(1)
		}

		if testSuite.Status == "pass" {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout successBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		} else if testSuite.Status == "fail" {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout failBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		} else {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout skipBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		}

		htmlString = htmlString + "\n" + packageInfoTemplateString
		testInfoTemplateString := template.HTML("")

		// display testCases
		for _, pt := range testSummary {
			testHTMLTemplateString := template.HTML("")
			if len(pt.TestCases) == 0 {
				log.Println("Test run failed for ", pt.TestSuiteName, "no testcases were executed")
				continue
			}
			if pt.TestSuiteName == testSuite.TestSuiteName {
				if testSuite.FailedTests == 0 {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout successBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				} else if testSuite.FailedTests > 0 {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout failBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				} else if testSuite.SkippedTests > 0 {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout skipBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				}
				testTemplate, err := template.New("Test").Parse(string(testHTMLTemplateString))
				if err != nil {
					log.Println("error parsing tests template: ", err)
					os.Exit(1)
				}
				var processedTestTemplate bytes.Buffer
				err = testTemplate.Execute(&processedTestTemplate, map[string]string{
					"testName": "TestCases",
				})
				if err != nil {
					log.Println("error applying test template: ", err)
					os.Exit(1)
				}

				testHTMLTemplateString = template.HTML(processedTestTemplate.Bytes())
				testCaseHTMLTemplateString := template.HTML("")

				for _, tC := range pt.TestCases {
					testCaseHTMLTemplateString = "<div>{{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}m</div>"
					testCaseTemplate, err := template.New("testCase").Parse(string(testCaseHTMLTemplateString))
					if err != nil {
						log.Println("error parsing test case template: ", err)
						os.Exit(1)
					}

					var processedTestCaseTemplate bytes.Buffer
					err = testCaseTemplate.Execute(&processedTestCaseTemplate, map[string]string{
						"testName":    tC.TestCaseName,
						"elapsedTime": fmt.Sprintf("%f", tC.ElapsedTime),
					})
					if err != nil {
						log.Println("error applying test case template: ", err)
						os.Exit(1)
					}

					if tC.Status == "passed" {
						testCaseHTMLTemplateString = "<div class=\"testCardLayout successBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"

					} else if tC.Status == "failed" {
						testCaseHTMLTemplateString = "<div  class=\"testCardLayout failBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"

					} else {
						testCaseHTMLTemplateString = "<div  class=\"testCardLayout skipBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"
					}
					testHTMLTemplateString = testHTMLTemplateString + "\n" + testCaseHTMLTemplateString
				}
				testHTMLTemplateString = testHTMLTemplateString + "\n" + "</div>" + "\n" + "</div>"
				testInfoTemplateString = testInfoTemplateString + "\n" + testHTMLTemplateString
			}
		}
		htmlString = htmlString + "\n" + "<div class=\"collapsibleHeadingContent\">\n" + testInfoTemplateString + "\n" + "</div>"
		htmlString = htmlString + "\n" + "</div>"
		templates = append(templates, htmlString)
	}
	reportTemplate := template.New("report-template.html")
	reportTemplateData, err := Asset("report-template.html")
	if err != nil {
		log.Println("error retrieving report-template.html: ", err)
		os.Exit(1)
	}

	report, err := reportTemplate.Parse(string(reportTemplateData))
	if err != nil {
		log.Println("error parsing report-template.html: ", err)
		os.Exit(1)
	}

	var processedTemplate bytes.Buffer
	type templateData struct {
		HTMLElements  []template.HTML
		FailedTests   int
		PassedTests   int
		SkippedTests  int
		TotalTestTime string
		TestDate      string
	}

	err = report.Execute(&processedTemplate,
		&templateData{
			HTMLElements:  templates,
			FailedTests:   totalFailedTests,
			PassedTests:   totalPassedTests,
			SkippedTests:  totalSkippedTests,
			TotalTestTime: totalTestTime,
			TestDate:      testDate,
		},
	)
	if err != nil {
		log.Println("error applying report-template.html: ", err)
		os.Exit(1)
	}
	htmlReport = strings.Split(fileName, ".")[0] + "_results.html"
	bucketName = strings.Split(htmlReport, "_")[0] + "-results"
	fmt.Println(bucketName)
	err = ioutil.WriteFile(htmlReport, processedTemplate.Bytes(), 0644)
	if err != nil {
		log.Println("error writing report.html file: ", err)
		os.Exit(1)
	}
}

// UploadReport used to upload test result to s3 bucket
func UploadReport(filename string) {
	file, err := os.Open(filename)

	if err != nil {
		log.Println("error opening file: ", err)
		os.Exit(1)
	}
	report1 := s3.New(session.New(), &aws.Config{Region: aws.String("us-east-2")})
	params1 := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(filename),
		ContentType: aws.String("text/html"),
		Body:        file,
	}
	_, err = report1.PutObject(params1)
	if err != nil {
		fmt.Println("Upload failed due to: ", err)
		os.Exit(1)
	}

	fmt.Println(filename, "is uploaded")

}

// LaunchHTML to reflect the changes dynamically to the matrix
func LaunchHTML() {
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":80", nil)
}
