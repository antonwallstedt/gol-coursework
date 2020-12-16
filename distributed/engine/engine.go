package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// Work : used to send work to the engine and to receive work from the engine
type Work struct {
	World [][]byte
	Turn  int
}

type AliveCells struct {
	NumAliveCells  int
	CompletedTurns int
}

const (
	// ALIVE : pixel value for alive cells
	ALIVE = 255

	// DEAD : pixel value for dead cells
	DEAD = 0
)

const (
	requestAliveCells = iota
	requestPgm
	requestPause
	requestStop
)

var running = false

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func mod(x, m int) int {
	return (x + m) % m
}

// Calculates the number of alive neighbours around a given cell
func calculateNeighbours(x, y int, world [][]byte) int {
	neighbours := 0
	height := len(world)
	width := len(world[0])
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, height)][mod(x+j, width)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

// numAliveCells : gets the number of alive cells from a given world
func numAliveCells(world [][]byte) int {
	aliveCells := 0
	for y := range world {
		for x := range world {
			if world[y][x] == ALIVE {
				aliveCells++
			}
		}
	}
	return aliveCells
}

// Computes one evolution of the Game of Life
func calculateNextState(world [][]byte) [][]byte {
	height := len(world)
	width := len(world[0])
	newWorld := makeWorld(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			neighbours := calculateNeighbours(x, y, world)
			if world[y][x] == ALIVE {
				if neighbours == 2 || neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			} else {
				if neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			}
		}
	}
	return newWorld
}

//To build workerWorld for the worker
func buildWorkerWorld(world [][]byte, workerHeight, imageHeight, imageWidth, currentThreads, Threads int) [][]byte {

	workerWorld := make([][]byte, workerHeight+2)
	for j := range workerWorld {
		workerWorld[j] = make([]byte, imageWidth)
	}

	if currentThreads == Threads-1 {
		workerHeight1 := workerHeight - imageHeight%Threads
		for x := 0; x < imageWidth; x++ {
			workerWorld[0][x] = world[(currentThreads*workerHeight1+imageHeight-1)%imageHeight][x]
		}
		for y := 1; y <= workerHeight; y++ {
			for x := 0; x < imageWidth; x++ {
				workerWorld[y][x] = world[currentThreads*workerHeight1+y-1][x]
			}
		}
		for x := 0; x < imageWidth; x++ {
			workerWorld[workerHeight+1][x] = world[0][x]
		}

	} else {
		for x := 0; x < imageWidth; x++ {
			workerWorld[0][x] = world[(currentThreads*workerHeight+imageHeight-1)%imageHeight][x]
		}
		for y := 1; y <= workerHeight; y++ {
			for x := 0; x < imageWidth; x++ {
				workerWorld[y][x] = world[currentThreads*workerHeight+y-1][x]
			}
		}
		for x := 0; x < imageWidth; x++ {
			workerWorld[workerHeight+1][x] = world[((currentThreads+1)*workerHeight+imageHeight)%imageHeight][x]
		}
	}

	return workerWorld
}

//Function use for worker handler
func requestFinishedWorkerWorld(client rpc.Client, world [][]byte, imageHeight, imageWidth, paramImageHeight int) [][]byte {
	request := stubs.RequestWorkerWorld{World: world, ImageHeight: imageHeight, ImageWidth: imageWidth, ParamsImageHeight: paramImageHeight}
	response := new(stubs.ResponseWorkerWorld)
	client.Call(stubs.DivideWorldHandler, request, response)
	newWorld := response.World
	return newWorld
}

// Evolves the Game of Life for a given number of turns and a given world
func gameOfLife(turns int, world [][]byte, Threads int, workChan chan Work, cmdChan chan int, aliveCellsChan chan AliveCells, responseMsgChan chan string, paused bool) {
	//array of address for worker for dailing to the worker
	// workerAdress := [...]string{"54.208.52.95", "52.90.22.150", "100.25.29.149", "34.229.67.59", "54.90.33.212", "3.90.85.38", " 54.158.221.223", "54.88.225.247", "3.89.160.119", "54.89.228.110"}

	turn := 0
	running = true
	ImageHeight := len(world)
	ImageWidth := len(world[0])
	workerHeight := ImageHeight / Threads
	newWorld := makeWorld(ImageHeight, ImageWidth)
	for (turn < turns) && running {
		select {
		case cmd := <-cmdChan:
			switch cmd {
			case requestAliveCells:
				aliveCellsChan <- AliveCells{NumAliveCells: numAliveCells(world), CompletedTurns: turn}
			case requestPgm:
				workChan <- Work{World: world, Turn: turn}
			case requestPause:
				if paused == false {
					responseMsg := fmt.Sprintf("Pausing on turn %d", turn)
					responseMsgChan <- responseMsg
					paused = true
					for paused {
						select {
						case c := <-cmdChan:
							if c == requestPause {
								responseMsgChan <- "Continuing"
								paused = false
							}
						}
					}
				}
			case requestStop:
				fmt.Println("Stopping computation")
				running = false
			}
		default:
		}

		// world = calculateNextState(world)
		for i := 0; i < Threads; i++ {
			fmt.Println("the thread right now is %d and the whole thread is %d and the turn is %d", i, Threads, turn)
			var serverIP string
			if flag.Lookup("server") != nil {
				serverIP = flag.Lookup("server").Value.String()
			} else {
				serverIP = "127.0.0.1:8040"
			}
			client, _ := rpc.Dial("tcp", serverIP)
			defer client.Close()

			go func() {

				if i == Threads-1 {
					fmt.Println("Under world")

					workerHeight1 := (ImageHeight / Threads) + (ImageHeight % Threads)
					workerWorld := buildWorkerWorld(world, workerHeight1, ImageHeight, ImageWidth, i, Threads)
					newWorkerWorld := requestFinishedWorkerWorld(*client, workerWorld, workerHeight1, ImageWidth, ImageHeight)

					for y := 0; y < workerHeight1; y++ {
						for x := 0; x < ImageWidth; x++ {
							newWorld[i*workerHeight+y][x] = newWorkerWorld[y][x]
						}
					}
				} else {
					fmt.Println("above world")
					workerWorld := buildWorkerWorld(world, workerHeight, ImageHeight, ImageWidth, i, Threads)
					newWorkerWorld := requestFinishedWorkerWorld(*client, workerWorld, workerHeight, ImageWidth, ImageHeight)

					for y := 0; y < workerHeight; y++ {
						for x := 0; x < ImageWidth; x++ {
							newWorld[i*workerHeight+y][x] = newWorkerWorld[y][x]
						}
					}
				}
			}()

		}

		if turn%10 == 0 && turn != 0 {
			fmt.Println("Turn ", turn, " computed")
		}
		x := world
		x = newWorld
		newWorld = x
		turn++
	}

	if running == true { // only send back if the engine has been running and hasn't been stopped by the controller
		fmt.Println("Sending world back\n")
		workChan <- Work{World: world, Turn: turn}
		running = false
	}
}

