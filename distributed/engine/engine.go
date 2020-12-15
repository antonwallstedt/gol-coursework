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

// IP addresses for AWS instances for each of the workers to be run on
// TODO: move everything to be run on AWS, change so the same port is being used and
// change all of the IP addresses to match each of the AWS instances.
var workerIPs = map[int]string{
	0: "127.0.0.1:8050",
	1: "127.0.0.1:8051",
	2: "127.0.0.1:8052",
	3: "127.0.0.1:8053",
	4: "127.0.0.1:8054",
	5: "127.0.0.1:8055",
	6: "127.0.0.1:8056",
	7: "127.0.0.1:8057",
	8: "127.0.0.1:8058",
	9: "127.0.0.1:8059",
}

// Work : used to send work to the engine and to receive work from the engine
type Work struct {
	World [][]byte
	Turn  int
}

// AliveCells : used to receive appropriate values and store them neatly in a struct from RPC calls made by the controller
type AliveCells struct {
	NumAliveCells  int
	CompletedTurns int
}

// TopBottomRows : holds top and bottom rows that are sent back by the workers after they've computed one step
type TopBottomRows struct {
	TopRow    []byte
	BottomRow []byte
}

// WorkerWorld : struct to allow for neat creation of a slice of worlds of type [][]byte
type WorkerWorld struct {
	world [][]byte
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

// makeWorkerHeights : calculate the worker heights for each of the worker and store in an array
func makeWorkerHeights(numWorkers, worldHeight int) []int {
	workerHeights := make([]int, numWorkers)
	defaultHeight := worldHeight / numWorkers // floor division
	for i := range workerHeights {
		workerHeights[i] = defaultHeight
	}

	// In case of uneven division, add remaining height to the last worker
	remainder := worldHeight % numWorkers
	if remainder != 0 {
		workerHeights[len(workerHeights)-1] += remainder
	}
	return workerHeights
}

// buildWorkerWorlds : takes in the number of workers and creates worlds for each of them to work on
func buildWorkerWorlds(workerHeights []int, world [][]byte) []WorkerWorld {
	worldHeight := len(world)
	worldWidth := len(world[0])
	numWorkers := len(workerHeights)
	workerWorlds := []WorkerWorld{}
	currHeight := 0
	for currThread, workerHeight := range workerHeights {
		currHeight += workerHeight
		paddedWorkerHeight := workerHeight + 2 // add extra top and bottom row to account for halo rows
		workerWorld := makeWorld(paddedWorkerHeight, worldWidth)
		if currThread == numWorkers-1 {
			workerWorld[0] = world[(currThread*workerHeights[0]+worldHeight-1)%worldHeight]
			for y := 1; y < paddedWorkerHeight-1; y++ {
				workerWorld[y] = world[currThread*workerHeights[0]+y-1]
			}
			workerWorld[paddedWorkerHeight-1] = world[0]
			workerWorlds = append(workerWorlds, WorkerWorld{world: workerWorld})
		} else {
			workerWorld[0] = world[(currThread*workerHeight+worldHeight-1)%worldHeight]
			for y := 1; y < paddedWorkerHeight-1; y++ {
				workerWorld[y] = world[currThread*workerHeight+y-1]
			}
			workerWorld[paddedWorkerHeight-1] = world[((currThread+1)*workerHeight+worldHeight)%worldHeight]
			workerWorlds = append(workerWorlds, WorkerWorld{world: workerWorld})
		}
	}
	return workerWorlds
}

func requestStartWorker(client rpc.Client, workerWorld [][]byte) TopBottomRows {
	request := stubs.RequestStartWorker{WorkerWorld: workerWorld}
	response := new(stubs.ResponseRows)
	client.Call(stubs.StartWorkerHandler, request, response)
	return TopBottomRows{TopRow: response.TopRow, BottomRow: response.BottomRow}
}

func requestNextState(client rpc.Client, halos TopBottomRows) TopBottomRows {
	request := stubs.RequestNextState{TopRow: halos.TopRow, BottomRow: halos.BottomRow}
	response := new(stubs.ResponseRows)
	client.Call(stubs.NextStateHandler, request, response)
	return TopBottomRows{TopRow: response.TopRow, BottomRow: response.BottomRow}
}

func requestWorkerResult(client rpc.Client) [][]byte {
	request := stubs.RequestWorkerResult{}
	response := new(stubs.ResponseWorkerResult)
	client.Call(stubs.WorkerResultHandler, request, response)
	return response.WorkerWorldPart
}

// Evolves the Game of Life for a given number of turns and a given world
func gameOfLife(numWorkers, turns int, world [][]byte, workChan chan Work, cmdChan chan int, aliveCellsChan chan AliveCells, responseMsgChan chan string, paused bool) {

	// Connect to each worker
	workerClients := make(map[int]*rpc.Client, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerClients[i], _ = rpc.Dial("tcp", workerIPs[i])
	}

	// Initiate each worker with their worker worlds
	topBottomRows := make([]TopBottomRows, numWorkers)
	if turns != 0 {
		if numWorkers != 1 {
			workerHeights := makeWorkerHeights(numWorkers, len(world))
			workerWorlds := buildWorkerWorlds(workerHeights, world)
			for i := range workerWorlds {
				rows := requestStartWorker(*workerClients[i], workerWorlds[i].world)
				topBottomRows[i].TopRow = rows.TopRow
				topBottomRows[i].BottomRow = rows.BottomRow
			}
		} else {
			// just start computation with one worker on the original world
		}
	}

	/*
		TODO: Fix so requestAliveCells, requestPgm make request to workers to send information back.

		TODO: Fix so turns evolve properly. Right now it passes tests for 1 turn, but fails for 100, so I'd assume it's because the halo
		communication is incorrect.

		TODO: Fix so that the workers aren't requested to sequentially, this defeats the purpose of having multiple workers.
		One idea is that instead of making a normal RPC request where we wait for a response which will block the program, start it
		as a goroutine, and have a select statement that will register whenever it's received from. When all workers have finished
		then proceed with next turn. What's happening right now is that we make a request to one worker, wait until we get a response back,
		then request another worker to process their part, wait for response and so on. What we want is: start all workers, wait until all of them
		have finished, then proceed.
	*/

	turn := 1 // 0th turn was computed when the workers started
	running = true
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

		for i := range topBottomRows {
			if numWorkers != 2 {
				newTopRow := topBottomRows[(i+numWorkers-1)%numWorkers].BottomRow
				newBottomRow := topBottomRows[(i+1)%numWorkers].TopRow
				topBottomRows[i] = requestNextState(*workerClients[i], TopBottomRows{TopRow: newTopRow, BottomRow: newBottomRow})
			} else {
				newTopRow := topBottomRows[(i+1)%numWorkers].BottomRow
				newBottomRow := topBottomRows[(i+1)%numWorkers].TopRow
				topBottomRows[i] = requestNextState(*workerClients[i], TopBottomRows{TopRow: newTopRow, BottomRow: newBottomRow})
			}
		}

		if turn%10 == 0 && turn != 0 {
			fmt.Println("Turn ", turn, " computed")
		}
		turn++
	}

	workerResults := map[int][][]byte{}
	for i := range workerClients {
		workerResults[i] = requestWorkerResult(*workerClients[i])
	}

	newWorld := makeWorld(0, 0)
	for i := range workerResults {
		part := workerResults[i]
		newWorld = append(newWorld, part...)
	}

	if running == true { // only send back if the engine has been running and hasn't been stopped by the controller
		fmt.Println("Sending world back" + "\n")
		workChan <- Work{World: newWorld, Turn: turn}
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
	go gameOfLife(req.NumWorkers, req.Turns, req.World, e.workChan, e.cmdChan, e.aliveCellsChan, e.responseMsgChan, false)
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

func main() {
	workChan := make(chan Work)
	aliveCellsChan := make(chan AliveCells)
	cmdChan := make(chan int)
	responseMsgChan := make(chan string)
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Engine{
		workChan:        workChan,
		aliveCellsChan:  aliveCellsChan,
		cmdChan:         cmdChan,
		responseMsgChan: responseMsgChan,
	})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
