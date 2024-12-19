// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func DefaultTable(title string, header table.Row) table.Writer {
	t := table.NewWriter()
	t.Style().Title.Align = text.AlignCenter
	t.Style().Title.Format = text.FormatUpper
	t.Style().Options.SeparateRows = true
	t.SetTitle(title)
	if header != nil {
		t.AppendHeader(header)
	}
	return t
}
