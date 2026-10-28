//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestServiceRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "busybox:1.37",
			ExposedPorts: []string{"8080/tcp"},
			Cmd:          []string{"sh", "-c", "mkdir -p /www; printf pitstop-service > /www/index.html; exec httpd -f -p 8080 -h /www"},
			WaitingFor:   wait.ForHTTP("/").WithPort("8080/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if c != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
			defer stop()
			if err := c.Terminate(cleanup); err != nil {
				t.Errorf("terminate service: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("start service: %v", err)
	}
	endpoint, err := c.Endpoint(ctx, "http")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		t.Fatalf("mapped port request to %s: %v", endpoint, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "pitstop-service" {
		t.Fatalf("unexpected response: %d %q", response.StatusCode, body)
	}
	t.Logf("mapped service round-trip succeeded at %s", endpoint)
	if os.Getenv("PITSTOP_FIXTURE_FAIL") == "1" {
		t.Fatal("intentional assertion failure after successful service round-trip")
	}
}
