# Let's go troet

A bot to toot things in Mastodon from IRC. And relay notifications back to IRC
as well.

## Setup

Change values in `.env.example` to your needs and save as `.env`.
`NICKSERV_PASSWORD` is optional.

## Usage

First: so you don't have to use full ids with hostnames or urls for every
reaction we use shorthand codes (a modified base64 so no easily mixed symbols
occur). From now on we will call these `message key`. The bot will print a
messages corresponding key together with the messsage. It is the code in between
the brackets at the beggining of the message.

In IRC there are a few commands to interact with the bot.

- `.t [status message]` Toots a message.
- `.r [message key] [reply message]` Replies to a message given by message key.
- `.d [message key]` Deletes the given toot. Only works on owned toots.
- `.b [message key]` Boosts/reblogs a toot. This is a toggle, repeated use will
  un-boost/reblog.
- `.f [message key]` Favourites a toot. Like `.b` this is a toggle.
- `.s [search term]` "Searches" for a toot to load via shorthand. The search
  term should be a direct link to a toot

## Learnings

Trying to fit Mastodon and IRC bots into one "Adapter" type/interface is
strange. This was done with the idea that one could exchange or expand to other
protocols in the future. The primary issue is that "sending a message" can mean
entierely different things in both cases:

- Mastodon allows to just publish a message. IRC always has a target. Fixing the
  target creates restrictions:

* this bot cannot handle bridging for multiple channels at once since the
  primary target to send a message to is fixed
* Mastodon direct Messages are just mentions with a specific privacy level.
  Currently mentioning others is not even supported, but replying _might_ work.
* IRC direct Messages are PRIVMSG messages where the channel is your own user
  name (when recieving, target's name when sending). So always using a fixed
  target needs workarounds to allow those

- Requiring a target for normal Mastodon posts is nonsense
- Mastodon has metadata attached to a post you might want to control (e.g.
  spoiler text, polls, sensitive tag, language indicator) which IRC does not
  have
- Mastodon has favoriting and reblogging, unheard of in IRC land

Subsequently I decided to split into "Adapter" and it's expansion the
"SocialAdapter". And is still feels _eeeeeeh_. The "generalization" by having a
spearate type is now much less useful and one could simply import the original
type and not deal with the intermediate types.

The "app/" part of the application was build so it could ignore protocol
specific implementation details. Yet the types of messages you need to handle
are defined and dictated by the specific implmentation.

## Todo

- Write proper README
- add a lot of documentation
- mention toot author from IRC in notification about replies
- detect and fix Mastodon connection issues
- Send PING to Server to measure Connection (and alert when no answer for > 60
  seconds)
- Respect Mastodon DMs (visibility level "direct") and respond with the same
  level. Also: mark them somehow when showing in IRC
- Implement Mastodon mute
