package ui

import "github.com/gdamore/tcell/v3"

// SnippetStop is an absolute range in the buffer for one snippet placeholder.
type SnippetStop struct {
	StartLine, StartCol int
	EndLine, EndCol     int
}

// SnippetSession tracks the placeholders of an accepted completion. Tab moves
// forward, Shift+Tab moves back, and edits inside a placeholder move the later
// stops with it.
type SnippetSession struct {
	stops   []SnippetStop
	current int
}

func NewSnippetSession(stops []SnippetStop) *SnippetSession {
	if len(stops) == 0 {
		return nil
	}
	return &SnippetSession{stops: stops}
}

func (s *SnippetSession) Current() SnippetStop {
	return s.stops[s.current]
}

func (s *SnippetSession) HasNext() bool {
	return s.current+1 < len(s.stops)
}

func (s *SnippetSession) Next() {
	if s.HasNext() {
		s.current++
	}
}

func (s *SnippetSession) HasPrev() bool {
	return s.current > 0
}

func (s *SnippetSession) Prev() {
	if s.HasPrev() {
		s.current--
	}
}

// ShiftLines moves every stop at or after line by delta lines.
func (s *SnippetSession) ShiftLines(line, delta int) {
	if delta == 0 {
		return
	}
	for i := range s.stops {
		stop := &s.stops[i]
		if stop.StartLine >= line {
			stop.StartLine += delta
		}
		if stop.EndLine >= line {
			stop.EndLine += delta
		}
	}
}

// ShiftForEdit moves stops after an edit on line at col by delta runes. The
// current stop keeps its start so going back still selects the edited text.
func (s *SnippetSession) ShiftForEdit(line, col, delta int) {
	if delta == 0 {
		return
	}
	for i := range s.stops {
		stop := &s.stops[i]
		if i == s.current {
			switch {
			case stop.StartLine == line && stop.StartCol > col:
				stop.StartCol += delta
				if stop.EndLine == line {
					stop.EndCol += delta
				}
			case stop.EndLine == line && stop.EndCol >= col:
				stop.EndCol += delta
				if stop.EndLine == stop.StartLine && stop.EndCol < stop.StartCol {
					stop.EndCol = stop.StartCol
				}
			}
			continue
		}
		switch {
		case stop.StartLine == line && stop.StartCol >= col:
			stop.StartCol += delta
			if stop.EndLine == line {
				stop.EndCol += delta
			}
		case stop.EndLine == line && stop.EndCol >= col:
			stop.EndCol += delta
		}
		if stop.StartCol < 0 {
			stop.StartCol = 0
		}
		if stop.EndLine == stop.StartLine && stop.EndCol < stop.StartCol {
			stop.EndCol = stop.StartCol
		}
	}
}

type snippetEditSnapshot struct {
	session   *SnippetSession
	lineLen   int
	lineCount int
	editLine  int
	editCol   int
	multiLine bool
}

func (e *EditorPaneWidget) beginSnippetEdit(key tcell.Key) *snippetEditSnapshot {
	if e.Snippet == nil {
		return nil
	}
	if e.isMultiActive() {
		e.Snippet = nil
		return nil
	}
	line := e.Cursor.Line
	if line < 0 || line >= len(e.Buf.Lines) {
		return nil
	}
	snap := &snippetEditSnapshot{
		session:   e.Snippet,
		lineLen:   len([]rune(e.Buf.Lines[line])),
		lineCount: len(e.Buf.Lines),
		editLine:  line,
		editCol:   e.Cursor.Col,
	}
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		if start.Line != end.Line {
			snap.multiLine = true
			return snap
		}
		snap.lineLen = len([]rune(e.Buf.Lines[start.Line]))
		snap.editLine = start.Line
		snap.editCol = start.Col
		return snap
	}
	switch key {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.Cursor.Col == 0 {
			snap.multiLine = true
			return snap
		}
		snap.editCol = e.Cursor.Col - 1
	case tcell.KeyDelete:
		if e.Cursor.Col >= snap.lineLen {
			snap.multiLine = true
			return snap
		}
	}
	return snap
}

func (e *EditorPaneWidget) endSnippetEdit(snap *snippetEditSnapshot) {
	if snap == nil || snap.session == nil || e.Snippet != snap.session {
		return
	}
	if snap.multiLine || len(e.Buf.Lines) != snap.lineCount || e.Cursor.Line != snap.editLine {
		e.Snippet = nil
		return
	}
	line := snap.editLine
	if line < 0 || line >= len(e.Buf.Lines) {
		e.Snippet = nil
		return
	}
	delta := len([]rune(e.Buf.Lines[line])) - snap.lineLen
	if delta == 0 {
		return
	}
	e.Snippet.ShiftForEdit(line, snap.editCol, delta)
}

func (e *EditorPaneWidget) snippetNext() bool {
	if e.Snippet == nil {
		return false
	}
	if e.isMultiActive() {
		e.Snippet = nil
		return false
	}
	stop := e.Snippet.Current()
	if e.Cursor.Line != stop.EndLine {
		e.Snippet = nil
		return false
	}
	if !e.Snippet.HasNext() {
		if e.Selection != nil {
			e.Selection.Clear()
		}
		e.Snippet = nil
		return true
	}
	e.Snippet.Next()
	e.snippetSelect(e.Snippet.Current())
	return true
}

func (e *EditorPaneWidget) snippetPrev() bool {
	if e.Snippet == nil {
		return false
	}
	if e.isMultiActive() {
		e.Snippet = nil
		return false
	}
	if e.Snippet.HasPrev() {
		e.Snippet.Prev()
	}
	e.snippetSelect(e.Snippet.Current())
	return true
}

func (e *EditorPaneWidget) snippetSelect(stop SnippetStop) {
	if e.Selection == nil {
		return
	}
	if stop.StartLine == stop.EndLine && stop.StartCol == stop.EndCol {
		e.Selection.Clear()
		e.Cursor.Line = stop.StartLine
		e.Cursor.Col = stop.StartCol
		return
	}
	e.Selection.Start(stop.StartLine, stop.StartCol)
	e.Cursor.Line = stop.EndLine
	e.Cursor.Col = stop.EndCol
}

func (e *EditorPaneWidget) snippetCancel() {
	if e.Selection != nil {
		e.Selection.Clear()
	}
	e.Snippet = nil
}
