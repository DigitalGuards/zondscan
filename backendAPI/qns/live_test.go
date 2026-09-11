package qns

import (
	"backendAPI/db"
	"backendAPI/qrladdress"
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveQNSResolution(t *testing.T) {
	if os.Getenv("QNS_LIVE") != "1" {
		t.Skip("set QNS_LIVE=1 with a loopback node and deployed registry")
	}

	name := os.Getenv("QNS_LIVE_NAME")
	if name == "" {
		t.Fatal("QNS_LIVE_NAME is required")
	}
	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("load live config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := NewClient(db.NodeRPC)
	deployment, err := client.CheckDeployment(ctx, config)
	if err != nil {
		t.Fatalf("check live deployment: %v", err)
	}
	resolution, err := client.ResolveName(ctx, deployment, name)
	if err != nil {
		t.Fatalf("resolve live name: %v", err)
	}
	if resolution.Name != name || resolution.Missing != MissingNone {
		t.Fatalf("unexpected live resolution metadata: %#v", resolution)
	}
	canonical, ok := qrladdress.Canonicalize(resolution.Address)
	if !ok || canonical != resolution.Address {
		t.Fatalf("live address is not canonical QIP-55: %q", resolution.Address)
	}
}
