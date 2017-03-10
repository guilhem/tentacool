// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"log/syslog"
	"os"

	log "github.com/Sirupsen/logrus"

	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "tentacool",
	Short: "REST API to manage Linux networking via netlink",
	Long: `tentacool is a Go server controlled via RESTful API.
Goal is to modify underlying parameter easily from an application`,

	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		log.SetOutput(os.Stderr)
		level, err := log.ParseLevel(viper.GetString("log-level"))
		if err != nil {
			log.WithError(err).Fatal("Logrus: ParseLevel")
		}
		log.SetLevel(level)
		log.SetOutput(os.Stdout)
		if hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, ""); err == nil {
			log.AddHook(hook)
		} else {
			log.WithError(err).Info("Failing to add syslog hook")
		}
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.WithError(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().String("log-level", "info", "one of debug, info, warn, error, or fatal")
	if err := viper.BindPFlag("log-level", RootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		log.WithError(err).Fatal("log-level")
	}

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tentacool.yaml)")

	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".tentacool") // name of config file (without extension)
	viper.AddConfigPath("$HOME")      // adding home directory as first search path
	viper.AutomaticEnv()              // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
