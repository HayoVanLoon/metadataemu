# Google Cloud Metadata Emulator

Provides (part of) the functionality of the Compute Engine instance metadata
server. The server wraps around the `gcloud` command line tool.

Currently supports:

* Getting service account ID tokens (see caveats)
  `computeEngine/v1/instance/service-accounts/default/identity`
* Project ID
  `computeEngine/v1/project/project-id`


## Dependencies

* `gcloud` command line tool
* Go (1.13 and up)


## Run the Server

Start a server with default options:
```shell script
make run
```

To see all available command line options:
```shell script
go run local/server.go -help
```

## Use the Server

```shell script
curl  http://localhost:9000/computeEngine/v1/project/project-id
```

Using the Go client library:
```go
import github.com/HayoVanLoon/metadataemu

...

client := metadata.NewClient("http://localhost:9000", "my-api-key", false)
projectId, err := client.ProjectID()
```

The official Google metadata library might also work (for supported endpoints). 
It uses the environment variable `GCE_METADATA_HOST`, so setting this to the 
server host (i.e. `localhost:9000`) might work. This has not yet been tested.


## Caveats

* The GCP instance metadata runs in a private network. This server might not. 
Hence an apiKey query parameter must be added in calls to this server. It is 
printed to the console on server start-up and refreshes on server restart.
* The provided identity token does not limit the audience (yet). Never, ever 
send this token to an untrusted source or over an untrusted medium. 
