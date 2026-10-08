package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

// TestArrowPreview renders sample arrows to PNG files for visual inspection.
func TestArrowPreview(t *testing.T) {
	W, H := 900, 480
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 245, G: 245, B: 245, A: 255}}, image.Point{}, draw.Src)

	arrows := []struct{ x1, y1, x2, y2 int }{
		{60, 60, 380, 60},    // horizontal
		{60, 120, 380, 220},  // diagonal 45-ish
		{60, 300, 380, 280},  // shallow angle
		{60, 420, 150, 460},  // short diagonal
		{500, 60, 860, 240},  // long diagonal
		{500, 300, 860, 300}, // long horizontal
		{520, 420, 521, 421}, // stray click -> should NOT render
	}
	for _, a := range arrows {
		drawArrow(img, a.x1, a.y1, a.x2, a.y2)
	}

	// Zoomed single arrow to inspect the head and the smoothing closely
	zoom := image.NewRGBA(image.Rect(0, 0, 260, 160))
	draw.Draw(zoom, zoom.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	drawArrow(zoom, 30, 90, 210, 60)

	base := os.Getenv("HOME") + "/.hermes/cache/scratch/micapp-perf/"
	f1, err := os.Create(base + "arrow_preview.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f1, img); err != nil {
		t.Fatal(err)
	}
	f1.Close()

	f2, err := os.Create(base + "arrow_zoom.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f2, zoom); err != nil {
		t.Fatal(err)
	}
	f2.Close()
}
