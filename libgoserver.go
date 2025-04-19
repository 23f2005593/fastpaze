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
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-playground/validator/v10"
)

// TaskResponse for JSON response
type TaskResponse struct {
	Message string `json:"message"`
	TaskID  string `json:"task_id"`
}

// ApiResponse for JSON response
type ApiResponse struct {
	Message        string       `json:"message"`
	BackgroundTask TaskResponse `json:"background_task,omitempty"`
}

// ErrorResponse for structured error responses
type ErrorResponse struct {
	Error string `json:"error"`
}

// RouteInfo stores route metadata for OpenAPI and handling
type RouteInfo struct {
	Path        string
	Method      string
	Message     string // Store the message directly instead of a handler for simplicity
	Description string
	Parameters  []ParameterInfo
	Responses   map[int]string
}

// ParameterInfo for OpenAPI documentation
type ParameterInfo struct {
	Name        string `json:"name"`
	In          string `json:"in"` // e.g., "query", "path"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
}

// OpenAPI structure for API documentation
type OpenAPI struct {
	OpenAPI    string                         `json:"openapi"`
	Info       map[string]string              `json:"info"`
	Paths      map[string]map[string]interface{} `json:"paths"`
	Components map[string]interface{}         `json:"components"`
}

// Global variables with thread-safe access
var (
	routes         = make(map[string]RouteInfo)
	routesMu       sync.RWMutex
	taskPool       = sync.Pool{New: func() interface{} { return make(chan struct{}, 10) }}
	taskCtx        context.Context
	taskCancel     context.CancelFunc
	validate       = validator.New()
	middlewares    = []func(http.Handler) http.Handler{}
	middlewaresMu  sync.RWMutex
	dependencies   = make(map[string]interface{})
	depsMu         sync.RWMutex
)

