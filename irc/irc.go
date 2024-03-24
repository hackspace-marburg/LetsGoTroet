package irc

import (
	"LetsGoTroet/app"
	"bytes"
	"crypto/tls"
	"database/sql"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
  "fmt"
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
	operators  map[string]map[string]bool
	chan_msg   app.MessageHandler
	db         *sql.DB
}

type handlerfn func(regex_condition []string, client *IrcClient)

type Handler struct {
	condition regexp.Regexp
	handler   handlerfn
}

func (c IrcClient) Eventloop() {
	buffer := make([]byte, 2000)
	active := true
  var leftover []byte 
  leftover = nil

  log.Println("IRC Adapter Loop started")
	timeoffset, _ := time.ParseDuration("1s")
	for active {
		c.connection.SetReadDeadline(time.Now().Add(timeoffset))
		n, err := c.connection.Read(buffer)
		if err != nil {
			// this error is most likely occuring since we did not get new messages within the set Deadline
			// so for now i go full YOLO by SENDING messages now instead
			//
			// TODO: check what kind of error this is.
			// a read time out essentially means we're good to go.
			// But a closed connection would mean we should do something different
			continue_sending := true
			for continue_sending {
				select {
				case next_msg := <-c.outgoing:
					log.Println("Sending:", next_msg)
					_, err := c.connection.Write(append([]byte(next_msg), []byte("\r\n")...))
					if err != nil {
						log.Println(err)
					}
				default:
					continue_sending = false
					time.Sleep(timeoffset)
				}
			}
		} else {
      messages := buffer
      // This handling of split messages DETECTS the split but there seem to be problems with reassembling
      if leftover != nil {
        messages = append(leftover, messages...)
        leftover = nil
      }
			lines := bytes.Split(messages[:n-1], []byte("\r\n"))
			if messages[n-1] != byte('\n') || messages[n-2] != byte('\r') {
        leftover = lines[len(lines)-1] // store last element as leftover (since it's not terminated by /r/n and might be followed up in next messages)
        lines = lines[:len(lines)-1] // removes last element
			}
			for _, line := range lines {
        log.Println(string(line))
				strline := string(line)
				for _, handler := range c.handlers {
					if handler.condition.MatchString(strline) {
						go handler.handler(handler.condition.FindStringSubmatch(strline), &c)
					}
				}
				// log.Println(string(line))
			}
		}
	}
	c.connection.Close()
}

// The IrcClient's Send function converts a message to a new PRIVMSG command
// to the channel configured during creation of the IrcClient (see irc.New).
// This command is not sent directly but appended to an outgoing messages queue handeled in IrcClient.Eventloop().
// Consequently it will always return nil, since we cannot track errors here.
func (c IrcClient) Send(content string) error {
	c.outgoing <- "PRIVMSG #" + c.channel + " :" + content
	return nil
}

// Replies to a message given by messageid
// If the given content contains a " %s " it will be treated as a format string and the person to whom is replied is sprintf'd into there
// Otherwise the reply message with start with the name of the originator
// MessageIDs not found in the Database will return an error containing the id as text
func (c IrcClient) Reply(messageid string, content string) error {
	// get message/user from DB
	// if not found return a not-found error
	// else return c.Send(user + ": " + content)
	// Just a dummy r.n. to comply to app.Handler
  id, err := strconv.Atoi(messageid)
  if err != nil {
    return err
  }
  row := c.db.QueryRow("SELECT user FROM messages_irc WHERE id=?", id)
  var sender string // irc user names are never longer than 9 characters
  if err := row.Scan(&sender); err != nil {
    if errors.Is(err, sql.ErrNoRows){
      return errors.New(fmt.Sprint("Message not found in IRC Message database:", id))
    } else {
      return err
    }
  }
  if strings.Contains(content, " %s "){
    return c.Send(fmt.Sprintf(content, sender))
  }
  return c.Send(fmt.Sprint(sender, ": ", content))
}

func (c *IrcClient) RegisterMessageHandler(handler app.MessageHandler) {
  log.Println("IRC -> RegisterMessageHandler")
  c.chan_msg = handler
}

func (c IrcClient) storeMessage(user string, message string) (string, error) {
  var id int64
  res, err := c.db.Exec("INSERT INTO messages_irc VALUES(NULL,?,?,?,?);", time.Now(), c.channel, user, message)
  if err != nil {
    return "", err
  }
  if id, err = res.LastInsertId(); err != nil {
    return "", err
  }
  return strconv.FormatInt(id, 10), nil
}

