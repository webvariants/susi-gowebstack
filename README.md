# susi-gowebstack
This is a webstack to provide an HTTP server to the [SUSI](https://github.com/webvariants/susi)
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
## Contributing
1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :D

## License
MIT License -> feel free to use it for any purpose!

See [LICENSE](LICENSE) file