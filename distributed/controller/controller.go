package controller

import (
	//"bufio"
	"flag"
	//"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
	//"os"
	//"uk.ac.bris.cs/gameoflife/stubs"
)

func main() {
	engineAddr := flag.String("engine", "127.0.0.1:8030", "IP:port address of engine instance")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *engineAddr)
	status := new(stubs.StatusReport)
	client.Call(stubs.CreateChannel)
}
