package states

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

type Camera struct {
	cmd                    *exec.Cmd
	width                  int
	height                 int
	fps                    int
	quality                int
	stream                 chan []byte
	subscribers            map[chan []byte]struct{}
	mu                     sync.Mutex
	running                bool
	bufferSize             int
	timeSinceNoSubscribers time.Time
	light                  *Light
}

// NewCamera initializes the camera with a buffered channel
func NewCamera(width, height, fps int, quality int, bufferSize int, light *Light) *Camera {
	return &Camera{
		width:                  width,
		height:                 height,
		fps:                    fps,
		quality:                quality,
		bufferSize:             bufferSize,
		stream:                 make(chan []byte, bufferSize),
		subscribers:            make(map[chan []byte]struct{}),
		timeSinceNoSubscribers: time.Now(),
		light:                  light,
	}
}

// Subscribe adds a new client stream channel
func (c *Camera) Subscribe() chan []byte {
	ch := make(chan []byte, c.fps*c.bufferSize)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers[ch] = struct{}{}
	log.Printf("New subscriber. Num subscribers now: %d", len(c.subscribers))
	c.timeSinceNoSubscribers = time.Time{}
	return ch
}

// Unsubscribe removes a client stream channel
func (c *Camera) Unsubscribe(ch chan []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, ch)
	numSubs := len(c.subscribers)
	log.Printf("Subscriber left. Num subscribers now: %d", numSubs)

	if numSubs <= 0 {
		c.timeSinceNoSubscribers = time.Now()
	}
}

// Start launches FFmpeg and streams frames into the channel
func (c *Camera) Start() error {
	if c.running {
		return nil
	}
	c.running = true
	c.stream = make(chan []byte, c.fps) // Buffer frames for 1 second

	log.Println("Starting camera")

	// For Raspberry Pi 5
	c.cmd = exec.Command(
		"rpicam-vid",
		"-t", "0",
		"--codec", "mjpeg",
		"--width", fmt.Sprintf("%d", c.width),
		"--height", fmt.Sprintf("%d", c.height),
		"--framerate", fmt.Sprintf("%d", c.fps),
		"--quality", fmt.Sprintf("%d", c.quality),
		"--inline",
		"-o", "-",
	)

	// For USB webcam
	// c.cmd = exec.Command(
	// 	"ffmpeg",
	// 	"-f", "video4linux2",
	// 	"-s", fmt.Sprintf("%dx%d", c.width, c.height),
	// 	"-i", "/dev/video0",
	// 	"-f", "mpjpeg",
	// 	"-q:v", fmt.Sprintf("%d", c.compression),
	// 	"-vf", fmt.Sprintf("scale=%d:%d", c.width, c.height),
	// 	"-r", fmt.Sprintf("%d", c.fps),
	// 	"pipe:1",
	// )

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := c.cmd.Start(); err != nil {
		return err
	}

	// Frame drop detection
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		defer ticker.Stop()
		defer close(c.stream)
		defer c.Stop()

		buf := make([]byte, 4096)

		// Read frames and send them over the channel
		var frameBuffer []byte
		const jpegSOI = "\xFF\xD8" // Start of Image marker
		const jpegEOI = "\xFF\xD9" // End of Image marker

		for c.running {
			n, err := stdout.Read(buf)
			if err != nil {
				if err == io.EOF {
					log.Println("Camera stream ended")
				} else {
					log.Println("Camera read error:", err)
				}
				return
			}

			frameBuffer = append(frameBuffer, buf[:n]...)

			// Look for a complete JPEG frame
			startIdx := bytes.Index(frameBuffer, []byte(jpegSOI))
			endIdx := bytes.Index(frameBuffer, []byte(jpegEOI))

			if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
				// Extract and send the complete frame
				frame := frameBuffer[startIdx : endIdx+2]
				select {
				case c.stream <- frame:
				default:
					log.Println("Frame dropped: channel full")
				}

				// Remove processed frame from buffer
				frameBuffer = frameBuffer[endIdx+2:]
			}
		}
	}()

	// Broadcast frames to all subscribers
	go func() {
		for frame := range c.stream {
			c.mu.Lock()
			for ch := range c.subscribers {
				select {
				case ch <- frame:
				default:
					log.Println("Frame dropped: subscriber channel full")
				}
			}
			c.mu.Unlock()
		}
	}()

	// Monitor time since last subscriber left and shut down
	go func() {
		for range ticker.C {
			if !c.timeSinceNoSubscribers.IsZero() && time.Since(c.timeSinceNoSubscribers) > 5*time.Second {
				log.Println("No subscribers for 5 seconds. Stopping camera.")
				c.Stop()
				return
			}
		}
	}()

	return nil
}

// Stop terminates the camera process
func (c *Camera) Stop() {
	if !c.running {
		return
	}
	c.running = false

	if c.cmd != nil && c.cmd.Process != nil {
		err := c.cmd.Process.Kill()
		if err != nil {
			log.Println("Failed to kill camera process:", err)
		}
	}
	log.Println("Camera stopped")

	c.light.TurnOff()
	c.light.Stop()
	log.Println("Light stopped too")
}

func (c *Camera) IsRunning() bool {
	return c.running
}

func (c *Camera) Width() int {
	return c.width
}

func (c *Camera) Height() int {
	return c.height
}
