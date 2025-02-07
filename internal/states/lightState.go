package states

import "fmt"

type Light struct {
	IsOn  bool
	Red   int
	Green int
	Blue  int
}

func (l *Light) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", l.Red, l.Green, l.Blue)
}

func (l *Light) FromHex(hex string) {
	fmt.Sscanf(hex, "#%02x%02x%02x", &l.Red, &l.Green, &l.Blue)
}
