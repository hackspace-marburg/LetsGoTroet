package main

import (
	"LetsGoTroet/irc"
	"log"
	"sync"
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
)

const CHANNEL = "#troet"

func main() {
	var wg sync.WaitGroup

  db := setupDB()
	bot, err := irc.New("irc.hackint.org:6697", "Troet2", "#Troet", db)
  
	if err != nil {
		log.Println(err)
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bot.Eventloop()
	}()

	wg.Wait()
}

func setupDB() *sql.DB{
  return nil
}
