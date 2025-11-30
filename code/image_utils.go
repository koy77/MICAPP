package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os/exec"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/go-vgo/robotgo"
)

// copyImageToClipboard copies image to clipboard using xclip
func copyImageToClipboard(imageData []byte) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write(imageData); err != nil {
		return err
	}

	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// captureScreenRegion captures a region of the screen.
// It first takes a full-screen screenshot and then crops the desired region.
func captureScreenRegion(x, y, width, height int) ([]byte, error) {
	log.Printf("captureScreenRegion called with x=%d, y=%d, width=%d, height=%d", x, y, width, height)

	// Capture full screen
	screenBitmap := robotgo.CaptureScreen()
	if screenBitmap == nil {
		return nil, fmt.Errorf("failed to capture full screen")
	}
	defer robotgo.FreeBitmap(screenBitmap)

	fullImg := robotgo.ToImage(screenBitmap)
	if fullImg == nil {
		return nil, fmt.Errorf("failed to convert screen bitmap to image")
	}

	bounds := fullImg.Bounds()

	// Clamp requested region to screen bounds
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	if x+width > bounds.Max.X {
		width = bounds.Max.X - x
	}
	if y+height > bounds.Max.Y {
		height = bounds.Max.Y - y
	}

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid cropped region after clamping to screen bounds")
	}

	region := image.Rect(x, y, x+width, y+height)

	subImager, ok := fullImg.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("image does not support SubImage")
	}

	cropped := subImager.SubImage(region)

	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil, fmt.Errorf("failed to encode cropped image as PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// captureSelection captures the selected region as screenshot
func (a *AppState) captureSelection() {
	log.Printf("captureSelection called")
	a.mouseHookMutex.Lock()
	startX := a.startX
	startY := a.startY
	endX := a.lastX
	endY := a.lastY
	a.mouseHookMutex.Unlock()

	log.Printf("Selection coordinates: start=(%d, %d), end=(%d, %d)", startX, startY, endX, endY)

	if startX == 0 && startY == 0 && endX == 0 && endY == 0 {
		log.Printf("Warning: Selection coordinates are all zero, skipping capture")
		return
	}

	// If end coordinates are zero, use current mouse position
	if endX == 0 && endY == 0 {
		endX, endY = robotgo.GetMousePos()
		log.Printf("End coordinates were zero, using current mouse position: (%d, %d)", endX, endY)
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

	// Capture screenshot using full-screen capture + crop
	imageData, err := captureScreenRegion(minX, minY, width, height)
	if err != nil {
		log.Printf("Failed to capture screenshot: %v", err)
	} else {
		log.Printf("Screenshot captured successfully, size: %d bytes", len(imageData))
		// Update UI with captured image
		a.updateCapturedImage(imageData)
		// Close all existing editor windows before opening new one
		log.Printf("Closing all existing image editor windows")
		closeAllImageEditorWindows(a)
		// Automatically open image editor with captured image
		log.Printf("Opening image editor automatically after CTRL+SHIFT capture")
		openImageEditorWithAppState(imageData, a)
	}
}

// updateCapturedImage updates the UI with the captured image
func (a *AppState) updateCapturedImage(imageData []byte) {
	log.Printf("updateCapturedImage called, image size: %d bytes", len(imageData))
	a.imageData = imageData

	// Verify image can be decoded
	_, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return
	}

	log.Printf("Image decoded successfully")

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

	// Automatically copy image to clipboard when it's added to UI
	log.Printf("Copying captured image to clipboard automatically")
	if err := copyImageToClipboard(imageData); err != nil {
		log.Printf("Failed to copy image to clipboard: %v", err)
		setStatusText(a.statusLabel, fmt.Sprintf("Image captured but copy failed: %v", err))
	} else {
		log.Printf("Image copied to clipboard successfully")
		setStatusText(a.statusLabel, "Image captured")
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
	// Create initial image with arrows
	log.Printf("Creating renderer for image editor canvas, image bounds: %v", c.baseImage.Bounds())
	imgData := c.drawImageWithArrows()
	log.Printf("Image data size: %d bytes", len(imgData))
	resource := fyne.NewStaticResource("canvas.png", imgData)
	imgObj := canvas.NewImageFromResource(resource)
	imgObj.FillMode = canvas.ImageFillOriginal
	bounds := c.baseImage.Bounds()
	imgObj.SetMinSize(fyne.NewSize(float32(bounds.Dx()), float32(bounds.Dy())))

	return &imageEditorCanvasRenderer{
		canvas: c,
		imgObj: imgObj,
	}
}

func (c *imageEditorCanvas) drawImageWithArrows() []byte {
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

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		log.Printf("Failed to encode image: %v", err)
		return c.imageData
	}
	return buf.Bytes()
}

type imageEditorCanvasRenderer struct {
	canvas *imageEditorCanvas
	imgObj *canvas.Image
}

func (r *imageEditorCanvasRenderer) Layout(size fyne.Size) {
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
	// Redraw image with arrows
	imgData := r.canvas.drawImageWithArrows()
	resource := fyne.NewStaticResource("canvas.png", imgData)
	r.imgObj.Resource = resource
	r.imgObj.Refresh()
}

func (r *imageEditorCanvasRenderer) Destroy() {
}

func drawArrow(img *image.RGBA, x1, y1, x2, y2 int) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}

	// Draw line
	drawLine(img, x1, y1, x2, y2, red, 2)

	// Draw arrowhead
	drawArrowhead(img, x1, y1, x2, y2, red)
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, c color.Color, width int) {
	dx := x2 - x1
	dy := y2 - y1
	steps := int(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy))))

	if steps == 0 {
		return
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(float64(x1) + float64(dx)*t)
		y := int(float64(y1) + float64(dy)*t)

		// Draw with width
		for wx := -width / 2; wx <= width/2; wx++ {
			for wy := -width / 2; wy <= width/2; wy++ {
				if x+wx >= 0 && x+wx < img.Bounds().Dx() && y+wy >= 0 && y+wy < img.Bounds().Dy() {
					img.Set(x+wx, y+wy, c)
				}
			}
		}
	}
}

