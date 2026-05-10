package main

import "fmt"

// AuthMiddleware validates incoming request credentials.
// Rename this function to AuthGuard throughout the package.
func AuthMiddleware(token string) bool {
	return token != ""
}

// ApplyAuth applies the authentication middleware to a request.
func ApplyAuth(token string) string {
	if AuthMiddleware(token) {
		return "authenticated"
	}
	return "rejected"
}

func main() {
	result := ApplyAuth("secret-token")
	fmt.Println(result)
}
