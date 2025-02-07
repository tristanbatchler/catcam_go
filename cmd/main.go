package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"catcam_go/internal/db"
	"catcam_go/internal/server"
	"catcam_go/internal/store/users"

	_ "github.com/joho/godotenv/autoload" // Automatically load .env file
)

func main() {
	logger := log.New(os.Stdout, "[Main] ", log.LstdFlags)

	port := 9000

	dbPool, err := sql.Open("sqlite", "db.sqlite")
	if err != nil {
		logger.Fatalf("Error when opening database: %s", err)
	}

	log.Println("Initializing database...")
	if err := db.GenSchema(dbPool); err != nil {
		log.Fatal(err)
	}

	logger.Print("Creating users store..")
	userStore := users.NewUserStore(db.New(dbPool), logger)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		logger.Fatalf("Error when hashing password: %s", err)
	}
	userStore.AddUser(context.Background(), db.AddUserParams{
		Username:     "saltytaro",
		PasswordHash: string(passwordHash),
	})

	srv, err := server.NewServer(logger, port, userStore)
	if err != nil {
		logger.Fatalf("Error when creating server: %s", err)
		os.Exit(1)
	}
	if err := srv.Start(); err != nil {
		logger.Fatalf("Error when starting server: %s", err)
		os.Exit(1)
	}
}
