package mastodon

import (
	"LetsGoTroet/app"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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
		"visibility": {"unlisted"},
	}
	return mc.postStatus(body)
}

func (mc MastodonClient) Reply(shorthand string, message string) (string, error) {
	toot, err := mc.retrieveStatus(shorthand)

	if err != nil {
		log.Println("Shorthand:", shorthand, "; Error:", err)
		return "", fmt.Errorf("Could not reply to:", shorthand)
	}

	body := url.Values{
		"status":         {message},
		"visibility":     {"unlisted"},
		"in_reply_to_id": {toot.Id},
	}
	return mc.postStatus(body)
}

func (mc MastodonClient) postStatus(body url.Values) (string, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf(`https://%s/api/v1/statuses`, mc.homeserver), strings.NewReader(body.Encode()))
	req := mc.authorizedRequest(request)
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
		return "", fmt.Errorf("Error unmarshaling response %s , %s", string(respBody), err)
	}
	return mc.saveStatus(posted)
}

func (mc MastodonClient) retrieveStatus(shorthand string) (status, error) {
	// TODO
	return status{}, nil
}

func (mc MastodonClient) saveStatus(to_store status) (string, error) {
	// TODO: Yoink Status into database
	shorthand := encodeId(to_store.Id)
	return shorthand, nil
}
func encodeId(id string) string {
	// exclude similar symbols (O and 0, I and l), but include some other quite unusual stuff for fun
	madEncoding := base64.NewEncoding("ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz123456789.,;#!?").WithPadding(base64.NoPadding)
	h := fnv.New32()
	h.Write([]byte(id))
	return madEncoding.EncodeToString(h.Sum(nil))
}

func (mc MastodonClient) Delete(messageID string) error {
	toot, err := mc.retrieveStatus(messageID)
  _ = toot
	return err
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
