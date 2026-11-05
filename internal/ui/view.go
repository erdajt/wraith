package ui

import ()

func (a *App) View() string { return "wraith\n\n" + a.Prompt.String() + "\n\n" + a.notice }
