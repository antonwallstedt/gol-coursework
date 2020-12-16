package stubs

/* Engine handlers */

var GameOfLifeHandler = "Engine.GameOfLife"
var AliveCellsHandler = "Engine.AliveCells"
var ResultsHandler = "Engine.GetResults"
var PGMHandler = "Engine.GetPGM"
var PauseHandler = "Engine.Pause"
var StopHandler = "Engine.Stop"
var StatusHandler = "Engine.Status"
var ReconnectHandler = "Engine.Reconnect"

/* Worker handlers */

var StartWorkerHandler = "Worker.StartWorker"
var NextStateHandler = "Worker.CalculateNextState"
var WorkerResultHandler = "Worker.GetResult"
var WorkerAliveCellsHandler = "Worker.AliveCells"
var WorkerPGMHandler = "Worker.GetPGM"

/* Response structs */

type ResponseStart struct {
	Message string
}

type ResponseAliveCells struct {
	CompletedTurns int
	NumAliveCells  int
}

type ResponseResult struct {
	World [][]byte
	Turn  int
}

type ResponsePGM struct {
	World [][]byte
	Turn  int
}

type ResponsePause struct {
	Message string
}

type ResponseStop struct {
	Message string
}

type ResponseReconnect struct {
	Message string
}

type ResponseStatus struct {
	Running bool
}

type ResponseRows struct {
	TopRow    []byte
	BottomRow []byte
}

type ResponseWorkerResult struct {
	WorkerWorldPart [][]byte
	WorkerID        int
}

type ResponseWorkerAliveCells struct {
	NumAliveCells int
}

/* Request structs */

type RequestStart struct {
	World      [][]byte
	Turns      int
	NumWorkers int
}

type RequestResult struct{}

type RequestAliveCells struct{}

type RequestPGM struct{}

type RequestPause struct{}

type RequestStop struct{}

type RequestStatus struct{}

type RequestReconnect struct{}

type RequestStartWorker struct {
	WorkerWorld [][]byte
	WorkerID    int
}

type RequestNextState struct {
	TopRow    []byte
	BottomRow []byte
}

type RequestWorkerResult struct {
	NumWorkers int
}
