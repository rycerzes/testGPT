package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"testGPT/utils"
)

// TestReport represents the structure of a Keploy test report
type TestReport struct {
	Total   int    `yaml:"total"`
	Success int    `yaml:"success"`
	Failure int    `yaml:"failure"`
	Status  string `yaml:"status"`
}

// TestSetReport represents details for a single test set
type TestSetReport struct {
	ID          string
	PassedTests int
	FailedTests int
}

// AggregatedReport holds the final aggregated test results
type AggregatedReport struct {
	TotalTests  int    `json:"total_tests"`
	PassedTests int    `json:"passed_tests"`
	FailedTests int    `json:"failed_tests"`
	Status      string `json:"status"`
}

func main() {
	// Get environment variables
	githubWorkspace := os.Getenv("GITHUB_WORKSPACE")
	workDir := os.Getenv("WORKDIR")
	keployPath := os.Getenv("KEPLOY_PATH")
	delay := os.Getenv("DELAY")
	command := os.Getenv("COMMAND")
	containerName := os.Getenv("CONTAINER_NAME")
	buildDelay := os.Getenv("BUILD_DELAY")

	workingDir := filepath.Join(githubWorkspace, workDir)

	if err := installKeploy(); err != nil {
		log.Fatalf("Failed to install Keploy: %v", err)
	}

	if err := os.Chdir(workingDir); err != nil {
		log.Fatalf("Failed to change to working directory %s: %v", workingDir, err)
	}
	fmt.Printf("Working directory: %s\n", workingDir)
	fmt.Printf("Keploy path: %s\n", keployPath)

	// Debug: List directory contents
	fmt.Println("Directory contents:")
	dirEntries, err := os.ReadDir(".")
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}
	for _, entry := range dirEntries {
		info, _ := entry.Info()
		var sizeStr string
		if info != nil {
			sizeStr = fmt.Sprintf("%d", info.Size())
		} else {
			sizeStr = "-"
		}
		fmt.Printf("%s %s\n", entry.Name(), sizeStr)
	}

	// Debug: Check for test sets
	fmt.Printf("Checking for test-sets in %s\n", keployPath)
	checkTestSets(keployPath)

	if strings.Contains(command, "go") {
		fmt.Println("go is present.")

		runCommand("go", "mod", "download")
		runCommand("go", "build", "-o", "application")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"./application\" --delay %s --path %s\n", delay, keployPath)

		runKeployTest("./application", delay, keployPath, "", "")

	} else if strings.Contains(command, "node") {
		fmt.Println("Node is present.")

		runCommand("npm", "install")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"%s\" --delay %s --path %s\n", command, delay, keployPath)

		runKeployTest(command, delay, keployPath, "", "")

	} else if strings.Contains(command, "java") || strings.Contains(command, "mvn") {
		fmt.Println("Java is present.")

		runCommand("mvn", "clean", "install")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"%s\" --delay %s --path %s\n", command, delay, keployPath)

		runKeployTest(command, delay, keployPath, "", "")

	} else if strings.Contains(command, "python") || strings.Contains(command, "python3") {
		fmt.Println("Python is present.")

		runCommand("pip", "install", "-r", "requirements.txt")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"%s\" --delay %s --path %s\n", command, delay, keployPath)

		runKeployTest(command, delay, keployPath, "", "")

	} else if strings.Contains(command, "docker-compose") || strings.Contains(command, "docker compose") {
		fmt.Println("Docker compose is present.")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"%s\" --delay %s --path %s --containerName %s --buildDelay %s\n",
			command, delay, keployPath, containerName, buildDelay)

		runKeployTest(command, delay, keployPath, containerName, buildDelay)

	} else if strings.Contains(command, "docker") {
		fmt.Println("Docker is present.")

		fmt.Println("Test Mode Starting 🎉")
		fmt.Printf("Running: sudo -E keploy test -c \"%s\" --delay %s --path %s --buildDelay %s\n",
			command, delay, keployPath, buildDelay)

		runKeployTest(command, delay, keployPath, "", buildDelay)

	} else {
		fmt.Println("Language not found")
		fmt.Println("Test Mode Shutting 🎉")
	}

	processTestReports(githubWorkspace, workDir)
}

