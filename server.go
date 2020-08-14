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
	"time"
)

const EndPointIdToken = "/instance/service-accounts/default/identity"
const EndPointProjectId = "/project/project-id"

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

func (s *server) GetGcloudIdToken() (*GcloudIdToken, error) {
	bs, err := s.getGcloudOutput([]string{"auth", "print-identity-token"})
	if err != nil {
		return nil, nil
	}
	token := &GcloudIdToken{}
	err = json.Unmarshal(bs, token)
	if err != nil {
		return nil, nil
	}
	return token, nil
}

func (s *server) GetProjectID() (string, error) {
	if s.projectId != "" {
		return s.projectId, nil
	}
	bs, err := s.getGcloudOutput([]string{"config", "get-value", "project"})
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (s *server) getGcloudOutput(params []string) ([]byte, error) {
	xs := append(params, "--format", "json")
	cmd := exec.Command(s.gcloudPath, xs...)
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
	token, err := s.GetGcloudIdToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(token.IdToken))
}

func (s *server) handleGetProjectId(w http.ResponseWriter, r *http.Request) {
	id, err := s.GetProjectID()
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

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Header().Add("allow", http.MethodGet)
		return
	}
	if !s.isLocal(r) {
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
		http.NotFound(w, r)
	}
}

func (s *server) isLocal(r *http.Request) bool {
	return r.Host == "localhost:"+s.port || r.Host == "127.0.0.1:"+s.port
}

func NewServer(port, gcloudPath, projectId string, noKey bool) Server {
	return &server{
		port:       port,
		gcloudPath: gcloudPath,
		projectId:  projectId,
		apiKey:     generateApiKey(),
		noKey:      noKey,
	}
}

func (s *server) Run() error {
	if !s.noKey {
		fmt.Printf("api key: %s\n", s.apiKey)
	}
	http.Handle("/", s)
	return http.ListenAndServe(":"+s.port, nil)
}
