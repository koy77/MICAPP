import gi
import sys
import os
import signal
import argparse

gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk, GLib, Pango

# Global flag to track if parent process is alive
_keep_running = True
_timeout_seconds = 120  # Auto-exit after 2 minutes as safety net

def create_overlay(dot_color="red", diameter_px=150, opacity=1.0):
    global _keep_running

    window = Gtk.Window(type=Gtk.WindowType.POPUP)
    window.set_title("Recording Dot")
    window.set_keep_above(True)
    window.set_accept_focus(False)
    window.set_decorated(False)

    # Enable transparency
    screen = window.get_screen()
    visual = screen.get_rgba_visual()
    if visual and screen.is_composited():
        window.set_visual(visual)

    window.set_app_paintable(True)

    def on_draw(widget, cr):
        # Background transparency (fully transparent)
        cr.set_source_rgba(0, 0, 0, 0)
        cr.set_operator(0)  # CAIRO_OPERATOR_CLEAR
        cr.paint()
        cr.set_operator(2)  # CAIRO_OPERATOR_OVER

        # Draw the dot
        if dot_color == "red":
            cr.set_source_rgb(1, 0, 0)
        elif dot_color == "green":
            cr.set_source_rgb(0, 1, 0)
        else:
            cr.set_source_rgb(1, 0, 0)

        radius = diameter_px / 2
        center_x = (diameter_px + 10) / 2
        center_y = (diameter_px + 10) / 2

        cr.arc(center_x, center_y, radius, 0, 2 * 3.14159)
        cr.fill_preserve()

        # White outline
        cr.set_source_rgb(1, 1, 1)
        cr.set_line_width(2)
        cr.stroke()
        return False

    window.connect("draw", on_draw)

    # Handle window delete event (close button) gracefully
    window.connect("delete-event", lambda w, e: Gtk.main_quit())

    # Calculate position: bottom center
    display = Gdk.Display.get_default()
    monitor = display.get_primary_monitor()
    geometry = monitor.get_geometry()

    screen_width = geometry.width
    screen_height = geometry.height

    win_size = diameter_px + 10
    x = (screen_width - win_size) // 2
    y = screen_height - win_size - 10  # 10px from bottom

    window.move(x, y)
    window.resize(win_size, win_size)

    window.show_all()

    # Periodic lift to ensure it stays on top
    def ensure_top():
        if not _keep_running:
            Gtk.main_quit()
            return False
        window.set_keep_above(True)
        window.present()
        return True

    GLib.timeout_add(1000, ensure_top)

    # Safety timeout: auto-quit after _timeout_seconds to prevent orphan processes
    def auto_quit():
        Gtk.main_quit()
        return False
    GLib.timeout_add(_timeout_seconds * 1000, auto_quit)

    Gtk.main()


def handle_signal(signum, frame):
    """Handle SIGTERM/SIGINT — gracefully quit GTK main loop."""
    global _keep_running
    _keep_running = False
    Gtk.main_quit()


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    parser = argparse.ArgumentParser(description="Red dot overlay utility (GTK3)")
    parser.add_argument("--color", default="red", help="Color of the dot")
    parser.add_argument("--size", type=int, default=150, help="Diameter of the dot in pixels")
    parser.add_argument("--timeout", type=int, default=120,
                        help="Auto-exit timeout in seconds (default: 120)")

    args = parser.parse_args()
    _timeout_seconds = args.timeout

    create_overlay(dot_color=args.color, diameter_px=args.size)
