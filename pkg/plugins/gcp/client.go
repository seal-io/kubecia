package gcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/apis"
	"github.com/seal-io/kubecia/pkg/bytespool"
	"github.com/seal-io/kubecia/pkg/cache"
	"github.com/seal-io/kubecia/pkg/consts"
	"github.com/seal-io/kubecia/pkg/token"
	"github.com/seal-io/kubecia/pkg/version"
)

type Client struct {
	Socket       string
	ClientID     string
	ClientSecret string
	Region       string
	Cluster      string
}

func (cli *Client) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&cli.Socket, "socket", consts.SocketPath(), "Socket path")
	flags.StringVar(&cli.ClientID, "client-id", "", "GCP client ID *")
	flags.StringVar(&cli.ClientSecret, "client-secret", "", "GCP client secret *")
	flags.StringVar(&cli.Region, "region", "", "GCP region *")
	flags.StringVar(&cli.Cluster, "cluster", "", "GCP cluster ID or name *")
}

func (cli *Client) GetToken(ctx context.Context) (*token.Token, error) {
	logger := klog.LoggerWithName(klog.Background(), Namespace)

	if si, err := os.Stat(cli.Socket); err == nil && si.Mode()&os.ModeSocket != 0 {
		logger.V(6).Info("getting from central service")

		tk, err := cli.GetTokenByHTTP(ctx, apis.Client(cli.Socket))
		if err == nil {
			logger.V(6).Info("got from central service")

			return tk, nil
		}

		var rce remoteCallError
		if !errors.As(err, &rce) {
			return nil, err
		}

		logger.Error(err, "error getting from central service, try getting locally")
	} else {
		logger.V(6).Info("getting locally")
	}

	tk, err := cli.getToken(ctx)
	if err == nil {
		logger.V(6).Info("got locally")

		return tk, nil
	}

	return nil, fmt.Errorf("error getting token locally: %w", err)
}

func (cli *Client) GetTokenByHTTP(ctx context.Context, httpc *http.Client) (*token.Token, error) {
	url := apis.Route(Namespace, cli.Region, cli.Cluster)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, wrapRemoteCallError(fmt.Errorf("error creating remote request: %w", err))
	}

	req.SetBasicAuth(cli.ClientID, cli.ClientSecret)

	req.Header.Set("User-Agent", version.Get())
	req.Header.Set("X-KubeCIA-DeCapsuled", "true")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, wrapRemoteCallError(fmt.Errorf("error making remote request: %w", err))
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response from remote: %s", resp.Status)
	}

	buf := bytespool.GetBuffer()
	defer bytespool.Put(buf)

	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error copying response body: %w", err)
	}

	var tk token.Token
	if err = tk.UnmarshalJSON(buf.Bytes()); err != nil {
		return nil, fmt.Errorf("error unmarshalling requested token: %w", err)
	}

	return &tk, nil
}

func (cli *Client) getToken(ctx context.Context) (*token.Token, error) {
	c, err := cache.NewFile(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating cache: %w", err)
	}

	defer func() { _ = c.Close() }()

	o := TokenOptions{
		ClientID:     cli.ClientID,
		ClientSecret: cli.ClientSecret,
		Region:       cli.Region,
		Cluster:      cli.Cluster,
	}

	return GetToken(ctx, o, c)
}

func wrapRemoteCallError(err error) error {
	return remoteCallError{err: err}
}

type remoteCallError struct {
	err error
}

func (e remoteCallError) Error() string {
	return e.err.Error()
}
