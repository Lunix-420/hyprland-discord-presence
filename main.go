package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"io/ioutil"
	"time"

	"github.com/hugolgst/rich-go/client"
)

type ActiveWindow struct {
	Address   string      `json:"address"`
	Workspace interface{} `json:"workspace"`
	Title     string      `json:"initialTitle"`
	Class     string      `json:"class"`
	PID       int         `json:"pid"`
}

var (
	debounceTimer *time.Timer
	useStream     bool
	startTime     time.Time
)

func init() {
	// Parse command-line flags
	flag.BoolVar(&useStream, "use-stream-window", false, "Enable real-time window tracking")
	flag.Parse()

	startTime = time.Now()
}

func getHyprlandSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	hyprDir := filepath.Join(runtimeDir, "hypr")

	files, err := ioutil.ReadDir(hyprDir)
	if err != nil || len(files) == 0 {
		fmt.Println("Error: No Hyprland instance found in", hyprDir)
		os.Exit(1)
	}

	instanceSignature := files[0].Name()
	return filepath.Join(hyprDir, instanceSignature, ".socket2.sock")
}

func getActiveWindowTitle(title, class string) string {
	if title != "" {
		return fmt.Sprintf("Opening %s", title)
	}
	if class != "" {
		return fmt.Sprintf("Opening %s", class)
	}
	return "No window opened"
}

func getActiveWindowDetails() (*ActiveWindow, error) {
	cmd := exec.Command("hyprctl", "activewindow", "-j")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run hyprctl: %w", err)
	}

	var window ActiveWindow
	if err := json.Unmarshal(out.Bytes(), &window); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &window, nil
}

func updateDiscordPresence(activeWindow string) {
	err := client.SetActivity(client.Activity{
		State:      activeWindow,
		Details:    "Using Hyprland on NixOS",
		LargeImage: "hyprland-dark",
		LargeText:  "Hyprland",
		Timestamps: &client.Timestamps{
			Start: &startTime,
		},
	})
	if err != nil {
		fmt.Println("Failed to update Discord presence:", err)
	}
}

func debounceUpdate(activeWindow string) {
	if debounceTimer != nil {
		debounceTimer.Stop()
	}
	debounceTimer = time.AfterFunc(2000*time.Millisecond, func() {
		updateDiscordPresence(activeWindow)
	})
}

func listenForActiveWindowChanges() {
	socketPath := getHyprlandSocketPath()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Println("Failed to connect to Hyprland socket:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte("subactivewindow\n"))
	if err != nil {
		fmt.Println("Failed to send subscription request:", err)
		return
	}

	fmt.Println("Listening for active window changes...")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		window, err := getActiveWindowDetails()
		if err != nil {
			fmt.Println("Error fetching active window details:", err)
			continue
		}

		activeWindow := getActiveWindowTitle(window.Title, window.Class)
		debounceUpdate(activeWindow)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from socket:", err)
	}
}

func main() {
	err := client.Login("1346300274087559259")
	if err != nil {
		fmt.Println("Failed to connect to Discord:", err)
		return
	}

	if useStream {
		listenForActiveWindowChanges()
	} else {
		updateDiscordPresence("Working on something great")
		select {} // Keep the program running
	}
}

