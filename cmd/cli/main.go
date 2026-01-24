package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "micro",
	Short: "Microservices CLI",
	Long:  `A CLI tool to interact with the Microservices Payment Platform.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.micro.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".micro")

		// Create config file if it doesn't exist
		configPath := filepath.Join(home, ".micro.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			f, err := os.Create(configPath)
			if err != nil {
				fmt.Printf("Warning: failed to create config file: %v\n", err)
			} else {
				f.Close()
			}
		}
	}

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	Execute()
}
