package gcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/cache"
	"github.com/seal-io/kubecia/pkg/token"
)

type TokenOptions struct {
	ClientID     string
	ClientSecret string
	Region       string
	Cluster      string
}

func (o *TokenOptions) Validate() error {
	var requiredTenant bool

	if strings.HasPrefix(o.ClientID, "$") {
		o.ClientID = os.ExpandEnv(o.ClientID)
		requiredTenant = true
	}

	if o.ClientID == "" {
		if requiredTenant {
			return errors.New("hosted client ID is required")
		}

		return errors.New("client ID is required")
	}

	if strings.HasPrefix(o.ClientSecret, "$") {
		o.ClientSecret = os.ExpandEnv(o.ClientSecret)
		requiredTenant = true
	}

	if o.ClientSecret == "" {
		if requiredTenant {
			return errors.New("hosted client secret is required")
		}

		return errors.New("client secret is required")
	}

	if o.Region == "" {
		return errors.New("region is required")
	}

	if o.Cluster == "" {
		return errors.New("cluster is required")
	}

	return nil
}

func (o *TokenOptions) Key() string {
	ss := []string{
		Namespace,
		o.ClientID,
		o.Region,
		o.Cluster,
	}

	return strings.Join(ss, "_")
}

func GetToken(ctx context.Context, opts TokenOptions, cacher cache.Cache) (*token.Token, error) {
	logger := klog.LoggerWithName(klog.Background(), Namespace)

	err := opts.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Retrieve the token from cache.
	ck := opts.Key()
	if cacher != nil {
		bs, err := cacher.Get(ctx, ck)
		if err != nil && !errors.Is(err, cache.ErrEntryNotFound) {
			logger.Error(err, "error retrieving token from cache")
		}

		if len(bs) != 0 {
			var tk token.Token
			if err = tk.UnmarshalBinary(bs); err == nil {
				if !tk.Expired() {
					return &tk, nil
				}
			}

			if err != nil {
				logger.Error(err, "error unmarshalling cached token")
			}
		}
	}

	// Request the token from remote.
	tk, err := getToken(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting credential token: %w", err)
	}

	// Save the token into cache.
	if cacher != nil {
		bs, err := tk.MarshalBinary()
		if err != nil {
			logger.Error(err, "error marshaling requested token")
		}

		if len(bs) != 0 {
			err = cacher.Set(ctx, ck, bs)
			if err != nil {
				logger.Error(err, "error saving token to cache")
			}
		}
	}

	return tk, nil
}

// getToken returns the token, inspired by
// https://github.com/kubernetes/client-go/blob/v0.22.17/plugin/pkg/client/auth/gcp/gcp.go.
func getToken(ctx context.Context, opts TokenOptions) (*token.Token, error) {
	apiCfg := &oauth2.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:   google.Endpoint.AuthURL,
			TokenURL:  google.Endpoint.TokenURL,
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}

	api := apiCfg.TokenSource(ctx, &oauth2.Token{})

	ak, err := api.Token()
	if err != nil {
		return nil, fmt.Errorf("error getting token: %w", err)
	}

	tk := &token.Token{
		Expiration: ak.Expiry,
		Value:      ak.AccessToken,
	}

	return tk, nil
}
