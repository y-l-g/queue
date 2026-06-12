package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestQueueEndToEnd(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(currentFile)
	rootDir := filepath.Dir(testDir)

	binPath := filepath.Join(rootDir, "frankenphp")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Fatalf("FrankenPHP binary not found at %s. You must build it before running tests.", binPath)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on ephemeral port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	workerPath := filepath.Join(testDir, "worker.php")

	tmpOut, err := os.CreateTemp("", "pogo_test_*")
	if err != nil {
		t.Fatal(err)
	}
	outputFile := tmpOut.Name()
	if err := tmpOut.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}
	if err := os.Remove(outputFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove output file: %v", err)
	}
	defer func() {
		if err := os.Remove(outputFile); err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: failed to remove output file: %v", err)
		}
	}()

	caddyfileContent := fmt.Sprintf(`
	{
		admin off
		persist_config off
		auto_https off
		frankenphp
		order php_server before file_server
		pogo_queue {
			backend memory {
				max_messages 10
				max_total_bytes 1048576
			}
			worker "%s"
			queues default
			concurrency 1
			worker_threads 1
		}
	}

	:%d {
		root "%s"
		php_server
	}
	`, workerPath, port, testDir)

	tmpCaddyfile, err := os.CreateTemp("", "Caddyfile.*")
	if err != nil {
		t.Fatalf("Failed to create temp Caddyfile: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpCaddyfile.Name()); err != nil {
			t.Logf("Warning: failed to remove temp Caddyfile: %v", err)
		}
	}()

	if _, err := tmpCaddyfile.WriteString(caddyfileContent); err != nil {
		t.Fatalf("Failed to write temp Caddyfile: %v", err)
	}
	if err := tmpCaddyfile.Close(); err != nil {
		t.Fatalf("Failed to close temp Caddyfile: %v", err)
	}

	cmd := exec.Command(binPath, "run", "--config", tmpCaddyfile.Name())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	defer func() {
		stopProcessGroup(t, cmd, waitDone)
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 2 * time.Second}
	if !waitForServer(client, baseURL+"/dispatch.php") {
		t.Fatalf("Server failed to start on port %d within timeout", port)
	}

	statusResp, err := client.Get(baseURL + "/status.php")
	if err != nil {
		t.Fatalf("Failed to get queue status: %v", err)
	}
	defer func() {
		if err := statusResp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close status response body: %v", err)
		}
	}()
	if statusResp.StatusCode != 200 {
		body, _ := io.ReadAll(statusResp.Body)
		t.Fatalf("Expected status code 200 for queue status, got %d. Body: %s", statusResp.StatusCode, string(body))
	}
	var status map[string]any
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("Queue status returned invalid JSON: %v", err)
	}
	if _, ok := status["queues"].([]any); !ok {
		t.Fatalf("Queue status did not include queues: %#v", status)
	}

	resp, err := client.Post(
		baseURL+"/dispatch.php",
		"text/plain",
		bytes.NewBufferString(outputFile),
	)
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status code 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Dispatched") {
		t.Fatalf("Expected response body 'Dispatched', got '%s'", string(body))
	}

	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	success := false
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for worker to process message")
		case <-ticker.C:
			content, err := os.ReadFile(outputFile)
			if err == nil && string(content) == "PROCESSED" {
				success = true
				goto Done
			}
		}
	}

Done:
	if !success {
		t.Error("Worker did not process the message correctly")
	}
}

func waitForServer(client *http.Client, url string) bool {
	for i := 0; i < 50; i++ {
		resp, err := client.Get(url)
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				return false
			}
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func stopProcessGroup(t *testing.T, cmd *exec.Cmd, waitDone <-chan error) {
	t.Helper()

	signalProcessGroup(cmd, syscall.SIGTERM)
	select {
	case err := <-waitDone:
		logProcessExit(t, err)
		return
	case <-time.After(2 * time.Second):
	}

	signalProcessGroup(cmd, syscall.SIGKILL)
	select {
	case err := <-waitDone:
		logProcessExit(t, err)
	case <-time.After(5 * time.Second):
		t.Log("Timed out waiting for FrankenPHP process to exit")
	}
}

func signalProcessGroup(cmd *exec.Cmd, signal syscall.Signal) {
	if cmd.Process == nil {
		return
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, signal)
		return
	}
	_ = cmd.Process.Signal(signal)
}

func logProcessExit(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Logf("Command wait error: %v", err)
		return
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() && (status.Signal() == syscall.SIGTERM || status.Signal() == syscall.SIGKILL) {
		return
	}
	t.Logf("Command wait error: %v", err)
}
