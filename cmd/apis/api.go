package apis

import "github.com/spf13/cobra"

func AddCommands(c *cobra.Command) {
	var (
		g = &cobra.Group{
			ID:    "api",
			Title: `API commands`,
		}
		cs = []*cobra.Command{
			NewServe(),
		}
	)

	c.AddGroup(g)

	for i := range cs {
		cs[i].GroupID = g.ID
		c.AddCommand(cs[i])
	}
}
