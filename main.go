package main

import (
	"fmt"
	"os"
	volumeplugin "github.com/docker/go-plugins-helpers/volume"
)

const socketAddress = "/run/docker/plugins/dobs.sock"

func main() {

	token := os.Getenv("TOKEN")

	d := newDobsDriver("/mnt", token)
	err := d.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	h := volumeplugin.NewHandler(d)
	h.ServeUnix(socketAddress, 0)
}