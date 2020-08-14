# Google Cloud Metadata Emulator

Provides (part of) the functionality of the Compute Engine instance metadata
server. 

Currently supports:

* Getting service account ID tokens (see caveats)


## Dependencies

* `gcloud` command line tool
* Go (1.13 and up)


## Run

Start a server with default options:
```shell script
make run
```

To see all available command line options:
```shell script
go run local/server.go -help
```

## Caveats

* The GCP instance metadata runs in a private network. This server might not. 
Hence an apiKey query parameter must be added in calls to this server. It is 
printed to the console on server start-up and refreshes on server restart.
* The provided identity token does not limit the audience (yet). Never, ever 
send this token to an untrusted source or over an untrusted medium. 
