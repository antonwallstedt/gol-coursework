package stubs

import "uk.ac.bris.cs/gameoflife/gol"

var CreateChannel = "Worker.CreateChannel"
var Publish = "Worker.Publish"
var Subscribe = "Worker.Subscribe"

type Work struct {
	World      [][]byte
	Parameters gol.Params
}

type PublishRequest struct {
	Topic string
	Work  Work
}

type ChannelRequest struct {
	Topic  string
	Buffer int
}

type Subscription struct {
	Topic         string
	WorkerAddress string
	Callback      string
}

type JobReport struct {
	Result [][]byte
}

type StatusReport struct {
	Message string
}
