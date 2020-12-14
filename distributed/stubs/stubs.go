package stubs

var GameOfLifeHandler = "Engine.GameOfLife"
var AliveCellsHandler = "Engine.AliveCells"
var ResultsHandler = "Engine.GetResults"
var PGMHandler = "Engine.GetPGM"
var PauseHandler = "Engine.Pause"
var StopHandler = "Engine.Stop"
var StatusHandler = "Engine.Status"
var ReconnectHandler = "Engine.Reconnect"

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

/* Request structs */

type RequestStart struct {
	World [][]byte
	Turns int
}

type RequestResult struct{}

type RequestAliveCells struct{}

type RequestPGM struct{}

type RequestPause struct{}

type RequestStop struct{}

type RequestStatus struct{}

type RequestReconnect struct{}
