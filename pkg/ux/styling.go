// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// PrintPrettyCode prints the code according to the syntax highlighting for the
// given language to the user. If stdout isn't a terminal styling is omitted.
func PrintPrettyCode(lang, code string) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println(code)
		return nil
	}

	out, err := prettifyCode(lang, code)
	if err != nil {
		return err
	}

	fmt.Println(out)
	return nil
}

func prettifyCode(lang, code string) (string, error) {
	var style gansi.StyleConfig
	if termenv.HasDarkBackground() {
		style = glamour.DarkStyleConfig
	} else {
		style = glamour.LightStyleConfig
	}
	zero := uint(0)
	style.CodeBlock.Margin = &zero

	formatter := &gansi.CodeBlockElement{
		Code:     code,
		Language: lang,
	}

	rctx := gansi.NewRenderContext(gansi.Options{
		Styles:       style,
		ColorProfile: termenv.ColorProfile(),
	})

	r := strings.Builder{}
	if err := formatter.Render(&r, rctx); err != nil {
		return "", err
	}

	return r.String(), nil
}
