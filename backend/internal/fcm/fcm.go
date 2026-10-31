package fcm

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Notifier interface {
	SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) error
}

type Client struct {
	msgClient *messaging.Client
	// CustomSend can be set in tests to verify FCM dispatches without calling Google servers.
	CustomSend func(ctx context.Context, tokens []string, title, body string, data map[string]string) error
}

// NewClient initializes an FCM client using a service account credentials file if provided.
// If credentialsFile is empty, FCM operates in no-op / mockable mode.
func NewClient(ctx context.Context, credentialsFile string) *Client {
	if credentialsFile == "" {
		return &Client{}
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		log.Printf("[FCM] warning: failed to initialize Firebase App: %v", err)
		return &Client{}
	}
	mc, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("[FCM] warning: failed to initialize Firebase Messaging client: %v", err)
		return &Client{}
	}
	return &Client{msgClient: mc}
}

func (c *Client) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}
	if c.CustomSend != nil {
		return c.CustomSend(ctx, tokens, title, body, data)
	}
	if c.msgClient == nil {
		return nil
	}
	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}
	_, err := c.msgClient.SendEachForMulticast(ctx, msg)
	return err
}

