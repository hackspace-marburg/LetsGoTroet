package mastodon

import (
	"LetsGoTroet/app"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

type MastodonClient struct {
	notificationHandler app.MessageHandler
	client              *http.Client
	token               string
	database            *sql.DB
}

func (mc MastodonClient) Send(message string) error {
	return nil
}

func (mc MastodonClient) Reply(messageID string, message string) error {
	return nil
}

func (mc MastodonClient) RegisterMessageHandler(handler app.MessageHandler) {
	mc.notificationHandler = handler
}

func (mc MastodonClient) Eventloop() {

}

func New(homeserver string, username string, password string, database *sql.DB) (*MastodonClient, error) {
	// TODO: Setup database.
	log.Println("Initializing Mastodon Bot")

	client := &http.Client{}
	reply, err := client.PostForm(fmt.Sprintf(`https://%s/api/v1/apps`, homeserver), url.Values{
		"client_name":   {"LetsGoTroet"},
		"redirect_uris": {"urn:ietf:wg:oauth:2.0:oob"},
		"scopes":        {"read write push"},
	})
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 2000)
	val, err := reply.Body.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var appsResponse appsReply
	err = json.Unmarshal(buf[:val], &appsResponse)
	if err != nil {
		log.Println("Unmarshalling /v1/apps response failed")
		log.Println(err)
	}

	reply, err = client.PostForm(fmt.Sprintf(`https://%s/oauth/token`, homeserver), url.Values{
		"client_id":     {appsResponse.ClientId},
		"client_secret": {appsResponse.ClientSecret},
		"username":      {username},
		"password":      {password},
		"grant_type":    {"password"},
		"scope":         {"read write push"},
	})
	if err != nil {
		return nil, err
	}

	buf = make([]byte, 2000)
	val, err = reply.Body.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var tokenResponse tokenReply
	err = json.Unmarshal(buf[:val], &tokenResponse)
	if err != nil {
		return nil, err
	}

	log.Println(tokenResponse.AccessToken)
	return &MastodonClient{
		notificationHandler: nil,
		token:               tokenResponse.AccessToken,
		client:              client,
		database:            database,
	}, nil
}
