package gol

import (
	"flag"
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var newWorld [][]byte
var world [][]byte
var turn int

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFileName chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
	ioWorld    chan<- [][]byte
	ioTurns    chan<- int
}

func makeWorld(imageHeight, imageWidth int) [][]byte {
	newWorld := make([][]byte, imageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, imageWidth)
	}
	return newWorld

}

func controller(p Params, d distributorChannels, keyPresses <-chan rune) {
	d.ioCommand <- ioInput
	d.ioFileName <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	world := makeWorld(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-d.ioInput
		}
	}
	world = world
	turn = p.Turns

}

func inputWorld(client rpc.Client, world [][]byte, turns int) {
	request := stubs.Request{World: world, Turns: turns}
	responce := new(stubs.Response)
	client.Call(stubs.NextStateHandler, request, responce)
	newWorld = responce.NewWorld
}

func main() {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	inputWorld(*client, world, turn)

}
