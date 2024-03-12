package main

import (
	"LetsGoTroet/irc"
	"log"
	"sync"
)

const CHANNEL = "#troet"

func main() {
	var wg sync.WaitGroup
  
  bot, err := irc.New("irc.hackint.org:6697", "Troet2", "#Troet")

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

