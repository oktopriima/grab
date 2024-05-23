package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxRange      = 100
	maxGoroutines = 1000
	timeout       = 1 * time.Second
)

func SingleFizzBuzz(n int) string {
	if n%3 == 0 && n%5 == 0 {
		return "FizzBuzz"
	} else if n%3 == 0 {
		return "Fizz"
	} else if n%5 == 0 {
		return "Buzz"
	}
	return strconv.Itoa(n)
}

func rangeFizzBuzzHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, errFrom := strconv.Atoi(fromStr)
	to, errTo := strconv.Atoi(toStr)

	if errFrom != nil || errTo != nil || from > to || to-from+1 > maxRange {
		http.Error(w, "request parameters invalid", http.StatusBadRequest)
		return
	}

	results := make([]string, to-from+1)
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxGoroutines)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	for i := from; i <= to; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
				results[i-from] = SingleFizzBuzz(i)
				<-sem
			}
		}(i)
	}

	wg.Wait()

	response := strings.Join(results, " ")
	log.Printf("Request: from=%d to=%d", from, to)
	log.Printf("Response: %s", response)
	log.Printf("Latency: %v", time.Since(start))

	_, err := fmt.Fprintln(w, response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	// Define the route
	http.HandleFunc("/range-fizzbuzz", rangeFizzBuzzHandler)

	server := &http.Server{Addr: ":8080"}

	// Gracefully shutdown the server
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on :8080: %v\n", err)
		}
	}()

	log.Println("Server started on :8080")
	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v\n", err)
	}

	log.Println("Server gracefully stopped")
}
