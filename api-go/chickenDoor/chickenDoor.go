package chickendoor

import (
	"context"
	"log"
)

// ChickenDoor represents the chicken door module
type ChickenDoor struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a new ChickenDoor instance
func New() *ChickenDoor {
	return &ChickenDoor{
		done: make(chan struct{}),
	}
}

// Run starts the chicken door service
func (cd *ChickenDoor) Run() {
	cd.ctx, cd.cancel = context.WithCancel(context.Background())
	log.Println("🐔 ChickenDoor service starting...")

	go cd.runLoop()
}

// runLoop is the main service loop
func (cd *ChickenDoor) runLoop() {
	defer func() {
		close(cd.done)
		log.Println("🐔 ChickenDoor service stopped")
	}()

	// TODO: Add your service logic here
	<-cd.ctx.Done()
}

// Stop gracefully stops the chicken door service
func (cd *ChickenDoor) Stop() {
	if cd.cancel != nil {
		log.Println("🐔 Stopping ChickenDoor service...")
		cd.cancel()
		<-cd.done
	}
}
