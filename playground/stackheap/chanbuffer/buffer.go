package chanbuffer

import "time"

const (
	stateIdle int32 = iota
)

type LaneBuffer[T any] struct {
	deadletter chan struct{}

	state int32

	lane chan T

	mainLane chan chan []T
	backLane chan chan []T

	mainItems int32
	backItems int32

	mainSize int32
	backSize int32

	readIndex int32
}

func NewLaneBuffer[T any](size int32) *LaneBuffer[T] {
	hBuffer := &LaneBuffer[T]{
		deadletter: make(chan struct{}),

		lane: make(chan T, size),

		mainLane: make(chan chan []T, size),
		backLane: make(chan chan []T, size),
		mainSize: size,
		backSize: size,
	}
	hBuffer.mainLane <- make(chan []T, size)
	go func() {
		for {
			ticker := time.NewTicker(1 * time.Nanosecond)
			select {
			case <-hBuffer.deadletter:
				// Triggered, do something
				return
			case <-ticker.C:
				if len(hBuffer.lane) > 0 {
					data := []T{}
					subLane := make(chan []T)
					for item := range hBuffer.lane {
						data = append(data, item)
					}
					subLane <- data
					hBuffer.mainLane <- subLane
				}
			}
		}
	}()
	return hBuffer
}

func (hb *LaneBuffer[T]) Push(item T) error {
	hb.lane <- item
	return nil
}

func (hb *LaneBuffer[T]) Close() {
	close(hb.mainLane)
	close(hb.backLane)
	close(hb.lane)
}
