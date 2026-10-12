package main

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"
)

func httpRequestForWorker(ctx context.Context, tx *sql.Tx) *http.Request {
	ctx = context.WithValue(ctx, requestTxKey, tx)
	ctx = context.WithValue(ctx, requestMetaKey, &requestMeta{})
	return (&http.Request{Method: "POST", Header: make(http.Header)}).WithContext(ctx)
}
func startExpansionWorkers(ctx context.Context) func() {
	var workers sync.WaitGroup
	workers.Add(4)
	go func() {
		defer workers.Done()
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				processAssistantJobs(ctx)
			}
		}
	}()
	go func() {
		defer workers.Done()
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				processWhatsApp(ctx)
			}
		}
	}()
	go func() {
		defer workers.Done()
		syncMailbox(ctx)
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				syncMailbox(ctx)
			}
		}
	}()
	go func() {
		defer workers.Done()
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				processTrashRetention(ctx)
				expireAIVoice(ctx)
				processAIAdvice(ctx)
			}
		}
	}()
	return workers.Wait
}
