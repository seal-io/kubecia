package apis

import (
	"context"
	"net"
	"net/http"
	"time"
)

func Client(sock string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sock, 1*time.Second)
			},
		},
	}
}
