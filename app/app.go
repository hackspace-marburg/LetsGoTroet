package app

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

type MessageHandler func(source string, message string, messageID string)

type Adapter interface {
	Send(message string) (string, error)
	Reply(messageID string, message string) (string, error)
	Delete(messageID string) error
	RegisterMessageHandler(MessageHandler)
	Eventloop()
}

type App struct {
	ircAdapter      Adapter
	mastodonAdapter Adapter
}

func New(irc Adapter, mastodon Adapter) App {
	app := App{
		ircAdapter:      irc,
		mastodonAdapter: mastodon,
	}
	irc.RegisterMessageHandler(app.handleIRCMessage)
	//mastodon.RegisterMessageHandler(app.handleMastodonMessage) // TODO einkkommentieren, wenn der masto-bot exisitiert.

	return app
}

func (app App) handleIRCMessage(source string, message string, messageID string) {
	var err error
	switch {
	case strings.HasPrefix(message, ".t "):
		// TODO: send toot now!
		var id string
		tootMessage := strings.TrimPrefix(message, ".t ")
		id, err = app.mastodonAdapter.Send(tootMessage)
		if err == nil {
			app.ircAdapter.Send(fmt.Sprintf("Successfully send Message and stored as %s", id))
		} else {
      app.ircAdapter.Send(fmt.Sprintf("Error during sending: %s", err))
    }

	case strings.HasPrefix(message, ".?"):
		_, err = app.ircAdapter.Reply(messageID, "I'm sorry %s I'm afraid I can't do that")
	}

	if err != nil {
		log.Println("Error when replying:", err)
	}
}

func (app App) handleMastodonMessage(source string, message string, messageID string) {

}

// Should be reworked, runs the eventloops of the adapters.
// The app itself does not not run in a dedicated thread. It is just called by event handling goroutines
func (app App) Run() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.ircAdapter.Eventloop()
	}()

	wg.Wait()
}
