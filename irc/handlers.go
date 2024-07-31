package irc

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var handlers = []Handler{
	{
		condition: *regexp.MustCompile(`PING (\S+)`),
		handler: func(s []string, ic *IrcClient) {
			name := s[1]
			ic.outgoing <- "PONG " + name
		},
	}, {
		// Join channel after MOTD
		condition: *regexp.MustCompile(":[a-z.0-9]+ 376 " + IRC_USER_REGEX + " :End of /MOTD command."),
		handler: func(s []string, ic *IrcClient) {
			// Join channel
			ic.outgoing <- "JOIN " + ic.channel
		},
	}, {
		condition: *regexp.MustCompile(":[a-z.0-9]+ 376 " + IRC_USER_REGEX + " :End of /MOTD command."),
		handler: func(s []string, ic *IrcClient) {
			if len(ic.password) != 0 {
				log.Println("Sending password to NickServ")
				ic.outgoing <- fmt.Sprintf("PRIVMSG NickServ :identify %s %s", ic.nick, ic.password)
			} else {
				log.Println("No password to identify nick")
			}
		},
	}, {
		// Handle NAMES result (provided on channel join as well)
		condition: *regexp.MustCompile(`:[a-z.0-9]+ 353 \S+ = (\S+) :(.+)$`),
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
	}, {
		// Listen to MODE messages promoting/demoting operators
		condition: *regexp.MustCompile(`:\S+ MODE (\S+) ([+-])o (` + IRC_USER_REGEX + `)`),
		handler: func(s []string, ic *IrcClient) {
			channel := s[1]
			change := s[2]
			user := s[3]
			channelops, ok := ic.operators[channel]
			if !ok {
				log.Println("WARNING: During handling of MODE a new channel in the operator list was created. This should not happen.")
				channelops = make(map[string]bool)
				ic.operators[channel] = channelops
			}
			channelops[user] = (change == "+")
		},
	}, {
		// Handle PRIVMSG
		condition: *regexp.MustCompile(`:(` + IRC_USER_REGEX + `)!\S+ PRIVMSG (\S+) :(.+)$`),
		handler: func(s []string, ic *IrcClient) {
			user := s[1]
			target := s[2]
			message := s[3]
			id, err := ic.storeMessage(user, s[2], message)
			if err != nil {
				log.Println("ERROR while trying to store IRC message:", err)
				log.Println("Due to the Error above this message will not be handeled")
        return
			}
			// Check if this is a channel message
			if strings.HasPrefix(target, "#") {
				channel := target
				operator, ok := ic.operators[channel][user]
				if !ok {
					operator = false
				}
				if channel == ic.channel && user != ic.nick && ic.app_handler != nil {
					log.Println("Handing off handling of Message:", user, ":", message)
					var msg_type string
					if operator {
						msg_type = "channel.op"
					} else {
						msg_type = "channel.user"
					}
					ic.app_handler(msg_type, message, id)
				}
			} else if target == ic.nick {
				if ic.app_handler != nil {
          log.Println("Handing off handling of direct message")
          // TODO: maybe introduce privileged users which can do more via /query and adjust the type
					ic.app_handler("direct.nopermissions", message, id)
				}
			} else {
				log.Println("Message with unexpected target. Did we change nick and did not realize? Targeted at:", target)
			}
		},
	},
}
