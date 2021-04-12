package metadataemu

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
)

func GetGcloudOutput(gcloudPath string, params []string) ([]byte, error) {
	// TODO(hvl): debug: log.Printf("gcloud %s", strings.Join(params, " "))
	log.Printf("gcloud %v", params)
	cmd := exec.Command(gcloudPath, params...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	bs, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Printf("error getting gcloud output: %s", err)
	}
	log.Printf("gcloud output: %s", bs)
	if err = cmd.Wait(); err != nil {
		log.Printf("gcloud error: %s", err)
		return nil, err
	}
	return bs, nil
}

func GetProjectID(gcloudPath string) (string, error) {
	bs, err := GetGcloudOutput(gcloudPath, []string{"config", "get-value", "project"})
	if err != nil {
		return "", err
	}
	str := strings.TrimSpace(string(bs))
	return str, nil
}

func GetGcloudAccessToken(gcloudPath, sa, audience string) (*AccessToken, error) {
	ps := []string{"auth", "print-access-token"}
	if sa != "default" && audience != "" {
		if sa == "" {
			return nil, fmt.Errorf("need service account for audiences, please specify one or set server default")
		}
		ps = append(ps,
			fmt.Sprintf("--audiences=%s", audience),
			fmt.Sprintf("--impersonate-service-account=%s", sa),
		)
	}
	bs, err := GetGcloudOutput(gcloudPath, append(ps, "--format=json"))
	if err != nil {
		return nil, err
	}
	token := &AccessToken{}
	err = json.Unmarshal(bs, token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func GetGcloudIdToken(gcloudPath, sa, audience string) (*GcloudIdToken, error) {
	ps := []string{"auth", "print-identity-token"}
	if sa != "default" && audience != "" {
		if sa == "" {
			return nil, fmt.Errorf("need service account for audiences, please specify one or set server default")
		}
		ps = append(ps,
			fmt.Sprintf("--audiences=%s", audience),
			fmt.Sprintf("--impersonate-service-account=%s", sa),
		)
	}
	bs, err := GetGcloudOutput(gcloudPath, append(ps, "--format=json"))
	if err != nil {
		return nil, err
	}
	token := &GcloudIdToken{}
	err = json.Unmarshal(bs, token)
	if err != nil {
		return nil, err
	}
	return token, nil
}
