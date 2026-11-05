package ui

func (a *App) bodyHeight() int { return max(5, a.height-12) }
func (a *App) mainWidth() int {
	if p := a.panes[1]; p != nil && p.Visible {
		return max(30, a.width-int(float64(a.width)*a.Config.Terminal.SplitWidth)-1)
	}
	return a.width
}

func (a *App) paneSize(index int) (int, int) {
	if index == 0 {
		return max(20, int(float64(a.width)*a.Config.Terminal.OverlayWidth)), max(6, int(float64(a.height)*a.Config.Terminal.OverlayHeight))
	}
	return max(20, min(a.width-31, int(float64(a.width)*a.Config.Terminal.SplitWidth))), max(6, a.height-3)
}
