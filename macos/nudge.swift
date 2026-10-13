// Moves the pointer 1px right and back. A real position change resets the HID idle
// timer that Teams (Electron powerMonitor) reads; zero-distance moves and key-only
// events (F15) do not. Needs Accessibility permission for the terminal app.
import CoreGraphics
import Foundation
let p = CGEvent(source: nil)?.location ?? .zero
for q in [CGPoint(x: p.x + 1, y: p.y), p] {
    CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: q, mouseButton: .left)?.post(tap: .cghidEventTap)
    usleep(50_000)
}
