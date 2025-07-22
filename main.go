package main

import (
	"context"
	"errors"
	"future/future"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func fx1() {
	appCtx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()

	fut := future.New[string]()

	go func() {
		time.Sleep(5 * time.Second)

		fut.Resolve("Success")
	}()

	val, err := fut.Wait(appCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			println("Operation was canceled")
		} else {
			println("Error:", err.Error())
		}
		return
	}
	println("Received value:", val)
	<-appCtx.Done()
	println("Application exiting gracefully")
}

func fx2() {
	appCtx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()

	fut := future.New[string]()

	go func() {
		time.Sleep(5 * time.Second)

		fut.Resolve("Success")
	}()

	val, err := fut.WaitTimeout(3 * time.Second)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			println("Operation timed out")
		} else {
			println("Error:", err.Error())
		}
		return
	}
	println("Received value:", val)
	<-appCtx.Done()
	println("Application exiting gracefully")
}

func main() {
	fx2()
}
