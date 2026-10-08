package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/go-vgo/robotgo"
	"github.com/vcaesar/screenshot"
)

// copyImageToClipboard copies image to clipboard using xclip (with a timeout,
// so a stuck xclip cannot freeze the capture pipeline).
func copyImageToClipboard(imageData []byte) error {
	return clipWrite("clipboard", "image/png", imageData, 3*time.Second)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// cropImage extracts a rectangular region from a captured screen image.
func cropImage(src *image.RGBA, x, y, width, height int) *image.RGBA {
	srcBounds := src.Bounds()
	cropRect := image.Rect(x, y, x+width, y+height).Add(srcBounds.Min)
	intersect := srcBounds.Intersect(cropRect)
	if intersect.Empty() {
		return nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, intersect.Sub(cropRect.Min), src, intersect.Min, draw.Src)
	return dst
}

// captureScreenRegion captures a region of the screen using native Go libraries.
// It tries a direct region capture first, then falls back to full-display capture + crop.
func captureScreenRegion(x, y, width, height int) ([]byte, error) {
	log.Printf("captureScreenRegion called with x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	img, err := screenshot.Capture(x, y, width, height)
	if err == nil && img != nil {
		if data, encErr := encodePNG(img); encErr == nil && len(data) > 0 {
			log.Printf("Native region capture successful, size: %d bytes", len(data))
			return data, nil
		} else if encErr != nil {
			log.Printf("Failed to encode region capture: %v", encErr)
		}
	} else {
		log.Printf("Native region capture failed: %v", err)
	}

	for displayIndex := 0; displayIndex < screenshot.NumActiveDisplays(); displayIndex++ {
		displayBounds := screenshot.GetDisplayBounds(displayIndex)
		region := image.Rect(x, y, x+width, y+height)
		if displayBounds.Intersect(region).Empty() {
			continue
		}

		displayImg, err := screenshot.CaptureDisplay(displayIndex)
		if err != nil {
			log.Printf("Display %d capture failed: %v", displayIndex, err)
			continue
		}

		relX := x - displayBounds.Min.X
		relY := y - displayBounds.Min.Y
		cropped := cropImage(displayImg, relX, relY, width, height)
		if cropped == nil {
			continue
		}

		data, err := encodePNG(cropped)
		if err != nil {
			log.Printf("Failed to encode cropped display %d capture: %v", displayIndex, err)
			continue
		}

		log.Printf("Display %d crop capture successful, size: %d bytes", displayIndex, len(data))
		return data, nil
	}

	return nil, fmt.Errorf("native screen capture failed for region %dx%d+%d+%d", width, height, x, y)
}

var (
	lastCaptureTime sync.Map   // Map of app state pointer to last capture time
	captureMutex    sync.Mutex // Global mutex to prevent concurrent captures opening multiple windows
)

// PERF instrumentation. The capture pipeline runs on a capture goroutine while
// the first-paint marker fires on the Fyne render thread, hence the atomics.
var (
	perfCaptureStartNanos int64 // UnixNano of the current capture start (0 = inactive)
	perfCaptureNum        int64 // running number of the current capture
	perfEditorOpenNanos   int64 // UnixNano when the current editor window started opening
	perfEditorClosed      int32 // 1 after the current editor window got closed
)

// perfStartCapture marks the start of a new capture→editor pipeline run.
func perfStartCapture() int64 {
	num := atomic.AddInt64(&perfCaptureNum, 1)
	atomic.StoreInt64(&perfCaptureStartNanos, time.Now().UnixNano())
	atomic.StoreInt64(&perfEditorOpenNanos, 0)
	atomic.StoreInt32(&perfEditorClosed, 0)
	return num
}

func perfCurNum() int64 { return atomic.LoadInt64(&perfCaptureNum) }

// perfAgeMs returns ms since the capture pipeline started, or -1 when inactive.
func perfAgeMs() int64 {
	start := atomic.LoadInt64(&perfCaptureStartNanos)
	if start == 0 {
		return -1
	}
	return (time.Now().UnixNano() - start) / int64(time.Millisecond)
}

// perfAge returns ms since the capture pipeline started, or "n/a"
func perfAge() string {
	if ms := perfAgeMs(); ms >= 0 {
		return fmt.Sprintf("%dms", ms)
	}
	return "n/a"
}

// perfStageMs returns ms since the given UnixNano mark, or -1 when unset.
func perfStageMs(markNanos int64) int64 {
	if markNanos == 0 {
		return -1
	}
	return (time.Now().UnixNano() - markNanos) / int64(time.Millisecond)
}

// perfWindows returns the number of live top-level windows (-1 = app not ready).
func perfWindows() int {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return -1
	}
	return len(app.Driver().AllWindows())
}

func windowTitles() []string {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return nil
	}
	var titles []string
	for _, w := range app.Driver().AllWindows() {
		if w != nil {
			titles = append(titles, w.Title())
		}
	}
	return titles
}

