package irc

import (
	"log"
	"regexp"
	"strings"
)

// TODO: find and handle NickServs "register or be renamed"
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
			ic.outgoing <- "JOIN #" + ic.channel
		},
	}, {
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
	}, {
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
	}, {
		// Handle PRIVMSG
		condition: *regexp.MustCompile(`:(` + IRC_USER_REGEX + `)!\S+ PRIVMSG #(\S+) :(.+)$`),
		handler: func(s []string, ic *IrcClient) {
			user := s[1]
			channel := s[2]
			message := s[3]
			operator, ok := ic.operators[channel][user]
			if !ok {
				operator = false
			}
			if operator && channel == ic.channel && user != ic.nick && ic.chan_msg != nil {
				// log.Println("Handing off handling of Message:", user, ":", message)
				if id, err := ic.storeMessage(user, message); err != nil {
					log.Println("ERROR while trying to store IRC message:", err)
					log.Println("Due to the Error above this message will not be handeled")
				} else {
					ic.chan_msg("channel.op", message, id)
				}
			}
		},
	},
}
