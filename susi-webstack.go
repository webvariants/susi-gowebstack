package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/webvariants/susigo"
	"golang.org/x/net/websocket"
)

var cert = flag.String("cert", "cert.pem", "certificate to use")
var key = flag.String("key", "key.pem", "key to use")
var susiaddr = flag.String("susiaddr", "localhost:4000", "susiaddr to use")
var webaddr = flag.String("webaddr", ":8080", "webaddr to use")
var assetDir = flag.String("assets", "./assets", "asset dir to use")
var uploadDir = flag.String("uploads", "./uploads", "upload dir to use")
var useHTTPS = flag.Bool("https", false, "whether to use https or not")

var susi *susigo.Susi
var sessionStore = sessions.NewCookieStore([]byte("my-very-secret-cookie-encryption-key"))

var sessionTimeouts map[string]time.Time
var sessionTimeoutsMutex sync.Mutex
var defaultSessionLifetime, _ = time.ParseDuration("1m")

func sessionHandling(w http.ResponseWriter, r *http.Request) (string, error) {
	session, err := sessionStore.Get(r, "session")
	if err != nil {
		return "", err
	}
	if session.Values["id"] == nil {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			return "", err
		}
		id := base64.StdEncoding.EncodeToString(b)
		session.Values["id"] = id
		session.Save(r, w)
		sessionTimeouts[id] = time.Now().Add(defaultSessionLifetime)
		log.Printf("new session %v", id)
		susi.Publish(susigo.Event{Topic: "webstack::session::new", Payload: id}, func(evt *susigo.Event) {})
		go func() {
			for {
				sessionTimeoutsMutex.Lock()
				if time.Now().After(sessionTimeouts[id]) {
					susi.Publish(susigo.Event{Topic: "webstack::session::lost", Payload: id}, func(evt *susigo.Event) {})
					log.Printf("lost session %v", id)
					delete(sessionTimeouts, id)
					sessionTimeoutsMutex.Unlock()
					break
				}
				sessionTimeoutsMutex.Unlock()
				time.Sleep(5 * time.Second)
			}
		}()
		log.Printf("created new session %v", session.Values["id"])
	}
	return session.Values["id"].(string), nil
}

