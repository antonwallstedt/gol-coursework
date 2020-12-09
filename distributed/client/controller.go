package client

import (
	"flag"
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	ioWorld    chan<- [][]byte
	ioTurns    chan<- int
}
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

func makeWorld(imageHeight, imageWidth int) [][]byte {
	newWorld := make([][]byte, imageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, imageWidth)
	}
	return newWorld

}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)
	ioWorld := make(chan [][]byte)
	ioTurns := make(chan int)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		ioFilename,
		ioOutput,
		ioInput,
		ioWorld,
		ioTurns,
	}
	go controller(p, distributorChannels)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
		world:    ioWorld,
		turns:    ioTurns,
	}
	go startIo(p, ioChannels)
}

func controller(p Params, d distributorChannels) {
	d.ioCommand <- ioInput
	d.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	world := makeWorld(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-d.ioInput
		}
	}
	d.ioWorld <- world
	d.ioTurns <- p.Turns

}
func inputWorld(client rpc.Client, world [][]byte, turns int) {
	request := stubs.Request{World: world, Turns: turns}
	responce := new(stubs.Response)
	client.Call(stubs.NextStateHandler, request, responce)
}
func main(io ioChannels) {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	world := <-io.world
	turns := <-io.turns
	inputWorld(*client, world, turns)

}
