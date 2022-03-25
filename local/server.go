// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by a licence
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/HayoVanLoon/metadataemu"
	"io/ioutil"
	"log"
	"os"
)

const defaultPort = "9000"

func main() {
	port := flag.String("port", defaultPort, "server port")
	gcloudPath := flag.String("gcloud-path", "", "path to gcloud")
	projectId := flag.String("project", "", "overrides gcloud's current project id")
	noKey := flag.Bool("no-key", false, "do not require API key (discouraged)")
	serviceAccount := flag.String("service-account", "", "service account to impersonate (required when using audience)")
	serviceAccountId := flag.String("service-account-id", "", "prefix of service account to impersonate (required when using audience)")
	configFile := flag.String("config-file", "", "path to config file")
	flag.Parse()

	conf := &metadataemu.ServerConfig{}
	if *configFile != "" {
		bs, err := ioutil.ReadFile(*configFile)
		if err != nil {
			fmt.Printf("could not read config file: %s", err)
			os.Exit(1)
		}
		err = json.Unmarshal(bs, conf)
		if err != nil {
			fmt.Printf("could not parse config file: %s", err)
			os.Exit(1)
		}
	}

	if *port != "" {
		conf.Port = *port
	}
	if *gcloudPath != "" {
		conf.GcloudPath = *gcloudPath
	}
	if *projectId != "" {
		conf.ProjectId = *projectId
	}
	if *noKey {
		conf.NoKey = *noKey
	}

	if *serviceAccount != "" {
		conf.ServiceAccount = *serviceAccount
	}
	if *serviceAccountId != "" {
		conf.ServiceAccountId = *serviceAccountId
	}

	if conf.ServiceAccountId != "" {
		if conf.ProjectId == "" {
			log.Println("Service account id has been specified but project has not. Ignoring service account id.")
		} else if conf.ServiceAccount != "" {
			log.Println("Both service account and service account id have been specified. Ignoring Service-Account-Id")
		} else {
			conf.ServiceAccount = fmt.Sprintf("%s@%s.iam.gserviceaccount.com", conf.ServiceAccountId, conf.ProjectId)
		}
	}

	if conf.GcloudPath == "" {
		log.Fatal("path to gcloud not specified")
	}

	s := metadataemu.NewServerFromConfig(conf)
	log.Fatal(s.Run())
}
