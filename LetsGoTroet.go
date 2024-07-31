package main

import (
	"LetsGoTroet/app"
	"LetsGoTroet/irc"
	"LetsGoTroet/mastodon"
	"database/sql"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
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
	nick_pw := os.Getenv("IRC_NICKPASS")

	baseurl := os.Getenv("MASTODON_BASEURL")
	username := os.Getenv("MASTODON_USERNAME")
	password := os.Getenv("MASTODON_PASSWORD")
	id := os.Getenv("MASTODON_ID")
	secret := os.Getenv("MASTODON_SECRET")
	access_token := os.Getenv("MASTODON_ACCESS_TOKEN")

	// Setup database
	db := setupDB(SQLITE_FILENAME)

	// Setup  IRC adapter
	bot, err := irc.New(irchost, nick, channel, db)
	if err != nil {
		log.Println("IRC Adapter creation failed:", err)
		return
	}
	if len(nick_pw) > 0 {
		bot.SetPassword(nick_pw)
	}
	// Setup Mastodon adapter
	mst, err := mastodon.New(baseurl, id, secret, access_token, username, password, db)
	if err != nil {
		log.Println(err)
		return
	}
	// Run service
	service := app.New(bot, mst)
	service.Run()
}

func setupDB(filename string) *sql.DB {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		log.Fatal("Could not open database, Error:", err)
	}
	return db
}
