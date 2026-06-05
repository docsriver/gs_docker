package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var docsRiverFilesPath string

type Request struct {
	Command     string `json:"command"`
	TestFile    string `json:"testfile,omitempty"`
	PSFile      string `json:"psfile,omitempty"`
	PDFFile     string `json:"pdffile,omitempty"`
	PCLFile     string `json:"pclfile,omitempty"`
	Start       int    `json:"start,omitempty"`
	End         int    `json:"end,omitempty"`
	Output      string `json:"output,omitempty"`
	IsMonochrom bool   `json:"is_monochrom,omitempty"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func normalizeFileName(name string) string {
	return strings.TrimPrefix(name, "/")
}

func cleanOutput(output string) string {
	lines := strings.Split(output, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "   **** ") {
			filtered = append(filtered, line)
		}
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func fullPath(name string) string {
	return filepath.Join(docsRiverFilesPath, normalizeFileName(name))
}

func writeJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

func handlePing(req Request) (int, Response) {
	if req.TestFile == "" {
		return http.StatusOK, Response{Status: "OK", Message: "pong"}
	}
	data, err := os.ReadFile(fullPath(req.TestFile))
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: err.Error()}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(data)}
}

func handlePS2PDF(req Request) (int, Response) {
	psPath := fullPath(req.PSFile)
	pdfPath := fullPath(req.PDFFile)
	log.Printf("PS File: %s", psPath)
	log.Printf("PDF File: %s", pdfPath)

	cmd := exec.Command("ps2pdf", psPath, pdfPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: string(output)}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(output)}
}

func handlePCL2PDF(req Request) (int, Response) {
	pclPath := fullPath(req.PCLFile)
	pdfPath := fullPath(req.PDFFile)
	log.Printf("[PCL2PDF] PCL File: %s", pclPath)
	log.Printf("[PCL2PDF] PDF File: %s", pdfPath)

	cmd := exec.Command("gpcl6", "-sDEVICE=pdfwrite", "-o", pdfPath, pclPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: string(output)}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(output)}
}

func handleCountPages(req Request) (int, Response) {
	pdfPath := fullPath(req.PDFFile)
	log.Printf("PDF File: %s", pdfPath)

	gsScript := fmt.Sprintf("(%s) (r) file runpdfbegin pdfpagecount = quit", pdfPath)
	cmd := exec.Command("gs", "-q", "-dNOPAUSE", "-dBATCH", "-dNOSAFER", "-dNODISPLAY", "-c", gsScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: cleanOutput(string(output))}
	}
	return http.StatusOK, Response{Status: "OK", Message: cleanOutput(string(output))}
}

func handleExtractPDFPages(req Request) (int, Response) {
	pdfPath := fullPath(req.PDFFile)
	outPath := fullPath(req.Output)
	log.Printf("PDF File: %s", pdfPath)
	log.Printf("Result PDF File: %s", outPath)

	cmd := exec.Command("gs",
		"-q", "-dNOPAUSE", "-sDEVICE=pdfwrite", "-dBATCH", "-dNOSAFER",
		fmt.Sprintf("-dFirstPage=%d", req.Start),
		fmt.Sprintf("-dLastPage=%d", req.End),
		"-dAutoRotatePages=/None",
		fmt.Sprintf("-sOutputFile=%s", outPath),
		pdfPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: string(output)}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(output)}
}

func handleInfo() (int, Response) {
	type InfoResult struct {
		GS    bool `json:"gs"`
		GPCL6 bool `json:"gpcl6"`
	}

	isInstalled := func(binary string) bool {
		_, err := exec.LookPath(binary)
		return err == nil
	}

	info := InfoResult{
		GS:    isInstalled("gs"),
		GPCL6: isInstalled("gpcl6"),
	}

	jsonBytes, err := json.Marshal(info)
	if err != nil {
		return http.StatusInternalServerError, Response{Status: "ERROR", Message: err.Error()}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(jsonBytes)}
}

func handleExtractPDFImagesPages(req Request) (int, Response) {
	pdfPath := fullPath(req.PDFFile)
	outPath := fullPath(req.Output)
	device := "pdfimage32"
	if req.IsMonochrom {
		device = "pdfimage8"
	}
	log.Printf("PDF File: %s", pdfPath)
	log.Printf("Result PDF File: %s", outPath)
	log.Printf("Device: %s", device)

	cmd := exec.Command("gs",
		"-q", "-dNOPAUSE",
		fmt.Sprintf("-sDEVICE=%s", device),
		"-r300", "-dBATCH", "-dNOSAFER",
		fmt.Sprintf("-dFirstPage=%d", req.Start),
		fmt.Sprintf("-dLastPage=%d", req.End),
		"-dAutoRotatePages=/None",
		fmt.Sprintf("-sOutputFile=%s", outPath),
		pdfPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadRequest, Response{Status: "ERROR", Message: string(output)}
	}
	return http.StatusOK, Response{Status: "OK", Message: string(output)}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Response{Status: "ERROR", Message: "method not allowed"})
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Status: "ERROR", Message: err.Error()})
		return
	}

	log.Printf("POST request: command=%s", req.Command)

	var statusCode int
	var resp Response

	switch req.Command {
	case "ping":
		statusCode, resp = handlePing(req)
	case "ps2pdf":
		statusCode, resp = handlePS2PDF(req)
	case "pcl2pdf":
		statusCode, resp = handlePCL2PDF(req)
	case "countPages":
		statusCode, resp = handleCountPages(req)
	case "extractPDFPages":
		statusCode, resp = handleExtractPDFPages(req)
	case "extractPDFImagesPages":
		statusCode, resp = handleExtractPDFImagesPages(req)
	case "info":
		statusCode, resp = handleInfo()
	default:
		statusCode = http.StatusBadRequest
		resp = Response{Status: "ERROR", Message: "unknown command: " + req.Command}
	}

	writeJSON(w, statusCode, resp)
}

func main() {
	docsRiverFilesPath = os.Getenv("DOCS_RIVER_FILES_PATH")
	if docsRiverFilesPath == "" {
		docsRiverFilesPath = "/data"
	}

	http.HandleFunc("/", handler)
	log.Printf("Started GS HTTP server on port 9080")
	if err := http.ListenAndServe(":9080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
