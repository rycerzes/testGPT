package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

	// Install Keploy
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

	// Execute based on the command type
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
}

func installKeploy() error {
	fmt.Println("Installing Keploy...")

	// Download the Keploy binary
	resp, err := http.Get("https://github.com/keploy/keploy/releases/latest/download/keploy_linux_amd64.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to download Keploy: %v", err)
	}
	defer resp.Body.Close()

	// Create a temporary file to save the tarball
	tarFile, err := os.CreateTemp("", "keploy_*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tarFile.Name())

	// Save the tarball to the temporary file
	_, err = io.Copy(tarFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save tarball: %v", err)
	}
	tarFile.Close()

	// Extract the tarball
	cmd := exec.Command("tar", "xz", "-C", "/tmp", "-f", tarFile.Name())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract tarball: %v", err)
	}

	// Move keploy to /usr/local/bin
	cmd = exec.Command("sudo", "mv", "/tmp/keploy", "/usr/local/bin/keploy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to move keploy to /usr/local/bin: %v", err)
	}

	// Make keploy executable
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
