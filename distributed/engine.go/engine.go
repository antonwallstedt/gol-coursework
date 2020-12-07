package engine

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var (
	topics  = make(map[string]chan stubs.Work)
	topicmx sync.RWMutex
)

// Create topic as buffered channel
func createTopic(topic string, buflen int) {
	topicmx.Lock()
	defer topicmx.Unlock()
	if _, ok := topics[topic]; !ok {
		topics[topic] = make(chan stubs.Work, buflen)
		fmt.Println("Created channel # ", topic)
	}
}

// Work is published to a topic
func publish(topic string, work stubs.Work) (err error) {
	topicmx.RLock()
	defer topicmx.RUnlock()
	if ch, ok := topics[topic]; ok {
		ch <- work
	} else {
		return errors.New("topic does not exist")
	}
	return
}

func subscriberLoop(topic chan stubs.Work, client *rpc.Client, callback string) {
	for {
		job := <-topic
		response := new(stubs.JobReport)
		err := client.Call(callback, job, response)
		if err != nil {
			fmt.Println("Error")
			fmt.Println(err)
			fmt.Println("Closing subscriber thread.")
			topic <- job
			break
		}
	}
}

func subscribe(topic string, workerAddress string, callback string) (err error) {
	fmt.Println("Subscription request")
	topicmx.RLock()
	ch := topics[topic]
	topicmx.RUnlock()
	client, err := rpc.Dial("tcp", workerAddress)
	if err == nil {
		go subscriberLoop(ch, client, callback)
	} else {
		fmt.Println("Error subscribing ", workerAddress)
		fmt.Println(err)
		return err
	}
	return
}

type Engine struct{}

func (e *Engine) CreateChannel(req stubs.ChannelRequest, res *stubs.StatusReport) (err error) {
	createTopic(req.Topic, req.Buffer)
	return
}

func (e *Engine) Subscribe(req stubs.Subscription, res *stubs.StatusReport) (err error) {
	err = subscribe(req.Topic, req.WorkerAddress, req.Callback)
	if err != nil {
		res.Message = "Error during subscription"
	}
	return err
}

func (e *Engine) Publish(req stubs.PublishRequest, res *stubs.StatusReport) (err error) {
	err = publish(req.Topic, req.Work)
	return err
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
