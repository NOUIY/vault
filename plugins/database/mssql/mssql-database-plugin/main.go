package main

import (
	"log"
	"os"

	"github.com/hashicorp/vault/plugins/database/mssql"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

func main() {
	err := Run()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Run instantiates a MSSQL object, and runs the RPC server for the plugin
func Run() error {
	var f func() (dbplugin.Database, error)
	f = mssql.New()

	dbplugin.ServeMultiplex(f)

	return nil
}
