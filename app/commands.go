package app

import (
	"fmt"
	"log"
	"strings"
)
// TODO: Build and use this instead
var channel_command_map = map[string]command{}

var channel_commands = []command{
	{
		name:  "t",
    description: "Posts a toot. Toot content is the text following after",
		nargs: 1,
    elevated_permissions: true,
    action: func(app *App, message_type, message, messageID string) {
			var id string
			tootMessage := strings.TrimPrefix(message, ".t ")
			id, err := app.mastodonAdapter.Send(tootMessage)
			if err == nil {
				app.ircAdapter.Send(fmt.Sprintf("[%s] Toot successfull", id))
				tootmessage, _ := app.mastodonAdapter.GetMessage(id)
				app.ircAdapter.Send(tootmessage)
			} else {
				app.ircAdapter.Reply(messageID, fmt.Sprintf("Error during sending: %v", err))
			}
		},
	}, {
		name:  "r",
    description: "Replies to a toot. First parameter is the ID to reply to, everything after is the content of the reply",
		nargs: 2,
    elevated_permissions: true,
		action: func(app *App, message_type, message, messageID string) {
			split := strings.SplitAfterN(strings.TrimPrefix(message, ".r "), " ", 2)
			replyTo := strings.TrimSuffix(split[0], " ")
			replyText := split[1]
			id, err := app.mastodonAdapter.Reply(replyTo, replyText)
			if err == nil {
				app.ircAdapter.Send(fmt.Sprintf("[%s] Reply successfull", id))
				tootmessage, _ := app.mastodonAdapter.GetMessage(id)
				app.ircAdapter.Send(tootmessage)
			} else {
				app.ircAdapter.Reply(messageID, fmt.Sprintf("Error replying: %v", err))
			}
		},
	}, {
		name:  "?",
    description: "The help command (redirects you to here)",
		nargs: 0,
    elevated_permissions: false,
		action: func(app *App, message_type, message, messageID string) {
      _, err := app.ircAdapter.Reply(messageID, "To get to know the commands please send me a message via /query")
      if err != nil {
        log.Println("Error replying:", err)
      }
		},
	}, {
		name:  "d",
    description: "Deletes a toot. One parameter with the toot's id expected",
		nargs: 1,
    elevated_permissions: true,
		action: func(app *App, message_type, message, messageID string) {
			tootID := strings.TrimPrefix(message, ".d ")
			err := app.mastodonAdapter.Delete(tootID)
			if err == nil {
				app.ircAdapter.Send(fmt.Sprintf("Successfully deleted toot %s", tootID))
			} else {
				app.ircAdapter.Send(fmt.Sprintf("Error deleting toot: %v", err))
			}
		},
	}, {
		name:  "s",
    description: "Search a toot & load it into the bot to get a ID for other commands. Parameter should be the permanent link",
		nargs: 1,
    elevated_permissions: true,
		action: func(app *App, message_type, message, messageID string) {
			// search & load toot
			content := strings.TrimPrefix(message, ".s ")
			tootMessage, err := app.mastodonAdapter.Search(content)
			if err == nil {
				if tootMessage != "" {
					app.ircAdapter.Send(tootMessage)
				} else {
					app.ircAdapter.Send(fmt.Sprintf("No toot found"))
				}
			} else {
				app.ircAdapter.Send(fmt.Sprintf("Error finding toot: %v", err))
			}
		},
	}, {
		name:  "b",
    description: "(Un-)Boosts a toot (toggle). Parameter is the ID of the toot to boost",
		nargs: 1,
    elevated_permissions: true,
		action: func(app *App, message_type, message, messageID string) {
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
		},
	}, {
		name:  "f",
    description: "(Un-)Favourites a toot (toggle). Parameter is the ID of the toot to favourite",
		nargs: 1,
    elevated_permissions: true,
		action: func(app *App, message_type, message, messageID string) {
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
		},
	},
}