// Middleware registration
//export RegisterMiddleware
func RegisterMiddleware(cName uintptr, cEnabled int) {
	namePtr := (*C.char)(unsafe.Pointer(cName))
	if namePtr == nil {
		log.Println("Error: cName is nil in RegisterMiddleware")
		return
	}
	name := C.GoString(namePtr)

	enabled := cEnabled != 0
	if !enabled {
		log.Printf("Middleware %s is disabled", name)
		return
	}

	middlewaresMu.Lock()
	defer middlewaresMu.Unlock()
	switch name {
	case "logging":
		middlewares = append(middlewares, loggingMiddleware)
		log.Printf("Registered middleware: %s", name)
	default:
		log.Printf("Unknown middleware: %s", name)
	}
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s from %s in %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// Dependency injection context (e.g., for auth or DB)
//export RegisterDependency
func RegisterDependency(cName uintptr, cValue uintptr) {
	namePtr := (*C.char)(unsafe.Pointer(cName))
	valuePtr := (*C.char)(unsafe.Pointer(cValue))
	if namePtr == nil || valuePtr == nil {
		log.Println("Error: One or more parameters are nil in RegisterDependency")
		return
	}
	name := C.GoString(namePtr)
	value := C.GoString(valuePtr)
	depsMu.Lock()
	dependencies[name] = value
	depsMu.Unlock()
}

// GetDependency retrieves a dependency by name
func GetDependency(name string) (interface{}, bool) {
	depsMu.RLock()
	defer depsMu.RUnlock()
	val, exists := dependencies[name]
	return val, exists
}

//export RegisterRoute
func RegisterRoute(cPath uintptr, cMethod uintptr, cMessage uintptr, cDesc uintptr) {
	pathPtr := (*C.char)(unsafe.Pointer(cPath))
	methodPtr := (*C.char)(unsafe.Pointer(cMethod))
	messagePtr := (*C.char)(unsafe.Pointer(cMessage))
	descPtr := (*C.char)(unsafe.Pointer(cDesc))

	if pathPtr == nil || methodPtr == nil || messagePtr == nil || descPtr == nil {
		log.Println("Error: One or more parameters are nil in RegisterRoute")
		return
	}

	path := C.GoString(pathPtr)
	method := strings.ToUpper(C.GoString(methodPtr))
	message := C.GoString(messagePtr)
	desc := C.GoString(descPtr)

	log.Printf("Registering route: %s for method: %s with message: %s", path, method, message)

	routesMu.Lock()
	key := path + method
	routes[key] = RouteInfo{
		Path:        path,
		Method:      method,
		Message:     message,
		Description: desc,
		Parameters:  []ParameterInfo{},
		Responses: map[int]string{
			200: "Successful response",
		},
	}
	log.Printf("Route registered with key: %s", key)
	routesMu.Unlock()
}

// TaskManager handles background tasks with limited concurrency
func TaskManager(ctx context.Context, taskID string, taskChan chan struct{}) {
	defer func() { <-taskChan }()
	select {
	case taskChan <- struct{}{}:
		log.Printf("Starting background task %s", taskID)
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

// ServeOpenAPI generates the OpenAPI JSON
func ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	openapi := OpenAPI{
		OpenAPI: "3.0.0",
		Info: map[string]string{
			"title":   "GoServer API",
			"version": "1.0.0",
		},
		Paths:      make(map[string]map[string]interface{}),
		Components: make(map[string]interface{}),
	}

	routesMu.RLock()
	for _, route := range routes {
		if _, exists := openapi.Paths[route.Path]; !exists {
			openapi.Paths[route.Path] = make(map[string]interface{})
		}
		openapi.Paths[route.Path][strings.ToLower(route.Method)] = map[string]interface{}{
			"summary":     route.Description,
			"responses":   map[string]interface{}{"200": map[string]string{"description": route.Responses[200]}},
			"parameters":  route.Parameters,
		}
	}
	routesMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(openapi); err != nil {
		http.Error(w, `{"error": "Failed to generate OpenAPI"}`, http.StatusInternalServerError)
	}
}

//export StartServer
func StartServer() {
	taskCtx, taskCancel = context.WithCancel(context.Background())
	defer taskCancel()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      nil,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create a router with middleware support
	mux := http.NewServeMux()
	middlewaresMu.RLock()
	handler := http.Handler(mux)
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	middlewaresMu.RUnlock()

	// Register OpenAPI and Swagger UI endpoints
	mux.HandleFunc("/openapi.json", ServeOpenAPI)
	mux.HandleFunc("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("swagger-ui"))).ServeHTTP)

	
	// Dynamic route handling with method support
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path + r.Method
		routesMu.RLock()
		route, exists := routes[key]
		routesMu.RUnlock()
		if !exists {
			// Check if the path exists with a different method
			var supportedMethod string
			routesMu.RLock()
			for _, rt := range routes {
				if rt.Path == r.URL.Path {
					supportedMethod = rt.Method
					break
				}
			}
			routesMu.RUnlock()
			errorMsg := fmt.Sprintf("Route not found for %s %s", r.Method, r.URL.Path)
			if supportedMethod != "" {
				errorMsg = fmt.Sprintf("%s - Try using method %s", errorMsg, supportedMethod)
			}
			log.Printf("Route not found for key: %s (Path: %s, Method: %s)", key, r.URL.Path, r.Method)
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, errorMsg), http.StatusNotFound)
			return
		}
		log.Printf("Route found for key: %s, serving response", key)
		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
		response := ApiResponse{
			Message: route.Message,
			BackgroundTask: TaskResponse{
				Message: fmt.Sprintf("Task started in background: %s", taskID),
				TaskID:  taskID,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
			return
		}
		// Start background task
		taskChan := taskPool.Get().(chan struct{})
		go TaskManager(taskCtx, taskID, taskChan)
	})

	// Set the server handler
	server.Handler = handler

	go func() {
		log.Printf("Go server running on http://localhost:8080")
		log.Printf("API docs available at http://localhost:8080/swagger/")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	taskCancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}

func main() {
	// Required for shared library
}