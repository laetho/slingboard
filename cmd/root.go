package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "sling",
	Short: "Sling Board CLI",
	Long:  `Sling Board is a real-time messaging board built on NATS.`,
	Args:  cobra.ArbitraryArgs, // Allow any number of arguments
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			slingMessage.Run(cmd, args)
		} else {
			_ = cmd.Help()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("~/.config/slignboard")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Warning: No configuration file found, using defaults")
	}
}
