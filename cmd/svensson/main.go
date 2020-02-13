package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/wstrm/go-xmpp"
	"github.com/wstrm/svensson/giphy"
)

var (
	giphyKey                    string
	nick                        string
	muc                         string
	address, password, username string
	noTLS, startTLS             bool
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.StringVar(&nick, "nick", "Svensson", "Nick of the bot")
	flag.StringVar(&muc, "muc", "", "MUC to join")
	flag.StringVar(&address, "address", "", "XMPP server address")
	flag.StringVar(&password, "password", "", "Password for the bot")
	flag.StringVar(&username, "username", "", "Username for the bot")
	flag.StringVar(&giphyKey, "giphy-key", "", "Giphy API key")
	flag.BoolVar(&noTLS, "no-tls", false, "Connect without TLS")
	flag.BoolVar(&startTLS, "starttls", false, "Connect with STARTTLS")
	flag.Parse()

	if muc == "" {
		log.Fatalln("MUC must be defined")
	}

	if address == "" {
		log.Fatalln("Address must be defined")
	}

	if password == "" {
		log.Fatalln("Password must be defined")
	}
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.Escape(&b, []byte(s))

	return b.String()
}

func sendOOB(client *xmpp.Client, uri string) {
	_, err := client.SendOOB(
		xmpp.Chat{
			Remote: muc,
			Type:   "groupchat",
			Text:   uri,
		}, uri)
	if err != nil {
		panic(err)
	}
}

func send(client *xmpp.Client, msg string) {
	_, err := client.Send(
		xmpp.Chat{
			Remote: muc,
			Type:   "groupchat",
			Text:   msg,
		})
	if err != nil {
		panic(err)
	}
}

func commandParser(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c)
}

func listen(client *xmpp.Client) {
	for {
		chat, err := client.Recv()
		if err != nil {
			log.Fatal(err)
		}

		switch v := chat.(type) {
		case xmpp.Chat:
			fmt.Println(v.Remote, v.Text)

			command := strings.FieldsFunc(strings.ToLower(v.Text), commandParser)

			// Make sure the message is for the bot and that the message contain
			// a command after the mention.
			if len(command) > 1 && strings.EqualFold(command[0], nick) {
				command = command[1:]
			} else {
				continue
			}

			switch command[0] {
			case "hi":
				send(client, "Hi!")

			case "gif":
				if gif, err := giphy.FindGif(giphyKey, fmt.Sprint(command[1:])); err == nil {
					sendOOB(client, gif)
				} else {
					log.Println(err)
				}

			default:
				send(client, "What?")

			}

		case xmpp.Presence:
			fmt.Println(v.From, v.Show)
		}
	}
}

func main() {
	var client *xmpp.Client
	var err error

	if !noTLS || startTLS {
		xmpp.DefaultConfig = tls.Config{
			ServerName:         strings.Split(address, ":")[0],
			InsecureSkipVerify: false,
		}
	}

	options := xmpp.Options{
		Host:          address,
		User:          username,
		Password:      password,
		NoTLS:         noTLS,
		StartTLS:      startTLS,
		Session:       false,
		Status:        "xa",
		StatusMessage: fmt.Sprintf("Hello! I'm %s", nick),
	}

	client, err = options.NewClient()
	if err != nil {
		log.Fatalln(err)
	}

	_, err = client.JoinMUCNoHistory(muc, nick)
	if err != nil {
		log.Fatalln(err)
	}

	listen(client)
}
