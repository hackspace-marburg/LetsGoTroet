package irc

import (
	"LetsGoTroet/app"
	"bytes"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MSG_BUF_LEN = 10
const IRC_USER_REGEX = `[a-zA-Z0-9-_\[\]\{\}\\\|` + "`" + `]{2,9}`
const create_table = `
  CREATE TABLE IF NOT EXISTS messages_irc(
    id INTEGER NOT NULL PRIMARY KEY,
    time DATETIME NOT NULL,
    channel TEXT NOT NULL,
    user TEXT NOT NULL,
    message TEXT NOT NULL
  );
`

type IrcClient struct {
	connection *tls.Conn
	outgoing   chan string
	nick       string
	channel    string
	handlers   []Handler
	password   string
	operators  map[string]map[string]bool
	chan_msg   app.MessageHandler
	db         *sql.DB
}

type handlerfn func(regex_condition []string, client *IrcClient)

type Handler struct {
	condition regexp.Regexp
	handler   handlerfn
}

func (c *IrcClient) Eventloop() {
	active := true
	var leftover []byte
	leftover = nil

	log.Println("IRC Adapter Loop started")
	timeoffset, _ := time.ParseDuration("1s")
	for active {
		// First catch a broken connection and reinitialize
		if c.connection == nil {

			config := &tls.Config{}
			irccon, err := tls.Dial("tcp", "irc.hackint.org:6697", config)
			if err != nil {
				log.Println(err)
				// Probably something horrible happened. Let's wait a bit
				duration, _ := time.ParseDuration("10s")
				time.Sleep(duration)
				continue
			} else {
				log.Println("IRC connection established")
			}

			outbound := make(chan string, MSG_BUF_LEN)
			outbound <- "NICK " + c.nick
			outbound <- "USER " + c.nick + " * * :LetsGoTroet Bot"

			c.connection = irccon
			c.outgoing = outbound
		}

		c.connection.SetReadDeadline(time.Now().Add(timeoffset))
		// this could, in theory, be exploited by a malicuous IRC server
		// but this depends on the connection being able to deliver several gigabytes in one Read()
		buffer, err := io.ReadAll(c.connection)
		if len(buffer) == 0 {
			if !strings.Contains(err.Error(), "i/o timeout") {
				// we assume that when we did not get anything this is due to no messages being sent to us within the timeout
				// if the error message is NOT a timeout this should be handeled somehow
				log.Println("Unusual Error on recieving IRC Messages:", err)
				log.Println("Turning off IRC client.")
				c.connection = nil
				// duration, _ :=time.ParseDuration("120s") // In this case we wait 120 seconds so the IRC server can timeout our client
				// time.Sleep(duration)
				continue // Don't do anything else, our client is broken
			}

			continue_sending := true
			for continue_sending {
				select {
				case next_msg := <-c.outgoing:
					// log.Println("Sending:", next_msg)
					_, err := c.connection.Write(append([]byte(next_msg), []byte("\r\n")...))
					if err != nil {
						log.Println(err)
					}
				default:
					continue_sending = false
				}
			}
		} else {
			messages := buffer
			if leftover != nil {
				messages = append(leftover, messages...)
				leftover = nil
			}
			lines := bytes.Split(messages, []byte("\r\n"))
			if messages[len(messages)-1] != byte('\n') || messages[len(messages)-2] != byte('\r') {
				leftover = lines[len(lines)-1] // store last element as leftover (since it's not terminated by /r/n and might be followed up in next messages)
				lines = lines[:len(lines)-1]   // removes last element
			}
			for _, line := range lines {
				// log.Println(string(line)) // Enable for very verbose Debug logging
				strline := string(line)
				for _, handler := range c.handlers {
					if handler.condition.MatchString(strline) {
						go handler.handler(handler.condition.FindStringSubmatch(strline), c)
					}
				}
			}
		}
	}
	c.connection.Close()
}

// Used to set the password given to NickServ to Identify upon being requested to do so
func (c *IrcClient) SetPassword(password string) {
	c.password = password
}

// The IrcClient's Send function converts a message to a new PRIVMSG command
// to the channel configured during creation of the IrcClient (see irc.New).
// This command is not sent directly but appended to an outgoing messages queue handeled in IrcClient.Eventloop().
// Consequently it will always return nil, since we cannot track errors here.
func (c IrcClient) Send(content string) (string, error) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		c.outgoing <- "PRIVMSG #" + c.channel + " :" + line
	}
	c.storeMessage(c.nick, content)
	return content, nil
}

// Replies to a message given by messageid
// If the given content contains a " %s " it will be treated as a format string and the person to whom is replied is sprintf'd into there
// Otherwise the reply message with start with the name of the originator
// MessageIDs not found in the Database will return an error containing the id as text
func (c IrcClient) Reply(messageid string, content string) (string, error) {
	// get message/user from DB
	// if not found return a not-found error
	// else return c.Send(user + ": " + content)
	// Just a dummy r.n. to comply to app.Handler
	id, err := strconv.Atoi(messageid)
	if err != nil {
		return "", err
	}
	row := c.db.QueryRow("SELECT user FROM messages_irc WHERE id=?", id)
	var sender string // irc user names are never longer than 9 characters
	if err := row.Scan(&sender); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("Message not found in IRC Message database: %s", messageid)
		} else {
			return "", err
		}
	}
	var toSend string
	if strings.Contains(content, " %s ") {
		toSend = fmt.Sprintf(content, sender)
	} else {
		toSend = fmt.Sprint(sender, ": ", content)
	}
	return c.Send(toSend)
}

func (c *IrcClient) RegisterMessageHandler(handler app.MessageHandler) {
	log.Println("IRC -> RegisterMessageHandler")
	c.chan_msg = handler
}

func (c IrcClient) storeMessage(user string, message string) (string, error) {
	var id int64
	res, err := c.db.Exec("INSERT INTO messages_irc VALUES(NULL,?,?,?,?);", time.Now(), c.channel, user, message)
	if err != nil {
		return "", fmt.Errorf("Error during inserting message in database: %w", err)
	}
	if id, err = res.LastInsertId(); err != nil {
		return "", fmt.Errorf("Error getting Id of latest databse insert: %w", err)
	}
	return strconv.FormatInt(id, 10), nil
}

// Creates a new IrcClient. Needs a server adress (anything a net.Dial() would understand),
// a username to use as nick and a channel to join upon connection.
// This function also sets up all internal message handlers (not application specific, but IRC-protool specific).
// Most actions perfomed are done via these handlers, i.e. join channel, irc PING/PONG, monitoring for channel OPs.
// Currently the bot will only react to messages in the configured channel.
// Otherwise the IRC client would need to create app.Adapter like structs for each channel. (Which it doesn't and I don't want to scope creep)
func New(adress string, username string, channel string, db *sql.DB) (*IrcClient, error) {

	if _, err := db.Exec(create_table); err != nil {
		return nil, err
	}

	if strings.HasPrefix(channel, "#") {
		channel = channel[1:]
	}
	channel = strings.ToLower(channel)

	return &IrcClient{
		connection: nil,
		outgoing:   nil,
		nick:       username,
		channel:    channel,
		password:   "",
		handlers:   handlers,
		operators:  make(map[string]map[string]bool),
		db:         db,
		chan_msg:   nil,
	}, nil
}
