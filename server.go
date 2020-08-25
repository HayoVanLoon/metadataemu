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
	"regexp"
	"strings"
	"time"
)

// Use 'real' metadata paths
// Source: https://cloud.google.com/compute/docs/storing-retrieving-metadata
const (
	ComputeMetadataPrefix   = "/computeMetadata/v1"
	EndPointServiceAccounts = ComputeMetadataPrefix + "/instance/service-accounts"
	EndPointProjectId       = ComputeMetadataPrefix + "/project/project-id"
)

var regexServiceAccount = regexp.MustCompile(`^/computeMetadata/v1/instance/service-accounts/([^/]+)(/[^/]+)?`)

type Server interface {
	Run() error
	http.Handler
}

type ServerConfig struct {
	Port           string `json:"port"`
	GcloudPath     string `json:"gcloudPath,omitempty"`
	NoKey          bool   `json:"noKey,omitempty"`
	ProjectId      string `json:"projectId,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

type server struct {
	port           string
	gcloudPath     string
	apiKey         string
	noKey          bool
	projectId      string
	serviceAccount string
}

type GcloudIdToken struct {
	AccessToken string    `json:"access_token"`
	IdToken     string    `json:"id_token"`
	TokenExpiry time.Time `json:"token_expiry"`
}

func (s *server) getGcloudIdToken(sa, audience string) (*GcloudIdToken, error) {
	ps := []string{"auth", "print-identity-token"}
	if sa != "default" && audience != "" {
		if sa == "" {
			sa = s.serviceAccount
		}
		if sa == "" {
			return nil, fmt.Errorf("need service account for audiences, please specify one or set server default")
		}
		ps = append(ps,
			fmt.Sprintf("--audiences=%s", audience),
			fmt.Sprintf("--impersonate-service-account=%s", sa),
		)
	}
	bs, err := s.getGcloudOutput(append(ps, "--format=json"))
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
	// TODO(hvl): debug: log.Printf("gcloud %s", strings.Join(params, " "))
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

func (s *server) handleServiceAccount(w http.ResponseWriter, r *http.Request, tail []string) {
	if len(tail) < 2 {
		http.NotFound(w, r)
		return
	}
	sa := tail[0]
	if matches(true, tail, sa, "identity") {
		s.handleGetIdentity(w, r, sa)
		return
	}
	http.NotFound(w, r)
}

func (s *server) handleGetIdentity(w http.ResponseWriter, r *http.Request, sa string) {
	aud := r.URL.Query().Get("audience")
	if aud != "" && sa == "" {
		msg := "need both service account and audience (or none at all)"
		log.Printf(msg)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(msg))
		return
	}

	token, err := s.getGcloudIdToken(sa, aud)
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
		log.Printf("forbidden: non-local origin")
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
		// only allow GET requests
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Header().Add("allow", http.MethodGet)
		log.Printf("405 due to %s on %s", r.Method, r.URL.Path)
		return
	}
	if ok, absent := s.checkApiKey(r); !ok {
		if absent {
			log.Printf("unauthorised: no api key")
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			log.Printf("forbidden: incorrect api key")
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	if !strings.HasPrefix(r.URL.Path, ComputeMetadataPrefix) {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(r.URL.Path, "/")[3:]
	if matches(false, parts, "instance", "service-accounts") {
		s.handleServiceAccount(w, r, parts[2:])
	} else if matches(true, parts, "project", "project-id") {
		s.handleGetProjectId(w, r)
	} else {
		log.Printf("not found: %s", r.URL.Path)
		http.NotFound(w, r)
	}
}

func matches(exact bool, ss []string, target ...string) bool {
	if len(ss) < len(target) {
		return false
	}
	for i := range target {
		if ss[i] != target[i] {
			return false
		}
	}
	return !exact || len(ss) == len(target)
}

func (s *server) isLocal(r *http.Request) bool {
	return r.Host == "localhost:"+s.port || r.Host == "127.0.0.1:"+s.port
}

// Creates a new metadata server.
func NewServer(port, gcloudPath, projectId string, noKey bool, serviceAccount string) Server {
	apiKey := ""
	if !noKey {
		apiKey = generateApiKey()
	}
	return &server{
		port:           port,
		gcloudPath:     gcloudPath,
		projectId:      projectId,
		apiKey:         apiKey,
		noKey:          noKey,
		serviceAccount: serviceAccount,
	}
}

// Creates a new metadata server from a ServerConfig
func NewServerFromConfig(conf *ServerConfig) Server {
	return NewServer(conf.Port, conf.GcloudPath, conf.ProjectId, conf.NoKey, conf.ServiceAccount)
}

// Creates a new metadata server from a ServerConfig
func NewServerFromConfigFile(path string) (Server, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %s", path, err)
	}
	conf := &ServerConfig{}
	err = json.Unmarshal(bs, conf)
	if err != nil {
		return nil, fmt.Errorf("could not parse config file: %s", err)
	}
	return NewServerFromConfig(conf), nil
}

// Starts the local metadata server.
func (s *server) Run() error {
	fmt.Printf("\nmetadata server listening on:\thttp://localhost:%s\n", s.port)
	if s.noKey {
		fmt.Println("no api key required; this is unsafe on open networks")
	} else {
		fmt.Printf("api key (refreshes on restart):\t%s\n", s.apiKey)
	}

	project := s.projectId
	if s.projectId == "" {
		var err error
		project, err = s.getProjectID()
		if err != nil {
			return fmt.Errorf("could not get project ID: %s", err)
		}
	}
	fmt.Printf("\nactive project:\t%s\n", project)

	if s.serviceAccount != "" {
		fmt.Printf("active service account:\t%s\n", s.serviceAccount)
	}

	fmt.Println()

	http.Handle("/", s)
	return http.ListenAndServe(":"+s.port, nil)
}
