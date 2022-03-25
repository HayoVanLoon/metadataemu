# Google Cloud Metadata Emulator

Provides (part of) the functionality of the Compute Engine instance metadata
server. The server wraps around the Google Cloud SDK.

Supports functionality:

* Getting service account ID tokens (see caveats)
* Active account email
* Project ID

Supported endpoints:

* `computeMetadata/v1/instance/service-accounts/default/identity`
* `computeMetadata/v1/instance/service-accounts/<service account>/identity`
* `computeMetadata/v1/instance/service-accounts/default/email`
* `computeMetadata/v1/instance/service-accounts/<service account>/email`
* `computeMetadata/v1/project/project-id`

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

### From the Command Line

```shell script
curl  http://localhost:9000/computeEngine/v1/project/project-id
```

### Using Google Client Libraries

The Google client libraries can also be 'tricked' into using this emulator.
Results might vary as it has been partially reverse-engineered and only tested
for a limited set of languages (Go, Python) and libraries (Pubsub, Firestore).

The following environment variables need to be set to achieve desired
functionality. Both `GCE_METADATA_HOST` and `GCE_METADATA_IP` should point to
server authority (host & port), i.e. `localhost:9000`. Do not include the
protocol (aka `http://`).

### Using the Included Library

```go
import github.com/HayoVanLoon/metadataemu

...

client := metadata.NewClient("http://localhost:9000", "my-api-key", false, "my-service-account")
projectId, err := client.ProjectID()
```

## Caveats

* The GCP instance metadata runs in a private network. This server might not.
  Hence an apiKey query parameter must be added in calls to this server. It is
  printed to the console on server start-up and refreshes on server restart.
* When no service account is set and no audience is added, the users default
  identity token is used and the audience is not limited. Never, ever send this
  token to an untrusted source or over an untrusted medium.

## License

Copyright 2021 Hayo van Loon

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
