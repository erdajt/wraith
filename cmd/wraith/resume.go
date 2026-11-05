package main

import "strings"

func resumeArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			out = append(out, args[i:]...)
			break
		}
		name := strings.TrimLeft(arg, "-")
		if !strings.Contains(arg, "=") && strings.Contains("|session|provider|model|theme|config-dir|data-dir|cache-dir|width|height|", "|"+name+"|") && i+1 < len(args) {
			out = append(out, arg, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--resume=") {
			out = append(out, "--resume", "--session", strings.TrimPrefix(arg, "--resume="))
			continue
		}
		out = append(out, arg)
		if arg == "--resume" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			out = append(out, "--session", args[i])
		}
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
	}
	return out
}
