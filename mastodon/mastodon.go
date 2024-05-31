package mastodon

import (
	"github.com/microcosm-cc/bluemonday"
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
	"time"
)

const create_table = `
  CREATE TABLE IF NOT EXISTS messages_mastodon(
    shorthand TEXT PRIMARY KEY,
    time DATETIME NOT NULL,
    tootid TEXT NOT NULL,
    content TEXT NOT NULL
  );
`

type MastodonClient struct {
	notificationHandler app.MessageHandler
	client              *http.Client
	token               string
	database            *sql.DB
	homeserver          string
	account             *account
}

func (mc MastodonClient) Send(message string) (string, error) {
	body := url.Values{
		"status":     {message},
		"visibility": {"unlisted"},
	}
	return mc.postStatus(body)
}

func (mc MastodonClient) Reply(shorthand string, message string) (string, error) {
	toot, err := mc.lookupShorthand(shorthand)

	if err != nil {
		log.Println("Shorthand:", shorthand, "; Error:", err)
		return "", fmt.Errorf("Could not reply to: %s", shorthand)
	}

	body := url.Values{
		"status":         {message},
		// "visibility":     {"unlisted"},
    "visibility":     {"private"},
		"in_reply_to_id": {toot.Id},
	}
	return mc.postStatus(body)
}

func (mc MastodonClient) lookupShorthand(messageID string) (*status, error) {
	row := mc.database.QueryRow("SELECT tootid FROM messages_mastodon WHERE shorthand=?;", messageID)
	var tootId string
	err := row.Scan(&tootId)
  if err != nil || tootId == "" {
      return nil, fmt.Errorf("Toot not found in database: %s", messageID)
	}
	toot, err := mc.getStatus(tootId)
	if err != nil && err.Error() == "404" {
		// If this happens the toot is not (most likely: no longer) existing
		// subsequently we can delete our entry about it
		mc.database.Query("DELETE FROM messages_mastodon WHERE shorthand=?;", messageID)
		return nil, fmt.Errorf("Toot not found (404)")
	}
	return toot, err
}

func (mc MastodonClient) GetMessage(messageID string) (string, error) {
	toot, err := mc.lookupShorthand(messageID)
	if err != nil {
		return "", fmt.Errorf("Error retrieving Toot: %w", err)
	}
  // TODO: replace <br/> with \n or something to handle multiline messages
  p := bluemonday.NewPolicy()
  plainContent := p.Sanitize(toot.Content)
  indentedContent := "> " + strings.Join(strings.Split(plainContent, "\n"), "\n> ")
	output := fmt.Sprintf("[%s] Toot by: %s\n%s\n%s", messageID, toot.Account.DisplayName, indentedContent, toot.Url)

  log.Println("Get Message output:", output)
	return output, err
}

// This calls a toggle for boosting, i.e. if already boosted this un-boosts. Currently defaults to "public" reblogs of toots.
func (mc MastodonClient) Boost(messageID string) (bool, error) {
	toot, err := mc.lookupShorthand(messageID)
	if err != nil {
		return false, err
	}
	toot, err = mc.toggleTootBoost(toot, "public")
	return toot.Reblogged, err
}

// Like Boost this toggles Favs on Toots.
func (mc MastodonClient) Favorite(messageID string) (bool, error) {
	toot, err := mc.lookupShorthand(messageID)
	if err != nil {
		return false, err
	}
	toot, err = mc.toggleTootFave(toot)
  if err != nil {
	  return false, err
  } else {
    return toot.Favorited, nil
  }
}

func (mc MastodonClient) Search(context string) (string, error) {
	// search for context in mastodon, return first related toot
	search, err := mc.search(context)
	if err != nil {
		return "", err
	}
	if len(search.Statuses) == 0 {
		return "", fmt.Errorf("No Results")
	}
	shorthand, err := mc.storeMessage(search.Statuses[0])
	if err != nil {
		return "", fmt.Errorf("Error during storing found toot: %w", err)
	}
	return mc.GetMessage(shorthand)
}

func encodeId(id string) string {
	// exclude similar symbols (O and 0, I and l), but include some other quite unusual stuff for fun
	madEncoding := base64.NewEncoding("ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz123456789.,;#!?").WithPadding(base64.NoPadding)
	h := fnv.New32()
	h.Write([]byte(id))
	return madEncoding.EncodeToString(h.Sum(nil))
}

func (mc MastodonClient) Delete(messageID string) error {
	toot, err := mc.lookupShorthand(messageID)
	if err != nil {
		return fmt.Errorf("%s was not recognized", messageID)
	}
	if toot.Account.Account != mc.account.Account {
		return fmt.Errorf("Hear ye: %s is not our (%s) toot but belongeth to %s and thus shall not be deleted", messageID, mc.account.Account, toot.Account.Account)
	}
	err = mc.deleteToot(toot)
	if err != nil {
		return err
	}
	mc.database.Query("DELETE FROM messages_mastodon WHERE shorthand=?;", messageID)
	return err
}

func (mc MastodonClient) RegisterMessageHandler(handler app.MessageHandler) {
	mc.notificationHandler = handler
}

func (mc MastodonClient) Eventloop() {
	// TODO
	// Check if Auth Token is still valid. If not, login again!
  
  // TODO
  // Check for new Notifications and Pawn them off to mc.MessageHandler
}

func (mc MastodonClient) storeMessage(message status) (string, error) {
	shorthand := encodeId(message.Id)
	_, err := mc.database.Exec("INSERT INTO messages_mastodon VALUES(?,?,?,?);", shorthand, time.Now(), message.Id, message.Content)
	if err != nil {
		_, err := mc.database.Exec("UPDATE messages_mastodon SET time=?, tootid=?, content=? WHERE shorthand=?", time.Now(), message.Id, message.Content, shorthand)
		if err != nil {
			return "", fmt.Errorf("Error during inserting message in database: %s", err)
		}
	}
	return shorthand, nil
}

func New(homeserver string, username string, password string, database *sql.DB) (*MastodonClient, error) {

	if _, err := database.Exec(create_table); err != nil {
		return nil, err
	}

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

	var mc = MastodonClient{
		notificationHandler: nil,
		token:               tokenResponse.AccessToken,
		client:              client,
		database:            database,
		homeserver:          homeserver,
	}
  acc, err := mc.getOwnAccount()
  if err != nil {
    return nil, fmt.Errorf("Unable to get account: %w", err)
  }
  mc.account = acc
	return &mc, err
}
