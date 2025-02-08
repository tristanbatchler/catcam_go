package states

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

type Camera struct {
	cmd     *exec.Cmd
	width   int
	height  int
	fps     int
	quality int
	data    chan []byte // Buffered channel for frames
	running bool
}

// NewCamera initializes the camera with a buffered channel
func NewCamera(width int, height int, fps int, quality int) *Camera {
	return &Camera{
		width:   width,
		height:  height,
		fps:     fps,
		quality: quality,
	}
}

// Start launches FFmpeg and streams frames into the channel
func (c *Camera) Start() error {
	if c.running {
		return nil
	}
	c.running = true
	c.data = make(chan []byte, c.fps) // Buffer frames for 1 second

	log.Println("Starting camera")
	c.cmd = exec.Command(
		"ffmpeg",
		"-f", "video4linux2",
		"-s", fmt.Sprintf("%dx%d", c.width, c.height),
		"-i", "/dev/video0",
		"-f", "mpjpeg",
		"-q:v", fmt.Sprintf("%d", c.quality),
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
		defer close(c.data)
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
			case c.data <- data:
				lastFrameTime = time.Now()
			default:
				log.Println("Frame dropped: channel full") // Prevent blocking
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
}

// Stream returns the frame data channel for consumers
func (c *Camera) Stream() <-chan []byte {
	return c.data
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