func installKeploy() error {
	fmt.Println("Installing Keploy...")

	// Download the Keploy binary
	resp, err := http.Get("https://github.com/keploy/keploy/releases/latest/download/keploy_linux_amd64.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to download Keploy: %v", err)
	}
	defer resp.Body.Close()

	tarFile, err := os.CreateTemp("", "keploy_*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tarFile.Name())

	_, err = io.Copy(tarFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save tarball: %v", err)
	}
	tarFile.Close()

	cmd := exec.Command("tar", "xz", "-C", "/tmp", "-f", tarFile.Name())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract tarball: %v", err)
	}

	cmd = exec.Command("sudo", "mv", "/tmp/keploy", "/usr/local/bin/keploy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to move keploy to /usr/local/bin: %v", err)
	}

	cmd = exec.Command("sudo", "chmod", "+x", "/usr/local/bin/keploy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to make keploy executable: %v", err)
	}

	fmt.Println("Keploy installed successfully 🎉")
	return nil
}

func checkTestSets(keployPath string) {
	cmd := exec.Command("find", keployPath, "-type", "d", "-name", "test-sets", "-o", "-name", "test-set-*")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error checking for test sets: %v\n", err)
		return
	}
	fmt.Println(string(output))
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runKeployTest(command, delay, keployPath, containerName, buildDelay string) {
	args := []string{"-E", "keploy", "test", "-c", command, "--delay", delay, "--path", keployPath}

	if containerName != "" {
		args = append(args, "--containerName", containerName)
	}

	if buildDelay != "" {
		args = append(args, "--buildDelay", buildDelay)
	}

	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running Keploy test: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("Exit code: %d\n", exitErr.ExitCode())
		}
	} else {
		fmt.Println("Keploy test completed successfully")
	}
}

