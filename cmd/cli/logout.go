package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of the microservices platform",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("api_key", "")
		viper.Set("email", "")
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Warning: failed to write config: %v\n", err)
		}
		fmt.Println("Successfully logged out.")
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