// perfState returns a compact runtime snapshot for leak hunting
func perfState() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("goroutines=%d heap=%dMB sys=%dMB gc=%d windows=%d",
		runtime.NumGoroutine(), m.HeapAlloc/1024/1024, m.Sys/1024/1024, m.NumGC, perfWindows())
}

// captureSelectionWithCoords captures the selected region as screenshot using provided coordinates
func (a *AppState) captureSelectionWithCoords(startX, startY, endX, endY int) {
	// Add a small cooldown and use a mutex to prevent double triggering
	captureMutex.Lock()
	defer captureMutex.Unlock()

	now := time.Now()
	if val, ok := lastCaptureTime.Load(a); ok {
		if lastTime, ok := val.(time.Time); ok {
			if now.Sub(lastTime) < 300*time.Millisecond {
				log.Printf("captureSelection: skipping duplicate call within cooldown period (300ms)")
				return
			}
		}
	}
	lastCaptureTime.Store(a, now)

	captureNum := perfStartCapture()
	t0 := time.Now()
	lastMark := t0
	mark := func(label string) {
		nowT := time.Now()
		log.Printf("PERF [capture #%d] %s: +%dms (total %dms)", captureNum, label, nowT.Sub(lastMark).Milliseconds(), nowT.Sub(t0).Milliseconds())
		lastMark = nowT
	}

	log.Printf("PERF [capture #%d] START selection=(%d,%d)-(%d,%d) state: %s", captureNum, startX, startY, endX, endY, perfState())
	log.Printf("captureSelectionWithCoords (under lock) called with: start=(%d, %d), end=(%d, %d)", startX, startY, endX, endY)

	if startX == 0 && startY == 0 && endX == 0 && endY == 0 {
		log.Printf("Warning: Selection coordinates are all zero, skipping capture")
		return
	}

	log.Printf("Selection region (before normalization): start=(%d, %d), end=(%d, %d)", startX, startY, endX, endY)

	// Calculate region
	minX := startX
	if endX < minX {
		minX = endX
	}
	minY := startY
	if endY < minY {
		minY = endY
	}
	width := startX - endX
	if width < 0 {
		width = -width
	}
	height := startY - endY
	if height < 0 {
		height = -height
	}

	// Ensure minimum size
	if width < 10 {
		width = 10
	}
	if height < 10 {
		height = 10
	}

	log.Printf("Normalized selection region: x=%d, y=%d, width=%d, height=%d", minX, minY, width, height)

	// Capture screenshot
	imageData, err := captureScreenRegion(minX, minY, width, height)
	if err != nil {
		log.Printf("Failed to capture screenshot: %v", err)
	} else {
		log.Printf("Screenshot captured successfully, size: %d bytes", len(imageData))
		mark("screen capture + png encode")
		// Close all existing editor windows before opening new one
		log.Printf("Closing all existing image editor windows")
		closeAllImageEditorWindows(a)
		mark("close old editor windows")
		// Automatically open image editor with captured image
		log.Printf("Opening image editor automatically after capture")
		openImageEditorWithAppState(imageData, a)
		mark("editor window created (Show returned)")
		// Update UI and paste in background to avoid blocking the editor window
		goSafe("update-captured-image", func() { a.updateCapturedImage(imageData) })
		mark("pipeline done")
		log.Printf("PERF [capture #%d] state: %s", captureNum, perfState())
	}
}

// captureSelection is a legacy wrapper that uses AppState coordinates
func (a *AppState) captureSelection() {
	a.mouseHookMutex.Lock()
	sX, sY := a.startX, a.startY
	eX, eY := a.lastX, a.lastY
	a.mouseHookMutex.Unlock()
	a.captureSelectionWithCoords(sX, sY, eX, eY)
}