func processTestReports(githubWorkspace, workDir string) {
	reportDir := filepath.Join(githubWorkspace, workDir, "keploy/reports/test-run-0")

	fmt.Printf("Looking for test reports in: %s\n", reportDir)

	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Keploy test reports directory not found at: %s\n", reportDir)
		listKeployFiles(filepath.Join(githubWorkspace, workDir, "keploy"))
		os.Exit(1)
	}

	var reportFiles []string
	err := filepath.Walk(reportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), "test-set-") &&
			strings.HasSuffix(filepath.Base(path), "-report.yaml") {
			reportFiles = append(reportFiles, path)
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking through report directory: %v\n", err)
		os.Exit(1)
	}

	if len(reportFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No Keploy test reports found in directory: %s\n", reportDir)
		os.Exit(1)
	}

	totalTests := 0
	passedTests := 0
	failedTests := 0
	status := "UNKNOWN"
	var testSets []TestSetReport

	fmt.Println("Processing test reports:")

	for _, reportPath := range reportFiles {
		fmt.Printf("Processing report: %s\n", reportPath)

		data, err := os.ReadFile(reportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", reportPath, err)
			continue
		}

		var report TestReport
		if err := yaml.Unmarshal(data, &report); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing YAML in file %s: %v\n", reportPath, err)
			continue
		}

		// Extract the test set ID from the filename
		base := filepath.Base(reportPath)
		testSetID := strings.TrimSuffix(base, "-report.yaml")

		testSetReport := TestSetReport{
			ID:          testSetID,
			PassedTests: report.Success,
			FailedTests: report.Failure,
		}
		testSets = append(testSets, testSetReport)

		fmt.Printf("  %s: Passed: %d, Failed: %d\n",
			testSetID, report.Success, report.Failure)

		totalTests += report.Total
		passedTests += report.Success
		failedTests += report.Failure

		if report.Status == "FAILED" {
			status = "FAILED"
		} else if status != "FAILED" && report.Status == "PASSED" {
			status = "PASSED"
		}
	}

	// Get PR details if we're in a PR context
	var prDetailsMarkdown string
	isPRContext := false
	prNumber, isPR := utils.GetPRNumberFromEnv()
	if isPR {
		isPRContext = true
		fmt.Printf("Running in PR context. PR number: %d\n", prNumber)
		client, err := utils.NewClient()
		if err != nil {
			fmt.Printf("Warning: Failed to create GitHub client: %v\n", err)
		} else {
			prDetails, err := client.GetPRDetails(prNumber)
			if err != nil {
				fmt.Printf("Warning: Failed to fetch PR details: %v\n", err)
			} else {
				prDetailsMarkdown = utils.FormatPRDetailsForComment(prDetails)
				fmt.Printf("Successfully fetched details for PR #%d\n", prNumber)
			}
		}
	} else {
		fmt.Println("Not running in a PR context, running in manual trigger mode")
	}

	// Get MegaLinter report if available
	var megaLinterMarkdown string
	megaLinterSummary, err := utils.GetMegaLinterReport(githubWorkspace, workDir)
	if err != nil {
		fmt.Printf("Warning: Failed to read MegaLinter report: %v\n", err)
	} else {
		runID := utils.GetGitHubRunID()
		megaLinterMarkdown = utils.FormatMegaLinterReport(megaLinterSummary, runID)
		fmt.Println("Successfully processed MegaLinter report")
	}

	outputDir := filepath.Join(githubWorkspace, workDir)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.MkdirAll(outputDir, 0755)
	}

	// Create a detailed report for all contexts
	var detailedReport strings.Builder
	detailedReport.WriteString("testrun summary\n")
	for _, testSet := range testSets {
		detailedReport.WriteString(fmt.Sprintf("id: %s\n", testSet.ID))
		detailedReport.WriteString(fmt.Sprintf("tests passed: %d\n", testSet.PassedTests))
		detailedReport.WriteString(fmt.Sprintf("test failed: %d\n\n", testSet.FailedTests))
	}

	detailedReportStr := detailedReport.String()

	os.WriteFile(
		filepath.Join(outputDir, "final_total_tests.out"),
		[]byte(fmt.Sprintf("COMPLETE TESTRUN SUMMARY. Total tests: %d\n", totalTests)),
		0644,
	)
	os.WriteFile(
		filepath.Join(outputDir, "final_total_passed.out"),
		[]byte(fmt.Sprintf("COMPLETE TESTRUN SUMMARY. Total test passed: %d\n", passedTests)),
		0644,
	)
	os.WriteFile(
		filepath.Join(outputDir, "final_total_failed.out"),
		[]byte(fmt.Sprintf("COMPLETE TESTRUN SUMMARY. Total test failed: %d\n", failedTests)),
		0644,
	)

	os.WriteFile(filepath.Join(outputDir, "final.out"), []byte(detailedReportStr), 0644)

	aggregatedReport := AggregatedReport{
		TotalTests:  totalTests,
		PassedTests: passedTests,
		FailedTests: failedTests,
		Status:      status,
	}

	jsonData, err := json.MarshalIndent(aggregatedReport, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating JSON report: %v\n", err)
		os.Exit(1)
	}

	os.WriteFile(filepath.Join(outputDir, "keploy_report.json"), jsonData, 0644)
	fmt.Println("Test report processing complete")

	// Create GitHub output for both PR and non-PR contexts
	var githubOutputBuilder strings.Builder
	githubOutputBuilder.WriteString("KEPLOY_REPORT<<EOF\n")
	githubOutputBuilder.WriteString("### **Keploy Test Results**\n\n")

	// Add test results summary
	githubOutputBuilder.WriteString(fmt.Sprintf("**Total Tests:** %d\n", totalTests))
	githubOutputBuilder.WriteString(fmt.Sprintf("**Total Passed:** %d\n", passedTests))
	githubOutputBuilder.WriteString(fmt.Sprintf("**Total Failed:** %d\n\n", failedTests))

	// Add PR details only if available in PR context
	if isPRContext && prDetailsMarkdown != "" {
		githubOutputBuilder.WriteString("<details>\n")
		githubOutputBuilder.WriteString("<summary>**🔍 PR Analysis**</summary>\n\n")
		githubOutputBuilder.WriteString(prDetailsMarkdown)
		githubOutputBuilder.WriteString("</details>\n\n")
	}

	// Add MegaLinter report if available
	if megaLinterMarkdown != "" {
		githubOutputBuilder.WriteString("<details>\n")
		githubOutputBuilder.WriteString("<summary>**🔍 MegaLinter Analysis**</summary>\n\n")
		githubOutputBuilder.WriteString(megaLinterMarkdown)
		githubOutputBuilder.WriteString("</details>\n\n")
	}

	githubOutputBuilder.WriteString("EOF\n")

	os.WriteFile(filepath.Join(outputDir, "github_output.txt"), []byte(githubOutputBuilder.String()), 0644)
}

func listKeployFiles(keployDir string) {
	fmt.Println("Contents of keploy directory:")
	cmd := exec.Command("find", keployDir, "-type", "f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
