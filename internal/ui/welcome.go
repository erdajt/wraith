package ui

import (
	"math"
)

func wraithOrbit(width, height, frame int) []string {
	width, height = max(1, width), max(1, height)
	cells := make([]byte, width*height)
	depth := make([]float64, len(cells))
	for i := range cells {
		cells[i] = ' '
	}
	angle := float64(frame) * 0.07
	shades := ".,:;irsXA253hMHGS#9B&@"
	for v := 0.0; v < 2*math.Pi; v += 0.09 {
		for u := 0.0; u < 2*math.Pi; u += 0.04 {
			ring := 1.5 + 0.55*math.Cos(v)
			x, y, z := ring*math.Cos(u), ring*math.Sin(u), 0.55*math.Sin(v)
			y, z = y*math.Cos(angle)-z*math.Sin(angle), y*math.Sin(angle)+z*math.Cos(angle)
			x, z = x*math.Cos(angle*0.7)-z*math.Sin(angle*0.7), x*math.Sin(angle*0.7)+z*math.Cos(angle*0.7)
			distance := 1 / (z + 5)
			column := width/2 + int(x*distance*float64(width)*0.85)
			row := height/2 + int(y*distance*float64(height)*0.85)
			if column < 0 || column >= width || row < 0 || row >= height {
				continue
			}
			index := row*width + column
			if distance <= depth[index] {
				continue
			}
			depth[index] = distance
			light := (math.Cos(v)*math.Cos(u-angle) + math.Sin(v-angle) + 2) / 4
			cells[index] = shades[min(len(shades)-1, max(0, int(light*float64(len(shades)))))]
		}
	}
	rows := make([]string, height)
	for y := range height {
		rows[y] = string(cells[y*width : (y+1)*width])
	}
	return rows
}

func (a *App) welcome(width, height int, p palette) []string {
	frame := a.frame
	if !a.Config.Animations {
		frame = 9
	}
	logo := []string{"W R A I T H"}
	if width >= 48 && height >= 18 {
		logo = []string{
			"#   #  ####    ###   #####  #####  #   #",
			"#   #  #   #  #   #    #      #    #   #",
			"# # #  ####   #####    #      #    #####",
			"## ##  #  #   #   #    #      #    #   #",
			"#   #  #   #  #   #  #####    #    #   #",
		}
	}
	var rows []string
	if height >= 13 {
		for i, line := range wraithOrbit(min(width-4, 36), min(8, height-len(logo)-6), frame) {
			shade := p.Accent
			if i%3 == 0 {
				shade = p.Second
			}
			rows = append(rows, ink(shade, line))
		}
	}
	for _, line := range logo {
		rows = append(rows, bold(p.Accent, line))
	}
	rows = append(rows, ink(p.Muted, "local intelligence · deliberate control"), "", ink(p.FG, "write a prompt, or bring your own evidence."), ink(p.Muted, a.Config.Keys["send"]+" send     "+a.Config.Keys["editor"]+" editor     /models switch"), ink(p.Muted, a.Config.Keys["terminal_overlay"]+" scratch shell     "+a.Config.Keys["terminal_split"]+" side shell"), ink(p.Second, "/remember preferences     /pin exact facts"))
	return rows
}
