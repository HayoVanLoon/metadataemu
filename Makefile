
PORT := 9000
GCLOUD_PATH := $(shell which gcloud)

run:
	go run local/server.go -port=$(PORT) -gcloud-path=$(GCLOUD_PATH) -no-key=false

unsafe:
	go run local/server.go -port=$(PORT) -gcloud-path=$(GCLOUD_PATH) -no-key=true
