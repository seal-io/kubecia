package apis

import (
	"github.com/spf13/cobra"

	"github.com/seal-io/kubecia/pkg/apis/server"
	"github.com/seal-io/kubecia/pkg/plugins/aws"
	"github.com/seal-io/kubecia/pkg/plugins/azure"
	"github.com/seal-io/kubecia/pkg/plugins/gcp"
)

func NewServe() *cobra.Command {
	var srv server.Server

	c := &cobra.Command{
		Use:          "serve",
		Short:        "Serve KubeCIA APIs.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			ss := server.ServeFuncs{
				aws.Serve,
				azure.Serve,
				gcp.Serve,
			}

			for i := range ss {
				srv.Register(ss[i])
			}

			return srv.Serve(c.Context())
		},
	}

	srv.AddFlags(c.Flags())

	return c
}
