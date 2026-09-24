package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"well-ambient/internal/config"
)

func TestServerShutdownWithActiveSSEClient(t *testing.T) {
	setupServerTestDB(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: port,
		},
	}
	srv := NewServer(cfg, "")

	token := superAdminToken(t, "testadmin", "Test Admin", []string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ctx)
	}()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := client.Get(serverURL + "/live")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("server did not become ready")
	}

	// Establish SSE connection
	sseClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, serverURL+"/api/notifications/sse?user_id=testadmin", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	sseResp, err := sseClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected SSE status code: %d", sseResp.StatusCode)
	}

	reader := bufio.NewReader(sseResp.Body)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("failed to read initial SSE event: %v", err)
	}

	shutdownStart := time.Now()
	cancel()

	select {
	case err := <-serveDone:
		shutdownDuration := time.Since(shutdownStart)
		if shutdownDuration > 2*time.Second {
			t.Fatalf("Server shutdown took %v, which is too long (expected < 2s)", shutdownDuration)
		}
		if err != nil {
			t.Fatalf("Serve returned error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Server shutdown timed out (still blocked)")
	}
}

func TestServerShutdownDrainsInFlightWriteRequests(t *testing.T) {
	setupServerTestDB(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: port,
		},
	}
	srv := NewServer(cfg, "")

	token := superAdminToken(t, "testadmin_drain", "Drain User", []string{"*"})

	// Register a slow write handler to simulate an in-flight business write request
	writeStarted := make(chan struct{}, 1)
	writeCompleted := make(chan struct{}, 1)
	srv.mux.HandleFunc("POST /api/test-slow-write", func(w http.ResponseWriter, r *http.Request) {
		select {
		case writeStarted <- struct{}{}:
		default:
		}
		// Simulate database transaction taking 200ms
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"written"}`))
		select {
		case writeCompleted <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ctx)
	}()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := client.Get(serverURL + "/live")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("server did not become ready")
	}

	// 1. Establish SSE connection
	sseClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, serverURL+"/api/notifications/sse?user_id=testadmin_drain", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	sseResp, err := sseClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}
	defer sseResp.Body.Close()

	reader := bufio.NewReader(sseResp.Body)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("failed to read initial SSE event: %v", err)
	}

	// 2. Start slow write request
	writeResultChan := make(chan int, 1)
	go func() {
		resp, err := http.Post(serverURL+"/api/test-slow-write", "application/json", nil)
		if err != nil {
			writeResultChan <- -1
			return
		}
		defer resp.Body.Close()
		writeResultChan <- resp.StatusCode
	}()

	// Wait for write to start
	<-writeStarted

	// 3. Trigger server shutdown while write is in-flight and SSE is connected
	shutdownStart := time.Now()
	cancel()

	// 4. In-flight write must succeed!
	select {
	case code := <-writeResultChan:
		if code != http.StatusOK {
			t.Fatalf("in-flight write request failed with status: %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("in-flight write request timed out")
	}

	// 5. Server shutdown completes cleanly after write is drained
	select {
	case err := <-serveDone:
		shutdownDuration := time.Since(shutdownStart)
		t.Logf("Serve exited cleanly in %v (in-flight write completed)", shutdownDuration)
		if shutdownDuration > 3*time.Second {
			t.Fatalf("Server shutdown took %v, which is too long", shutdownDuration)
		}
		if err != nil {
			t.Fatalf("Serve returned error on shutdown: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatalf("Server shutdown timed out")
	}
}

func TestSetupServerShutdownResponsive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "setup",
		},
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: port,
		},
	}
	setupServer, err := NewSetupServer(cfg, "", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create setup server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- setupServer.Serve(ctx)
	}()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := client.Get(serverURL + "/live")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("setup server did not become ready")
	}

	shutdownStart := time.Now()
	cancel()

	select {
	case err := <-serveDone:
		shutdownDuration := time.Since(shutdownStart)
		if shutdownDuration > 2*time.Second {
			t.Fatalf("SetupServer shutdown took %v, which is too long (expected < 2s)", shutdownDuration)
		}
		if err != nil {
			t.Fatalf("SetupServer Serve returned error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("SetupServer shutdown timed out (still blocked)")
	}
}
