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

## Todo

- Write proper README
- IRC: React on private messages (use msgtype in app)
- add a lot of documentation
- mention toot author from IRC in notification about replies (?)
- detect and fix Mastodon connection issues
- Send PING to Server to measure Connection (and alert when no answer for > 60
  seconds)
