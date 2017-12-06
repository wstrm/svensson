package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattn/go-xmpp"
)

var (
	sendTo                      string
	address, password, username string
	noTLS, startTLS             bool
)

func init() {
	flag.StringVar(&sendTo, "send-to", "", "User or group to send to")
	flag.StringVar(&address, "address", "", "XMPP server address")
	flag.StringVar(&password, "password", "", "Password for the bot")
	flag.StringVar(&username, "username", "", "Username for the bot")
	flag.BoolVar(&noTLS, "no-tls", false, "Connect without TLS")
	flag.BoolVar(&startTLS, "starttls", false, "Connect with STARTTLS")
	flag.Parse()

	if sendTo == "" {
		log.Fatalln("Send-to must be defined")
	}

	if address == "" {
		log.Fatalln("Address must be defined")
	}

	if password == "" {
		log.Fatalln("Password must be defined")
	}
}

const (
	originalExam = "original exam"
	resitExam    = "resit exam"
)

type exam struct {
	month    time.Month
	day      int
	examType string
}

func newExam(month time.Month, day int, examType string) exam {
	return exam{month, day, examType}
}

func (e *exam) time() time.Time {
	now := time.Now()
	year := now.Year()

	if now.Month() >= e.month {
		year++
	} else if now.Month() == e.month && now.Day() > e.day {
		year++
	}

	log.Println(e, year)

	return time.Date(year, e.month, e.day, 0, 0, 0, 0, time.UTC)
}

func findClosest(exams []exam) exam {
	closest := exams[0]
	var current exam
	for i := 1; i < len(exams); i++ {
		current = exams[i]

		if time.Until(current.time()).Nanoseconds() < time.Until(closest.time()).Nanoseconds() {
			closest = current
		}
	}

	return closest
}

func main() {
	exams := []exam{
		newExam(time.December, 4, resitExam),
		newExam(time.December, 18, originalExam),
		newExam(time.February, 22, resitExam),
		newExam(time.May, 3, originalExam),
		newExam(time.October, 2, originalExam),
	}

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
		StatusMessage: "Hello! I'm using XMPP",
	}

	client, err = options.NewClient()
	if err != nil {
		log.Fatalln(err)
	}

	_, err = client.JoinMUCNoHistory(sendTo, "Tentabot")
	if err != nil {
		log.Fatalln(err)
	}

	_, err = client.Send(
		xmpp.Chat{
			Remote: sendTo,
			Type:   "groupchat",
			Text:   "Hejsvejs, jag är Tentabot! Jag kommer påminna er om tentadatum så inte Edvin glömmer :)",
		})
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			chat, err := client.Recv()
			if err != nil {
				log.Fatal(err)
			}
			switch v := chat.(type) {
			case xmpp.Chat:
				fmt.Println(v.Remote, v.Text)
			case xmpp.Presence:
				fmt.Println(v.From, v.Show)
			}
		}
	}()

	for {
		nextExam := findClosest(exams)

		log.Println("Next exam: ", nextExam)
		time.Sleep(time.Until(nextExam.time()))

		client.Send(
			xmpp.Chat{
				Remote: sendTo,
				Type:   "chat",
				Text: fmt.Sprintf(
					"Last day to sign up for next exam period today! (%d/%d) [%s]",
					nextExam.day, nextExam.month, nextExam.examType)})

	}

}
