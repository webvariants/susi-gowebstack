package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "github.com/webvariants/susigo"
    "golang.org/x/net/websocket"
    "io"
    "net/http"
    "os"
)

var cert = flag.String("cert", "cert.pem", "certificate to use")
var key = flag.String("key", "key.pem", "key to use")
var susiaddr = flag.String("susiaddr", "localhost:4000", "susiaddr to use")
var webaddr = flag.String("webaddr", ":8080", "webaddr to use")
var assetDir = flag.String("assets", "./assets", "asset dir to use")
var uploadDir = flag.String("uploads", "./uploads", "upload dir to use")

type Event struct {
    Topic   string      `json:"topic"`
    Payload interface{} `json:"payload"`
}

var susi *susigo.Susi = nil

func publishHandler(w http.ResponseWriter, r *http.Request) {
    decoder := json.NewDecoder(r.Body)
    evt := Event{}
    err := decoder.Decode(&evt)
    if err != nil {
        w.WriteHeader(400)
        w.Write([]byte(err.Error()))
    }
    ready := make(chan bool)
    susi.Publish(evt.Topic, evt.Payload, func(evt map[string]interface{}) {
        encoder := json.NewEncoder(w)
        encoder.Encode(evt)
        ready <- true
    })
    <-ready
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
    file, header, err := r.FormFile("file")
    if err != nil {
        fmt.Fprintln(w, err)
        return
    }
    defer file.Close()
    out, err := os.Create(*uploadDir + "/" + header.Filename)
    if err != nil {
        fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege")
        return
    }
    defer out.Close()
    _, err = io.Copy(out, file)
    if err != nil {
        fmt.Fprintln(w, err)
    }
    fmt.Fprintf(w, "File uploaded successfully : ")
    fmt.Fprintf(w, header.Filename)
}

type WebsocketMessage struct {
    Type string `json:"type"`
    Data Event  `json:"data"`
}

func websocketHandler(ws *websocket.Conn) {
    decoder := json.NewDecoder(ws)
    encoder := json.NewEncoder(ws)
    consumerIds := make(map[string]int64)
    for {
        msg := WebsocketMessage{}
        decoder.Decode(&msg)
        switch msg.Type {
        case "publish":
            {
                susi.Publish(msg.Data.Topic, msg.Data.Payload, func(evt map[string]interface{}) {
                    packet := map[string]interface{}{
                        "type": "ack",
                        "data": evt,
                    }
                    encoder.Encode(packet)
                })
                break
            }
        case "register":
            {
                if _, ok := consumerIds[msg.Data.Topic]; !ok {
                    id, err := susi.RegisterConsumer(msg.Data.Topic, func(evt map[string]interface{}) {
                        packet := map[string]interface{}{
                            "type": "event",
                            "data": evt,
                        }
                        encoder.Encode(packet)
                    })
                    if err != nil {
                        packet := map[string]interface{}{
                            "type": "error",
                            "data": err.Error(),
                        }
                        encoder.Encode(packet)
                    }
                    consumerIds[msg.Data.Topic] = id
                } else {
                    packet := map[string]interface{}{
                        "type": "error",
                        "data": "you are already registered to " + msg.Data.Topic,
                    }
                    encoder.Encode(packet)
                }
                break
            }
        case "unregister":
            {
                if _, ok := consumerIds[msg.Data.Topic]; ok {
                    susi.UnregisterConsumer(consumerIds[msg.Data.Topic])
                    delete(consumerIds, msg.Data.Topic)
                } else {
                    packet := map[string]interface{}{
                        "type": "error",
                        "data": "you are not registered to " + msg.Data.Topic,
                    }
                    encoder.Encode(packet)
                }
                break
            }
        }
    }
}

func main() {
    flag.Parse()
    s, err := susigo.NewSusi(*susiaddr, *cert, *key)
    if err != nil {
        fmt.Println(err)
        return
    }
    susi = s
    http.HandleFunc("/publish", publishHandler)
    http.HandleFunc("/upload", uploadHandler)
    http.Handle("/ws", websocket.Handler(websocketHandler))
    http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assetDir))))
    http.ListenAndServe(*webaddr, nil)
}
