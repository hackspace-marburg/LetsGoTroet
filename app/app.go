package app

import (
	"log"
	"strings"
	"sync"
)

type MessageHandler func(source string, message string, messageID string)

type Adapter interface {
  Send(message string) (string, error)
  Reply(messageID string, message string) error
  Delete(messageID string) error
  RegisterMessageHandler(MessageHandler)
  Eventloop()
}

type App struct {
  ircAdapter Adapter
  mastodonAdapter Adapter
}

func New(irc Adapter, mastodon Adapter) App {
  app := App {
    ircAdapter: irc,
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
    log.Println("We should be tooting now...")
  case strings.HasPrefix(message, ".?"):
     err = app.ircAdapter.Reply(messageID, "I'm sorry %s I'm afraid I can't do that")
  }

  if err != nil {
    log.Println("Error when replying:", err)
  }
}

func (app App) handleMastodonMessage(source string, message string, messageID string) {

}

// Should be reworked, runs the eventloops of the adapters.
// The app itself does not not run in a dedicated thread. It is just called by event handling goroutines
func (app App) Run(){
  var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.ircAdapter.Eventloop()
	}()

	wg.Wait()
}