// updateCapturedImage updates the UI with the captured image
func (a *AppState) updateCapturedImage(imageData []byte) {
	captureNum := perfCurNum()
	t0 := time.Now()
	lastMark := t0
	mark := func(label string) {
		nowT := time.Now()
		log.Printf("PERF [post #%d] %s: +%dms (total %dms)", captureNum, label, nowT.Sub(lastMark).Milliseconds(), nowT.Sub(t0).Milliseconds())
		lastMark = nowT
	}
	log.Printf("updateCapturedImage called, image size: %d bytes", len(imageData))
	a.imageData = imageData

	// Verify image can be decoded
	_, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return
	}

	log.Printf("Image decoded successfully")
	mark("verify decode")

	// Create image resource
	resource := fyne.NewStaticResource("captured.png", imageData)

	// Update UI in main thread using RunOnMainThread
	if a.imageContainer == nil {
		log.Printf("imageContainer is nil, cannot update UI")
		return
	}

	// Create canvas image from resource
	img := canvas.NewImageFromResource(resource)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(150, 100))

	// Create clickable container for the image
	clickableContainer := container.NewWithoutLayout(img)

	var lastClickTime int64
	var clickCount int
	var clickMutex sync.Mutex

	// Handle mouse events on the image
	clickableContainer.Add(img)

	// Use a custom widget that handles clicks
	imageWidget := newClickableImage(img, a.imageData, a.statusLabel, &lastClickTime, &clickCount, &clickMutex, a)

	// Update container - Fyne widgets should be thread-safe, but let's be explicit
	log.Printf("Updating image container")
	if a.imageContainer == nil {
		log.Printf("ERROR: imageContainer is nil, cannot update UI")
		return
	}

	// Update container directly - Fyne handles thread safety
	// But we'll also try to refresh the window
	a.imageContainer.RemoveAll()
	a.imageContainer.Add(imageWidget)
	a.imageContainer.Refresh()

	// Try to refresh the main window if available
	if myApp := fyne.CurrentApp(); myApp != nil {
		if windows := myApp.Driver().AllWindows(); len(windows) > 0 {
			// Refresh the window content
			windows[0].Content().Refresh()
		}
	}

	log.Printf("Image container updated successfully")
	mark("thumbnail container update")

	// Automatically copy image to clipboard and paste when it's added to UI
	log.Printf("Copying captured image to clipboard automatically")
	if err := copyImageToClipboard(imageData); err != nil {
		log.Printf("Failed to copy image to clipboard: %v", err)
		setStatusText(a.statusLabel, fmt.Sprintf("Image captured but copy failed: %v", err))
	} else {
		mark("xclip image copy")
		log.Printf("Image copied to clipboard successfully")
		setStatusText(a.statusLabel, "Image captured")

		// Broadcast image via WebSocket (base64)
		tB64 := time.Now()
		base64Image := base64.StdEncoding.EncodeToString(imageData)
		a.wsServer.Broadcast("image_update", map[string]string{
			"image":  base64Image,
			"format": "png",
		})
		log.Printf("PERF [post #%d] base64+ws broadcast: +%dms (total %dms)", captureNum, time.Since(tB64).Milliseconds(), time.Since(t0).Milliseconds())
		mark("ws broadcast")

		// Minimal delay and then simulate Ctrl+V to paste image
		time.Sleep(50 * time.Millisecond)
		log.Printf("Simulating Ctrl+V to paste image...")
		robotgo.KeyDown("control")
		time.Sleep(10 * time.Millisecond)
		robotgo.KeyTap("v")
		time.Sleep(10 * time.Millisecond)
		robotgo.KeyUp("control")
		mark("paste simulation")
		log.Printf("PERF [post #%d] total: %dms, %s", captureNum, time.Since(t0).Milliseconds(), perfState())
	}
}

// clickableImage is a custom widget that handles clicks and double-clicks on images
type clickableImage struct {
	widget.BaseWidget
	img           *canvas.Image
	imageData     []byte
	statusLabel   fyne.Widget // Can be *widget.Label or *clickableStatusLabel
	lastClickTime *int64
	clickCount    *int
	clickMutex    *sync.Mutex
	appState      *AppState // Reference to AppState for updating image
}

func newClickableImage(img *canvas.Image, imageData []byte, statusLabel fyne.Widget, lastClickTime *int64, clickCount *int, clickMutex *sync.Mutex, appState *AppState) *clickableImage {
	c := &clickableImage{
		img:           img,
		imageData:     imageData,
		statusLabel:   statusLabel,
		lastClickTime: lastClickTime,
		clickCount:    clickCount,
		clickMutex:    clickMutex,
		appState:      appState,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *clickableImage) CreateRenderer() fyne.WidgetRenderer {
	return &clickableImageRenderer{img: c.img}
}

type clickableImageRenderer struct {
	img *canvas.Image
}

func (r *clickableImageRenderer) Layout(size fyne.Size) {
	r.img.Resize(size)
	r.img.Move(fyne.NewPos(0, 0))
}

func (r *clickableImageRenderer) MinSize() fyne.Size {
	return r.img.MinSize()
}

func (r *clickableImageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.img}
}

func (r *clickableImageRenderer) Refresh() {
	r.img.Refresh()
}

func (r *clickableImageRenderer) Destroy() {
}

