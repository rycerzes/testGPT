package utils

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v70/github"
	"golang.org/x/oauth2"
)

type PRDetails struct {
	Number       int
	Title        string
	State        string
	Author       string
	CreatedAt    string
	UpdatedAt    string
	ChangedFiles []FileDetails
}

type FileDetails struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type Client struct {
	client *github.Client
	owner  string
	repo   string
}

func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	repoFullName := os.Getenv("GITHUB_REPOSITORY")
	if repoFullName == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is not set")
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", repoFullName)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &Client{
		client: client,
		owner:  parts[0],
		repo:   parts[1],
	}, nil
}

func (c *Client) GetPRDetails(prNumber int) (*PRDetails, error) {
	ctx := context.Background()

	pr, _, err := c.client.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR #%d: %v", prNumber, err)
	}

	files, _, err := c.client.PullRequests.ListFiles(ctx, c.owner, c.repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for PR #%d: %v", prNumber, err)
	}

	fileDetails := make([]FileDetails, 0, len(files))
	for _, file := range files {
		fileDetails = append(fileDetails, FileDetails{
			Filename:  file.GetFilename(),
			Status:    file.GetStatus(),
			Additions: file.GetAdditions(),
			Deletions: file.GetDeletions(),
			Patch:     file.GetPatch(),
		})
	}

	return &PRDetails{
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		State:        pr.GetState(),
		Author:       pr.GetUser().GetLogin(),
		CreatedAt:    pr.GetCreatedAt().Format("2006-01-02 15:04:05"),
		UpdatedAt:    pr.GetUpdatedAt().Format("2006-01-02 15:04:05"),
		ChangedFiles: fileDetails,
	}, nil
}

func FormatPRDetailsForComment(pr *PRDetails) string {
	var sb strings.Builder

	sb.WriteString("### **Test Results Summary**\n\n")
	sb.WriteString(fmt.Sprintf("**PR Number**: #%d\n", pr.Number))
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", pr.Title))
	sb.WriteString(fmt.Sprintf("**State**: %s\n", pr.State))
	sb.WriteString(fmt.Sprintf("**Author**: %s\n", pr.Author))
	sb.WriteString(fmt.Sprintf("**Created At**: %s\n", pr.CreatedAt))
	sb.WriteString(fmt.Sprintf("**Updated At**: %s\n\n", pr.UpdatedAt))

	sb.WriteString("<details>\n")
	sb.WriteString("<summary>**🔍 PR Analysis Details**</summary>\n\n")
	sb.WriteString("#### **Changed Files**\n\n")

	for _, file := range pr.ChangedFiles {
		sb.WriteString(fmt.Sprintf("- **File**: `%s` (%s)\n", file.Filename, file.Status))
		sb.WriteString(fmt.Sprintf("  - **Additions**: %d\n", file.Additions))
		sb.WriteString(fmt.Sprintf("  - **Deletions**: %d\n\n", file.Deletions))
	}

	sb.WriteString("</details>\n\n")
	return sb.String()
}

func GetPRNumberFromEnv() (int, bool) {
	ref := os.Getenv("GITHUB_REF")
	if ref == "" || !strings.HasPrefix(ref, "refs/pull/") {
		return 0, false
	}

	parts := strings.Split(ref, "/")
	if len(parts) < 3 {
		return 0, false
	}

	prNum, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, false
	}

	return prNum, true
}
