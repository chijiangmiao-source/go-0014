package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"steam-sterilization-thermal-validation/internal/api"
	"steam-sterilization-thermal-validation/internal/store"
	"steam-sterilization-thermal-validation/internal/workflow"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "sterilization.db")
	}
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	repo, err := store.OpenSQLite(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	svc := workflow.NewService(repo, workflow.RealClock{})
	handler := api.NewServer(svc)

	log.Printf("serving steam validation foundation on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
	if recovered, err := svc.RecoverExpiredLeases(workflow.LeaseID("startup", workflow.RealClock{}.Now())); err != nil {
		log.Fatal(err)
	} else if recovered > 0 {
		log.Printf("recovered %d expired analysis job leases", recovered)
	}
}
