package fcm

import (
	"context"
	"testing"
)

func TestFCMClient(t *testing.T) {
	ctx := context.Background()
	client := NewClient(ctx, "")

	// Empty tokens or nil msgClient should succeed as no-op
	if err := client.SendMulticast(ctx, nil, "Title", "Body", nil); err != nil {
		t.Fatalf("expected nil error for empty tokens, got %v", err)
	}
	if err := client.SendMulticast(ctx, []string{"token-1"}, "Title", "Body", nil); err != nil {
		t.Fatalf("expected nil error in no-op mode, got %v", err)
	}

	var capturedTokens []string
	var capturedTitle, capturedBody string
	var capturedData map[string]string

	client.CustomSend = func(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
		capturedTokens = tokens
		capturedTitle = title
		capturedBody = body
		capturedData = data
		return nil
	}

	err := client.SendMulticast(ctx, []string{"tok1", "tok2"}, "Assigned", "You were assigned", map[string]string{"link": "/projects/1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedTokens) != 2 || capturedTitle != "Assigned" || capturedBody != "You were assigned" || capturedData["link"] != "/projects/1" {
		t.Fatalf("unexpected custom send capture: tokens=%v title=%s body=%s data=%v", capturedTokens, capturedTitle, capturedBody, capturedData)
	}
}

