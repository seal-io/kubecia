package aws

import (
	"context"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/apis"
	"github.com/seal-io/kubecia/pkg/apis/server"
)

const (
	Namespace = "aws"
)

func Serve(ctx context.Context, mux *http.ServeMux, opts server.ServeOptions) error {
	klog.Infof("serving %[1]s: /%[1]s/{region}/{cluster}[/{assume-role-arn}]\n", Namespace)

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

	// Authorization: Bearer {accessKeyID:secretAccessKey}.
	{
		var found bool

		o.AccessKeyID, o.SecretAccessKey, found = r.BasicAuth()
		if !found {
			c := http.StatusUnauthorized
			http.Error(w, http.StatusText(c), c)

			return
		}
	}

	// Path: {region}/{cluster}[/{assume-role-arn}].
	{
		paths := strings.SplitN(r.URL.Path, "/", 3)
		if len(paths) < 2 {
			c := http.StatusBadRequest
			http.Error(w, http.StatusText(c), c)

			return
		}

		o.Region = paths[0]
		o.Cluster = paths[1]

		if len(paths) == 3 {
			o.AssumeRoleARN = paths[2]
		}
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
