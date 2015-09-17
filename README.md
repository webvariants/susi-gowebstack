# susi-gowebstack
This is a webstack to provide an HTTP server to the [SUSI](https://github.com/webvariants/susi) ecosystem.
## Installation
To install just use the go command:
```
go get github.com/webvariants/susi-gowebstack
```
This will install the webstack with its dependencies to your $GOROOT
## Usage
The susi-gowebstack binary need some commandline options
```
Usage of susi-gowebstack:
  -assets="./assets"         asset dir to use
  -cert="cert.pem"           certificate to use
  -key="key.pem"             key to use
  -susiaddr="localhost:4000" susiaddr to use
  -uploads="./uploads"       upload dir to use
  -webaddr=":8080"           webaddr to use
  -help                      show this help
```

susiaddr, key and cert are manadatory to communicate to your local susi-core instance (See the [Susi-Readme](https://github.com/webvariants/susi) to get an susi-core instance running)

assets and uploads are for specifying where your http server get files from respectively where it puts uploaded files.

The webstack does not only delivers your website but it gives you access to the susi-eventsystem via websockets + you can publish events via POST requests to the API endpoint /publish.

It implements the level-1 SUSI protocol, therefore the structure of a valid websocket message is like this:
```json
{
	"type": "publish",
	"data": {
		"topic": "test-topic",
		"payload": {
			"some": "data"
		}
	}
}
``` 

There are three valid "type"'s: publish, register and unregister. The "topic" field in the "data" field is mandatory for all of the types.
If you publish an event, you will get an packet with type "ack", which signales that the eventprocessing finished. If you "register" for an topic you will get a packet with type "event".

## Contributing
1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :D

## License
MIT License -> feel free to use it for any purpose!

See [LICENSE](LICENSE) file