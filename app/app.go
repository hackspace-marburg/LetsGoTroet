package app

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
  mastodon.RegisterMessageHandler(app.handleMastodonMessage)

  return app
}

func (app App) handleIRCMessage(source string, message string, messageID string) {

}

func (app App) handleMastodonMessage(source string, message string, messageID string) {

}
