package main

import (
	"fmt"
	"os"

	"github.com/0xPolygonHermez/zkevm-sequencer"
	"github.com/0xPolygonHermez/zkevm-sequencer/config"
	"github.com/0xPolygonHermez/zkevm-sequencer/log"
	"github.com/urfave/cli/v2"
)

const appName = "zkevm-sequencer"

const (
	// SEQUENCER is the sequencer component identifier
	SEQUENCER = "sequencer"
	// SEQUENCE_SENDER is the sequence sender component identifier
	SEQUENCE_SENDER = "sequence-sender"
)

const (
	// SEQUENCER_CONFIGFILE name to identify the sequencer config-file
	SEQUENCER_CONFIGFILE = "sequencer"
	// NETWORK_CONFIGFILE name to identify the netowk_custom (genesis) config-file
	NETWORK_CONFIGFILE = "custom_network"
)

var (
	configFileFlag = cli.StringFlag{
		Name:     config.FlagCfg,
		Aliases:  []string{"c"},
		Usage:    "Configuration `FILE`",
		Required: true,
	}
	networkFlag = cli.StringFlag{
		Name:     config.FlagNetwork,
		Aliases:  []string{"net"},
		Usage:    "Load default network configuration. Supported values: [`mainnet`, `testnet`, `custom`]",
		Required: true,
	}
	customNetworkFlag = cli.StringFlag{
		Name:     config.FlagCustomNetwork,
		Aliases:  []string{"net-file"},
		Usage:    "Load the network configuration file if --network=custom",
		Required: false,
	}
	yesFlag = cli.BoolFlag{
		Name:     config.FlagYes,
		Aliases:  []string{"y"},
		Usage:    "Automatically accepts any confirmation to execute the command",
		Required: false,
	}
	componentsFlag = cli.StringSliceFlag{
		Name:     config.FlagComponents,
		Aliases:  []string{"co"},
		Usage:    "List of components to run",
		Required: false,
		Value:    cli.NewStringSlice(SEQUENCER, SEQUENCE_SENDER),
	}
	outputFileFlag = cli.StringFlag{
		Name:     config.FlagOutputFile,
		Usage:    "Indicate the output file",
		Required: true,
	}
	documentationFileTypeFlag = cli.StringFlag{
		Name:     config.FlagDocumentationFileType,
		Usage:    fmt.Sprintf("Indicate the type of file to generate json-schema: %v,%v ", SEQUENCER_CONFIGFILE, NETWORK_CONFIGFILE),
		Required: true,
	}
)

func main() {
	app := cli.NewApp()
	app.Name = appName
	app.Version = zkevm.Version
	flags := []cli.Flag{
		&configFileFlag,
		&yesFlag,
		&componentsFlag,
	}
	app.Commands = []*cli.Command{
		{
			Name:    "version",
			Aliases: []string{},
			Usage:   "Application version and build",
			Action:  versionCmd,
		},
		{
			Name:    "run",
			Aliases: []string{},
			Usage:   "Run the zkevm-sequencer",
			Action:  start,
			Flags:   append(flags, &networkFlag, &customNetworkFlag),
		},
		{
			Name:   "generate-json-schema",
			Usage:  "Generate the json-schema for the configuration file, and store it on docs/schema.json",
			Action: genJSONSchema,
			Flags:  []cli.Flag{&outputFileFlag, &documentationFileTypeFlag},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
