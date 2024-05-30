# Let's go troet

A bot to toot things in Mastodon from IRC. And relay notifications back to IRC as well.

## Setup
Change values in `.env.example` to your needs and save as `.env`. `NICKSERV_PASSWORD` is optional.

## Usage
In IRC there are a few commands to interact with the bot. 

- `.t [status message]`
- `.r [message key] [reply message]`
- `.d [message key]`
- `.b [message key]`
- `.f [message key]`
- `.s [search term]`


## Todo
- Write proper README
- Handling of timeouts (primarily IRC)
- Handling of outdated token (Mastodon)
- NickServ support
- Rework errors (less verbose, more helpful)
- Eventloop / Notification Handling
- add a lot of documentation 
- mention toot author in notification about replies
