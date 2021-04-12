package metadataemu

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	gchttp "github.com/HayoVanLoon/go-commons/http"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"log"
	"net/http"
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

const (
	HeaderMetadataFlavour      = "metadata-flavor"
	HeaderValueMetadataFlavour = "Google"
	HeaderContentType          = "content-type"
	HeaderValueTextPlain       = "text/plain"
	HeaderValueApplicationJson = "application/json"
)

type ServerConfig struct {
	Port             string `json:"port"`
	GcloudPath       string `json:"gcloudPath,omitempty"`
	NoKey            bool   `json:"noKey,omitempty"`
	ProjectId        string `json:"projectId,omitempty"`
	ServiceAccount   string `json:"serviceAccount,omitempty"`
	ServiceAccountId string `json:"serviceAccountId,omitempty"`
}

type Server interface {
	Run() error
}

type server struct {
	port           string
	gcloudPath     string
	apiKey         string
	noKey          bool
	projectId      string
	serviceAccount string
}

// NewServer creates a new metadata server.
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

// NewServerFromConfig creates a new metadata server from a ServerConfig.
func NewServerFromConfig(conf *ServerConfig) Server {
	return NewServer(conf.Port, conf.GcloudPath, conf.ProjectId, conf.NoKey, conf.ServiceAccount)
}

// NewServerFromConfigFile creates a new metadata server from a ServerConfig.
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

type GcloudIdToken struct {
	AccessToken string    `json:"access_token"`
	IdToken     string    `json:"id_token"`
	TokenExpiry time.Time `json:"token_expiry"`
}

type AccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresInSec int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (s *server) getGcloudIdToken(sa, audience string) (*GcloudIdToken, error) {
	if sa == "" {
		sa = s.serviceAccount
	}
	return GetGcloudIdToken(s.gcloudPath, sa, audience)
}

func (s *server) getGcloudAccessToken(sa, audience string, scopes []string) (*AccessToken, error) {
	if sa == "" {
		sa = s.serviceAccount
	}
	log.Printf("getting access token for: %s", sa)
	return GetGcloudAccessToken(s.gcloudPath, sa, audience)
}

func (s *server) getProjectID() (string, error) {
	if s.projectId != "" {
		return s.projectId, nil
	}
	return GetProjectID(s.gcloudPath)
}

func BadRequest(w http.ResponseWriter, s string) {
	log.Printf(s)
	w.Header().Add(HeaderMetadataFlavour, HeaderValueMetadataFlavour)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(s))
}

func InternalServerError(w http.ResponseWriter, bs []byte) {
	log.Printf(string(bs))
	w.Header().Add(HeaderMetadataFlavour, HeaderValueMetadataFlavour)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(bs)
}

func Ok(w http.ResponseWriter, contentType string, bs []byte) {
	w.Header().Add(HeaderMetadataFlavour, HeaderValueMetadataFlavour)
	w.Header().Add(HeaderContentType, contentType)
	_, _ = w.Write(bs)
}

func OkPlainText(w http.ResponseWriter, bs []byte) {
	Ok(w, HeaderValueTextPlain, bs)
}

func OkJson(w http.ResponseWriter, bs []byte) {
	Ok(w, HeaderValueApplicationJson, bs)
}

func (s *server) handleGetIdentity(w http.ResponseWriter, r *http.Request) {
	sa := strings.Split(r.URL.Path, "/")[5]
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
		InternalServerError(w, []byte(err.Error()))
		return
	}

	OkPlainText(w, []byte(token.IdToken))
}

func (s *server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	sa := strings.Split(r.URL.Path, "/")[5]
	aud := r.URL.Query().Get("audience")
	if aud != "" && sa == "" {
		BadRequest(w, "need both service account and audience (or none at all)")
		return
	}
	scopes := strings.Split(r.URL.Query().Get("scopes"), ",")
	bs, err := s.getAccessTokenFromSource(scopes)
	if err != nil {
		InternalServerError(w, []byte(err.Error()))
		return
	}
	OkJson(w, bs)
}

func (s *server) getAccessTokenFromSource(scopes []string) ([]byte, error) {
	ctx := context.Background()
	source, err := google.DefaultTokenSource(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source: %s", err)
	}
	t, err := source.Token()
	if err != nil {
		return nil, fmt.Errorf("error creating token: %s", err)
	}

	resp := struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}{
		AccessToken:  t.AccessToken,
		ExpiresInSec: int(t.Expiry.Sub(time.Now()).Seconds()),
		TokenType:    t.TokenType,
	}
	return json.Marshal(resp)
}

func (s *server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Email string `json:"email"`
	}{
		Email: s.serviceAccount,
	}
	bs, _ := json.Marshal(resp)
	OkJson(w, bs)
}

func (s *server) handleGetAccountEmail(w http.ResponseWriter, r *http.Request) {
	sa := strings.Split(r.URL.Path, "/")[5]
	if sa != "default" {
		_, _ = w.Write([]byte(sa))
		return
	}
	ps := []string{"config", "get-value", "account"}
	bs, err := GetGcloudOutput(s.gcloudPath, ps)
	if err != nil {
		InternalServerError(w, []byte(err.Error()))
		return
	}
	str := strings.TrimSpace(string(bs))

	OkPlainText(w, []byte(str))
}

func (s *server) handleGetProjectId(w http.ResponseWriter, _ *http.Request) {
	id, err := s.getProjectID()
	if err != nil {
		InternalServerError(w, []byte(err.Error()))
		return
	}

	OkPlainText(w, []byte(id))
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

func (s *server) filter(fn func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

		fn(w, r)
	}
}

func (s *server) isLocal(r *http.Request) bool {
	// TODO(hvl): wonky
	return r.Host == "localhost:"+s.port || r.Host == "127.0.0.1:"+s.port
}

func notFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("not found: %s", r.URL)
	http.NotFound(w, r)
}

func pong(w http.ResponseWriter, r *http.Request) {
	log.Printf("pong: %s", r.URL)
	OkPlainText(w, []byte{})
}

// Run starts the local metadata server.
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

	tm := gchttp.NewTreeMuxWithNotFound(notFound)
	tm.HandleFunc("/computeMetadata/v1/project/project-id", s.filter(s.handleGetProjectId))
	tm.HandleFunc("/computeMetadata/v1/instance/service-accounts/*/email", s.filter(s.handleGetAccountEmail))
	tm.HandleFunc("/computeMetadata/v1/instance/service-accounts/*/identity", s.filter(s.handleGetIdentity))
	tm.HandleFunc("/computeMetadata/v1/instance/service-accounts/*/token", s.filter(s.handleGetToken))
	tm.HandleFunc("/computeMetadata/v1/instance/service-accounts/*/", s.filter(s.handleGetAccount))

	// pinged by Python GCE metadata
	tm.HandleFunc("/", pong)

	http.Handle("/", tm)
	return http.ListenAndServe(":"+s.port, nil)
}
