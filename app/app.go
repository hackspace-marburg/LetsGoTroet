package app

import (
  "log"
  "sync"
)

type MessageHandler func(source string, message string, messageID string)

type Adapter interface {
  Send(message string) error
  Reply(messageID string, message string) error
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
  log.Println("Service got message:", message, "by:", source, "with id:", messageID)
  err := app.ircAdapter.Reply(messageID, "I'm sorry %s I'm afraid I cannot do that")
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
