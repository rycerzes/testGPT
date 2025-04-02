package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MegaLinterSummary struct {
	Status   string
	Table    string
	ReportID string
}

func GetMegaLinterReport(githubWorkspace, workDir string) (*MegaLinterSummary, error) {
	reportPath := filepath.Join(githubWorkspace, workDir, "megalinter-reports", "megalinter-report.md")

	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MegaLinter report: %v", err)
	}

	content := string(data)

	status := "UNKNOWN"
	if strings.Contains(content, "⚠️ WARNING") {
		status = "WARNING"
	} else if strings.Contains(content, "❌ ERROR") {
		status = "ERROR"
	} else if strings.Contains(content, "✅ SUCCESS") {
		status = "SUCCESS"
	}

	lines := strings.Split(content, "\n")
	var tableLines []string
	inTable := false

	for _, line := range lines {
		if strings.HasPrefix(line, "|") {
			inTable = true
			tableLines = append(tableLines, line)
		} else if inTable && len(line) == 0 {
			break
		}
	}

	table := strings.Join(tableLines, "\n")

	reportID := fmt.Sprintf("megalinter-report-%d", os.Getpid())

	return &MegaLinterSummary{
		Status:   status,
		Table:    table,
		ReportID: reportID,
	}, nil
}

func FormatMegaLinterReport(summary *MegaLinterSummary, artifactLink string) string {
	var sb strings.Builder

	statusEmoji := "⚠️"
	if summary.Status == "SUCCESS" {
		statusEmoji = "✅"
	} else if summary.Status == "ERROR" {
		statusEmoji = "❌"
	}

	sb.WriteString(fmt.Sprintf("### %s **MegaLinter Results**\n\n", statusEmoji))

	sb.WriteString(summary.Table)

	actionsLink := fmt.Sprintf("https://github.com/%s/actions/runs/%s",
		os.Getenv("GITHUB_REPOSITORY"),
		os.Getenv("GITHUB_RUN_ID"))

	sb.WriteString(fmt.Sprintf("\n- [MegaLinter Full Report](%s)\n", actionsLink))

	// Use the provided artifact link instead of constructing one
	if artifactLink != "" {
		sb.WriteString(fmt.Sprintf("- [Download MegaLinter Report](%s)\n", artifactLink))
	}

	return sb.String()
}

func GetGitHubRunID() string {
	return os.Getenv("GITHUB_RUN_ID")
}
