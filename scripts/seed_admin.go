// scripts/seed_admin.go
// Run this ONCE after your first migration to create the initial admin account.
//
// Usage:
//
//	go run scripts/seed_admin.go
//
// It will create an admin user with credentials you can change after first login.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/cappyHoding/ptdpn-eform-service/pkg/crypto"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

func main() {
	// Load .env
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		// Try without .env (use real env vars)
		fmt.Println("No .env file found, using environment variables")
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&allowNativePasswords=true&tls=false",
		viper.GetString("DB_USER"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetInt("DB_PORT"),
		viper.GetString("DB_NAME"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot reach database:", err)
	}

	// Default credentials — CHANGE THESE AFTER FIRST LOGIN
	username := "admin"
	password := "Admin@1234"
	fullName := "System Administrator"
	email := "admin@bprperdana.co.id"

	// Override with args if provided
	if len(os.Args) >= 3 {
		username = os.Args[1]
		password = os.Args[2]
	}

	// Hash the password
	hashed, err := crypto.HashPassword(password)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}

	id := uuid.New().String()

	// Get admin role ID
	var roleID int
	err = db.QueryRow("SELECT id FROM roles WHERE name = 'admin' LIMIT 1").Scan(&roleID)
	if err != nil {
		log.Fatal("Admin role not found. Have you run the migrations?", err)
	}

	// Insert the admin user
	_, err = db.Exec(
		`INSERT INTO internal_users (id, username, full_name, email, password, role_id, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, 1)
		 ON DUPLICATE KEY UPDATE username = username`,
		id, username, fullName, email, hashed, roleID,
	)
	if err != nil {
		log.Fatal("Failed to create admin user:", err)
	}

	fmt.Println("========================================")
	fmt.Println("  Admin user created successfully!")
	fmt.Println("========================================")
	fmt.Printf("  Username : %s\n", username)
	fmt.Printf("  Password : %s\n", password)
	fmt.Printf("  Role     : admin\n")
	fmt.Println("----------------------------------------")
	fmt.Println("  IMPORTANT: Change your password after")
	fmt.Println("  first login!")
	fmt.Println("========================================")
}