func (c *clickableImage) Tapped(ev *fyne.PointEvent) {
	c.clickMutex.Lock()
	defer c.clickMutex.Unlock()

	now := time.Now().UnixNano()
	timeSinceLastClick := now - *c.lastClickTime

	*c.lastClickTime = now

	if timeSinceLastClick < 500000000 { // 500ms for double click
		*c.clickCount++
		if *c.clickCount == 2 {
			*c.clickCount = 0
			// Double click - open editor window
			log.Printf("Double click detected, opening image editor")
			openImageEditorWithAppState(c.imageData, c.appState)
			return
		}
	} else {
		*c.clickCount = 1
	}

	// Single click - copy to clipboard
	log.Printf("Single click detected, copying image to clipboard")
	if err := copyImageToClipboard(c.imageData); err != nil {
		log.Printf("Failed to copy image to clipboard: %v", err)
		if c.statusLabel != nil {
			setStatusText(c.statusLabel, fmt.Sprintf("Copy failed: %v", err))
		}
	} else {
		log.Printf("Image copied to clipboard successfully")
		if c.statusLabel != nil {
			setStatusText(c.statusLabel, "Image copied to clipboard")
		}
	}
}

// Arrow represents a drawn arrow
type Arrow struct {
	StartX, StartY int
	EndX, EndY     int
}

// imageEditorCanvas is a custom canvas for drawing arrows on images
type imageEditorCanvas struct {
	widget.BaseWidget
	baseImage    image.Image
	arrows       []Arrow
	currentArrow *Arrow
	isDrawing    bool
	imageData    []byte
	imageOffsetX float32 // Offset of image in container (for centering)
	imageOffsetY float32
}

func newImageEditorCanvas(imageData []byte) (*imageEditorCanvas, error) {
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	c := &imageEditorCanvas{
		baseImage: img,
		arrows:    make([]Arrow, 0),
		imageData: imageData,
	}
	c.ExtendBaseWidget(c)
	return c, nil
}

// convertMouseToImageCoords converts mouse coordinates to image coordinates
func (c *imageEditorCanvas) convertMouseToImageCoords(mouseX, mouseY float32) (int, int) {
	// Subtract image offset to get coordinates relative to image
	imgX := int(mouseX - c.imageOffsetX)
	imgY := int(mouseY - c.imageOffsetY)

	// Clamp to image bounds
	bounds := c.baseImage.Bounds()
	if imgX < 0 {
		imgX = 0
	} else if imgX >= bounds.Dx() {
		imgX = bounds.Dx() - 1
	}
	if imgY < 0 {
		imgY = 0
	} else if imgY >= bounds.Dy() {
		imgY = bounds.Dy() - 1
	}

	return imgX, imgY
}

// MouseDown implements desktop.Mouseable
func (c *imageEditorCanvas) MouseDown(ev *desktop.MouseEvent) {
	log.Printf("MouseDown at %v (image offset: %v, %v)", ev.Position, c.imageOffsetX, c.imageOffsetY)
	imgX, imgY := c.convertMouseToImageCoords(ev.Position.X, ev.Position.Y)
	log.Printf("Converted to image coordinates: (%d, %d)", imgX, imgY)
	c.isDrawing = true
	c.currentArrow = &Arrow{
		StartX: imgX,
		StartY: imgY,
		EndX:   imgX,
		EndY:   imgY,
	}
	c.Refresh()
}

// MouseUp implements desktop.Mouseable
func (c *imageEditorCanvas) MouseUp(ev *desktop.MouseEvent) {
	if c.isDrawing && c.currentArrow != nil {
		imgX, imgY := c.convertMouseToImageCoords(ev.Position.X, ev.Position.Y)
		c.currentArrow.EndX = imgX
		c.currentArrow.EndY = imgY
		c.arrows = append(c.arrows, *c.currentArrow)
		log.Printf("Arrow drawn: start=(%d,%d), end=(%d,%d), total arrows: %d",
			c.currentArrow.StartX, c.currentArrow.StartY,
			c.currentArrow.EndX, c.currentArrow.EndY, len(c.arrows))
		c.currentArrow = nil
		c.isDrawing = false
		c.Refresh()
	}
}

// MouseDragged implements desktop.Mouseable
func (c *imageEditorCanvas) MouseDragged(ev *desktop.MouseEvent) {
	if c.isDrawing && c.currentArrow != nil {
		imgX, imgY := c.convertMouseToImageCoords(ev.Position.X, ev.Position.Y)
		c.currentArrow.EndX = imgX
		c.currentArrow.EndY = imgY
		c.Refresh()
	}
}

