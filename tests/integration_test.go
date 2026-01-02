package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	listener.Close()

	workerPath := filepath.Join(testDir, "worker.php")

	tmpOut, err := os.CreateTemp("", "pogo_test_*")
	if err != nil {
		t.Fatal(err)
	}
	outputFile := tmpOut.Name()
	tmpOut.Close()
	os.Remove(outputFile)
	defer os.Remove(outputFile)

	caddyfileContent := fmt.Sprintf(`
	{
		auto_https off
		frankenphp
		order php_server before file_server
		pogo_queue {
			worker "%s"
			size 10
			num_threads 1
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
	defer os.Remove(tmpCaddyfile.Name())

	if _, err := tmpCaddyfile.WriteString(caddyfileContent); err != nil {
		t.Fatalf("Failed to write temp Caddyfile: %v", err)
	}
	tmpCaddyfile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "run", "--config", tmpCaddyfile.Name())

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if !waitForServer(baseURL + "/dispatch.php") {
		t.Fatalf("Server failed to start on port %d within timeout", port)
	}

	resp, err := http.Post(
		baseURL+"/dispatch.php",
		"text/plain",
		bytes.NewBufferString(outputFile),
	)
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

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

func waitForServer(url string) bool {
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