// Creates a new IrcClient. Needs a server adress (anything a net.Dial() would understand),
// a username to use as nick and a channel to join upon connection.
// This function also sets up all internal message handlers (not application specific, but IRC-protool specific).
// Most actions perfomed are done via these handlers, i.e. join channel, irc PING/PONG, monitoring for channel OPs.
// Currently the bot will only react to messages in the configured channel.
// Otherwise the IRC client would need to create app.Adapter like structs for each channel. (Which it doesn't and I don't want to scope creep)
// TODO: handle NickServ login
func New(adress string, username string, channel string, db *sql.DB) (*IrcClient, error) {

  if _, err := db.Exec(create_table); err != nil {
    return nil, err
  }

	config := &tls.Config{}
	irccon, err := tls.Dial("tcp", "irc.hackint.org:6697", config)
	if err != nil {
		log.Println(err)
		return nil, err
	}
  if strings.HasPrefix(channel, "#") {
    channel = channel[1:]
  }
  channel = strings.ToLower(channel)

	outbound := make(chan string, MSG_BUF_LEN)
	outbound <- "NICK " + username
	outbound <- "USER " + username + " * * :LetsGoTroet Bot"

	var handlers []Handler
	handlers = append(handlers, Handler{
		condition: *regexp.MustCompile(`PING (\S+)`),
		handler: func(s []string, ic *IrcClient) {
			name := s[1]
			ic.outgoing <- "PONG " + name
		},
	}, Handler{
		// Join channel after MOTD
		condition: *regexp.MustCompile(":[a-z.0-9]+ 376 " + username + " :End of /MOTD command."),
		handler: func(s []string, ic *IrcClient) {
			// Join channel
			ic.outgoing <- "JOIN #" + ic.channel
		},
	}, Handler{
		// Handle NAMES result (provided on channel join as well)
		condition: *regexp.MustCompile(`:[a-z.0-9]+ 353 \S+ = #(\S+) :(.+)$`),
		handler: func(s []string, ic *IrcClient) {
			channel := s[1]
			names := strings.Split(s[2], " ")
			for _, name := range names {
				if strings.HasPrefix(name, "@") {
					// if someone is an operator, save them in the Operators map
					cu, ok := ic.operators[channel]
					if !ok {
						cu = make(map[string]bool)
						ic.operators[channel] = cu
					}
					cu[name[1:]] = true
					log.Println("Found OP:", name, "in Channel", channel)
				}
			}
		},
	}, Handler{
		// Listen to MODE messages promoting/demoting operators
		condition: *regexp.MustCompile(`:\S+ MODE #(\S+) ([+-])o (` + IRC_USER_REGEX + `)`),
		handler: func(s []string, ic *IrcClient) {
			channel := s[1]
			change := s[2]
			user := s[3]
			channelops, ok := ic.operators[channel]
			if !ok {
				log.Println("WARNING: During handling of MODE a new channel in operator list was created. This should not happen and might loose knowledge about existing ops")
				channelops = make(map[string]bool)
				ic.operators[channel] = channelops
			}
			channelops[user] = (change == "+")
		},
	}, Handler{
		// Handle PRIVMSG
		condition: *regexp.MustCompile(`:(` + IRC_USER_REGEX + `)!\S+ PRIVMSG #(\S+) :(.+)$`),
		handler: func(s []string, ic *IrcClient) {
			user := s[1]
			channel := s[2]
			message := s[3][:len(s[3])-1]
			operator, ok := ic.operators[channel][user]
			if !ok {
				operator = false
			}
			if operator && channel == ic.channel && user != ic.nick && ic.chan_msg != nil {
				log.Println("Handing off handling of Message:", user, ":", message)
        if id, err := ic.storeMessage(user, message);err != nil {
          log.Println("ERROR while trying to store IRC message:", err)
          log.Println("Due to the Error above this message will not be handeled")
        } else {
				  ic.chan_msg(user, message, id)
        }
			}
		},
	},
	)

	return &IrcClient{
		connection: irccon,
		outgoing:   outbound,
		nick:       username,
		channel:    channel,
		handlers:   handlers,
		operators:  make(map[string]map[string]bool),
    db:         db,
    chan_msg:   nil,
	}, nil
}