func (c *imageEditorCanvas) CreateRenderer() fyne.WidgetRenderer {
	// Create initial image with arrows (raster only, no PNG round-trip)
	log.Printf("Creating renderer for image editor canvas, image bounds: %v", c.baseImage.Bounds())
	tDraw := time.Now()
	raster := c.drawImageRaster()
	log.Printf("PERF [editor #%d] first draw (raster): %dms", perfCurNum(), time.Since(tDraw).Milliseconds())
	bounds := c.baseImage.Bounds()
	imgObj := canvas.NewImageFromImage(raster)
	imgObj.FillMode = canvas.ImageFillOriginal
	imgObj.SetMinSize(fyne.NewSize(float32(bounds.Dx()), float32(bounds.Dy())))

	return &imageEditorCanvasRenderer{
		canvas: c,
		imgObj: imgObj,
	}
}

// drawImageRaster composites the base image and all arrows into a fresh RGBA.
// No PNG encoding here: the interactive refresh used to PNG-encode the whole
// image (and Fyne then re-decoded it) on every mouse-drag frame, which made
// drawing on the screenshot visibly laggy.
func (c *imageEditorCanvas) drawImageRaster() *image.RGBA {
	bounds := c.baseImage.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, c.baseImage, bounds.Min, draw.Src)

	// Draw all arrows
	for _, arrow := range c.arrows {
		drawArrow(rgba, arrow.StartX, arrow.StartY, arrow.EndX, arrow.EndY)
	}

	// Draw current arrow if drawing
	if c.currentArrow != nil {
		drawArrow(rgba, c.currentArrow.StartX, c.currentArrow.StartY,
			c.currentArrow.EndX, c.currentArrow.EndY)
	}

	return rgba
}

// drawImageWithArrows returns the composited image PNG-encoded (used for saving).
func (c *imageEditorCanvas) drawImageWithArrows() []byte {
	rgba := c.drawImageRaster()

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		log.Printf("Failed to encode image: %v", err)
		return c.imageData
	}
	return buf.Bytes()
}

type imageEditorCanvasRenderer struct {
	canvas            *imageEditorCanvas
	imgObj            *canvas.Image
	firstLayoutLogged bool
}

func (r *imageEditorCanvasRenderer) Layout(size fyne.Size) {
	if !r.firstLayoutLogged {
		r.firstLayoutLogged = true
		log.Printf("PERF [editor #%d] first paint (layout on render thread): open stage %dms (capture→%s)",
			perfCurNum(), perfStageMs(atomic.LoadInt64(&perfEditorOpenNanos)), perfAge())
	}
	// Center image in container
	bounds := r.canvas.baseImage.Bounds()
	imgWidth := float32(bounds.Dx())
	imgHeight := float32(bounds.Dy())

	// Calculate offset to center image
	offsetX := (size.Width - imgWidth) / 2
	offsetY := (size.Height - imgHeight) / 2

	// Update canvas offsets
	r.canvas.imageOffsetX = offsetX
	r.canvas.imageOffsetY = offsetY

	// Set image size to original size
	r.imgObj.Resize(fyne.NewSize(imgWidth, imgHeight))
	r.imgObj.Move(fyne.NewPos(offsetX, offsetY))
}

func (r *imageEditorCanvasRenderer) MinSize() fyne.Size {
	bounds := r.canvas.baseImage.Bounds()
	return fyne.NewSize(float32(bounds.Dx()), float32(bounds.Dy()))
}

func (r *imageEditorCanvasRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.imgObj}
}

func (r *imageEditorCanvasRenderer) Refresh() {
	// Redraw image with arrows (raster only — no PNG round-trip, see drawImageRaster)
	tDraw := time.Now()
	r.imgObj.Image = r.canvas.drawImageRaster()
	r.imgObj.Refresh()
	if d := time.Since(tDraw); d >= 30*time.Millisecond {
		log.Printf("PERF [editor #%d] slow refresh (raster): %dms", perfCurNum(), d.Milliseconds())
	}
}

func (r *imageEditorCanvasRenderer) Destroy() {
}

