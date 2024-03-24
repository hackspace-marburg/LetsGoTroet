package main

import (
	"LetsGoTroet/irc"
  "LetsGoTroet/app"
	"log"
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
)

const CHANNEL = "#troet"
const NICK = "Troet2"
const SQLITE_FILENAME = "messages.db"

func main() {
  
  db := setupDB(SQLITE_FILENAME)
	// Setup  IRC adapter
  bot, err := irc.New("irc.hackint.org:6697", NICK, CHANNEL, db)
	if err != nil {
    log.Println("IRC Adapter creation failed:", err)
		return
	}
  // TODO: Setup Mastodon adapter


  // Run service
  service := app.New(bot, nil)
  service.Run()
}

func setupDB(filename string) *sql.DB{
  db, err := sql.Open("sqlite3", filename)
  if err != nil {
    log.Fatal("Could not open database, Error:", err)
  }
  return db
}
