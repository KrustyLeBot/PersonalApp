package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"helloauth/internal/auth"
	"helloauth/internal/db"
	lolcalendar "helloauth/internal/lol-calendar"
	"helloauth/internal/portfolio"
	"helloauth/internal/projection"
	"helloauth/internal/telework"
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

	projRepo := projection.NewRepo(database)
	projCAGR := projection.NewCAGRClient()
	projSvc := projection.NewService(projRepo, projCAGR, repo)
	svc.OnTickerRefresh(projSvc.RefreshTickerCAGRs)
	projHandler := projection.NewHandler(projRepo, projSvc, repo)
	projHandler.RegisterRoutes(mux)
	if err := projRepo.SeedDefaults(); err != nil {
		log.Printf("projection seed defaults: %v", err)
	}
	// Populate CAGR rates on first boot if none exist yet.
	go projSvc.BootstrapCAGRs()

	twRepo := telework.NewRepo(database)
	twSvc := telework.NewService(twRepo)
	twHandler := telework.NewHandler(twRepo, twSvc)
	twHandler.RegisterRoutes(mux)

	lolRepo := lolcalendar.NewRepo(database)
	lolSvc := lolcalendar.NewService(lolRepo, lolcalendar.NewClient())
	lolHandler := lolcalendar.NewHandler(lolRepo, lolSvc)
	lolHandler.RegisterRoutes(mux)

	mux.Handle("/", http.FileServer(http.Dir("./static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("server started on port %s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
