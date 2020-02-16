package main

import (
	"../../pkg/tftp"
	"log"
)

func main() {

	var server tftp.Server
	if e := server.Init(); e != nil {
		log.Println(e)
	}
	defer server.DeInit()

	if e := server.AcceptLoop(); e != nil {
		log.Println(e)
	}

}
