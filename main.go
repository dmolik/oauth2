package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	o "golang.org/x/oauth2"
	oauth2 "golang.org/x/oauth2/clientcredentials"
)

type lokiLabels struct {
	Status string   `json:"status"`
	Labels []string `json:"data"`
}
type wellKnown struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

type config struct {
	Issuer       string
	ClientID     string `envconfig:"client_id"`
	ClientSecret string `envconfig:"client_secret"`
	Loki         string
	Server       string
}

type fetcher struct {
	ctx           context.Context
	client        *http.Client
	tokenEndpoint string
	cfg           config
}

func (f *fetcher) authenticate() {
	conf := &oauth2.Config{
		ClientID:     f.cfg.ClientID,
		ClientSecret: f.cfg.ClientSecret,
		Scopes:       []string{"openid", "profile", "email"},
		TokenURL:     f.tokenEndpoint,
	}
	f.ctx = context.WithValue(f.ctx, o.HTTPClient, f.client) // surprisingly, this jank works
	f.client = conf.Client(f.ctx)
}

func NewFetcher(cfg config) *fetcher {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 3,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
				ServerName:         cfg.Server,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
				MinVersion: tls.VersionTLS13,
			},
		},
	}

	f := &fetcher{
		ctx:    context.Background(),
		client: client,
		cfg:    cfg,
	}
	_ = f.getTokenEndpoint()
	f.authenticate()
	return f
}

func (f *fetcher) getJson(url string, target interface{}) error {
	res, err := f.client.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func (f *fetcher) getTokenEndpoint() string {
	if f.tokenEndpoint != "" {
		return f.tokenEndpoint
	}
	res, err := f.client.Get(f.cfg.Issuer + "/.well-known/openid-configuration")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var wellKnown wellKnown
	err = json.Unmarshal(body, &wellKnown)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Issuer: "+wellKnown.Issuer, "TokenEndpoint: "+wellKnown.TokenEndpoint)
	f.tokenEndpoint = wellKnown.TokenEndpoint

	return f.tokenEndpoint
}

func main() {

	var cfg config
	envconfig.Process("", &cfg)
	f := NewFetcher(cfg)

	// now ready to use the new client
	log.Println("Getting labels from Loki at " + cfg.Loki)
	var labels lokiLabels
	err := f.getJson(cfg.Loki+"/loki/api/v1/labels", &labels)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Labels: " + strings.Join(labels.Labels, ", "))
}
