# Copyright 2021 Hayo van Loon.
# Use of this source code is governed by an Apache-style
# license that can be found in the LICENSE file.

PORT := 9000
GCLOUD_PATH := $(shell which gcloud)

# Should point to a valid config file.
CONFIG_FILE := etc/config.json


run:
	go run local/server.go \
		-port=$(PORT) \
		-gcloud-path=$(GCLOUD_PATH) \
		-no-key=false

# Use gcloud retrieved from shell and a config file for the rest
run-with-config:
	go run local/server.go \
		-config-file=$(CONFIG_FILE) \
		-gcloud-path=$(GCLOUD_PATH)

unsafe:
	go run local/server.go \
		-port=$(PORT) \
		-gcloud-path=$(GCLOUD_PATH) \
		-no-key=true