// drawArrow draws an anti-aliased red arrow (smooth stroke + filled head) onto img.
// Instead of stamping whole pixels step by step, every pixel in the arrow's bounding
// box receives subpixel coverage from analytic distance fields, so edges stay smooth
// at any angle.
func drawArrow(img *image.RGBA, x1, y1, x2, y2 int) {
	const (
		strokeHalf = 1.5  // half of the stroke width (3px line)
		headLen    = 20.0 // arrowhead length in pixels
		headHalf   = 8.0  // half of the arrowhead base (16px wide)
	)

	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}

	ax, ay := float64(x1), float64(y1)
	tipX, tipY := float64(x2), float64(y2)

	dx, dy := tipX-ax, tipY-ay
	length := math.Hypot(dx, dy)
	if length < 2 { // ignore stray single clicks
		return
	}
	ux, uy := dx/length, dy/length // unit vector along the arrow
	px, py := -uy, ux              // unit perpendicular

	// Head: filled triangle with the tip at (x2, y2)
	baseX, baseY := tipX-ux*headLen, tipY-uy*headLen
	h1x, h1y := baseX+px*headHalf, baseY+py*headHalf
	h2x, h2y := baseX-px*headHalf, baseY-py*headHalf

	// Stroke runs from the tail into the head base (small overlap hides the seam)
	endX, endY := baseX+ux*2, baseY+uy*2

	// Bounding box of the whole arrow, with a feather margin
	minX := int(math.Floor(math.Min(math.Min(ax, tipX), math.Min(h1x, h2x)) - strokeHalf - 1))
	maxX := int(math.Ceil(math.Max(math.Max(ax, tipX), math.Max(h1x, h2x)) + strokeHalf + 1))
	minY := int(math.Floor(math.Min(math.Min(ay, tipY), math.Min(h1y, h2y)) - strokeHalf - 1))
	maxY := int(math.Ceil(math.Max(math.Max(ay, tipY), math.Max(h1y, h2y)) + strokeHalf + 1))

	bounds := img.Bounds()
	if minX < bounds.Min.X {
		minX = bounds.Min.X
	}
	if minY < bounds.Min.Y {
		minY = bounds.Min.Y
	}
	if maxX > bounds.Max.X-1 {
		maxX = bounds.Max.X - 1
	}
	if maxY > bounds.Max.Y-1 {
		maxY = bounds.Max.Y - 1
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			// Pixel center in image coordinates
			fx, fy := float64(x)+0.5, float64(y)+0.5

			// Round-capped stroke (capsule) coverage
			cov := clamp01(strokeHalf - distToSegment(fx, fy, ax, ay, endX, endY) + 0.5)

			// Filled head coverage
			if t := triangleCoverage(fx, fy, tipX, tipY, h1x, h1y, h2x, h2y); t > cov {
				cov = t
			}

			if cov > 0 {
				blendPixelCoverage(img, x, y, red, cov)
			}
		}
	}
}

// distToSegment returns the distance from point (px, py) to segment (ax, ay)-(bx, by).
func distToSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(px-(ax+dx*t), py-(ay+dy*t))
}

// triangleCoverage returns anti-aliased coverage (0..1) of the point inside triangle abc.
// Distance to the nearest edge is feathered over one pixel; sharp corners get slightly
// rounded by the same distance field.
func triangleCoverage(px, py, ax, ay, bx, by, cx, cy float64) float64 {
	d1 := edgeDistance(px, py, ax, ay, bx, by, cx, cy)
	d2 := edgeDistance(px, py, bx, by, cx, cy, ax, ay)
	d3 := edgeDistance(px, py, cx, cy, ax, ay, bx, by)
	return clamp01(math.Min(d1, math.Min(d2, d3)) + 0.5)
}

