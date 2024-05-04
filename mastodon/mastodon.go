package mastodon

import (
	"LetsGoTroet/app"
	"database/sql"
	"encoding/json"
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
	homeserver          string
}

func (mc MastodonClient) Send(message string) (string, error) {
	body := url.Values{
		"status":     {message},
		"visibility": {"private"},
	}
  req, err := mc.authorizedRequest(body)
  if err != nil {
    return "", fmt.Errorf("Error during building request: %s", err)
  }
  resp, err := mc.client.Do(req)
  if err != nil {
    return "", fmt.Errorf("Error during executing request: %s", err)
  }
  respBody, err := io.ReadAll(resp.Body)
  if err != nil {
    return "", fmt.Errorf("Error reading HTTP response: %s", err)
  }
  var posted status
  if err = json.Unmarshal(respBody, &posted); err != nil {
    return "", fmt. Errorf("Error unmarshaling response %s , %s", string(respBody), err)
  }
  
  // TODO: return own internal ID related to databse entry here.
  return posted.Id, nil
}

func (mc MastodonClient) Reply(messageID string, message string) error {
	return nil
}

func (mc MastodonClient) Delete(messageID string) error{
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

	body, err := io.ReadAll(reply.Body)
	if err != nil {
		return nil, err
	}
	var appsResponse appsReply
	err = json.Unmarshal(body, &appsResponse)
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

	body, err = io.ReadAll(reply.Body)
	if err != nil {
		return nil, err
	}
	var tokenResponse tokenReply
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return nil, err
	}

	log.Println(tokenResponse.AccessToken)
	return &MastodonClient{
		notificationHandler: nil,
		token:               tokenResponse.AccessToken,
		client:              client,
		database:            database,
		homeserver:          homeserver,
	}, nil
}
