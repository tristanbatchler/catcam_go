#!/home/t/catcam_go/scripts/.venv/bin/python
import time
import argparse
import adafruit_pixelbuf
import board
from adafruit_led_animation.animation.rainbow import Rainbow
from adafruit_led_animation.animation.rainbowchase import RainbowChase
from adafruit_led_animation.animation.rainbowcomet import RainbowComet
from adafruit_led_animation.animation.rainbowsparkle import RainbowSparkle
from adafruit_led_animation.sequence import AnimationSequence
from adafruit_raspberry_pi5_neopixel_write import neopixel_write

class Pi5Pixelbuf(adafruit_pixelbuf.PixelBuf):
    def __init__(self, pin, size, **kwargs):
        self._pin = pin
        super().__init__(size=size, **kwargs)

    def _transmit(self, buf):
        neopixel_write(self._pin, buf)

def main():
    parser = argparse.ArgumentParser(description="Control NeoPixels on a Raspberry Pi 5.")
    parser.add_argument("pin", type=str, help="GPIO pin for NeoPixels (e.g., D14)")
    parser.add_argument("num_pixels", type=int, help="Number of pixels in the strip")
    parser.add_argument("animation", type=str, choices=["rainbow", "rainbow_chase", "rainbow_comet", "rainbow_sparkle", "cycle", "solid"], help="Animation to run")
    parser.add_argument("--color", type=str, default="FFFFFF", help="Hex color for solid mode (default: white)")
    args = parser.parse_args()

    try:
        pin = getattr(board, args.pin)  # Convert string to board pin
        pixels = Pi5Pixelbuf(pin, args.num_pixels, auto_write=True, byteorder="GRB")

        rainbow = Rainbow(pixels, speed=0.02, period=2)
        rainbow_chase = RainbowChase(pixels, speed=0.02, size=5, spacing=3)
        rainbow_comet = RainbowComet(pixels, speed=0.02, tail_length=7, bounce=True)
        rainbow_sparkle = RainbowSparkle(pixels, speed=0.02, num_sparkles=15)

        animations = AnimationSequence(
            rainbow,
            rainbow_chase,
            rainbow_comet,
            rainbow_sparkle,
            advance_interval=5,
            auto_clear=True,
        )

        animation_map = {
            "rainbow": rainbow,
            "rainbow_chase": rainbow_chase,
            "rainbow_comet": rainbow_comet,
            "rainbow_sparkle": rainbow_sparkle,
            "cycle": animations,
        }

        if args.animation == "solid":
            color = tuple(int(args.color[i:i+2], 16) for i in (0, 2, 4))
            print(f"Displaying solid color #{args.color}")
            pixels.fill(color)
            pixels.show()
        else:
            selected_animation = animation_map[args.animation]
            print(f"Running {args.animation} animation...")
            selected_animation.animate()
    
    except KeyboardInterrupt:
        print("\nStopping animation...")
        pixels.fill(0)
        pixels.show()

if __name__ == "__main__":
    main()