package states

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

type Camera struct {
	cmd         *exec.Cmd
	width       int
	height      int
	fps         int
	compression int
	stream      chan []byte // Buffered channel for frames
	subscribers map[chan []byte]struct{}
	mu          sync.Mutex
	running     bool
	bufferSize  int
}

// NewCamera initializes the camera with a buffered channel
func NewCamera(width, height, fps int, compression int, bufferSize int) *Camera {
	return &Camera{
		width:       width,
		height:      height,
		fps:         fps,
		compression: compression,
		bufferSize:  bufferSize,
		stream:      make(chan []byte, bufferSize),
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Subscribe adds a new client stream channel
func (c *Camera) Subscribe() chan []byte {
	log.Println("New subscriber")
	ch := make(chan []byte, c.fps*c.bufferSize)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a client stream channel
func (c *Camera) Unsubscribe(ch chan []byte) {
	log.Println("Subscriber left")
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, ch)
}

// Start launches FFmpeg and streams frames into the channel
func (c *Camera) Start() error {
	if c.running {
		return nil
	}
	c.running = true
	c.stream = make(chan []byte, c.fps) // Buffer frames for 1 second

	log.Println("Starting camera")
	c.cmd = exec.Command(
		"ffmpeg",
		"-f", "video4linux2",
		"-s", fmt.Sprintf("%dx%d", c.width, c.height),
		"-i", "/dev/video0",
		"-f", "mpjpeg",
		"-q:v", fmt.Sprintf("%d", c.compression),
		"-vf", fmt.Sprintf("scale=%d:%d", c.width, c.height),
		"-r", fmt.Sprintf("%d", c.fps),
		"pipe:1",
	)

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := c.cmd.Start(); err != nil {
		return err
	}

	// Frame drop detection
	lastFrameTime := time.Now()
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		defer ticker.Stop()
		defer close(c.stream)
		defer c.Stop()

		buf := make([]byte, 4096)

		// Monitor frame reading
		go func() {
			for range ticker.C {
				if time.Since(lastFrameTime) > 5*time.Second {
					log.Println("No frames received for 5 seconds. Stopping camera.")
					c.Stop()
					return
				}
			}
		}()

		// Read frames and send them over the channel
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

			data := make([]byte, n)
			copy(data, buf[:n])

			// Send frame to the channel (drop old frames if full)
			select {
			case c.stream <- data:
				lastFrameTime = time.Now()
			default:
				log.Println("Frame dropped: channel full") // Prevent blocking
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
