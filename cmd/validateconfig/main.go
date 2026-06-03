package main

import (
	"fmt"
	"os"

	config "github.com/seanocca/consensus-proxy/cmd/config"
)

func main() {
	path := "config.toml"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if _, err := config.Load(path); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("TOML config valid")
}
