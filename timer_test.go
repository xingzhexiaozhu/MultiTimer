package Exp

import (
	"fmt"
	"testing"
	"time"
)

func TestJob(t *testing.T) {
	scheduler := NewScheduler()

	go scheduler.Start()

	tid := scheduler.AddTask(&Task{
		LastRunTime:time.Now().Unix(),
		Interval:5,
		Job:FuncJob(func() {
			fmt.Println("task1")
		}),
	})

	fmt.Println("tid = ", tid)
	fmt.Println(scheduler.ScanTask())

	// 阻塞
	block := make(chan int)
	<-block
}
