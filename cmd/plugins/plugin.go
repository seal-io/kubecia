package plugins

import (
	"github.com/spf13/cobra"
)

func AddCommands(c *cobra.Command) {
	var (
		g = &cobra.Group{
			ID:    "plugin",
			Title: `Plugin commands`,
		}
		cs = []*cobra.Command{
			NewAWS(),
			NewAzure(),
			NewGCP(),
		}
	)

	c.AddGroup(g)

	for i := range cs {
		cs[i].GroupID = g.ID
		c.AddCommand(cs[i])
	}
}
