package main

import "fmt"

// ProcessRequest handles an incoming API request and returns a response string.
// TODO: update signature to accept context.Context as first parameter.
func ProcessRequest(userID string, payload string) string {
	return fmt.Sprintf("user=%s payload=%s", userID, payload)
}

// HandleHTTP is a caller of ProcessRequest.
func HandleHTTP(userID string, body string) string {
	return ProcessRequest(userID, body)
}

func main() {
	resp := HandleHTTP("user-42", "hello")
	fmt.Println(resp)
}
