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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/guilhem/tentacool/web"
)

const (
	appName       = "tentacool"
	addressBucket = "address"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: web.Web,
}

func init() {
	RootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("setip", "", "CLI to set an IP without launching the Tentacool server ('ID:CIDR')")
	viper.BindPFlag("setip", serveCmd.Flags().Lookup("setip"))

	serveCmd.Flags().String("bind", "/var/run/"+appName, "Adress to bind. Format Path or IP:PORT")
	viper.BindPFlag("bind", serveCmd.Flags().Lookup("bind"))

	serveCmd.Flags().String("owner", "tentacool", "Ownership for socket")
	viper.BindPFlag("owner", serveCmd.Flags().Lookup("owner"))

	serveCmd.Flags().Int("group", -1, "Group for socket")
	viper.BindPFlag("group", serveCmd.Flags().Lookup("group"))

	serveCmd.Flags().String("db", "/var/lib/"+appName+"/db", "Path for DB")
	viper.BindPFlag("db", serveCmd.Flags().Lookup("db"))

	serveCmd.Flags().Bool("stdout", false, "Log in stdout for debug purposes")
	viper.BindPFlag("stdout", serveCmd.Flags().Lookup("stdout"))
}
