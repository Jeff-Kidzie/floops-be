package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Connect() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	//Connection string
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	//Connect to the database
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return err
	}
	DB = db

	// Verify connection
	err = DB.Ping()
	if err != nil {
		return err
	}

	fmt.Println("Successfully connected to the database!")
	return nil
}
