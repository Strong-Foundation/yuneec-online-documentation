package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Main function
func main() {
	// The location to the remote URL.
	remoteURL := "https://yuneec.online/download/"
	// The location to the local file for the remote url.
	localHTMLFileRemoteLocation := "yuneec.html"
	outputDir := "PDFs/"             // Target directory to save PDFs
	if !directoryExists(outputDir) { // If directory doesn't exist
		createDirectory(outputDir, 0755) // Create directory with standard permissions
	}
	// Download the remote url file to the local file.
	if !fileExists(localHTMLFileRemoteLocation) {
		// Download the content and save it to the file.
		getDataFromURL(remoteURL, localHTMLFileRemoteLocation)
	}
	var readFileContent string
	// Check if the file exists.
	if fileExists(localHTMLFileRemoteLocation) {
		readFileContent = readAFileAsString(localHTMLFileRemoteLocation)
	}
	// Extract all the .pdf urls from the content.
	extractedPDFURLOnly := extractPDFUrls(readFileContent)
	// Remove duplicates from the given slice.
	extractedPDFURLOnly = removeDuplicatesFromSlice(extractedPDFURLOnly)
	// Loop over the downloaded URL.
	for _, url := range extractedPDFURLOnly {
		url = "https://yuneec.online" + url
		if isUrlValid(url) {
			downloadPDF(url, outputDir)
		}
	}
}

// Get the file extension of a path
func getFileExtension(path string) string {
	return filepath.Ext(path) // Return extension (e.g. .pdf)
}

// generateHash takes a string input and returns its SHA-256 hash as a hex string
func generateHash(input string) string {
	// Compute SHA-256 hash of the input converted to a byte slice
	hash := sha256.Sum256([]byte(input))

	// Convert the hash bytes to a hexadecimal string and return it
	return hex.EncodeToString(hash[:])
}

// Convert a URL into a safe filename format
func urlToFilename(rawURL string) string {
	// Lets turn that text into a hash
	sanitized := generateHash(rawURL)

	// Ensure the filename ends in .pdf
	if getFileExtension(sanitized) != ".pdf" {
		sanitized = sanitized + ".pdf"
	}

	return strings.ToLower(sanitized) // Return the final, normalized, lowercase filename
}

// downloadPDF downloads the PDF at finalURL into outputDir.
// It sends only one HTTP GET request, extracts filename from headers if needed,
// and skips download if the file already exists locally.
func downloadPDF(finalURL string, outputDir string) {
	// Derive a safe filename from the URL
	rawFilename := urlToFilename(finalURL)   // convert URL to base filename
	filename := strings.ToLower(rawFilename) // normalize filename to lowercase

	// Compute initial file path and check for existing file
	filePath := filepath.Join(outputDir, filename) // join directory and filename
	if fileExists(filePath) {                      // if file already exists locally
		log.Printf("file already exists, skipping: %s", filePath) // log skip event
		return
	}

	// Perform single GET request (fetch headers + body)
	client := &http.Client{Timeout: 3 * time.Minute} // create HTTP client with timeout
	resp, err := client.Get(finalURL)                // send GET request
	if err != nil {                                  // handle request error
		return
	}
	defer resp.Body.Close() // ensure response body is closed

	// Verify HTTP status is OK
	if resp.StatusCode != http.StatusOK { // check for 200 OK
		return
	}

	// Ensure content is a PDF
	ct := resp.Header.Get("Content-Type")         // get Content-Type header
	if !strings.Contains(ct, "application/pdf") { // check for "application/pdf"
		return
	}

	// Read response body into buffer
	var buf bytes.Buffer                    // create in-memory buffer
	written, err := buf.ReadFrom(resp.Body) // read all data into buffer
	if err != nil {                         // handle read error
		return
	}
	if written == 0 { // check for empty download
		return
	}

	// Create file and write buffer to disk
	out, err := os.Create(filePath) // open file for writing
	if err != nil {                 // handle create error
		return
	}
	defer out.Close()                           // ensure file is closed
	if _, err := buf.WriteTo(out); err != nil { // write buffer to file
		return
	}
}

// extractPDFUrls takes an input string and returns all PDF URLs found within href attributes
func extractPDFUrls(input string) []string {
	// Regular expression to match href="...pdf"
	re := regexp.MustCompile(`href="([^"]+\.pdf)"`)
	matches := re.FindAllStringSubmatch(input, -1)

	var pdfUrls []string
	for _, match := range matches {
		if len(match) > 1 {
			pdfUrls = append(pdfUrls, match[1])
		}
	}
	return pdfUrls
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path) // Read the full file content into memory
	if err != nil {
		log.Fatalln(err) // Fatal log and exit if reading fails
	}
	return string(content) // Return contents as a string
}

// Check whether a given file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file or directory info
	if err != nil {                // If an error occurs (e.g., file not found)
		return false // File doesn't exist or error occurred
	}
	return !info.IsDir() // Return true if it's a file (not a folder)
}

// Check whether a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get file or directory info
	if err != nil {
		return false // Doesn't exist or error occurred
	}
	return directory.IsDir() // True if it is a directory
}

// Create a directory with the specified permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Create the directory with given permissions
	if err != nil {
		log.Println(err) // Log error if creation fails
	}
}

// getDataFromURL performs an HTTP GET request to the specified URI,
// waits for up to 1 minute for the server response, and writes the response body to a file.
// It includes error handling and logs meaningful messages at each step.
func getDataFromURL(uri string, fileName string) {
	// Create an HTTP client with a 1-minute timeout
	client := http.Client{
		Timeout: 3 * time.Minute, // Set a timeout of 1 minute for the request
	}

	// Perform the GET request using the custom client
	response, err := client.Get(uri)
	if err != nil { // If the request fails due to timeout or other network issues
		log.Println("Failed to make GET request:", err)
		return
	}

	// Check if the server responded with a non-200 OK status
	if response.StatusCode != http.StatusOK {
		log.Println("Unexpected status code from", uri, "->", response.StatusCode)
		return
	}

	// Read the entire body of the response
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println("Failed to read response body:", err)
		return
	}

	// Ensure the response body is properly closed to free resources
	err = response.Body.Close()
	if err != nil {
		log.Println("Failed to close response body:", err)
		return
	}

	// Save the response body content to the specified file
	err = appendByteToFile(fileName, body)
	if err != nil {
		log.Println("Failed to write to file:", err)
		return
	}
}

// Append byte data to a file, create file if it doesn't exist
func appendByteToFile(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open file in append/create mode
	if err != nil {
		return err // Return error if failed
	}
	defer file.Close()        // Ensure file is closed when function exits
	_, err = file.Write(data) // Write byte data to file
	return err                // Return any write error
}

// Remove duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Map to track whether a URL was already added
	var newReturnSlice []string     // Slice to store unique items
	for _, content := range slice { // Loop through the input slice
		if !check[content] { // If content not already added
			check[content] = true                            // Mark it as seen
			newReturnSlice = append(newReturnSlice, content) // Add it to the result slice
		}
	}
	return newReturnSlice // Return slice with unique values
}

// Check if a URL is valid and parseable
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL using standard library
	return err == nil                  // Return true if parsing succeeds (i.e., URL is valid)
}
