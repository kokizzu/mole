package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/davrodpin/mole/rpc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	showInstancesCmd = &cobra.Command{
		Use:   "instances [name]",
		Short: "Shows information about a running application instances",
		Long:  "Shows information about a running application instances",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				id = args[0]
			}

			return nil
		},
		Run: func(cmd *cobra.Command, arg []string) {
			ctx := context.Background()

			if id == "" {
				info, err := rpc.ShowAll(ctx)
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"id": id,
					}).Error("could not retrieve information about application instance(s)")
					os.Exit(1)
				}

				json, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"id": id,
					}).Error("could not retrieve information about application instance(s)")
					os.Exit(1)
				}

				fmt.Printf("%s", string(json))
			} else {
				client := rpc.Client{Id: id}
				info, err := client.Show(ctx)
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"id": id,
					}).Error("could not retrieve information about application instance(s)")
					os.Exit(1)
				}

				json, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"id": id,
					}).Error("could not retrieve information about application instance(s)")
					os.Exit(1)
				}

				fmt.Printf("%s\n", string(json))
			}

		},
	}
)

func init() {
	showCmd.AddCommand(showInstancesCmd)
}
