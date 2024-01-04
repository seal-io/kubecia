package azure

import (
	"context"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/apis"
	"github.com/seal-io/kubecia/pkg/apis/server"
)

const (
	Namespace = "azure"
)

func Serve(ctx context.Context, mux *http.ServeMux, opts server.ServeOptions) error {
	klog.Infof("serving %[1]s: /%[1]s/{tenant}/{resource}\n", Namespace)

	rp := apis.RoutePrefix(Namespace)
	hd := http.StripPrefix(rp, &apiServer{
		ServeOptions: opts,
		Logger:       klog.LoggerWithName(klog.Background(), Namespace),
	})

	mux.Handle(rp, hd)

	return nil
}

type apiServer struct {
	server.ServeOptions

	Logger klog.Logger
}

func (s *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		c := http.StatusMethodNotAllowed
		http.Error(w, http.StatusText(c), c)

		return
	}

	var o TokenOptions

	// Authorization: Bearer {clientID:clientSecret}.
	{
		var found bool

		o.ClientID, o.ClientSecret, found = r.BasicAuth()
		if !found {
			c := http.StatusUnauthorized
			http.Error(w, http.StatusText(c), c)

			return
		}
	}

	// Path: {tenant}/{resource}.
	{
		paths := strings.SplitN(r.URL.Path, "/", 2)
		if len(paths) < 2 {
			c := http.StatusBadRequest
			http.Error(w, http.StatusText(c), c)

			return
		}

		o.Tenant = paths[0]
		o.Resource = paths[1]
	}

	tk, err := GetToken(r.Context(), o, s.Cache)
	if err != nil {
		s.Logger.Error(err, "error getting token")
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var bs []byte
	if r.Header.Get("X-KubeCIA-DeCapsuled") == "true" {
		bs, err = tk.MarshalJSON()
	} else {
		bs, err = tk.ToKubeClientExecCredentialJSON()
	}

	if err != nil {
		s.Logger.Error(err, "error marshaling token")
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(bs)
	if err != nil {
		s.Logger.Error(err, "error writing response")
		return
	}
}
