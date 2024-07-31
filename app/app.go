package app

import (
	"fmt"
	// "log"
	"strings"
	"sync"
)

const command_prefix = "."

type commandfn func(app *App, message_type string, message string, messageID string)

type command struct {
	name                 string
	description          string
	nargs                int
	elevated_permissions bool
	action               commandfn
}

type MessageHandler func(source string, message string, messageID string)

type MessageID = string

type Adapter interface {
	Send(message string) (MessageID, error)
	Reply(messageID MessageID, message string) (MessageID, error)
	RegisterMessageHandler(MessageHandler)
	Eventloop()
}

type SocialAdapter interface {
	Adapter
	Boost(messageID MessageID) (bool, error)
	Favorite(messageID MessageID) (bool, error)
	Search(context string) (MessageID, error)
	Delete(messageID MessageID) error
	GetMessage(messageID MessageID) (string, error)
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
	mastodon.RegisterMessageHandler(app.handleMastodonMessage)

	return app
}

func (app *App) handleIRCMessage(msgtype string, message string, messageID string) {
	// var err error
	if strings.HasPrefix(msgtype, "channel.") {
    // log.Println("Handling Channel Message")
		for _, cmd := range channel_commands {
      // TODO: This is kind of jank. Split message at earliest possible whitespace and put that part into a map[string, command]
      // Maybe have that map even replace the current commands slice
			var space string
			if cmd.nargs > 0 {
				space = " "
			} else {
				space = ""
			}
      // log.Printf("Looking for '%s'\n", command_prefix+cmd.name+space)
			if (!cmd.elevated_permissions || strings.HasSuffix(msgtype, ".op")) &&
				strings.HasPrefix(message, command_prefix+cmd.name+space) {
          cmd.action(app, msgtype, message, messageID)
			}
		}
	}
	if strings.HasPrefix(msgtype, "direct.") {
    // log.Println("Handling /query message")
		// TODO: Define specific commands for direct messages?

    // For now always return a command list
    reply := "Hi %s\n the only reply via query currently supported is a command list:\n"
		command_descriptions := ""
		for _, cmd := range channel_commands {
			command_descriptions = command_descriptions + fmt.Sprintln(command_prefix+cmd.name, cmd.description)
		}
		app.ircAdapter.Reply(messageID, reply + command_descriptions)
	}
}

func (app App) handleMastodonMessage(msgtype string, message string, messageID string) {
	switch msgtype {
	case "mention":
		// We've been mentioned!
		app.ircAdapter.Send("We've been mentioned!")
		message, err := app.mastodonAdapter.GetMessage(messageID)
		if err != nil {
			app.ircAdapter.Send("But I failed to get the message")
			app.ircAdapter.Send(err.Error())
		} else {
			app.ircAdapter.Send(message)
		}
	case "status":
		message, err := app.mastodonAdapter.GetMessage(messageID)
		if err != nil {
			app.ircAdapter.Send("I failed to get the message %s") // TODO
			app.ircAdapter.Send(err.Error())
		} else {
			app.ircAdapter.Send(message)
		}
	case "favourite":
		app.ircAdapter.Send(fmt.Sprintf("%s favourited a toot of ours", message))
	case "reblog":
		app.ircAdapter.Send(fmt.Sprintf("%s reblogged a toot of ours", message))
	case "moin":
		app.ircAdapter.Send(fmt.Sprintf("@%s sagt moin!", message))
	}
}

// Should be reworked, runs the eventloops of the adapters.
// The app itself does not not run in a dedicated thread. It is just called by event handling goroutines
func (app App) Run() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		app.ircAdapter.Eventloop()
	}()
	go func() {
		defer wg.Done()
		app.mastodonAdapter.Eventloop()
	}()

	wg.Wait()
}
