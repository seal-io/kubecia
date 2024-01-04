package azure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"k8s.io/klog/v2"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/seal-io/kubecia/pkg/cache"
	"github.com/seal-io/kubecia/pkg/token"
)

func init() {
	logger := klog.LoggerWithName(klog.Background(), Namespace)

	log.SetListener(func(event log.Event, msg string) {
		logger.V(5).Info(msg, "event", event)
	})
}

type TokenOptions struct {
	ClientID     string
	ClientSecret string
	Tenant       string
	Resource     string
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

	if o.Tenant == "" {
		return errors.New("tenant is required")
	}

	if o.Resource == "" {
		return errors.New("resource is required")
	}

	if match, err := regexp.MatchString("^[0-9a-zA-Z-.:/]+$", o.Resource); err != nil || !match {
		return errors.New("resource ID must be alphanumeric and contain only '.', ';', '-', and '/' characters")
	}

	return nil
}

func (o *TokenOptions) Key() string {
	ss := []string{
		Namespace,
		o.ClientID,
		o.Tenant,
		o.Resource,
	}

	return strings.Join(ss, "_")
}

// GetToken retrieves a token from cache or remote.
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
// https://github.com/Azure/kubelogin/blob/2b43d04d1a57229d67970bf0741c4433faf52f98/pkg/internal/token/azurecli.go#L43.
func getToken(ctx context.Context, opts TokenOptions) (*token.Token, error) {
	api, err := azidentity.NewClientSecretCredential(
		opts.Tenant, opts.ClientID, opts.ClientSecret,
		&azidentity.ClientSecretCredentialOptions{
			AdditionallyAllowedTenants: []string{"*"},
		})
	if err != nil {
		return nil, fmt.Errorf("error creating azure client: %w", err)
	}

	ak, err := api.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{opts.Resource}})
	if err != nil {
		return nil, fmt.Errorf("error getting token: %w", err)
	}

	if ak.Token == "" {
		return nil, errors.New("no token found")
	}

	tk := &token.Token{
		Expiration: ak.ExpiresOn,
		Value:      ak.Token,
	}

	return tk, nil
}
