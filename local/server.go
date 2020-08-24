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

	if conf.GcloudPath == "" {
		log.Fatal("path to gcloud not specified")
	}

	s := metadataemu.NewServerFromConfig(conf)
	log.Fatal(s.Run())
}
