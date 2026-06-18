package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"helloauth/internal/auth"
	"helloauth/internal/db"
	"helloauth/internal/portfolio"
)

func main() {
	godotenv.Load()

	if err := auth.InitSession(); err != nil {
		log.Fatalf("session init: %v", err)
	}
	if err := auth.InitAllowedEmails(); err != nil {
		log.Fatalf("allowed emails: %v", err)
	}
	if err := auth.InitOAuth(); err != nil {
		log.Fatalf("oauth init: %v", err)
	}

	database, err := db.New()
	if err != nil {
		log.Fatalf("database init: %v", err)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("database migration: %v", err)
	}

	mux := http.NewServeMux()

	// /api/status is protected — DB status is internal information.
	mux.HandleFunc("/api/status", auth.RequireAuth(func(w http.ResponseWriter, r *http.Request, _ string) {
		database.HealthHandler()(w, r)
	}))

	authHandler := auth.NewHandler()
	authHandler.RegisterRoutes(mux)

	repo := portfolio.NewRepo(database)
	ticker := portfolio.NewTickerClient()
	svc := portfolio.NewService(repo, ticker)
	portfolioHandler := portfolio.NewHandler(repo, svc)
	portfolioHandler.RegisterRoutes(mux)

	mux.Handle("/", http.FileServer(http.Dir("./static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("server started on port %s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