func drawArrowhead(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	// Calculate angle
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	angle := math.Atan2(dy, dx)

	// Arrowhead size
	size := 15.0

	// Calculate arrowhead points
	arrowAngle := math.Pi / 6 // 30 degrees

	// Point 1
	px1 := x2 - int(size*math.Cos(angle-arrowAngle))
	py1 := y2 - int(size*math.Sin(angle-arrowAngle))

	// Point 2
	px2 := x2 - int(size*math.Cos(angle+arrowAngle))
	py2 := y2 - int(size*math.Sin(angle+arrowAngle))

	// Draw arrowhead triangle
	drawLine(img, x2, y2, px1, py1, c, 2)
	drawLine(img, x2, y2, px2, py2, c, 2)
	drawLine(img, px1, py1, px2, py2, c, 2)
}

// closeAllImageEditorWindows closes all open image editor windows
func closeAllImageEditorWindows(appState *AppState) {
	currentApp := fyne.CurrentApp()
	if currentApp == nil {
		return
	}

	// First, close the window stored in AppState if it exists
	if appState != nil && appState.imageEditorWindow != nil {
		log.Printf("Closing image editor window from AppState")
		windowToClose := appState.imageEditorWindow
		appState.imageEditorWindow = nil
		// Clear CloseIntercept to avoid recursion and issues
		windowToClose.SetCloseIntercept(nil)
		windowToClose.Close()
		// Small delay to ensure window is closed
		time.Sleep(50 * time.Millisecond)
	}

	// Close all windows with title "Editor" (editor windows)
	// Get all windows and close those that are editor windows
	allWindows := currentApp.Driver().AllWindows()
	editorWindowsToClose := make([]fyne.Window, 0)

	for _, window := range allWindows {
		if window != nil && window.Title() == "Editor" {
			log.Printf("Found image editor window to close: %s", window.Title())
			editorWindowsToClose = append(editorWindowsToClose, window)
		}
	}

	// Close all found editor windows
	for _, window := range editorWindowsToClose {
		log.Printf("Closing image editor window: %s", window.Title())
		// Clear CloseIntercept to avoid recursion
		window.SetCloseIntercept(nil)
		window.Close()
		// Small delay between closing windows
		time.Sleep(50 * time.Millisecond)
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
}

// openImageEditor opens a new window with image editor
func openImageEditor(imageData []byte) {
	openImageEditorWithAppState(imageData, nil)
}

// openImageEditorWithAppState opens a new window with image editor and saves to AppState
func openImageEditorWithAppState(imageData []byte, appState *AppState) {
	// Use existing app instead of creating new one
	currentApp := fyne.CurrentApp()
	if currentApp == nil {
		log.Printf("No current Fyne app available")
		return
	}

	editorWindow := currentApp.NewWindow("Editor")

	// Store reference to editor window in AppState if provided
	if appState != nil {
		appState.imageEditorWindow = editorWindow
	}

	canvasWidget, err := newImageEditorCanvas(imageData)
	if err != nil {
		log.Printf("Failed to create image editor canvas: %v", err)
		return
	}

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
	// Use Max container to fill window, canvas will center itself in Layout
	canvasContainer := container.NewMax(canvasWidget)

	editorWindow.SetContent(canvasContainer)

	// Add Escape key handler to close window without saving
	// Add W key handler to close window and save image
	editorWindow.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		if event.Name == fyne.KeyEscape {
			log.Printf("Escape pressed in image editor, closing window without saving")
			// Clear reference when closing
			if appState != nil {
				appState.imageEditorWindow = nil
			}
			// Close window without saving
			editorWindow.Close()
		} else if event.Name == fyne.KeyW {
			log.Printf("W pressed in image editor, closing window and saving image")

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
		}
	})

	// Clear reference when window is closed (for Escape key or window close button)
	editorWindow.SetCloseIntercept(func() {
		if appState != nil {
			appState.imageEditorWindow = nil
		}
		editorWindow.Close()
	})

	// The canvas widget implements desktop.Mouseable interface
	// Fyne will automatically call MouseDown, MouseUp, MouseDragged methods

	editorWindow.Show()
	// Don't call Run() - the main app is already running
}
