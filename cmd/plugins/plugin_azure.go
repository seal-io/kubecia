package plugins

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seal-io/kubecia/pkg/plugins/azure"
)

func NewAzure() *cobra.Command {
	var cli azure.Client

	c := &cobra.Command{
		Use:          "azure",
		Short:        "Get Azure token.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			tk, err := cli.GetToken(c.Context())
			if err != nil {
				return err
			}

			bs, err := tk.ToKubeClientExecCredentialJSON()
			if err != nil {
				return fmt.Errorf("error converting token to kube client exec credential json: %w", err)
			}

			c.Print(string(bs))
			return nil
		},
	}

	cli.AddFlags(c.Flags())

	return c
}
