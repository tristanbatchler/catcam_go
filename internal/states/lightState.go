package states

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type Light struct {
	isOn  bool
	red   int
	green int
	blue  int
	cmd   *exec.Cmd
}

func NewLight() *Light {
	return &Light{
		red:   255,
		green: 255,
		blue:  255,
	}
}

func (l *Light) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", l.red, l.green, l.blue)
}

func (l *Light) FromHex(hex string) {
	fmt.Sscanf(hex, "#%02x%02x%02x", &l.red, &l.green, &l.blue)
	if l.isOn {
		l.updateLights(hex)
	}
}

func (l *Light) IsOn() bool {
	return l.isOn
}

func (l *Light) Toggle() {
	if l.isOn {
		l.TurnOff()
	} else {
		l.TurnOn()
	}
}

func (l *Light) TurnOn() {
	l.isOn = true
	l.updateLights(l.Hex())
}

func (l *Light) TurnOff() {
	l.isOn = false
	l.updateLights("#000000")
}

func (l *Light) updateLights(hex string) {
	command := fmt.Sprintf("./scripts/control_leds.py D14 24 solid --color %s", strings.TrimPrefix(hex, "#"))

	l.cmd = exec.Command("sh", "-c", command)
	output, err := l.cmd.Output()
	if err != nil {
		log.Printf("Error controlling LEDs: %v", err)
	} else {
		log.Println(string(output))
	}
}

func (l *Light) Stop() error {
	if l.cmd != nil && l.cmd.Process != nil {
		err := l.cmd.Process.Kill()
		if err != nil {
			log.Println("Failed to kill light controller process:", err)
			return err
		}
	}
	return nil
}