func keepAliveHandler(w http.ResponseWriter, r *http.Request) {
	id, err := sessionHandling(w, r)
	if err != nil {
		log.Printf("error in session handling (%v)", err)
		fmt.Fprintf(w, "Error: %v", err)
		return
	}
	sessionTimeoutsMutex.Lock()
	sessionTimeouts[id] = time.Now().Add(defaultSessionLifetime)
	sessionTimeoutsMutex.Unlock()
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
	id, err := sessionHandling(w, r)
	if err != nil {
		log.Printf("error in session handling (%v)", err)
		fmt.Fprintf(w, "Error: %v", err)
		return
	}
	decoder := json.NewDecoder(r.Body)
	evt := susigo.Event{}
	err = decoder.Decode(&evt)
	if err != nil {
		log.Printf("error while decoding event from http-publish-request body: %v", err)
		w.WriteHeader(400)
		w.Write([]byte("malformed payload"))
		return
	}
	if evt.Topic == "" {
		log.Printf("error while decoding event from http-publish-request body: %v", "topic empty")
		w.WriteHeader(400)
		w.Write([]byte("you MUST specify at least a topic for your event"))
		return
	}
	evt.SessionID = id
	ready := make(chan bool)

	susi.Publish(evt, func(evt *susigo.Event) {
		encoder := json.NewEncoder(w)
		encoder.Encode(evt)
		ready <- true
	})

	log.Printf("publish event with topic '%v' for session %v via http", evt.Topic, id)
	<-ready
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	id, err := sessionHandling(w, r)
	if err != nil {
		log.Printf("error in session handling (%v)", err)
		fmt.Fprintf(w, "Error: %v", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer file.Close()
	out, err := os.Create(*uploadDir + "/" + header.Filename)
	if err != nil {
		log.Printf("Unable to create the file %v for writing. Check your write access privilege (session: %v, error: %v)", *uploadDir+"/"+header.Filename, id, err)
		fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege")
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprintln(w, err)
	}
	log.Printf("successfully accepted file %v from session %v", *uploadDir+"/"+header.Filename, id)
	fmt.Fprintf(w, "File uploaded successfully : ")
	fmt.Fprintf(w, header.Filename)
}

type websocketMessage struct {
	Type string       `json:"type"`
	Data susigo.Event `json:"data"`
}

func websocketHandler(ws *websocket.Conn) {
	session, _ := sessionStore.Get(ws.Request(), "session")
	id := session.Values["id"].(string)
	decoder := json.NewDecoder(ws)
	encoder := json.NewEncoder(ws)
	consumerIds := make(map[string]int64)

	for {
		msg := websocketMessage{}
		err := decoder.Decode(&msg)
		if err != nil {
			log.Println("lost websocket connection")
			break
		}
		switch msg.Type {
		case "publish":
			{
				msg.Data.SessionID = id
				susi.Publish(msg.Data, func(evt *susigo.Event) {
					packet := map[string]interface{}{
						"type": "ack",
						"data": evt,
					}
					err := encoder.Encode(packet)
					if err != nil {
						log.Printf("lost websocket client (error: %v)", err)
						return
					}
				})
				log.Printf("publish event with topic '%v' for session %v via websocket", msg.Data.Topic, id)
				break
			}
		case "register":
			{
				if _, ok := consumerIds[msg.Data.Topic]; !ok {
					id, err := susi.RegisterConsumer(msg.Data.Topic, func(evt *susigo.Event) {
						packet := map[string]interface{}{
							"type": "event",
							"data": evt,
						}
						err := encoder.Encode(packet)
						if err != nil {
							log.Printf("lost websocket client (error: %v)", err)
							return
						}
					})
					if err != nil {
						log.Printf("failed registering session %v to topic '%v' (%v)", id, msg.Data.Topic, err)
						packet := map[string]interface{}{
							"type": "error",
							"data": err.Error(),
						}
						err := encoder.Encode(packet)
						if err != nil {
							log.Printf("lost websocket client (error: %v)", err)
							return
						}
					}
					consumerIds[msg.Data.Topic] = id
				} else {
					log.Printf("failed registering session %v to topic '%v' (%v)", id, msg.Data.Topic, "session already registered")
					packet := map[string]interface{}{
						"type": "error",
						"data": "you are already registered to " + msg.Data.Topic,
					}
					err := encoder.Encode(packet)
					if err != nil {
						log.Printf("lost websocket client (error: %v)", err)
						return
					}
				}

				log.Printf("successfully registered session %v to topic '%v'", id, msg.Data.Topic)
				break
			}
		case "unregister":
			{
				if _, ok := consumerIds[msg.Data.Topic]; ok {
					err := susi.UnregisterConsumer(consumerIds[msg.Data.Topic])
					if err != nil {
						log.Printf("error unregistering %v from topic '%v' (%v)", id, msg.Data.Topic, err)
						packet := map[string]interface{}{
							"type": "error",
							"data": fmt.Sprintf("susi error: %v", err),
						}
						err := encoder.Encode(packet)
						if err != nil {
							log.Printf("lost websocket client (error: %v)", err)
							return
						}
					}
					log.Printf("successfully registered session %v to topic '%v'", id, msg.Data.Topic)
					delete(consumerIds, msg.Data.Topic)
				} else {
					log.Printf("error unregistering %v from topic '%v' (%v)", id, msg.Data.Topic, "(user not registered)")
					packet := map[string]interface{}{
						"type": "error",
						"data": "you are not registered to " + msg.Data.Topic,
					}
					err := encoder.Encode(packet)
					if err != nil {
						log.Printf("lost websocket client (error: %v)", err)
						return
					}
				}
				break
			}
		}
	}
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {
	sessionHandling(w, r)
	http.Redirect(w, r, "/assets/index.html", 301)
}

func main() {
	flag.Parse()
	s, err := susigo.NewSusi(*susiaddr, *cert, *key)
	if err != nil {
		log.Printf("Error while creating susi connection: %v", err)
		return
	}
	susi = s
	log.Println("successfully create susi connection")
	sessionTimeouts = make(map[string]time.Time)
	http.HandleFunc("/publish", publishHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.Handle("/ws", websocket.Handler(websocketHandler))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assetDir))))
	http.HandleFunc("/", redirectToIndex)
	log.Printf("starting http server on %v...", *webaddr)
	if *useHTTPS {
		log.Fatal(http.ListenAndServeTLS(*webaddr, *cert, *key, context.ClearHandler(http.DefaultServeMux)))
	} else {
		log.Fatal(http.ListenAndServe(*webaddr, context.ClearHandler(http.DefaultServeMux)))
	}
}
