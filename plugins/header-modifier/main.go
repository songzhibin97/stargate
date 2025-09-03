package main

import (
	"encoding/json"
	"unsafe"
)

// PluginRequest represents the request data from the host
type PluginRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Query   map[string]string `json:"query"`
}

// PluginResponse represents the response to the host
type PluginResponse struct {
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Status   int               `json:"status,omitempty"`
	Error    string            `json:"error,omitempty"`
	Continue bool              `json:"continue,omitempty"`
}

// Global variables to store allocated memory pointers
var (
	responsePtr uintptr
	responseLen int
)

// malloc allocates memory and returns a pointer
//export malloc
func malloc(size uint32) uint32 {
	// Simple memory allocation using make
	data := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&data[0])))
}

// free deallocates memory (no-op in Go with GC)
//export free
func free(ptr uint32) {
	// No-op in Go, garbage collector handles memory
}

// process_request is the main plugin function called by the host
//export process_request
func process_request(requestPtr, requestLen uint32) (uint32, uint32) {
	// Read request data from memory
	requestData := (*[1 << 30]byte)(unsafe.Pointer(uintptr(requestPtr)))[:requestLen:requestLen]
	
	// Parse request JSON
	var request PluginRequest
	if err := json.Unmarshal(requestData, &request); err != nil {
		return returnError("Failed to parse request JSON: " + err.Error())
	}

	// Log the request (this will call the host's log function)
	logMessage("Processing request: " + request.Method + " " + request.Path)

	// Create response with modified headers
	response := PluginResponse{
		Headers:  make(map[string]string),
		Continue: true,
	}

	// Copy existing headers
	for key, value := range request.Headers {
		response.Headers[key] = value
	}

	// Add custom headers based on the request
	response.Headers["X-Plugin-Name"] = "header-modifier"
	response.Headers["X-Plugin-Version"] = "1.0.0"
	response.Headers["X-Processed-At"] = getCurrentTimestamp()
	
	// Add request-specific headers
	if request.Method == "POST" {
		response.Headers["X-Post-Request"] = "true"
	}
	
	if request.Path == "/api/transform" {
		response.Headers["X-Transform-Request"] = "true"
	}

	// Modify request body if it's JSON
	if request.Headers["Content-Type"] == "application/json" && request.Body != "" {
		modifiedBody, err := modifyJSONBody(request.Body)
		if err != nil {
			logMessage("Failed to modify JSON body: " + err.Error())
		} else {
			response.Body = modifiedBody
		}
	}

	// Serialize response to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return returnError("Failed to serialize response: " + err.Error())
	}

	// Allocate memory for response
	responsePtr = uintptr(malloc(uint32(len(responseJSON))))
	responseLen = len(responseJSON)
	
	// Copy response data to allocated memory
	responseData := (*[1 << 30]byte)(unsafe.Pointer(responsePtr))[:responseLen:responseLen]
	copy(responseData, responseJSON)

	return uint32(responsePtr), uint32(responseLen)
}

// modifyJSONBody modifies JSON request body by adding plugin metadata
func modifyJSONBody(body string) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body, err // Return original body if not valid JSON
	}

	// Add plugin metadata
	if data == nil {
		data = make(map[string]interface{})
	}
	
	data["_plugin_processed"] = true
	data["_plugin_name"] = "header-modifier"
	data["_plugin_timestamp"] = getCurrentTimestamp()

	modifiedJSON, err := json.Marshal(data)
	if err != nil {
		return body, err
	}

	return string(modifiedJSON), nil
}

// getCurrentTimestamp returns current timestamp as string
func getCurrentTimestamp() string {
	// Since we can't use time package in WASM easily, return a placeholder
	return "2025-09-02T06:40:00Z"
}

// returnError creates an error response
func returnError(message string) (uint32, uint32) {
	response := PluginResponse{
		Error:    message,
		Continue: false,
	}

	responseJSON, _ := json.Marshal(response)
	responsePtr = uintptr(malloc(uint32(len(responseJSON))))
	responseLen = len(responseJSON)
	
	responseData := (*[1 << 30]byte)(unsafe.Pointer(responsePtr))[:responseLen:responseLen]
	copy(responseData, responseJSON)

	return uint32(responsePtr), uint32(responseLen)
}

// logMessage sends a log message to the host
func logMessage(message string) {
	messageBytes := []byte(message)
	messagePtr := uintptr(unsafe.Pointer(&messageBytes[0]))
	log(uint32(messagePtr), uint32(len(messageBytes)))
}

// log is imported from the host environment
//go:wasmimport env log
func log(ptr, len uint32) {
	// This function is implemented by the WASM host
}

// get_header is imported from the host environment
//go:wasmimport env get_header
func get_header(keyPtr, keyLen, valuePtr, valueLen uint32) uint32 {
	// This function is implemented by the WASM host
	return 0
}

// set_header is imported from the host environment
//go:wasmimport env set_header
func set_header(keyPtr, keyLen, valuePtr, valueLen uint32) {
	// This function is implemented by the WASM host
}

// main function is required but not used in WASM plugins
func main() {
	// This function is required for TinyGo but won't be called
	// The actual entry points are the exported functions above
}