// Gets the results back from the work channel
func getResults(workChan chan Work) Work {
	result := <-workChan
	return Work{World: result.World, Turn: result.Turn}
}

// Gets the number of alive cells and number of completed turns from the alive cells channel
func getAliveCells(aliveCellsChan chan AliveCells, cmdChan chan int) AliveCells {
	cmdChan <- requestAliveCells
	aliveCells := <-aliveCellsChan
	return AliveCells{NumAliveCells: aliveCells.NumAliveCells, CompletedTurns: aliveCells.CompletedTurns}
}

// Gets the board state
func getPGM(workChan chan Work, cmdChan chan int) Work {
	cmdChan <- requestPgm
	work := <-workChan
	return Work{work.World, work.Turn}
}

func pause(cmdChan chan int, responseMsgChan chan string) string {
	cmdChan <- requestPause
	response := <-responseMsgChan
	return response
}

func stop(cmdChan chan int) string {
	if running == true {
		cmdChan <- requestStop
		return "Stopping engine"
	}
	return "Engine is not running"
}

func reconnect() string {
	return "Controller reconnected to engine"
}

// Engine : used to run functions that respond to requests made by the controller.
// 			Can communicate the work that's being done using a channel
type Engine struct {
	workChan        chan Work
	aliveCellsChan  chan AliveCells
	cmdChan         chan int
	responseMsgChan chan string
}

// GameOfLife : runs the game of life after getting a request from the controller
func (e *Engine) GameOfLife(req stubs.RequestStart, res *stubs.ResponseStart) (err error) {
	if req.World == nil {
		err = errors.New("a world must be specified")
		res.Message = "invalid world"
		return
	}
	fmt.Println("Starting game of life")
	go gameOfLife(req.Turns, req.World, req.Threads, e.workChan, e.cmdChan, e.aliveCellsChan, e.responseMsgChan, false)
	res.Message = "received world"
	return
}

// GetResults : gets the result after all turns have been computed
func (e *Engine) GetResults(req stubs.RequestResult, res *stubs.ResponseResult) (err error) {
	result := getResults(e.workChan)
	res.World = result.World
	res.Turn = result.Turn
	return
}

// AliveCells : gets the number of alive cells when requested by the controller
func (e *Engine) AliveCells(req stubs.RequestAliveCells, res *stubs.ResponseAliveCells) (err error) {
	aliveCells := getAliveCells(e.aliveCellsChan, e.cmdChan)
	res.NumAliveCells = aliveCells.NumAliveCells
	res.CompletedTurns = aliveCells.CompletedTurns
	return
}

// GetPGM : gets the board state so it can be sent to the controller to be saved as a PGM image
func (e *Engine) GetPGM(req stubs.RequestPGM, res *stubs.ResponsePGM) (err error) {
	boardState := getPGM(e.workChan, e.cmdChan)
	res.World = boardState.World
	res.Turn = boardState.Turn
	return
}

// Pause : pauses the computation
func (e *Engine) Pause(req stubs.RequestPause, res *stubs.ResponsePause) (err error) {
	response := pause(e.cmdChan, e.responseMsgChan)
	res.Message = response
	return
}

// Stop : stops the computation
func (e *Engine) Stop(req stubs.RequestStop, res *stubs.ResponseStop) (err error) {
	res.Message = stop(e.cmdChan)
	return
}

// Status : checks if engine is already running
func (e *Engine) Status(req stubs.RequestStatus, res *stubs.ResponseStatus) (err error) {
	status := running
	res.Running = status
	return
}

// Reconnect : reconnects a controller to the engine while it's processing work
func (e *Engine) Reconnect(req stubs.RequestReconnect, res *stubs.ResponseReconnect) (err error) {
	res.Message = reconnect()
	return
}

var engine Engine

func main() {
	workChan := make(chan Work)
	aliveCellsChan := make(chan AliveCells)
	cmdChan := make(chan int)
	responseMsgChan := make(chan string)
	engine.workChan = workChan
	engine.aliveCellsChan = aliveCellsChan
	engine.cmdChan = cmdChan
	engine.responseMsgChan = responseMsgChan
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&engine)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
