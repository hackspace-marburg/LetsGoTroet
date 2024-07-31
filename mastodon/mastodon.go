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
	"time"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

const create_table = `
  CREATE TABLE IF NOT EXISTS messages_mastodon(
    shorthand TEXT PRIMARY KEY,
    time DATETIME NOT NULL,
    tootid TEXT NOT NULL,
    content TEXT NOT NULL
  );
`
// exclude similar symbols (O and 0, I and l), but include some other quite unusual stuff for fun
const base64mod = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz123456789.,;#!?"

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
		"status":     {message},
		"visibility": {"unlisted"},
		//"visibility":     {"private"},
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
  p := bluemonday.StrictPolicy()
	plainContent := p.Sanitize( strings.TrimRight(strings.ReplaceAll(toot.Content, "</p>", "</p>\n"), "\n"))
  unescapedContent := html.UnescapeString(plainContent)
	indentedContent := "> " + strings.Join(strings.Split(unescapedContent, "\n"), "\n> ")
  for i, media := range toot.Attachments {
    indentedContent += fmt.Sprintf("\n Attachment %d: %s", i+1, media.Url)
  }
	output := fmt.Sprintf("[%s] Toot by: %s (%s)\n%s\n%s", messageID, toot.Account.DisplayName, toot.Account.Username, indentedContent, toot.Url)
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
	madEncoding := base64.NewEncoding(base64mod).WithPadding(base64.NoPadding)
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

// This eventloop performs 2 tasks, one visibile in code and one is a pure (wanted) side effect
// First we request every 15 seconds from mastodon if we have new notifications
// If so these are given to the notificationHandler with a bit of an unusual use of the parameters:
// - mention and status notifications set the type according to their names, use the message as the reformatted status and provide the shorthand as mesasgeId
// - reblog and favourite don't need to show the full toot, so message is the user who performed the action and messageId is the URL of the toot
//
// The second use is to remind the mastodon server that we still exsist. Since mastodon bearer tokens do not have an expiration date, we want to make sure we're still known
// otherwise our token might be invalidated at some point.
func (mc MastodonClient) Eventloop() {
	log.Println("Masotdon Adapter Loop started")
	active := false
	timeoffset, _ := time.ParseDuration("15s")
	for active {
		nots, err := mc.getNotifications()
		if err != nil {
			log.Println("Error getting Notifications:", err)
		} else {
			notifications := *nots
			for _, value := range notifications {
				switch value.Type {
				case "mention":
					shorthand, err := mc.storeMessage(value.Status)
					if err != nil {
						log.Println("Error storing message:", err.Error())
						continue
					}
					formatted, err := mc.GetMessage(shorthand)
					if err != nil {
						log.Println("Error getting message:", err.Error())
						continue
					}
					mc.notificationHandler("mention", formatted, shorthand)
				case "status":
					shorthand, err := mc.storeMessage(value.Status)
					if err != nil {
						log.Println("Error storing message:", err.Error())
						continue
					}
					formatted, err := mc.GetMessage(shorthand)
					if err != nil {
						log.Println("Error getting message:", err.Error())
						continue
					}
					mc.notificationHandler("status", formatted, shorthand)
				case "reblog":
					mc.notificationHandler("reblog", value.Account.DisplayName, value.Status.Url)
				case "favourite":
					if value.Status.Content == "<p>moin</p>" {
						mc.notificationHandler("moin", value.Account.Account, value.Status.Url)
					} else {
						mc.notificationHandler("favourite", value.Account.DisplayName, value.Status.Url)
					}
				default:
					continue // If it's a notification we can't handle we also don't want to dismiss it
				}
				if err = mc.dismissNotification(value); err != nil {
					log.Println("Error during dismissing notification:", err.Error())
				}
			}
		}
		time.Sleep(timeoffset)
	}
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

func New(homeserver string, client_id string, client_secret string, access_token string, username string, password string, database *sql.DB) (*MastodonClient, error) {
	if _, err := database.Exec(create_table); err != nil {
		return nil, err
	}

	log.Println("Initializing Mastodon Bot")

	client := &http.Client{}
	var reply *http.Response
	var err error
	if len(access_token) == 0 {
		log.Println("No Access Token provided. Trying to login with client and user credentials")
		if len(client_id) == 0 || len(client_secret) == 0 {
			log.Println("No Client ID provided. Generating new one.")
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
			log.Println("Generated ClientID and Secret for OOB Auth with scopes ", appsResponse.Scopes, ". Please keep save and add to config")
			log.Println("ID:", appsResponse.ClientId)
			log.Println("Secret:", appsResponse.ClientSecret)
			client_id = appsResponse.ClientId
			client_secret = appsResponse.ClientSecret
		}
		if len(username) == 0 || len(password) == 0 {
			// TODO: Assume client id does OOB and do oauth. In this Case the client needs to be created in the app (based on input from IRC).
			return nil, fmt.Errorf("Neither Access Token, nor Credentials provided. Please do out of band OAuth manually for now")
		}
		reply, err = client.PostForm(fmt.Sprintf(`https://%s/oauth/token`, homeserver), url.Values{
			"client_id":     {client_id},
			"client_secret": {client_secret},
			"username":      {username},
			"password":      {password},
			"grant_type":    {"password"},
			"scope":         {"read write push"},
		})
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(reply.Body)
		if err != nil {
			return nil, err
		}
		var tokenResponse tokenReply
		err = json.Unmarshal(body, &tokenResponse)
		if err != nil {
			return nil, err
		}
		access_token = tokenResponse.AccessToken
	}
	// TODO: if no access token and missing username, password do proper OOB oauth
	// missing client_id and secret can be generated against
	var mc = MastodonClient{
		notificationHandler: nil,
		token:               access_token,
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
