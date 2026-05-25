package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSmoke_UCPProfile(t *testing.T) {
	if !servicesUp() {
		t.Skip("smoke: need 'docker compose up postgres redis nats' running")
	}

	port := freePort()
	os.Setenv("PORT", port)
	os.Setenv("DATABASE_URL", "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable")
	os.Setenv("REDIS_ADDR", "localhost:6379")

	done := make(chan struct{}, 1)
	go func() {
		main()
		close(done)
	}()

	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%s", port)

	t.Run("ucp profile", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/.well-known/ucp")
		if err != nil {
			t.Fatalf("GET /.well-known/ucp: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode profile: %v", err)
		}
		if result["ucp_version"] != "1.0" {
			t.Errorf("expected ucp_version 1.0, got %v", result["ucp_version"])
		}
	})

	t.Run("register and login", func(t *testing.T) {
		regBody := map[string]string{
			"email":    fmt.Sprintf("smoke-%d@example.com", time.Now().UnixMilli()),
			"password": "testpassword123",
			"name":     "Smoke Test",
		}
		regData, _ := json.Marshal(regBody)

		resp, err := http.Post(baseURL+"/api/v1/auth/register", "application/json", bytes.NewReader(regData))
		if err != nil {
			t.Fatalf("POST register: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	})

	t.Cleanup(func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_ADDR")
	})
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 2*time.Second)
	if err != nil {
		return false
	}
	pg.Close()

	rd, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		return false
	}
	rd.Close()

	n, err := net.DialTimeout("tcp", "localhost:4222", 2*time.Second)
	if err != nil {
		return false
	}
	n.Close()

	return true
}

func freePort() string {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}
