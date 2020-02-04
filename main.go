package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-xmpp"
)

var (
	giphyKey                    string
	nick                        string
	muc                         string
	address, password, username string
	noTLS, startTLS             bool
)

var httpClient *http.Client

type GiphyImage struct {
	URL string `json:"url"`
}

type GiphySearchResult struct {
	Images map[string]GiphyImage `json:"images"`
}

type GiphySearchResults []GiphySearchResult

type GiphyResponse map[string]*json.RawMessage

func findGif(query string) (uri string, err error) {
	req, err := url.Parse("https://api.giphy.com/v1/gifs/search")
	if err != nil {
		log.Fatal(err)
	}

	q := req.Query()

	q.Set("api_key", giphyKey)
	q.Set("q", query)
	q.Set("limit", "1")
	q.Set("offset", "0")
	q.Set("rating", "PG-13")
	q.Set("lang", "en")

	req.RawQuery = q.Encode()

	res, err := httpClient.Get(req.String())
	if err != nil {
		return
	}

	var g GiphyResponse
	err = json.NewDecoder(res.Body).Decode(&g)
	if err != nil {
		return
	}

	var (
		data *json.RawMessage
		ok   bool
	)
	if data, ok = g["data"]; !ok {
		err = errors.New("missing data in Giphy response")
		return
	}

	var s GiphySearchResults
	err = json.Unmarshal(*data, &s)
	if err != nil {
		return
	}

	if len(s) == 0 {
		err = errors.New("empty search result from Giphy")
		return
	}

	images := s[0].Images
	originalImage, ok := images["downsized"]
	if !ok {
		err = errors.New("missing downsized image from Giphy images result")
		return
	}

	url, err := url.Parse(originalImage.URL)
	if err != nil {
		return
	}

	url.RawQuery = "" // Remove tracking from URL.
	uri = url.String()

	return
}

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

	httpTransport := &http.Transport{
		MaxIdleConns:    1,
		IdleConnTimeout: 5 * time.Second,
	}

	httpClient = &http.Client{Transport: httpTransport}
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.Escape(&b, []byte(s))

	return b.String()
}

func cnonce() string {
	randSize := big.NewInt(0)
	randSize.Lsh(big.NewInt(1), 64)
	cn, err := rand.Int(rand.Reader, randSize)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%016x", cn)
}

func sendOOB(client *xmpp.Client, uri string) {
	chat := xmpp.Chat{
		Remote: muc,
		Type:   "groupchat",
		Text:   uri,
	}

	stanza := "<message to='%s' type='%s' id='%s' xml:lang='en'><body>%s</body><x xmlns='jabber:x:oob'><url>%s</url></x></message>"

	_, err := client.SendOrg(
		fmt.Sprintf(stanza,
			xmlEscape(chat.Remote),
			xmlEscape(chat.Type),
			cnonce(),
			xmlEscape(chat.Text),
			uri,
		))
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
				if gif, err := findGif(fmt.Sprint(command[1:])); err == nil {
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
