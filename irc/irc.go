package irc

import (
	"bytes"
	"crypto/tls"
	"log"
	"regexp"
	"time"
)

const MSG_BUF_LEN = 10

type IrcClient struct {
	connection *tls.Conn
	outgoing   chan string
	channel    string
	handlers   []Handler
}

type handlerfn func([]string, *IrcClient)

type Handler struct {
	condition regexp.Regexp
	handler   handlerfn
}

func (c IrcClient) Eventloop() {
	buffer := make([]byte, 2000)
	active := true

	// re_ping := regexp.MustCompile("PING (\\S+)")
	timeoffset, _ := time.ParseDuration("1s")
	for active {
		c.connection.SetReadDeadline(time.Now().Add(timeoffset))
		n, err := c.connection.Read(buffer)
		if err != nil {
			// this error is most likely occuring since we did not get new messages within the set Deadline
			// so for now i go full YOLO by SENDING messages now instead
			//
			// Better future state: check what kind of error this is.
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
			if buffer[n-1] != byte('\n') {
				log.Println("WARNING: Latest READ did not end with a finished message")
			}
			lines := bytes.Split(buffer[:n-1], []byte("\r\n"))
			for _, line := range lines {
				strline := string(line)
				for _, handler := range c.handlers {
					if handler.condition.MatchString(strline) {
						go handler.handler(handler.condition.FindStringSubmatch(strline), &c)
					}
				}
				log.Println(string(line))
			}
		}
	}
	c.connection.Close()
}

func New(adress string, username string, channel string) (*IrcClient, error) {

	config := &tls.Config{}
	irccon, err := tls.Dial("tcp", "irc.hackint.org:6697", config)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	outbound := make(chan string, MSG_BUF_LEN)
	outbound <- "NICK " + username
	outbound <- "USER " + username + " * * :LetsGoTroet Bot"
	//outbound <- "JOIN " + channel // should happen after ":End of /MOTD command"

	var handlers []Handler
	handlers = append(handlers, Handler{
		condition: *regexp.MustCompile("PING (\\S+)"),
		handler: func(s []string, ic *IrcClient) {
			name := s[1]
			ic.outgoing <- "PONG " + name
		},
	})

	handlers = append(handlers, Handler{
		condition: *regexp.MustCompile(":[a-z.0-9]+ [0-9]{3} " + username + " :End of /MOTD command."),
		handler: func(s []string, ic *IrcClient) {
			ic.outgoing <- "JOIN " + channel
		},
	})

	return &IrcClient{
		connection: irccon,
		outgoing:   outbound,
		channel:    channel,
		handlers:   handlers,
	}, nil
}
