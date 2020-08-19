package metadataemu

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Use 'real' metadata paths
// Source: https://cloud.google.com/compute/docs/storing-retrieving-metadata
const (
	ComputeEnginePrefix = "/computeEngine/v1"
	EndPointIdToken     = ComputeEnginePrefix + "/instance/service-accounts/default/identity"
	EndPointProjectId   = ComputeEnginePrefix + "/project/project-id"
)

type Server interface {
	Run() error
	http.Handler
}

type server struct {
	port       string
	gcloudPath string
	apiKey     string
	noKey      bool
	projectId  string
}

type GcloudIdToken struct {
	AccessToken string    `json:"access_token"`
	IdToken     string    `json:"id_token"`
	TokenExpiry time.Time `json:"token_expiry"`
}

func (s *server) getGcloudIdToken() (*GcloudIdToken, error) {
	bs, err := s.getGcloudOutput([]string{"auth", "print-identity-token", "--format", "json"})
	if err != nil {
		return nil, nil
	}
	token := &GcloudIdToken{}
	err = json.Unmarshal(bs, token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (s *server) getProjectID() (string, error) {
	if s.projectId != "" {
		return s.projectId, nil
	}
	bs, err := s.getGcloudOutput([]string{"config", "get-value", "project"})
	if err != nil {
		return "", err
	}
	str := strings.TrimSpace(string(bs))
	return str, nil
}

func (s *server) getGcloudOutput(params []string) ([]byte, error) {
	cmd := exec.Command(s.gcloudPath, params...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(stdout)
}

func (s *server) handleGetIdentity(w http.ResponseWriter, r *http.Request) {
	token, err := s.getGcloudIdToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(token.IdToken))
}

func (s *server) handleGetProjectId(w http.ResponseWriter, r *http.Request) {
	id, err := s.getProjectID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(id))
}

func generateApiKey() string {
	h := sha256.New()
	h.Write([]byte(uuid.New().String()))
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

func (s *server) checkApiKey(r *http.Request) (bool, bool) {
	if s.noKey {
		return true, true
	}
	key := r.URL.Query().Get("apiKey")
	return key == s.apiKey, key == ""
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic: %s", r)
		}
	}()
	log.Printf("%s requested %s", r.RemoteAddr, r.URL.Path)

	if !s.isLocal(r) {
		// be rude, drop connection if supported
		if wr, ok := w.(http.Hijacker); ok {
			conn, _, err := wr.Hijack()
			if err == nil {
				if err = conn.Close(); err == nil {
					return
				}
			}
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Header().Add("allow", http.MethodGet)
		return
	}
	if ok, absent := s.checkApiKey(r); !ok {
		if absent {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	if r.URL.Path == EndPointIdToken {
		s.handleGetIdentity(w, r)
	} else if r.URL.Path == EndPointProjectId {
		s.handleGetProjectId(w, r)
	} else {
		fmt.Println(r.URL.Path)
		http.NotFound(w, r)
	}
}

func (s *server) isLocal(r *http.Request) bool {
	return r.Host == "localhost:"+s.port || r.Host == "127.0.0.1:"+s.port
}

// Creates a new metadata server.
func NewServer(port, gcloudPath, projectId string, noKey bool) Server {
	return &server{
		port:       port,
		gcloudPath: gcloudPath,
		projectId:  projectId,
		apiKey:     generateApiKey(),
		noKey:      noKey,
	}
}

// Starts the local metadata server.
func (s *server) Run() error {
	fmt.Printf("metadata server listening on: http://localhost:%s\n", s.port)
	if s.noKey {
		fmt.Println("no api key required; this is unsafe on open networks")
	} else {
		fmt.Printf("api key (refreshes on restart): %s\n", s.apiKey)
	}
	http.Handle("/", s)
	return http.ListenAndServe(":"+s.port, nil)
}
