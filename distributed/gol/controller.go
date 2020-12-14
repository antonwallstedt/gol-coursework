package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const (
	ALIVE = 255
	DEAD  = 0
)

type Work struct {
	World [][]byte
	Turn  int
}

type AliveCells struct {
	NumAliveCells  int
	CompletedTurns int
}

type controllerChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func calculateAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	height := len(world)
	width := len(world[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == ALIVE {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

/* Functions to send RPC requests to the engine */

func startGameOfLife(client rpc.Client, world [][]byte, turns int) string {
	request := stubs.RequestStart{World: world, Turns: turns}
	response := new(stubs.ResponseStart)
	client.Call(stubs.GameOfLifeHandler, request, response)
	return response.Message
}

func requestResults(client rpc.Client) Work {
	request := stubs.RequestResult{}
	response := new(stubs.ResponseResult)
	client.Call(stubs.ResultsHandler, request, response)
	return Work{World: response.World, Turn: response.Turn}
}

func requestAliveCells(client rpc.Client) AliveCells {
	request := stubs.RequestAliveCells{}
	response := new(stubs.ResponseAliveCells)
	client.Call(stubs.AliveCellsHandler, request, response)
	return AliveCells{NumAliveCells: response.NumAliveCells, CompletedTurns: response.CompletedTurns}
}

func requestPGM(client rpc.Client) Work {
	request := stubs.RequestPGM{}
	response := new(stubs.ResponsePGM)
	client.Call(stubs.PGMHandler, request, response)
	return Work{World: response.World, Turn: response.Turn}
}

func requestPause(client rpc.Client) string {
	request := stubs.RequestPause{}
	response := new(stubs.ResponsePause)
	client.Call(stubs.PauseHandler, request, response)
	return response.Message
}
func requestContinue(client rpc.Client) string {
	request := stubs.RequestContinue{}
	response := new(stubs.ResponceContinue)
	client.Call(stubs.ContinueHandler, request, response)
	return response.Message
}
func requestStop(client rpc.Client) string {
	request := stubs.RequestStop{}
	response := new(stubs.ResponseStop)
	client.Call(stubs.StopHandler, request, response)
	return response.Message
}

func requestStatus(client rpc.Client) bool {
	request := stubs.RequestStatus{}
	response := new(stubs.ResponseStatus)
	client.Call(stubs.StatusHandler, request, response)
	return response.Running
}

func requestReconnect(client rpc.Client) string {
	request := stubs.RequestReconnect{}
	response := new(stubs.ResponseReconnect)
	client.Call(stubs.ReconnectHandler, request, response)
	return response.Message
}

func controller(p Params, c controllerChannels) {

	// Dial server
	var serverIP string
	if flag.Lookup("server") != nil {
		serverIP = flag.Lookup("server").Value.String()
	} else {
		serverIP = "127.0.0.1:8030"
	}
	client, _ := rpc.Dial("tcp", serverIP)
	defer client.Close()

	engineRunning := requestStatus(*client)
	if p.Reconnect == false {
		// Check if engine is already running, if it is stop it and load in the initial board state and start processing from the beginning
		if engineRunning == true {
			fmt.Println(requestStop(*client))
		}

		// Request IO to read image file
		c.ioCommand <- ioInput
		c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)

		// Load world in
		world := makeWorld(p.ImageHeight, p.ImageWidth)
		for y := range world {
			for x := range world {
				world[y][x] = <-c.ioInput
			}
		}

		// Make call to server to start Game of Life
		startGameOfLife(*client, world, p.Turns)
	} else {
		if engineRunning == false {
			fmt.Println("Engine is not currently processing Game of Life, cannot reconnect. Exiting...")
			os.Exit(0)
		} else {
			fmt.Println("I'm here")
			fmt.Println(requestReconnect(*client))
		}
	}

	// Anonymous goroutine to allow for ticker to be run in the background along with registering keypresses
	ticker := time.NewTicker(2 * time.Second)
	// i := 0
	go func(paused bool) {
		for {
			select {
			case <-ticker.C:
				aliveCells := requestAliveCells(*client)

				c.events <- AliveCellsCount{CompletedTurns: aliveCells.CompletedTurns, CellsCount: aliveCells.NumAliveCells}

			case keyPress := <-c.keyPresses:
				switch keyPress {
				case 's':
					boardState := requestPGM(*client)
					printBoard(c, p, boardState.World, boardState.Turn)
				case 'q':
					close(c.events)
				case 'p':
					response := requestPause(*client)
					fmt.Println(response)
					for {
						tempPress := <-c.keyPresses
						if tempPress == 'p' {
							response := requestContinue(*client)
							fmt.Println(response)
							break
						}

					}
				}
			default:
			}
		}
	}(false)

	// Request results
	resultWork := requestResults(*client)
	ticker.Stop()
	printBoard(c, p, resultWork.World, p.Turns)
	// Calculate alive cells
	c.events <- FinalTurnComplete{CompletedTurns: resultWork.Turn, Alive: calculateAliveCells(resultWork.World)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{resultWork.Turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.

	close(c.events)

}

func printBoard(c controllerChannels, p Params, world [][]byte, turn int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageHeight, p.ImageWidth, turn)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}
