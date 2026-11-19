package plugin

import "github.com/erdajt/wraith/internal/keymap"

// SurfacePosition uses zero-based line and rune columns.
type SurfacePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// SurfaceStyle selects theme colors or explicit rgb colors without terminal escape sequences.
type SurfaceStyle struct {
	Foreground string `json:"foreground,omitempty"`
	Background string `json:"background,omitempty"`
	Bold       bool   `json:"bold,omitempty"`
	Italic     bool   `json:"italic,omitempty"`
	Underline  bool   `json:"underline,omitempty"`
}

// SurfaceSpan styles a half-open range of rune positions.
type SurfaceSpan struct {
	Start SurfacePosition `json:"start"`
	End   SurfacePosition `json:"end"`
	Style SurfaceStyle    `json:"style"`
}

// SurfaceBuffer contains detached canonical text shared by independent views.
type SurfaceBuffer struct {
	Projection string           `json:"projection,omitempty"`
	Truncated  bool             `json:"truncated,omitempty"`
	ID         string           `json:"id"`
	Owner      string           `json:"owner"`
	Name       string           `json:"name"`
	Revision   uint64           `json:"revision"`
	Editable   bool             `json:"editable"`
	Lines      []string         `json:"lines"`
	Spans      []SurfaceSpan    `json:"spans,omitempty"`
	Keymaps    []keymap.Binding `json:"keymaps,omitempty"`
}

// SurfaceWindow describes an owned view; cursor coordinates remain independent of other views.
type SurfaceWindow struct {
	ID         string          `json:"id"`
	Owner      string          `json:"owner"`
	BufferID   string          `json:"buffer_id"`
	Revision   uint64          `json:"revision"`
	Kind       string          `json:"kind"`
	Position   string          `json:"position,omitempty"`
	RelativeTo string          `json:"relative_to,omitempty"`
	Title      string          `json:"title,omitempty"`
	Width      int             `json:"width,omitempty"`
	Height     int             `json:"height,omitempty"`
	Row        int             `json:"row,omitempty"`
	Column     int             `json:"column,omitempty"`
	Focus      bool            `json:"focus,omitempty"`
	Cursor     SurfacePosition `json:"cursor"`
}

// SurfaceRegion places a bounded text frame beneath interactive dialogs.
type SurfaceRegion struct {
	ID       string        `json:"id"`
	Owner    string        `json:"owner"`
	Slot     string        `json:"slot"`
	Revision uint64        `json:"revision"`
	Order    uint64        `json:"order"`
	Width    int           `json:"width,omitempty"`
	Height   int           `json:"height,omitempty"`
	Row      int           `json:"row,omitempty"`
	Column   int           `json:"column,omitempty"`
	Lines    []string      `json:"lines"`
	Spans    []SurfaceSpan `json:"spans,omitempty"`
}

// SurfaceMutation carries an ordered edit and the caller lease that created it.
type SurfaceMutation struct {
	Operation string          `json:"operation"`
	ID        string          `json:"id"`
	Owner     string          `json:"owner"`
	Lease     string          `json:"lease"`
	Expected  uint64          `json:"expected"`
	Buffer    *SurfaceBuffer  `json:"buffer,omitempty"`
	Window    *SurfaceWindow  `json:"window,omitempty"`
	Region    *SurfaceRegion  `json:"region,omitempty"`
	Start     int             `json:"start,omitempty"`
	End       int             `json:"end,omitempty"`
	Lines     []string        `json:"lines,omitempty"`
	Spans     []SurfaceSpan   `json:"spans,omitempty"`
	Binding   *keymap.Binding `json:"binding,omitempty"`
}

// SurfaceDelta reports committed changes for the UI cache without executing renderer callbacks.
type SurfaceDelta struct {
	ChangedBuffers []SurfaceBuffer
	ChangedWindows []SurfaceWindow
	Operation      string
	Buffer         *SurfaceBuffer
	Window         *SurfaceWindow
	Region         *SurfaceRegion
	RemovedBuffers []string
	RemovedWindows []string
	RemovedRegions []string
}

// SurfaceBufferInfo lists text metadata without copying the buffer contents.
type SurfaceBufferInfo struct {
	Projection string `json:"projection,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	Revision   uint64 `json:"revision"`
	Editable   bool   `json:"editable"`
	LineCount  int    `json:"line_count"`
	ByteCount  int    `json:"byte_count"`
}

// SurfaceIndex lists bounded live resources without serializing full buffer contents.
type SurfaceIndex struct {
	Buffers []SurfaceBufferInfo `json:"buffers"`
	Windows []SurfaceWindow     `json:"windows"`
	Regions []SurfaceRegionInfo `json:"regions"`
}

// SurfaceBufferPage carries a detached bounded line window and its observed revision.
type SurfaceBufferPage struct {
	SurfaceBufferInfo
	Start int      `json:"start"`
	Lines []string `json:"lines"`
	Next  int      `json:"next"`
}

// SurfaceRegionInfo lists frame dimensions without copying text or style spans.
type SurfaceRegionInfo struct {
	ID        string `json:"id"`
	Owner     string `json:"owner"`
	Slot      string `json:"slot"`
	Revision  uint64 `json:"revision"`
	Order     uint64 `json:"order"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Row       int    `json:"row"`
	Column    int    `json:"column"`
	LineCount int    `json:"line_count"`
}
