package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/go-fsnotify/fsnotify"
)

var (
	msgChan    = make(chan string, 100)
	msgPool    = make(map[string][]string)
	msgTimeout = 1 * time.Minute
)

type messageData struct {
	Subject, Body string
}

func LogWatcher(file string) {
	// First we get the original status of the file
	statOrig, err := os.Stat(file)
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("File ", event.Name, "modified.")
					stat, err := os.Stat(file)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Println(stat)
					ReadDiff(file, statOrig.Size()-stat.Size())
					statOrig = stat
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(file)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func ReadDiff(file string, offset int64) {

	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	f.Seek(offset, 2)
	b := make([]byte, 512)
	n, _ := f.Read(b)
	//	fmt.Println(string(b[:n]))

	// Handle type of message?
	msgChan <- string(b[:n])
}

func parseAuthlog() {
	var (
		timeout  <-chan time.Time
		tracking bool
	)

	for {
		// Handle if several events from same IP in <1 minute
		select {
		case message := <-msgChan:
			if !tracking {
				timeout = time.After(msgTimeout)
				tracking = true
			}

			reIP := regexp.MustCompile("[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}")
			ip := reIP.FindString(message)

			if isIgnored(message) {
				// Ignore
				fmt.Println("Ignoring...")
			} else {
				// Notify
				msgPool[ip] = append(msgPool[ip], message)
				//notifySmtp(message)
			}
		case <-timeout:
			tracking = false
			for k, v := range msgPool {
				fmt.Println(k, v)
			}
		}
	}
}

func isIgnored(message string) bool {
	// Ignore entries if matching
	reIgnore := []*regexp.Regexp{
		regexp.MustCompile(`.*CRON\[\d+\]: pam_unix\(cron:session\):.*`),
	}
	for _, re := range reIgnore {
		if re.MatchString(message) {
			// Ignoring
			return true
		}
	}
	return false
}

func notifySmtp(message messageData) {
	// Set up authentication information.
	/*
		auth := smtp.PlainAuth(
			"",
			"[InsertYourEmailHere]@gmail.com",
			"[ObviouslyThisIsNotMyPassword]",
			"[SMTP_Server]",
		)
	*/
	// Build smtp message
	msg := &bytes.Buffer{}
	if err := mailTemplate.Execute(msg, message); err != nil {
		log.Fatal(err)
	}

	fmt.Println("MESSAGE: ", msg)
	/*	err := smtp.SendMail(
		        "[SMTP_SERVER:PORT]",
						auth,
						"[EMAIL]@gmail.com",
						[]string{"[EMAIL]@gmail.com"},
						msg.Bytes(),
					)
				if err != nil {
					log.Fatal(err)
				}
	*/
}

func main() {
	go parseAuthlog()
	LogWatcher(os.Args[1])
}

// https://github.com/jroimartin/gfm/blob/master/feedmailer/feedmailer.go

var mailTemplate = template.Must(template.New("mail").Parse(mailMessage))

const mailMessage = `Subject: {{.Subject}}
MIME-version: 1.0;
Content-Type: text/html; charset="UTF-8";

{{.Body}}`
