# Complete Fyne.io Manual

## Table of Contents
1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Basic Application Structure](#basic-application-structure)
4. [Widgets](#widgets)
5. [Layouts](#layouts)
6. [Windows and Sizing](#windows-and-sizing)
7. [Data Binding](#data-binding)
8. [Themes and Styling](#themes-and-styling)
9. [Advanced Topics](#advanced-topics)

---

## Introduction

Fyne is a modern, cross-platform GUI toolkit for Go that makes it easy to build beautiful applications for desktop and mobile. It uses OpenGL for rendering and provides a native look and feel across all platforms.

**Key Features:**
- Cross-platform (Windows, macOS, Linux, iOS, Android, Web)
- Material Design inspired
- Easy to use API
- Built-in themes
- Responsive layouts
- No platform-specific dependencies

---

## Installation

### Prerequisites
- Go 1.17 or later
- GCC compiler (for CGO)

### Platform-Specific Requirements

**Windows:**
```bash
# Install TDM-GCC or MinGW-w64
```

**macOS:**
```bash
xcode-select --install
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev
```

### Install Fyne
```bash
go get fyne.io/fyne/v2
go install fyne.io/fyne/v2/cmd/fyne@latest
```

---

## Basic Application Structure

### Minimal Application
```go
package main

import (
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myApp := app.New()
    myWindow := myApp.NewWindow("Hello")
    
    myWindow.SetContent(widget.NewLabel("Hello Fyne!"))
    myWindow.ShowAndRun()
}
```

### Application Lifecycle
```go
package main

import (
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myApp := app.New()
    myWindow := myApp.NewWindow("Lifecycle Demo")
    
    // Set up UI
    myWindow.SetContent(widget.NewLabel("Application Running"))
    
    // Set close intercept
    myWindow.SetCloseIntercept(func() {
        // Cleanup before closing
        myWindow.Close()
    })
    
    // Set master window (app quits when this closes)
    myWindow.SetMaster()
    
    myWindow.ShowAndRun()
}
```

---

## Widgets

### Basic Widgets

**Label**
```go
label := widget.NewLabel("Simple Text")
boldLabel := widget.NewLabelWithStyle("Bold Text", 
    fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
```

**Button**
```go
button := widget.NewButton("Click Me", func() {
    println("Button clicked!")
})

// Button with icon
btnWithIcon := widget.NewButtonWithIcon("Save", 
    theme.DocumentSaveIcon(), func() {
    println("Saving...")
})
```

**Entry (Text Input)**
```go
entry := widget.NewEntry()
entry.SetPlaceHolder("Enter text...")
entry.OnChanged = func(content string) {
    println("Text changed:", content)
}

// Multiline entry
multiline := widget.NewMultiLineEntry()
multiline.SetPlaceHolder("Enter multiple lines...")

// Password entry
password := widget.NewPasswordEntry()
```

**Check and Radio**
```go
check := widget.NewCheck("Enable Feature", func(checked bool) {
    println("Checked:", checked)
})

radio := widget.NewRadioGroup([]string{"Option 1", "Option 2", "Option 3"}, 
    func(selected string) {
        println("Selected:", selected)
    })
```

**Select (Dropdown)**
```go
selector := widget.NewSelect([]string{"Red", "Green", "Blue"}, 
    func(selected string) {
        println("Selected color:", selected)
    })
selector.SetSelected("Red")
```

**ProgressBar**
```go
progress := widget.NewProgressBar()
progress.SetValue(0.5) // 50%

infiniteProgress := widget.NewProgressBarInfinite()
```

**Slider**
```go
slider := widget.NewSlider(0, 100)
slider.Value = 50
slider.OnChanged = func(value float64) {
    println("Slider value:", value)
}
```

### Container Widgets

**List**
```go
data := []string{"Item 1", "Item 2", "Item 3", "Item 4"}

list := widget.NewList(
    func() int {
        return len(data)
    },
    func() fyne.CanvasObject {
        return widget.NewLabel("template")
    },
    func(id widget.ListItemID, obj fyne.CanvasObject) {
        obj.(*widget.Label).SetText(data[id])
    },
)
```

**Table**
```go
table := widget.NewTable(
    func() (int, int) { return 5, 3 }, // rows, cols
    func() fyne.CanvasObject {
        return widget.NewLabel("Cell")
    },
    func(id widget.TableCellID, obj fyne.CanvasObject) {
        label := obj.(*widget.Label)
        label.SetText(fmt.Sprintf("Cell %d,%d", id.Row, id.Col))
    },
)
```

**Tree**
```go
data := map[string][]string{
    "":  {"Root"},
    "Root": {"Child 1", "Child 2"},
}

tree := widget.NewTree(
    func(uid string) []string {
        return data[uid]
    },
    func(uid string) bool {
        children, ok := data[uid]
        return ok && len(children) > 0
    },
    func(branch bool) fyne.CanvasObject {
        return widget.NewLabel("Branch template")
    },
    func(uid string, branch bool, obj fyne.CanvasObject) {
        obj.(*widget.Label).SetText(uid)
    },
)
```

**Accordion**
```go
accordion := widget.NewAccordion(
    widget.NewAccordionItem("Section 1", 
        widget.NewLabel("Content 1")),
    widget.NewAccordionItem("Section 2", 
        widget.NewLabel("Content 2")),
)
```

**TabContainer**
```go
tabs := container.NewAppTabs(
    container.NewTabItem("Tab 1", widget.NewLabel("Content 1")),
    container.NewTabItem("Tab 2", widget.NewLabel("Content 2")),
)
tabs.SetTabLocation(container.TabLocationTop)
```

---

## Layouts

### Basic Layouts

**VBox (Vertical Box)**
```go
content := container.NewVBox(
    widget.NewLabel("Top"),
    widget.NewLabel("Middle"),
    widget.NewLabel("Bottom"),
)
```

**HBox (Horizontal Box)**
```go
content := container.NewHBox(
    widget.NewLabel("Left"),
    widget.NewLabel("Center"),
    widget.NewLabel("Right"),
)
```

**Border Layout**
```go
content := container.NewBorder(
    widget.NewLabel("Top"),    // top
    widget.NewLabel("Bottom"), // bottom
    widget.NewLabel("Left"),   // left
    widget.NewLabel("Right"),  // right
    widget.NewLabel("Center"), // center
)
```

**Grid Layout**
```go
// Fixed columns
grid := container.NewGridWithColumns(3,
    widget.NewLabel("1"),
    widget.NewLabel("2"),
    widget.NewLabel("3"),
    widget.NewLabel("4"),
    widget.NewLabel("5"),
    widget.NewLabel("6"),
)

// Fixed rows
grid := container.NewGridWithRows(2,
    widget.NewLabel("1"),
    widget.NewLabel("2"),
)
```

**Form Layout**
```go
form := widget.NewForm(
    widget.NewFormItem("Name", widget.NewEntry()),
    widget.NewFormItem("Email", widget.NewEntry()),
    widget.NewFormItem("Password", widget.NewPasswordEntry()),
)
form.OnSubmit = func() {
    println("Form submitted")
}
form.OnCancel = func() {
    println("Form cancelled")
}
```

**Center Layout**
```go
content := container.NewCenter(
    widget.NewLabel("Centered"),
)
```

**Max Layout**
```go
// All objects fill the container (stacked)
content := container.NewMax(
    widget.NewLabel("Background"),
    widget.NewButton("Foreground", func() {}),
)
```

### Advanced Layouts

**Custom Layout**
```go
type myLayout struct{}

func (m *myLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
    for _, obj := range objects {
        obj.Resize(size)
        obj.Move(fyne.NewPos(0, 0))
    }
}

func (m *myLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
    return fyne.NewSize(100, 100)
}

// Usage
customContainer := container.New(&myLayout{}, 
    widget.NewLabel("Custom Layout"))
```

**Split Container**
```go
// Horizontal split
split := container.NewHSplit(
    widget.NewLabel("Left"),
    widget.NewLabel("Right"),
)
split.Offset = 0.3 // 30% left, 70% right

// Vertical split
vsplit := container.NewVSplit(
    widget.NewLabel("Top"),
    widget.NewLabel("Bottom"),
)
```

**Scroll Container**
```go
content := container.NewVBox()
for i := 0; i < 50; i++ {
    content.Add(widget.NewLabel(fmt.Sprintf("Item %d", i)))
}

scroll := container.NewScroll(content)
scroll.SetMinSize(fyne.NewSize(200, 400))
```

---

## Windows and Sizing

### Window Management

**Creating Windows**
```go
mainWindow := myApp.NewWindow("Main Window")
secondWindow := myApp.NewWindow("Second Window")

// Show multiple windows
mainWindow.Show()
secondWindow.Show()
```

**Window Size**
```go
// Set fixed size
window.Resize(fyne.NewSize(800, 600))

// Set minimum size
window.SetFixedSize(false)
window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {})

// Fullscreen
window.SetFullScreen(true)

// Get size
currentSize := window.Canvas().Size()
```

**Window Position**
```go
// Center on screen
window.CenterOnScreen()

// Set position
window.SetFixedSize(false)
// Note: Direct positioning is platform-dependent
```

**Widget Sizing**

**For your Text Editor height question:**
```go
// Method 1: Use VBox with layout.NewMaxSize()
textEditor := widget.NewMultiLineEntry()
content := container.NewVBox(
    // Header elements
    container.NewHBox(
        widget.NewLabel("Language:"),
        languageSelect,
        startBtn,
        correctBtn,
    ),
    // Text editor takes remaining space
    container.NewMax(textEditor), // This makes it fill available space
)

// Method 2: Use Border layout
textEditor := widget.NewMultiLineEntry()
content := container.NewBorder(
    // Top: your controls
    container.NewHBox(startBtn, correctBtn),
    nil,  // bottom
    nil,  // left
    nil,  // right
    // Center: text editor fills remaining space
    textEditor,
)

// Method 3: Use VSplit
textEditor := widget.NewMultiLineEntry()
content := container.NewVSplit(
    // Top: controls (small portion)
    container.NewHBox(startBtn, correctBtn),
    // Bottom: editor (large portion)
    textEditor,
)
content.Offset = 0.1 // 10% for controls, 90% for editor

// Method 4: For TabContainer specifically
tabs := container.NewAppTabs(
    container.NewTabItem("Text Editor", 
        container.NewMax(widget.NewMultiLineEntry())),
    container.NewTabItem("Audio Files", 
        widget.NewLabel("Audio content")),
)
// The NewMax makes the editor expand to fill the tab
```

---

## Data Binding

Data binding allows automatic UI updates when data changes.

### Basic Binding
```go
// String binding
str := binding.NewString()
str.Set("Initial value")

label := widget.NewLabelWithData(str)
entry := widget.NewEntryWithData(str)

// Changing either updates both
str.Set("New value")
```

### Binding Types
```go
// String
stringData := binding.NewString()

// Bool
boolData := binding.NewBool()
check := widget.NewCheckWithData("Enable", boolData)

// Float
floatData := binding.NewFloat()
slider := widget.NewSliderWithData(0, 100, floatData)

// Int
intData := binding.NewInt()

// List
listData := binding.NewStringList()
listData.Append("Item 1")
```

### Listeners
```go
str := binding.NewString()
str.AddListener(binding.NewDataListener(func() {
    val, _ := str.Get()
    println("Value changed to:", val)
}))
```

---

## Themes and Styling

### Built-in Themes
```go
// Dark theme (default)
myApp.Settings().SetTheme(theme.DarkTheme())

// Light theme
myApp.Settings().SetTheme(theme.LightTheme())
```

### Custom Theme
```go
type myTheme struct{}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
    if name == theme.ColorNameBackground {
        return color.RGBA{R: 30, G: 30, B: 30, A: 255}
    }
    return theme.DefaultTheme().Color(name, variant)
}

func (m myTheme) Font(style fyne.TextStyle) fyne.Resource {
    return theme.DefaultTheme().Font(style)
}

func (m myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
    return theme.DefaultTheme().Icon(name)
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
    return theme.DefaultTheme().Size(name)
}

// Apply custom theme
myApp.Settings().SetTheme(&myTheme{})
```

### Canvas Objects (Custom Drawing)

**Rectangle**
```go
rect := canvas.NewRectangle(color.RGBA{R: 255, G: 0, B: 0, A: 255})
rect.Resize(fyne.NewSize(100, 100))
```

**Circle**
```go
circle := canvas.NewCircle(color.RGBA{R: 0, G: 255, B: 0, A: 255})
circle.Resize(fyne.NewSize(100, 100))
```

**Line**
```go
line := canvas.NewLine(color.Black)
line.StrokeWidth = 2
```

**Text**
```go
text := canvas.NewText("Custom Text", color.White)
text.TextSize = 24
text.Alignment = fyne.TextAlignCenter
```

**Image**
```go
img := canvas.NewImageFromFile("path/to/image.png")
img.FillMode = canvas.ImageFillContain
```

---

## Advanced Topics

### Dialogs

**Information Dialog**
```go
dialog.ShowInformation("Title", "Message content", window)
```

**Confirmation Dialog**
```go
dialog.ShowConfirm("Confirm", "Are you sure?", func(confirmed bool) {
    if confirmed {
        println("User confirmed")
    }
}, window)
```

**Custom Dialog**
```go
content := widget.NewLabel("Custom dialog content")
customDialog := dialog.NewCustom("Title", "Close", content, window)
customDialog.Show()
```

**File Dialog**
```go
dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
    if err != nil {
        dialog.ShowError(err, window)
        return
    }
    if reader == nil {
        return
    }
    defer reader.Close()
    
    data, _ := io.ReadAll(reader)
    println(string(data))
}, window)
```

### Menus

**Main Menu**
```go
menu := fyne.NewMainMenu(
    fyne.NewMenu("File",
        fyne.NewMenuItem("New", func() { println("New") }),
        fyne.NewMenuItem("Open", func() { println("Open") }),
        fyne.NewMenuItemSeparator(),
        fyne.NewMenuItem("Quit", func() { myApp.Quit() }),
    ),
    fyne.NewMenu("Edit",
        fyne.NewMenuItem("Cut", func() { println("Cut") }),
        fyne.NewMenuItem("Copy", func() { println("Copy") }),
        fyne.NewMenuItem("Paste", func() { println("Paste") }),
    ),
)
window.SetMainMenu(menu)
```

**Context Menu**
```go
label := widget.NewLabel("Right click me")
menu := fyne.NewMenu("",
    fyne.NewMenuItem("Option 1", func() { println("Option 1") }),
    fyne.NewMenuItem("Option 2", func() { println("Option 2") }),
)

// Show on right-click
label.SetOnTapped(func() {
    // Regular click
})
```

### System Tray

```go
if desk, ok := myApp.(desktop.App); ok {
    menu := fyne.NewMenu("MyApp",
        fyne.NewMenuItem("Show", func() {
            window.Show()
        }),
    )
    desk.SetSystemTrayMenu(menu)
}
```

### Notifications

```go
myApp.SendNotification(&fyne.Notification{
    Title:   "Notification Title",
    Content: "Notification content message",
})
```

### Keyboard Shortcuts

```go
// Add shortcut to window
window.Canvas().AddShortcut(&desktop.CustomShortcut{
    KeyName:  fyne.KeyS,
    Modifier: fyne.KeyModifierControl,
}, func(shortcut fyne.Shortcut) {
    println("Ctrl+S pressed")
})
```

### Preferences

```go
// Save preferences
myApp.Preferences().SetString("username", "john")
myApp.Preferences().SetInt("count", 42)
myApp.Preferences().SetBool("enabled", true)

// Load preferences
username := myApp.Preferences().String("username")
count := myApp.Preferences().Int("count")
enabled := myApp.Preferences().Bool("enabled")
```

### Testing

```go
import (
    "testing"
    "fyne.io/fyne/v2/test"
)

func TestButton(t *testing.T) {
    app := test.NewApp()
    defer app.Quit()
    
    clicked := false
    button := widget.NewButton("Test", func() {
        clicked = true
    })
    
    test.Tap(button)
    
    if !clicked {
        t.Error("Button was not clicked")
    }
}
```

### Packaging

**Command Line Tool**
```bash
# Package for current OS
fyne package -icon myicon.png

# Package for specific OS
fyne package -os windows -icon myicon.png
fyne package -os darwin -icon myicon.png
fyne package -os linux -icon myicon.png

# Mobile
fyne package -os android -appID com.example.myapp
fyne package -os ios -appID com.example.myapp
```

---

## Complete Example Application

```go
package main

import (
    "fmt"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myApp := app.New()
    myWindow := myApp.NewWindow("Complete Example")
    
    // Create widgets
    nameEntry := widget.NewEntry()
    nameEntry.SetPlaceHolder("Enter your name...")
    
    counter := 0
    counterLabel := widget.NewLabel(fmt.Sprintf("Count: %d", counter))
    
    incrementBtn := widget.NewButton("Increment", func() {
        counter++
        counterLabel.SetText(fmt.Sprintf("Count: %d", counter))
    })
    
    multiline := widget.NewMultiLineEntry()
    multiline.SetPlaceHolder("Enter multiple lines...")
    
    // Create layout
    content := container.NewBorder(
        // Top
        container.NewVBox(
            widget.NewLabel("Name:"),
            nameEntry,
            container.NewHBox(incrementBtn, counterLabel),
        ),
        // Bottom
        widget.NewButton("Submit", func() {
            fmt.Printf("Name: %s, Count: %d, Text: %s\n", 
                nameEntry.Text, counter, multiline.Text)
        }),
        nil, // left
        nil, // right
        // Center - editor fills remaining space
        container.NewScroll(multiline),
    )
    
    myWindow.SetContent(content)
    myWindow.Resize(fyne.NewSize(600, 400))
    myWindow.ShowAndRun()
}
```

---

## Best Practices

1. **Layout Selection**: Use Border layout for complex UIs, VBox/HBox for simple ones
2. **Performance**: Avoid creating too many widgets; reuse when possible
3. **Responsiveness**: Let layouts handle sizing; avoid fixed pixel sizes
4. **Theming**: Use theme colors instead of hardcoded colors
5. **Data Binding**: Use for automatic UI updates
6. **Error Handling**: Always handle errors in callbacks
7. **Testing**: Write tests using the test package
8. **Concurrency**: Update UI only from main goroutine; use `goroutine` with care

---

## Resources

- Official Documentation: https://developer.fyne.io/
- GitHub: https://github.com/fyne-io/fyne
- Examples: https://github.com/fyne-io/examples
- Community: https://github.com/fyne-io/fyne/discussions