// edgeDistance returns the signed distance from point p to line a-b; positive on the
// side of the reference point c (inside the triangle by construction).
func edgeDistance(px, py, ax, ay, bx, by, cx, cy float64) float64 {
	ex, ey := bx-ax, by-ay
	length := math.Hypot(ex, ey)
	if length == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	d := (ex*(py-ay) - ey*(px-ax)) / length
	if ex*(cy-ay)-ey*(cx-ax) < 0 {
		d = -d
	}
	return d
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// blendPixelCoverage composites color c over the pixel with the given coverage.
func blendPixelCoverage(img *image.RGBA, x, y int, c color.RGBA, cov float64) {
	i := img.PixOffset(x, y)
	inv := 1 - cov
	img.Pix[i] = uint8(float64(c.R)*cov + float64(img.Pix[i])*inv + 0.5)
	img.Pix[i+1] = uint8(float64(c.G)*cov + float64(img.Pix[i+1])*inv + 0.5)
	img.Pix[i+2] = uint8(float64(c.B)*cov + float64(img.Pix[i+2])*inv + 0.5)
	img.Pix[i+3] = 255
}

// closeAllImageEditorWindows closes all open image editor windows
func closeAllImageEditorWindows(appState *AppState) {
	currentApp := fyne.CurrentApp()
	if currentApp == nil {
		return
	}
	windowsBefore := len(currentApp.Driver().AllWindows())

	// First, close the window stored in AppState if it exists
	if appState != nil && appState.imageEditorWindow != nil {
		log.Printf("Closing image editor window from AppState")
		windowToClose := appState.imageEditorWindow
		appState.imageEditorWindow = nil
		// Clear CloseIntercept to avoid recursion and issues
		windowToClose.SetCloseIntercept(nil)
		windowToClose.Close()
	}

	// Close all windows with title "Editor" (editor windows)
	// Get all windows and close those that are editor windows
	allWindows := currentApp.Driver().AllWindows()
	for _, window := range allWindows {
		if window != nil && window.Title() == "Editor" {
			log.Printf("Found image editor window to close: %s", window.Title())
			// Clear CloseIntercept to avoid recursion
			window.SetCloseIntercept(nil)
			window.Close()
		}
	}

	// Final check: clear AppState reference if it still points to something
	if appState != nil && appState.imageEditorWindow != nil {
		// Check if window is still in the list of open windows
		stillOpen := false
		for _, window := range currentApp.Driver().AllWindows() {
			if window == appState.imageEditorWindow {
				stillOpen = true
				break
			}
		}
		if !stillOpen {
			log.Printf("Clearing stale reference to closed editor window")
			appState.imageEditorWindow = nil
		}
	}

	windowsAfter := len(currentApp.Driver().AllWindows())
	if windowsAfter != windowsBefore {
		log.Printf("Editor windows: %d before close → %d after (open now: %v)", windowsBefore, windowsAfter, windowTitles())
	}
}

// openImageEditor opens a new window with image editor
func openImageEditor(imageData []byte) {
	openImageEditorWithAppState(imageData, nil)
}

// openImageEditorWithAppState opens a new window with image editor and saves to AppState
func openImageEditorWithAppState(imageData []byte, appState *AppState) {
	captureNum := perfCurNum()
	editorOpenNanos := time.Now().UnixNano()
	atomic.StoreInt64(&perfEditorOpenNanos, editorOpenNanos)
	atomic.StoreInt32(&perfEditorClosed, 0)
	t0 := time.Now()
	log.Printf("PERF [editor #%d] open: %d bytes, %s", captureNum, len(imageData), perfState())
	// Use existing app instead of creating new one
	currentApp := fyne.CurrentApp()
	if currentApp == nil {
		log.Printf("ERROR: No current Fyne app available, cannot open editor")
		return
	}

	log.Printf("Creating new editor window")
	editorWindow := currentApp.NewWindow("Editor")
	log.Printf("PERF [editor #%d] NewWindow: %dms", captureNum, time.Since(t0).Milliseconds())

	// Store reference to editor window in AppState if provided
	if appState != nil {
		appState.imageEditorWindow = editorWindow
	}

	canvasWidget, err := newImageEditorCanvas(imageData)
	if err != nil {
		log.Printf("Failed to create image editor canvas: %v", err)
		return
	}
	log.Printf("PERF [editor #%d] decode image + canvas: %dms", captureNum, time.Since(t0).Milliseconds())

	// Get image bounds
	bounds := canvasWidget.baseImage.Bounds()
	imgWidth := float32(bounds.Dx())
	imgHeight := float32(bounds.Dy())

	// Window size: image size + padding, but at least 400x300
	windowWidth := imgWidth + 100
	windowHeight := imgHeight + 100
	if windowWidth < 400 {
		windowWidth = 400
	}
	if windowHeight < 300 {
		windowHeight = 300
	}

	// Limit window size to screen
	if windowWidth > 1920 {
		windowWidth = 1920
	}
	if windowHeight > 1080 {
		windowHeight = 1080
	}

	editorWindow.Resize(fyne.NewSize(windowWidth, windowHeight))
	editorWindow.CenterOnScreen()

	// Create container that centers the canvas (no scroll, image stays original size)
	canvasContainer := container.NewMax(canvasWidget)

	// Create recording indicator (red circle)
	indicatorCircle := canvas.NewCircle(color.RGBA{R: 255, G: 0, B: 0, A: 255})
	indicatorCircle.StrokeWidth = 0

	// Wrap indicator in a container that will be positioned manually
	recordingIndicator := container.NewGridWrap(fyne.NewSize(30, 30), indicatorCircle)
	recordingIndicator.Hide()

	// Store reference in AppState so main recording logic can toggle it
	if appState != nil {
		appState.recordingIndicator = recordingIndicator
	}

	// Use a container with no layout for the indicator to position it at the very top
	indicatorOverlay := container.NewWithoutLayout(recordingIndicator)

	// Wrap everything in a Stack to overlay the indicator on top of the image
	mainStack := container.NewStack(canvasContainer, indicatorOverlay)
	editorWindow.SetContent(mainStack)
	log.Printf("PERF [editor #%d] SetContent: %dms", captureNum, time.Since(t0).Milliseconds())

	// Wait (cheaply) until the window is actually mapped, then position the
	// recording indicator. The previous version polled every 100ms and forced a
	// full-stack refresh 20 times over 2s; each forced refresh re-encoded the
	// image to PNG and re-uploaded its texture — exactly the "editor feels slow"
	// symptom. Refreshes are cheap now, but we still only do one.
	goSafe("editor-open-monitor", func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if atomic.LoadInt32(&perfEditorClosed) == 1 {
				log.Printf("PERF [editor #%d] closed before it became visible", captureNum)
				return
			}
			size := editorWindow.Canvas().Size()
			if size.Width > 0 && size.Height > 0 {
				captureToVisible := perfAgeMs()
				openToVisible := time.Since(t0).Milliseconds()
				log.Printf("PERF [editor #%d] WINDOW VISIBLE: open=%dms (capture→visible=%s) state: %s",
					captureNum, openToVisible, perfAge(), perfState())
				if captureToVisible > 800 {
					log.Printf("⚠ SLOW [editor #%d] capture→visible took %dms", captureNum, captureToVisible)
				}
				if captureToVisible > 1500 {
					crashf("SLOW editor open: capture→visible=%dms (window stage=%dms, open→visible=%dms, %s)",
						captureToVisible, perfStageMs(editorOpenNanos), openToVisible, perfState())
				}
				posX := (size.Width - 30) / 2
				recordingIndicator.Move(fyne.NewPos(posX, 10))
				recordingIndicator.Refresh()
				editorWindow.Canvas().Refresh(mainStack) // single refresh: indicator position
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		log.Printf("⚠ SLOW [editor #%d] window STILL not visible after 5s (%s)", captureNum, perfState())
		crashf("editor window did not become visible within 5s — window mapping/render stall (%s)", perfState())
	})

	// Add Escape key handler to close window without saving
	// Add W key handler to close window and save image
	// Add Q key handler to toggle recording
	editorWindow.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		log.Printf("Image Editor: Key pressed: %v", event.Name)
		if event.Name == fyne.KeyEscape {
			atomic.StoreInt32(&perfEditorClosed, 1)
			log.Printf("Escape pressed in image editor, closing window without saving")
			// If recording, stop it
			if appState != nil && appState.isRecording {
				appState.StopRecording()
			}
			// Clear reference when closing
			if appState != nil {
				appState.imageEditorWindow = nil
				appState.recordingIndicator = nil // Also clear indicator reference
			}
			// Close window without saving
			editorWindow.Close()
		} else if event.Name == fyne.KeyQ {
			log.Printf("Q key detected in image editor, toggling recording")
			if appState != nil {
				if !appState.isRecording {
					log.Printf("Starting recording from editor")
					appState.recordingMode = "start" // Match "Start" button behavior
					err := appState.StartRecording()
					if err != nil {
						log.Printf("Failed to start recording from editor: %v", err)
					}
					// Note: StartRecording handles showing the indicator
				} else {
					log.Printf("Stopping recording from editor")
					appState.StopRecording()
					// Note: StopRecording handles hiding the indicator
				}
			}
		} else if event.Name == fyne.KeyW {
			atomic.StoreInt32(&perfEditorClosed, 1)
			log.Printf("W key detected in image editor, closing window and saving image")

			// If recording, stop it and process
			if appState != nil && appState.isRecording {
				log.Printf("Stopping active recording before closing editor")
				appState.StopRecording()
			}

			// Get final image with all arrows
			finalImageData := canvasWidget.drawImageWithArrows()

			// Update main UI if AppState is provided
			if appState != nil {
				appState.updateCapturedImage(finalImageData)
				appState.imageEditorWindow = nil // Clear reference when closing
			}

			// Copy to clipboard
			if err := copyImageToClipboard(finalImageData); err != nil {
				log.Printf("Failed to copy edited image to clipboard: %v", err)
			} else {
				log.Printf("Edited image copied to clipboard")
			}

			// Close window
			editorWindow.Close()
		} else {
			log.Printf("Other key pressed in editor: %v", event.Name)
		}
	})

	// Clear reference when window is closed (for Escape key or window close button)
	editorWindow.SetCloseIntercept(func() {
		atomic.StoreInt32(&perfEditorClosed, 1)
		// If recording, stop it
		if appState != nil && appState.isRecording {
			log.Printf("Closing editor: stopping active recording")
			appState.StopRecording()
		}
		if appState != nil {
			appState.imageEditorWindow = nil
			appState.recordingIndicator = nil // Also clear indicator reference
		}
		editorWindow.Close()
	})

	// The canvas widget implements desktop.Mouseable interface
	// Fyne will automatically call MouseDown, MouseUp, MouseDragged methods

	tShow := time.Now()
	editorWindow.Show()
	log.Printf("PERF [editor #%d] Show() returned: %dms (open so far %dms, capture→%s)",
		captureNum, time.Since(tShow).Milliseconds(), time.Since(t0).Milliseconds(), perfAge())
	// Don't call Run() - the main app is already running
}
