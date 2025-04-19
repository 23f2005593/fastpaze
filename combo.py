//fastpaze/libgoserver.go//

package main

import (
	"C"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// TaskResponse for JSON response
type TaskResponse struct {
	Message string `json:"message"`
	TaskID  string `json:"task_id"`
}

// ApiResponse for JSON response
type ApiResponse struct {
	Message        string       `json:"message"`
	BackgroundTask TaskResponse `json:"background_task"`
}

// Global routes map with mutex for thread-safe access
var (
	routes     = make(map[string]func() string)
	routesMu   sync.RWMutex
	taskPool   = sync.Pool{New: func() interface{} { return make(chan struct{}, 10) }} // Pool for task channels
	taskCtx    context.Context
	taskCancel context.CancelFunc
)

//export RegisterRoute
func RegisterRoute(cPath *C.char, cMessage *C.char) {
	path := C.GoString(cPath)
	message := C.GoString(cMessage)
	routesMu.Lock()
	routes[path] = func() string { return message }
	routesMu.Unlock()
}

// TaskManager handles background tasks with limited concurrency
func TaskManager(ctx context.Context, taskID string, taskChan chan struct{}) {
	defer func() { <-taskChan }() // Release slot
	select {
	case taskChan <- struct{}{}: // Acquire slot
		log.Printf("Starting background task %s", taskID)
		// Simulate work with context cancellation support
		select {
		case <-time.After(2 * time.Second):
			log.Printf("Completed background task %s", taskID)
		case <-ctx.Done():
			log.Printf("Cancelled background task %s", taskID)
		}
	case <-ctx.Done():
		log.Printf("Task %s not started due to shutdown", taskID)
	}
}

//export StartServer
func StartServer() {
	// Initialize context for task cancellation
	taskCtx, taskCancel = context.WithCancel(context.Background())
	defer taskCancel()

	// Create server with timeouts
	server := &http.Server{
		Addr:         ":8080",
		Handler:      nil,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// HTTP handler with logging and concurrency control
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			log.Printf("Invalid method %s from %s", r.Method, r.RemoteAddr)
			return
		}

		routesMu.RLock()
		handler, exists := routes[r.URL.Path]
		routesMu.RUnlock()
		if !exists {
			http.NotFound(w, r)
			log.Printf("Not found: %s from %s", r.URL.Path, r.RemoteAddr)
			return
		}

		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
		taskChan := taskPool.Get().(chan struct{})
		go TaskManager(taskCtx, taskID, taskChan)

		response := ApiResponse{
			Message:        handler(),
			BackgroundTask: TaskResponse{Message: fmt.Sprintf("Task started in background: %s", taskID), TaskID: taskID},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
		log.Printf("Handled %s from %s in %v", r.URL.Path, r.RemoteAddr, time.Since(startTime))
	})

	// Start server in a goroutine
	go func() {
		log.Printf("Go server running on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	log.Println("Shutting down server...")

	// Create shutdown context with 5-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cancel background tasks
	taskCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}

func main() {
	// Required for shared library
}

//fastpaze/goserver.py//

from ctypes import cdll, c_char_p
import os

class GoServer:
    def __init__(self):
        try:
            self.lib = cdll.LoadLibrary("./libgoserver.so")
            self.lib.RegisterRoute.argtypes = [c_char_p, c_char_p]
        except OSError as e:
            raise RuntimeError(f"Failed to load libgoserver.so: {e}")

    def route(self, path):
        def decorator(func):
            self.lib.RegisterRoute(path.encode('utf-8'), func().encode('utf-8'))
            return func
        return decorator

    def start(self):
        self.lib.StartServer()


//fastpaze/example.py//
from goserver import GoServer
server = GoServer()
@server.route("/api/hello")
def hello():
    return "Hello, World!"
@server.route("/api/other")
def other():
    return "Other Endpoint"
@server.route("/")
def index():
    return "Index Page"
if __name__ == "__main__":
    # Start the server
    print("Starting server...")
    server.start()
        