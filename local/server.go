package main

import (
	"flag"
	"github.com/HayoVanLoon/metadataemu"
	"log"
)

const defaultPort = "9000"

func main() {
	port := flag.String("port", defaultPort, "server port")
	gcloudPath := flag.String("gcloud-path", "", "path to gcloud")
	noKey := flag.Bool("no-key", false, "do not require API key (discouraged)")
	flag.Parse()

	if *gcloudPath == "" {
		log.Fatal("path to gcloud not specified")
	}

	s := metadataemu.NewServer(*port, *gcloudPath, *noKey)
	log.Fatal(s.Run())
}
