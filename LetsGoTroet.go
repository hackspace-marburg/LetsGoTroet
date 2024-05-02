package main

import (
	"LetsGoTroet/irc"
  "LetsGoTroet/app"
  "LetsGoTroet/mastodon"
	"log"
  "os"
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
  "github.com/joho/godotenv"
)

const SQLITE_FILENAME = "messages.db"

func main() {
  err := godotenv.Load(".env")
  if err != nil {
    log.Println("Did not load .env file")
  }

  // Get variables from environment
  irchost := os.Getenv("IRC_HOST")
  channel := os.Getenv("IRC_CHANNEL")
  nick := os.Getenv("IRC_NICK")

  baseurl := os.Getenv("MASTODON_BASEURL")
  username := os.Getenv("MASTODON_USERNAME")
  password := os.Getenv("MASTODON_PASSWORD")
  
  // Setup database
  db := setupDB(SQLITE_FILENAME)

	// Setup  IRC adapter
  bot, err := irc.New(irchost, nick, channel, db)
	if err != nil {
    log.Println("IRC Adapter creation failed:", err)
	  return
	}
  // Setup Mastodon adapter
  mst, err := mastodon.New(baseurl, username, password, db) 
  if err != nil {
    log.Println(err)
    return
  }
  // Run service
  service := app.New(bot, mst)
  _ = service
  //service.Run()
}

func setupDB(filename string) *sql.DB{
  db, err := sql.Open("sqlite3", filename)
  if err != nil {
    log.Fatal("Could not open database, Error:", err)
  }
  return db
}
