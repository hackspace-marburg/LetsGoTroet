package app

import (
	"fmt"
	"strings"
	"sync"
)

type MessageHandler func(source string, message string, messageID string)

type MessageID = string

type Adapter interface {
	Send(message string) (MessageID, error)
	Reply(messageID MessageID, message string) (MessageID, error)
	RegisterMessageHandler(MessageHandler)
	GetMessage(messageID MessageID) (string, error)
	Eventloop()
}

type SocialAdapter interface {
	Adapter
	Boost(messageID MessageID) (bool, error)
	Favorite(messageID MessageID) (bool, error)
	Search(context string) (MessageID, error)
	Delete(messageID MessageID) error
}

type App struct {
	ircAdapter      Adapter
	mastodonAdapter SocialAdapter
}

func New(irc Adapter, mastodon SocialAdapter) App {
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
		var id string
		tootMessage := strings.TrimPrefix(message, ".t ")
		id, err = app.mastodonAdapter.Send(tootMessage)
		if err == nil {
			app.ircAdapter.Send(fmt.Sprintf("[%s] Toot successfull", id))
			tootmessage, _ := app.mastodonAdapter.GetMessage(id)
			app.ircAdapter.Send(tootmessage)
		} else {
			app.ircAdapter.Reply(messageID, fmt.Sprintf("Error during sending: %v", err))
		}

	case strings.HasPrefix(message, ".r "):
		split := strings.SplitAfterN(strings.TrimPrefix(message, ".r "), " ", 2)
		replyTo := strings.TrimSuffix(split[0], " ")
		message := split[1]
		id, err := app.mastodonAdapter.Reply(replyTo, message)
		if err == nil {
			app.ircAdapter.Send(fmt.Sprintf("[%s] Reply successfull", id))
			tootmessage, _ := app.mastodonAdapter.GetMessage(id)
			app.ircAdapter.Send(tootmessage)
		} else {
			app.ircAdapter.Reply(messageID, fmt.Sprintf("Error replying: %v", err))
		}

	case strings.HasPrefix(message, ".?"):
		_, err = app.ircAdapter.Reply(messageID, "I'm sorry %s I'm afraid I can't do that")

	case strings.HasPrefix(message, ".d "):
		tootID := strings.TrimPrefix(message, ".d ")
		err = app.mastodonAdapter.Delete(tootID)
		if err == nil {
			app.ircAdapter.Send(fmt.Sprintf("Successfully deleted toot %s", tootID))
		} else {
			app.ircAdapter.Send(fmt.Sprintf("Error deleting toot: %v", err))
		}

	case strings.HasPrefix(message, ".s "):
		// search & load toot
		content := strings.TrimPrefix(message, ".s ")
		tootID, err := app.mastodonAdapter.Search(content)
		if err != nil {
			if tootID != "" {
				tootmessage, _ := app.ircAdapter.GetMessage(tootID)
				app.ircAdapter.Send(tootmessage)
			} else {
				app.ircAdapter.Send(fmt.Sprintf("No toot found"))
			}
		} else {
			app.ircAdapter.Send(fmt.Sprintf("Error finding toot: %v", err))
		}

	case strings.HasPrefix(message, ".b "):
		tootID := strings.TrimPrefix(message, ".b ")
		boosted, err := app.mastodonAdapter.Boost(tootID)
		if err == nil {
			var action string
			if boosted {
				action = "Boosted"
			} else {
				action = "Un-Boosted"
			}
			app.ircAdapter.Send(fmt.Sprintf("%s %s", action, tootID))
		} else {
			app.ircAdapter.Send(fmt.Sprintf("Error boosting toot: %v", err))
		}

	case strings.HasPrefix(message, ".f "):
		tootID := strings.TrimPrefix(message, ".f ")
		boosted, err := app.mastodonAdapter.Favorite(tootID)
		if err == nil {
			var action string
			if boosted {
				action = "Faved"
			} else {
				action = "Un-Faved"
			}
			app.ircAdapter.Send(fmt.Sprintf("%s %s", action, tootID))
		} else {
			app.ircAdapter.Send(fmt.Sprintf("Error favoriting toot: %s", err))
		}
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
