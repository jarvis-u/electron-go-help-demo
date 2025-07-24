package main

import (
	"flag"
	"fmt"
	"os"

	"kt-connect/privileged-helper-tool/helper"
	"kt-connect/privileged-helper-tool/helper/logger"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version information")
	installFlag := flag.Bool("install", false, "Install helper service")
	//serveFlag := flag.Bool("serve", false, "Run helper service")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s", version)
		return
	}

	h := helper.NewHelperManager(version)
	if *installFlag {
		if err := h.Install(); err != nil {
			logger.Error("install error: %w", err)
			os.Exit(1)
		}
		return
	}

	// start helper
	h.Serve()
}
