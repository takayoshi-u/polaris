package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/takayoshi-u/polaris/backend/internal/api"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/store"
	"github.com/takayoshi-u/polaris/backend/migrations"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://polaris:polaris@localhost:5432/polaris?sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:3000"
	}
	corsOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = appBaseURL
	}
	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		encKey = "0123456789abcdef0123456789abcdef"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	if err := migrations.Run(ctx, db); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	mailer := mail.NewSMTPSender()
	fcmClient := fcm.NewClient(ctx, os.Getenv("FIREBASE_CREDENTIALS_FILE"))
	st := store.New(db, encKey, appBaseURL, mailer, fcmClient)

	if err := st.EnsureSystemDefaults(
		ctx,
		os.Getenv("INIT_ADMIN_EMAIL"),
		os.Getenv("INIT_ADMIN_PASSWORD"),
		os.Getenv("INIT_ADMIN_NAME"),
	); err != nil {
		log.Fatalf("failed to initialize system defaults: %v", err)
	}

	srv := api.NewServer(st, corsOrigin, api.GoogleOAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	})

	log.Printf("Polaris API server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

