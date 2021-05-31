package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"time"
)

var ErrTimeOut = errors.New("执行超时")
var ErrInterrupt = errors.New("执行被中断")


type Runner struct {
	tasks []func(int)
	complete chan error
	timeout <-chan time.Time
	interrupt chan os.Signal
}

func NewRunner(tm time.Duration) *Runner {
	return &Runner{
		complete: make(chan error),
		timeout: time.After(tm),
		interrupt: make(chan os.Signal, 1),
	}
}

func (r *Runner) AddTask(tasks ...func(int))  {
	r.tasks = append(r.tasks, tasks...)
}

func (r *Runner) run() error {
	for index, task := range r.tasks {
		if r.isInterrupt() {
			return ErrInterrupt
		}
		task(index)
	}
	return nil
}

func (r *Runner) isInterrupt() bool {
	select {
	case <-r.interrupt:
		signal.Stop(r.interrupt)
		return true
	default:
		return false
	}
}

func (r *Runner) Start() error {
	signal.Notify(r.interrupt, os.Interrupt)
	go func() {
		r.complete <- r.run()
	}()

	select {
	case err := <-r.complete:
		return err
	case <-r.timeout:
		return ErrTimeOut
	}
}



func createTask() func(int) {
	return func(id int) {
		log.Printf("正在执行的任务%d", id)
		time.Sleep(time.Duration(id) * time.Second)
	}
}

func main()  {
	log.Println("开始执行任务")

	r := NewRunner(time.Second * 10)
	r.AddTask(createTask(), createTask(), createTask())
	if err := r.Start(); err != nil {
		switch err {
		case ErrTimeOut:
			log.Println(err)
			os.Exit(1)
		case ErrInterrupt:
			log.Println(err)
			os.Exit(2)
		}
	}
	log.Println("...任务执行结束...")
}
